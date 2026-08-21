package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreFinderFoundLifecycleIsAtomicAcrossRuntimes(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-finder-lifecycle-%d", time.Now().UnixNano())
	writer, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	broker := pubsub.NewMemoryPubSub()
	if err := broker.Subscribe("petStatusChanged", func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	writerService := foundpet.NewReportService(writer, broker)
	readerService := foundpet.NewReportService(reader, broker)
	now := time.Now().UTC()
	petID := fmt.Sprintf("found-finder-lifecycle-%d", now.UnixNano())
	finder := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "firestore-finder",
	}
	reported, err := writerService.ReportFoundPet(ctx, foundpet.ReportCommand{
		PetID: petID, ImageURL: "https://storage.petspotr.io/found.jpg", FoundAt: now.Add(-time.Hour),
		Location: "Seattle, WA", FinderEmail: "finder@example.com", OwnedBy: &finder,
	}, foundpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	var lifecycleEventID string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if data, getErr := writer.GetState(cleanupCtx, store.FoundPetsCollection, petID); getErr == nil {
			var record domain.FoundPetRecord
			if json.Unmarshal(data, &record) == nil && record.FinderIdentityRef != "" {
				_ = writer.DeleteState(cleanupCtx, store.ReportContactsCollection, record.FinderIdentityRef)
			}
		}
		_ = writer.DeleteState(cleanupCtx, store.FoundPetsCollection, petID)
		_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, reported.EventID)
		if lifecycleEventID != "" {
			_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, lifecycleEventID)
		}
	})

	result, err := readerService.ResolveFoundPet(ctx, foundpet.LifecycleCommand{
		PetID: petID, Status: domain.FoundPetStatusResolved,
		OperationID: "resolve-firestore-finder", Actor: finder,
	})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleEventID = result.EventID
	retry, err := writerService.ResolveFoundPet(ctx, foundpet.LifecycleCommand{
		PetID: petID, Status: domain.FoundPetStatusResolved,
		OperationID: "resolve-firestore-finder", Actor: finder,
	})
	if err != nil || retry.EventID != result.EventID {
		t.Fatalf("cross-runtime exact retry = %#v, %v", retry, err)
	}
	if _, err := writerService.ResolveFoundPet(ctx, foundpet.LifecycleCommand{
		PetID: petID, Status: domain.FoundPetStatusResolved,
		OperationID: "different-operation", Actor: finder,
	}); !errors.Is(err, domain.ErrFoundPetLifecycleConflict) {
		t.Fatalf("competing terminal operation error = %v", err)
	}
	recordData, err := writer.GetState(ctx, store.FoundPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	var record domain.FoundPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil ||
		record.Status != domain.FoundPetStatusResolved || record.LifecycleAudit == nil ||
		record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("durable lifecycle record = %#v, %v", record, err)
	}
	durableEvent, err := outbox.GetRecord(ctx, reader, result.EventID)
	if err != nil || durableEvent.Topic != "petStatusChanged" {
		t.Fatalf("durable lifecycle outbox = %#v, %v", durableEvent, err)
	}
	event, envelope, err := domain.DecodePetStatusChanged(durableEvent.Payload)
	if err != nil || envelope.PayloadVersion != domain.PetStatusChangedFoundPayloadVersion ||
		event.Status != "resolved" {
		t.Fatalf("durable lifecycle event = %#v / %#v, %v", envelope, event, err)
	}
}
