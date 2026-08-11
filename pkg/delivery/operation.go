// Package delivery defines durable, idempotent provider-delivery operations.
package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrOperationInProgress asks an at-least-once consumer to retry after the
// active delivery lease expires or completes.
var ErrOperationInProgress = errors.New("delivery: operation already in progress")

// Status describes the durable lifecycle of a provider delivery.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusFailed     Status = "failed"
	StatusCompleted  Status = "completed"
)

// ClaimState describes the result of transactionally claiming an operation.
type ClaimState string

const (
	ClaimAcquired   ClaimState = "acquired"
	ClaimInProgress ClaimState = "in-progress"
	ClaimCompleted  ClaimState = "completed"
)

// Claim is a fenced lease returned to a delivery worker.
type Claim struct {
	State   ClaimState
	Attempt int
}

// Operation is the durable result and lease record for one event, recipient,
// and delivery channel. ID is also the idempotency key sent to the provider.
type Operation struct {
	ID             string     `json:"id"`
	EventID        string     `json:"eventId"`
	RecipientID    string     `json:"recipientId"`
	Channel        string     `json:"channel"`
	IdempotencyKey string     `json:"idempotencyKey"`
	Status         Status     `json:"status"`
	Attempt        int        `json:"attempt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	LeaseUntil     *time.Time `json:"leaseUntil,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
}

// NewOperation constructs the stable pending operation for one channel.
func NewOperation(eventID, recipientID, channel string, createdAt time.Time) (Operation, error) {
	eventID = strings.TrimSpace(eventID)
	recipientID = strings.TrimSpace(recipientID)
	channel = strings.TrimSpace(channel)
	if eventID == "" || recipientID == "" || channel == "" || createdAt.IsZero() {
		return Operation{}, errors.New("delivery: event ID, recipient ID, channel, and creation time are required")
	}
	id := operationID(eventID, recipientID, channel)
	createdAt = createdAt.UTC()
	return Operation{
		ID:             id,
		EventID:        eventID,
		RecipientID:    recipientID,
		Channel:        channel,
		IdempotencyKey: id,
		Status:         StatusPending,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}, nil
}

// ResolveEventID uses the current envelope ID or derives a stable identity for
// an exact legacy payload so in-flight pre-envelope messages remain dedupable.
func ResolveEventID(envelopeID, eventType string, legacyPayload []byte) (string, error) {
	if envelopeID = strings.TrimSpace(envelopeID); envelopeID != "" {
		return envelopeID, nil
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" || len(legacyPayload) == 0 {
		return "", errors.New("delivery: legacy event type and payload are required")
	}
	digest := sha256.New()
	_, _ = fmt.Fprintf(digest, "legacy\x00%s\x00", eventType)
	_, _ = digest.Write(legacyPayload)
	return "evt_legacy_" + hex.EncodeToString(digest.Sum(nil)), nil
}

// MarshalOperation validates and serializes an operation.
func MarshalOperation(operation Operation) ([]byte, error) {
	if err := ValidateOperation(operation); err != nil {
		return nil, err
	}
	return json.Marshal(operation)
}

// UnmarshalOperation decodes and validates an operation.
func UnmarshalOperation(data []byte) (Operation, error) {
	var operation Operation
	if err := json.Unmarshal(data, &operation); err != nil {
		return Operation{}, fmt.Errorf("delivery: decode operation: %w", err)
	}
	if err := ValidateOperation(operation); err != nil {
		return Operation{}, err
	}
	return operation, nil
}

// ValidateOperation verifies identity and lifecycle invariants.
func ValidateOperation(operation Operation) error {
	if operation.ID == "" || operation.EventID == "" || operation.RecipientID == "" || operation.Channel == "" ||
		operation.IdempotencyKey == "" || operation.CreatedAt.IsZero() || operation.UpdatedAt.IsZero() {
		return errors.New("delivery: incomplete operation metadata")
	}
	wantID := operationID(operation.EventID, operation.RecipientID, operation.Channel)
	if operation.ID != wantID || operation.IdempotencyKey != wantID {
		return errors.New("delivery: operation identity does not match its fields")
	}
	switch operation.Status {
	case StatusPending:
		if operation.Attempt != 0 || operation.LeaseUntil != nil || operation.CompletedAt != nil || operation.LastError != "" {
			return errors.New("delivery: invalid pending operation state")
		}
	case StatusProcessing:
		if operation.Attempt < 1 || operation.LeaseUntil == nil || operation.LeaseUntil.IsZero() || operation.CompletedAt != nil {
			return errors.New("delivery: invalid processing operation state")
		}
	case StatusFailed:
		if operation.Attempt < 1 || operation.LeaseUntil != nil || operation.CompletedAt != nil || strings.TrimSpace(operation.LastError) == "" {
			return errors.New("delivery: invalid failed operation state")
		}
	case StatusCompleted:
		if operation.Attempt < 1 || operation.LeaseUntil != nil || operation.CompletedAt == nil || operation.CompletedAt.IsZero() || operation.LastError != "" {
			return errors.New("delivery: invalid completed operation state")
		}
	default:
		return fmt.Errorf("delivery: invalid operation status %q", operation.Status)
	}
	return nil
}

func operationID(eventID, recipientID, channel string) string {
	digest := sha256.New()
	_, _ = fmt.Fprintf(digest, "%s\x00%s\x00%s", strings.TrimSpace(eventID), strings.TrimSpace(recipientID), strings.TrimSpace(channel))
	return "delivery_" + hex.EncodeToString(digest.Sum(nil))
}
