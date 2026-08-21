package foundpet

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestResolveFoundPetPersistsAtomicLifecycleAndPublishesOnce(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	published := make(chan []byte, 2)
	if err := broker.Subscribe("petStatusChanged", func(_ context.Context, data []byte) error {
		published <- append([]byte(nil), data...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	service := NewReportService(state, broker)
	finder := domain.PrincipalRef{Issuer: "issuer", Subject: "finder-service"}
	reported, err := service.ReportFoundPet(ctx, ReportCommand{
		PetID: "found-service-resolution", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), Location: "Seattle, WA",
		FinderEmail: "finder@example.com", OwnedBy: &finder,
	}, ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ResolveFoundPet(ctx, LifecycleCommand{
		PetID: "found-service-resolution", Status: domain.FoundPetStatusResolved,
		OperationID: "resolve-service-found", Actor: finder,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PetID != "found-service-resolution" || result.Status != domain.FoundPetStatusResolved || result.EventID == "" {
		t.Fatalf("lifecycle result = %#v", result)
	}
	data, err := state.GetState(ctx, store.FoundPetsCollection, result.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var record domain.FoundPetRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Status != domain.FoundPetStatusResolved ||
		record.LifecycleAudit == nil || record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("persisted lifecycle record = %#v, %v", record, err)
	}
	durable, err := outbox.GetRecord(ctx, state, result.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if durable.Topic != "petStatusChanged" || durable.Status != outbox.StatusPublished {
		t.Fatalf("durable lifecycle outbox = %#v", durable)
	}
	var envelope domain.EventEnvelope
	if err := json.Unmarshal(durable.Payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.PayloadVersion != domain.PetStatusChangedFoundPayloadVersion {
		t.Fatalf("payload version = %d", envelope.PayloadVersion)
	}
	if decoded, _, err := domain.DecodePetStatusChanged(durable.Payload); err != nil ||
		decoded.ReportType != "found" || decoded.Status != "resolved" {
		t.Fatalf("decoded lifecycle payload = %#v, %v", decoded, err)
	}
	for _, private := range []string{finder.Issuer, finder.Subject, "finder@example.com"} {
		if strings.Contains(string(durable.Payload), private) {
			t.Fatalf("outbox exposed private identity %q: %s", private, durable.Payload)
		}
	}
	select {
	case got := <-published:
		if string(got) != string(durable.Payload) {
			t.Fatalf("published payload = %s, want %s", got, durable.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("petStatusChanged event was not published")
	}

	retry, err := service.ResolveFoundPet(ctx, LifecycleCommand{
		PetID: result.PetID, Status: domain.FoundPetStatusResolved,
		OperationID: "resolve-service-found", Actor: finder,
	})
	if err != nil || retry.EventID != result.EventID {
		t.Fatalf("exact retry = %#v, %v", retry, err)
	}
	select {
	case duplicate := <-published:
		t.Fatalf("exact retry republished event: %s", duplicate)
	default:
	}
	if _, err := service.ResolveFoundPet(ctx, LifecycleCommand{
		PetID: result.PetID, Status: domain.FoundPetStatusResolved,
		OperationID: "different-operation", Actor: finder,
	}); !errors.Is(err, domain.ErrFoundPetLifecycleConflict) {
		t.Fatalf("competing operation error = %v", err)
	}

	if err := state.DeleteState(ctx, store.OutboxCollection, reported.EventID); err != nil {
		t.Fatal(err)
	}
}

func TestResolveFoundPetHidesMissingWrongOwnerAndCorruptState(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	service := NewReportService(state, pubsub.NewMemoryPubSub())
	finder := domain.PrincipalRef{Issuer: "issuer", Subject: "finder-hidden"}
	_, err := service.ReportFoundPet(ctx, ReportCommand{
		PetID: "found-hidden", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), Location: "Seattle, WA", OwnedBy: &finder,
	}, ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range []LifecycleCommand{
		{PetID: "missing", Status: domain.FoundPetStatusResolved, OperationID: "missing", Actor: finder},
		{PetID: "found-hidden", Status: domain.FoundPetStatusResolved, OperationID: "wrong-owner", Actor: domain.PrincipalRef{Issuer: "issuer", Subject: "stranger"}},
	} {
		if _, err := service.ResolveFoundPet(ctx, command); !errors.Is(err, ErrLifecycleHidden) {
			t.Fatalf("hidden command %#v error = %v", command, err)
		}
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, "found-corrupt", []byte(`{"petId":"found-corrupt"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveFoundPet(ctx, LifecycleCommand{
		PetID: "found-corrupt", Status: domain.FoundPetStatusResolved,
		OperationID: "corrupt", Actor: finder,
	}); !errors.Is(err, ErrLifecycleHidden) {
		t.Fatalf("corrupt state error = %v", err)
	}
}
