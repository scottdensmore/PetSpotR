package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreDeliveryOperationClaimAndFencing(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	stateStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-delivery-operation", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	operation, err := delivery.NewOperation(
		fmt.Sprintf("evt-delivery-%d", now.UnixNano()),
		"lost-firestore-101",
		"email",
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stateStore.DeleteState(context.Background(), store.NotificationDeliveriesCollection, operation.ID)
	})

	const callers = 8
	start := make(chan struct{})
	results := make(chan delivery.Claim, callers)
	errorsSeen := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			claim, claimErr := stateStore.ClaimDeliveryOperation(ctx, operation, now, time.Minute)
			results <- claim
			errorsSeen <- claimErr
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errorsSeen)

	for claimErr := range errorsSeen {
		if claimErr != nil {
			t.Fatalf("concurrent claim error = %v", claimErr)
		}
	}
	acquired := 0
	for claim := range results {
		if claim.State == delivery.ClaimAcquired {
			acquired++
		} else if claim.State != delivery.ClaimInProgress {
			t.Fatalf("unexpected claim = %#v", claim)
		}
	}
	if acquired != 1 {
		t.Fatalf("acquired claims = %d, want 1", acquired)
	}

	reclaimed, err := stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(time.Minute), time.Minute)
	if err != nil || reclaimed.State != delivery.ClaimAcquired || reclaimed.Attempt != 2 {
		t.Fatalf("reclaimed claim = %#v, %v; want acquired attempt 2", reclaimed, err)
	}
	if err := stateStore.CompleteDeliveryOperation(ctx, operation.ID, 1, now.Add(time.Minute)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale completion error = %v, want ErrConflict", err)
	}
	if err := stateStore.CompleteDeliveryOperation(ctx, operation.ID, 2, now.Add(time.Minute+time.Second)); err != nil {
		t.Fatalf("CompleteDeliveryOperation() error = %v", err)
	}
	replay, err := stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(2*time.Minute), time.Minute)
	if err != nil || replay.State != delivery.ClaimCompleted || replay.Attempt != 2 {
		t.Fatalf("completed replay = %#v, %v; want completed attempt 2", replay, err)
	}
	stored, err := stateStore.GetDeliveryOperation(ctx, operation.ID)
	if err != nil {
		t.Fatalf("GetDeliveryOperation() error = %v", err)
	}
	if stored.Status != delivery.StatusCompleted || stored.CompletedAt == nil {
		t.Fatalf("stored operation = %#v, want completed", stored)
	}
}
