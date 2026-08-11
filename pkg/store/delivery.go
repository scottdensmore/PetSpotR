package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
)

const maxDeliveryErrorLength = 1024

// ClaimDeliveryOperation transactionally obtains a fenced lease for a
// provider delivery. Completed operations remain successful no-ops.
func (m *MemoryStore) ClaimDeliveryOperation(
	ctx context.Context,
	operation delivery.Operation,
	now time.Time,
	leaseDuration time.Duration,
) (delivery.Claim, error) {
	if err := validateDeliveryClaim(operation, now, leaseDuration); err != nil {
		return delivery.Claim{}, err
	}
	if err := ctx.Err(); err != nil {
		return delivery.Claim{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.items[NotificationDeliveriesCollection]; !exists {
		m.items[NotificationDeliveriesCollection] = make(map[string][]byte)
	}
	storedData, exists := m.items[NotificationDeliveriesCollection][operation.ID]
	stored := operation
	if exists {
		var err error
		stored, err = delivery.UnmarshalOperation(storedData)
		if err != nil {
			return delivery.Claim{}, fmt.Errorf("store: decode delivery operation %s: %w", operation.ID, err)
		}
		if !sameDeliveryIdentity(stored, operation) {
			return delivery.Claim{}, fmt.Errorf("%w: delivery operation %s", ErrConflict, operation.ID)
		}
		if stored.Status == delivery.StatusCompleted {
			return delivery.Claim{State: delivery.ClaimCompleted, Attempt: stored.Attempt}, nil
		}
		if stored.Status == delivery.StatusProcessing && stored.LeaseUntil != nil && stored.LeaseUntil.After(now) {
			return delivery.Claim{State: delivery.ClaimInProgress, Attempt: stored.Attempt}, nil
		}
	}

	claimOperation(&stored, now, leaseDuration)
	data, err := delivery.MarshalOperation(stored)
	if err != nil {
		return delivery.Claim{}, err
	}
	m.items[NotificationDeliveriesCollection][operation.ID] = data
	return delivery.Claim{State: delivery.ClaimAcquired, Attempt: stored.Attempt}, nil
}

// CompleteDeliveryOperation persists successful provider delivery when the
// caller still owns the fenced attempt.
func (m *MemoryStore) CompleteDeliveryOperation(ctx context.Context, id string, attempt int, completedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" || attempt < 1 || completedAt.IsZero() {
		return errors.New("store: delivery ID, positive attempt, and completion time are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	operation, err := memoryDeliveryOperation(m, id)
	if err != nil {
		return err
	}
	if operation.Status == delivery.StatusCompleted {
		return nil
	}
	if operation.Status != delivery.StatusProcessing || operation.Attempt != attempt {
		return fmt.Errorf("%w: delivery operation %s attempt %d", ErrConflict, id, attempt)
	}
	completeOperation(&operation, completedAt)
	data, err := delivery.MarshalOperation(operation)
	if err != nil {
		return err
	}
	m.items[NotificationDeliveriesCollection][id] = data
	return nil
}

// FailDeliveryOperation persists a provider failure and releases the lease for
// immediate retry when the caller still owns the fenced attempt.
func (m *MemoryStore) FailDeliveryOperation(ctx context.Context, id string, attempt int, failedAt time.Time, failure string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" || attempt < 1 || failedAt.IsZero() || strings.TrimSpace(failure) == "" {
		return errors.New("store: delivery ID, positive attempt, failure time, and failure are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	operation, err := memoryDeliveryOperation(m, id)
	if err != nil {
		return err
	}
	if operation.Status == delivery.StatusCompleted {
		return nil
	}
	if operation.Status != delivery.StatusProcessing || operation.Attempt != attempt {
		return fmt.Errorf("%w: delivery operation %s attempt %d", ErrConflict, id, attempt)
	}
	failOperation(&operation, failedAt, failure)
	data, err := delivery.MarshalOperation(operation)
	if err != nil {
		return err
	}
	m.items[NotificationDeliveriesCollection][id] = data
	return nil
}

// GetDeliveryOperation retrieves one durable provider-delivery result.
func (m *MemoryStore) GetDeliveryOperation(ctx context.Context, id string) (delivery.Operation, error) {
	if err := ctx.Err(); err != nil {
		return delivery.Operation{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return memoryDeliveryOperation(m, id)
}

func validateDeliveryClaim(operation delivery.Operation, now time.Time, leaseDuration time.Duration) error {
	if err := delivery.ValidateOperation(operation); err != nil {
		return err
	}
	if operation.Status != delivery.StatusPending || operation.Attempt != 0 {
		return errors.New("store: new delivery claim must be a pending attempt-zero operation")
	}
	if now.IsZero() || leaseDuration <= 0 {
		return errors.New("store: delivery claim time and positive lease duration are required")
	}
	return nil
}

func sameDeliveryIdentity(left, right delivery.Operation) bool {
	return left.ID == right.ID && left.EventID == right.EventID && left.RecipientID == right.RecipientID &&
		left.Channel == right.Channel && left.IdempotencyKey == right.IdempotencyKey
}

func claimOperation(operation *delivery.Operation, now time.Time, leaseDuration time.Duration) {
	now = now.UTC()
	leaseUntil := now.Add(leaseDuration)
	operation.Status = delivery.StatusProcessing
	operation.Attempt++
	operation.UpdatedAt = now
	operation.LeaseUntil = &leaseUntil
	operation.CompletedAt = nil
	operation.LastError = ""
}

func completeOperation(operation *delivery.Operation, completedAt time.Time) {
	completedAt = completedAt.UTC()
	operation.Status = delivery.StatusCompleted
	operation.UpdatedAt = completedAt
	operation.LeaseUntil = nil
	operation.CompletedAt = &completedAt
	operation.LastError = ""
}

func failOperation(operation *delivery.Operation, failedAt time.Time, failure string) {
	failedAt = failedAt.UTC()
	failure = strings.TrimSpace(failure)
	if len(failure) > maxDeliveryErrorLength {
		failure = failure[:maxDeliveryErrorLength]
	}
	operation.Status = delivery.StatusFailed
	operation.UpdatedAt = failedAt
	operation.LeaseUntil = nil
	operation.CompletedAt = nil
	operation.LastError = failure
}

func memoryDeliveryOperation(m *MemoryStore, id string) (delivery.Operation, error) {
	data, exists := m.items[NotificationDeliveriesCollection][id]
	if !exists {
		return delivery.Operation{}, fmt.Errorf("%w: %s in store %s", ErrNotFound, id, NotificationDeliveriesCollection)
	}
	operation, err := delivery.UnmarshalOperation(data)
	if err != nil {
		return delivery.Operation{}, fmt.Errorf("store: decode delivery operation %s: %w", id, err)
	}
	if operation.ID != id {
		return delivery.Operation{}, fmt.Errorf("store: delivery operation ID %q does not match key %q", operation.ID, id)
	}
	return operation, nil
}
