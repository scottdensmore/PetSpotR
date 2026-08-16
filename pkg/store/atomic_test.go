package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
)

type pendingRecord struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func TestMemoryStoreCreateStateAndOutboxIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	state := store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-101", Data: []byte(`{"petId":"lost-101"}`)}
	outbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-101", Data: []byte(`{"id":"evt-101"}`)}

	created, err := stateStore.CreateStateAndOutbox(ctx, state, outbox)
	if err != nil || !created {
		t.Fatalf("CreateStateAndOutbox() = %t, %v; want true, nil", created, err)
	}

	created, err = stateStore.CreateStateAndOutbox(ctx, state, outbox)
	if err != nil || created {
		t.Fatalf("idempotent CreateStateAndOutbox() = %t, %v; want false, nil", created, err)
	}

	competingOutbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-102", Data: []byte(`{"id":"evt-102"}`)}
	created, err = stateStore.CreateStateAndOutbox(ctx, state, competingOutbox)
	if !errors.Is(err, store.ErrConflict) || created {
		t.Fatalf("competing CreateStateAndOutbox() = %t, %v; want false, ErrConflict", created, err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	created, err = stateStore.CreateStateAndOutbox(canceled,
		store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-102", Data: []byte("state")},
		store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-103", Data: []byte("event")},
	)
	if !errors.Is(err, context.Canceled) || created {
		t.Fatalf("canceled CreateStateAndOutbox() = %t, %v; want false, context.Canceled", created, err)
	}
	if _, err := stateStore.GetState(ctx, store.LostPetsCollection, "lost-102"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("state from canceled transaction error = %v, want ErrNotFound", err)
	}
	if _, err := stateStore.GetState(ctx, store.OutboxCollection, "evt-103"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("outbox from canceled transaction error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreCreateStatesAndOutboxIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	report := store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-private-101", Data: []byte(`{"petId":"lost-private-101"}`)}
	contact := store.StateWrite{StoreName: store.ReportContactsCollection, Key: "lost/lost-private-101/owner", Data: []byte(`{"email":"owner@example.com"}`)}
	outbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-private-101", Data: []byte(`{"id":"evt-private-101"}`)}

	created, err := stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{report, contact}, outbox)
	if err != nil || !created {
		t.Fatalf("CreateStatesAndOutbox() = %t, %v; want true, nil", created, err)
	}

	created, err = stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{report, contact}, outbox)
	if err != nil || created {
		t.Fatalf("idempotent CreateStatesAndOutbox() = %t, %v; want false, nil", created, err)
	}

	changedContact := contact
	changedContact.Data = []byte(`{"email":"other@example.com"}`)
	created, err = stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{report, changedContact}, outbox)
	if !errors.Is(err, store.ErrConflict) || created {
		t.Fatalf("competing CreateStatesAndOutbox() = %t, %v; want false, ErrConflict", created, err)
	}
	storedContact, err := stateStore.GetState(ctx, contact.StoreName, contact.Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(storedContact) != string(contact.Data) {
		t.Fatalf("competing create replaced contact = %s, want %s", storedContact, contact.Data)
	}

	partialReport := store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-partial-101", Data: []byte(`{"petId":"lost-partial-101"}`)}
	partialContact := store.StateWrite{StoreName: store.ReportContactsCollection, Key: "lost/lost-partial-101/owner", Data: []byte(`{"email":"owner@example.com"}`)}
	partialOutbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-partial-101", Data: []byte(`{"id":"evt-partial-101"}`)}
	if err := stateStore.SaveState(ctx, partialReport.StoreName, partialReport.Key, partialReport.Data); err != nil {
		t.Fatal(err)
	}
	created, err = stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{partialReport, partialContact}, partialOutbox)
	if !errors.Is(err, store.ErrConflict) || created {
		t.Fatalf("partial CreateStatesAndOutbox() = %t, %v; want false, ErrConflict", created, err)
	}
	if _, err := stateStore.GetState(ctx, partialContact.StoreName, partialContact.Key); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("partial transaction contact error = %v, want ErrNotFound", err)
	}
	if _, err := stateStore.GetState(ctx, partialOutbox.StoreName, partialOutbox.Key); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("partial transaction outbox error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreListsBoundedPendingOutboxByTopic(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	now := time.Now().UTC()
	for _, record := range []pendingRecord{
		{ID: "found-later", Topic: "foundPet", Status: "pending", CreatedAt: now.Add(time.Minute)},
		{ID: "found-first", Topic: "foundPet", Status: "pending", CreatedAt: now},
		{ID: "lost-first", Topic: "lostPet", Status: "pending", CreatedAt: now.Add(-time.Minute)},
		{ID: "found-published", Topic: "foundPet", Status: "published", CreatedAt: now.Add(-time.Hour)},
	} {
		data, _ := json.Marshal(record)
		if err := stateStore.SaveState(ctx, store.OutboxCollection, record.ID, data); err != nil {
			t.Fatal(err)
		}
	}

	ids, err := stateStore.ListPendingOutbox(ctx, "foundPet", 1)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "found-first" {
		t.Fatalf("ListPendingOutbox() = %#v, want [found-first]", ids)
	}
	if _, err := stateStore.ListPendingOutbox(ctx, "foundPet", 0); err == nil {
		t.Fatal("ListPendingOutbox(limit=0) error = nil, want non-nil")
	}
}
