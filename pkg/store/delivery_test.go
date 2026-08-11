package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryStoreDeliveryOperationLifecycle(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	operation, err := delivery.NewOperation("evt-match-101", "lost-101", "email", now)
	if err != nil {
		t.Fatal(err)
	}

	claim, err := stateStore.ClaimDeliveryOperation(ctx, operation, now, 30*time.Second)
	if err != nil || claim.State != delivery.ClaimAcquired || claim.Attempt != 1 {
		t.Fatalf("first claim = %#v, %v; want acquired attempt 1", claim, err)
	}
	claim, err = stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(time.Second), 30*time.Second)
	if err != nil || claim.State != delivery.ClaimInProgress || claim.Attempt != 1 {
		t.Fatalf("active claim = %#v, %v; want in-progress attempt 1", claim, err)
	}
	claim, err = stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(31*time.Second), 30*time.Second)
	if err != nil || claim.State != delivery.ClaimAcquired || claim.Attempt != 2 {
		t.Fatalf("expired claim = %#v, %v; want acquired attempt 2", claim, err)
	}
	if err := stateStore.CompleteDeliveryOperation(ctx, operation.ID, 1, now.Add(32*time.Second)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale completion error = %v, want ErrConflict", err)
	}
	if err := stateStore.FailDeliveryOperation(ctx, operation.ID, 2, now.Add(32*time.Second), "provider unavailable"); err != nil {
		t.Fatalf("FailDeliveryOperation() error = %v", err)
	}
	claim, err = stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(33*time.Second), 30*time.Second)
	if err != nil || claim.State != delivery.ClaimAcquired || claim.Attempt != 3 {
		t.Fatalf("failed retry claim = %#v, %v; want acquired attempt 3", claim, err)
	}
	if err := stateStore.CompleteDeliveryOperation(ctx, operation.ID, 3, now.Add(34*time.Second)); err != nil {
		t.Fatalf("CompleteDeliveryOperation() error = %v", err)
	}
	claim, err = stateStore.ClaimDeliveryOperation(ctx, operation, now.Add(time.Minute), 30*time.Second)
	if err != nil || claim.State != delivery.ClaimCompleted || claim.Attempt != 3 {
		t.Fatalf("completed replay = %#v, %v; want completed attempt 3", claim, err)
	}

	stored, err := stateStore.GetDeliveryOperation(ctx, operation.ID)
	if err != nil {
		t.Fatalf("GetDeliveryOperation() error = %v", err)
	}
	if stored.Status != delivery.StatusCompleted || stored.CompletedAt == nil || stored.LastError != "" {
		t.Fatalf("stored operation = %#v, want completed result", stored)
	}
}

func TestMemoryStoreDeliveryClaimIsAtomic(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	operation, err := delivery.NewOperation("evt-concurrent", "lost-202", "push", now)
	if err != nil {
		t.Fatal(err)
	}

	const callers = 16
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

	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("concurrent claim error = %v", err)
		}
	}
	acquired := 0
	inProgress := 0
	for claim := range results {
		switch claim.State {
		case delivery.ClaimAcquired:
			acquired++
		case delivery.ClaimInProgress:
			inProgress++
		default:
			t.Fatalf("unexpected concurrent claim %#v", claim)
		}
	}
	if acquired != 1 || inProgress != callers-1 {
		t.Fatalf("concurrent claims acquired=%d in-progress=%d, want 1/%d", acquired, inProgress, callers-1)
	}
}
