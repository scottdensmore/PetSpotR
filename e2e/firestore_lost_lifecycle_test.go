package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreOwnerLostLifecycleIsAtomicAcrossRuntimes(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-owner-lifecycle-%d", time.Now().UnixNano())
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
	writerService := lostpet.NewService(writer, broker)
	readerService := lostpet.NewService(reader, broker)
	now := time.Now().UTC()
	petID := fmt.Sprintf("lost-owner-lifecycle-%d", now.UnixNano())
	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "firestore-owner"}
	reported, err := writerService.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: petID, ReporterEmail: "owner@example.com", ReportedAt: now.Add(-time.Hour),
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: &domain.LocationPoint{Latitude: 47.61, Longitude: -122.33}, OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	var lifecycleEventID string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if data, getErr := writer.GetState(cleanupCtx, store.LostPetsCollection, petID); getErr == nil {
			var record domain.LostPetRecord
			if json.Unmarshal(data, &record) == nil && record.OwnerIdentityRef != "" {
				_ = writer.DeleteState(cleanupCtx, store.ReportContactsCollection, record.OwnerIdentityRef)
			}
		}
		_ = writer.DeleteState(cleanupCtx, store.LostPetsCollection, petID)
		_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, reported.EventID)
		if lifecycleEventID != "" {
			_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, lifecycleEventID)
		}
	})

	candidates := store.LostPetCandidateStore(reader)
	query := store.LostPetCandidateQuery{
		Status: string(domain.LostPetStatusLost), GeocodingStatus: string(domain.GeocodingVerified),
		ReportedAfter: now.Add(-24 * time.Hour), ReportedBefore: now.Add(time.Hour),
		MinLatitude: 47, MaxLatitude: 48, MinLongitude: -123, MaxLongitude: -122,
	}
	before, err := candidates.QueryLostPetCandidates(ctx, query)
	if err != nil || before[petID] == nil {
		t.Fatalf("candidate before lifecycle = %#v, %v", before, err)
	}

	result, err := readerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "reunite-firestore-owner", Actor: owner,
	})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleEventID = result.EventID
	retry, err := writerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "reunite-firestore-owner", Actor: owner,
	})
	if err != nil || retry.EventID != result.EventID {
		t.Fatalf("cross-runtime exact retry = %#v, %v", retry, err)
	}
	if _, err := writerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "different-operation", Actor: owner,
	}); !errors.Is(err, domain.ErrLostPetLifecycleConflict) {
		t.Fatalf("competing terminal operation error = %v", err)
	}
	after, err := candidates.QueryLostPetCandidates(ctx, query)
	if err != nil || after[petID] != nil {
		t.Fatalf("candidate after lifecycle = %#v, %v", after, err)
	}
	recordData, err := writer.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	var record domain.LostPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil || record.Status != domain.LostPetStatusReunited ||
		record.LifecycleAudit == nil || record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("durable lifecycle record = %#v, %v", record, err)
	}
	if durableEvent, err := outbox.GetRecord(ctx, reader, result.EventID); err != nil || durableEvent.Topic != "petStatusChanged" {
		t.Fatalf("durable lifecycle outbox = %#v, %v", durableEvent, err)
	}
}
