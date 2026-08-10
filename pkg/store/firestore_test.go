package store

import (
	"context"
	"strings"
	"testing"
)

func TestNewFirestoreStoreRejectsMissingProject(t *testing.T) {
	t.Parallel()

	if _, err := NewFirestoreStore(context.Background(), ""); err == nil {
		t.Fatal("NewFirestoreStore() error = nil, want non-nil")
	}
}

func TestFirestoreDocumentIDSupportsOpaqueKeys(t *testing.T) {
	t.Parallel()

	firstKey := "https://push.example.test/send/a/b?token=one"
	secondKey := "https://push.example.test/send/a/b?token=two"
	firstID := firestoreDocumentID(firstKey)
	secondID := firestoreDocumentID(secondKey)

	if firstID == secondID {
		t.Fatal("different opaque keys produced the same document ID")
	}
	if len(firstID) != 64 {
		t.Fatalf("document ID length = %d, want 64", len(firstID))
	}
	if strings.Contains(firstID, "/") {
		t.Fatalf("document ID %q contains a path separator", firstID)
	}
}

func TestNewFirestoreEmulatorStoreRejectsMismatchedEnvironment(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8085")

	if _, err := NewFirestoreEmulatorStore(context.Background(), "petspotr-test", "127.0.0.1:8086"); err == nil {
		t.Fatal("NewFirestoreEmulatorStore() error = nil, want non-nil")
	}
}
