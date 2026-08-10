package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

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
