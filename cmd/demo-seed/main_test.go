package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunRefusesNonEmulatorMode(t *testing.T) {
	t.Setenv("PETSPOTR_RUNTIME_MODE", "memory")
	t.Setenv("FIRESTORE_EMULATOR_HOST", "")
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "requires local-emulator runtime mode") {
		t.Fatalf("expected local-emulator mode refusal error, got: %v", err)
	}
}
