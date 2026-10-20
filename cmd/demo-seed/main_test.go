package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestSeedStoreAndShouldSeedFromEnv(t *testing.T) {
	mem := store.NewMemoryStore()
	var out bytes.Buffer
	if err := seedStore(context.Background(), mem, &out); err != nil {
		t.Fatalf("seedStore failed: %v", err)
	}
	if !strings.Contains(out.String(), "Seeded demo matches and pets") {
		t.Fatalf("expected seed confirmation output, got: %s", out.String())
	}

	matches, err := mem.ListState(context.Background(), store.MatchesCollection)
	if err != nil || len(matches) == 0 {
		t.Fatalf("expected matches to be seeded, got %d (err=%v)", len(matches), err)
	}
	lostPets, err := mem.ListState(context.Background(), store.LostPetsCollection)
	if err != nil || len(lostPets) == 0 {
		t.Fatalf("expected lost pets to be seeded, got %d (err=%v)", len(lostPets), err)
	}
	foundPets, err := mem.ListState(context.Background(), store.FoundPetsCollection)
	if err != nil || len(foundPets) == 0 {
		t.Fatalf("expected found pets to be seeded, got %d (err=%v)", len(foundPets), err)
	}
}

func TestRunSkipAndModeRefusal(t *testing.T) {
	t.Setenv("SEED_DEMO_DATA", "false")
	var out bytes.Buffer
	if err := run(context.Background(), &out); err != nil {
		t.Fatalf("expected nil error when SEED_DEMO_DATA=false, got: %v", err)
	}
	if !strings.Contains(out.String(), "Skipping demo data seed") {
		t.Fatalf("expected skip message, got: %s", out.String())
	}

	t.Setenv("SEED_DEMO_DATA", "true")
	t.Setenv("PETSPOTR_RUNTIME_MODE", "memory")
	t.Setenv("FIRESTORE_EMULATOR_HOST", "")
	if err := run(context.Background(), &out); err == nil || !strings.Contains(err.Error(), "requires local-emulator runtime mode") {
		t.Fatalf("expected local-emulator mode refusal error, got: %v", err)
	}
}
