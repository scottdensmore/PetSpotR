package delivery_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
)

func TestNewOperationDerivesStableOpaqueIdentity(t *testing.T) {
	createdAt := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	first, err := delivery.NewOperation("evt-match-101", "lost-101", "email", createdAt)
	if err != nil {
		t.Fatalf("NewOperation() error = %v", err)
	}
	second, err := delivery.NewOperation("evt-match-101", "lost-101", "email", createdAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewOperation() retry error = %v", err)
	}
	if first.ID != second.ID || first.IdempotencyKey != second.IdempotencyKey {
		t.Fatalf("retry identity = %q/%q, want %q/%q", second.ID, second.IdempotencyKey, first.ID, first.IdempotencyKey)
	}
	if first.ID == "" || first.ID == "evt-match-101:lost-101:email" {
		t.Fatalf("operation ID = %q, want non-empty opaque identity", first.ID)
	}
	if first.Status != delivery.StatusPending || first.Attempt != 0 {
		t.Fatalf("new operation = %#v, want pending attempt zero", first)
	}

	differentChannel, err := delivery.NewOperation("evt-match-101", "lost-101", "sms", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if differentChannel.ID == first.ID {
		t.Fatal("different channels produced the same operation ID")
	}
}

func TestResolveEventIDPreservesEnvelopeAndStabilizesLegacyPayload(t *testing.T) {
	if got, err := delivery.ResolveEventID("evt-envelope", "ignored", nil); err != nil || got != "evt-envelope" {
		t.Fatalf("ResolveEventID(envelope) = %q, %v", got, err)
	}

	payload := []byte(`{"foundPetId":"found-1","matchedPetId":"lost-1"}`)
	first, err := delivery.ResolveEventID("", "petspotr.match.found", payload)
	if err != nil {
		t.Fatalf("ResolveEventID(legacy) error = %v", err)
	}
	second, err := delivery.ResolveEventID("", "petspotr.match.found", payload)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == "" {
		t.Fatalf("legacy IDs = %q/%q, want stable non-empty IDs", first, second)
	}
	if _, err := delivery.ResolveEventID("", "", payload); err == nil {
		t.Fatal("ResolveEventID() error = nil, want missing type error")
	}
	if _, err := delivery.ResolveEventID("", "petspotr.match.found", nil); err == nil {
		t.Fatal("ResolveEventID() error = nil, want missing payload error")
	}
}
