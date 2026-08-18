package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryStateStore(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	t.Run("save and get state", func(t *testing.T) {
		err := s.SaveState(ctx, "pets", "pet-123", []byte(`{"name":"Buddy"}`))
		if err != nil {
			t.Fatalf("failed to save state: %v", err)
		}

		data, err := s.GetState(ctx, "pets", "pet-123")
		if err != nil {
			t.Fatalf("failed to get state: %v", err)
		}

		if string(data) != `{"name":"Buddy"}` {
			t.Errorf("got %s, want %s", string(data), `{"name":"Buddy"}`)
		}
	})

	t.Run("get non-existent key returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetState(ctx, "pets", "non-existent")
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected store.ErrNotFound, got %v", err)
		}
	})

	t.Run("get non-existent store returns ErrStoreNotFound", func(t *testing.T) {
		_, err := s.GetState(ctx, "missing-store", "key")
		if !errors.Is(err, store.ErrStoreNotFound) {
			t.Fatalf("expected store.ErrStoreNotFound, got %v", err)
		}
	})

	t.Run("delete state", func(t *testing.T) {
		_ = s.SaveState(ctx, "pets", "pet-to-delete", []byte("data"))
		err := s.DeleteState(ctx, "pets", "pet-to-delete")
		if err != nil {
			t.Fatalf("failed to delete state: %v", err)
		}

		_, err = s.GetState(ctx, "pets", "pet-to-delete")
		if err == nil {
			t.Fatal("expected key to be deleted")
		}
	})

	t.Run("byte slice isolation on save", func(t *testing.T) {
		input := []byte("original-data")
		_ = s.SaveState(ctx, "pets", "isolated-key", input)

		input[0] = 'X'

		retrieved, err := s.GetState(ctx, "pets", "isolated-key")
		if err != nil {
			t.Fatalf("failed to get state: %v", err)
		}
		if string(retrieved) != "original-data" {
			t.Errorf("stored data mutated: got %s, want original-data", string(retrieved))
		}
	})

	t.Run("atomic update replaces existing state without exposing storage bytes", func(t *testing.T) {
		if err := s.SaveState(ctx, "updates", "key", []byte("before")); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateState(ctx, "updates", "key", func(current []byte) ([]byte, error) {
			current[0] = 'X'
			return []byte("after"), nil
		}); err != nil {
			t.Fatalf("UpdateState() error = %v", err)
		}
		got, err := s.GetState(ctx, "updates", "key")
		if err != nil || string(got) != "after" {
			t.Fatalf("updated state = %q, %v; want after", got, err)
		}
		updateErr := errors.New("update rejected")
		if err := s.UpdateState(ctx, "updates", "key", func([]byte) ([]byte, error) {
			return []byte("discarded"), updateErr
		}); !errors.Is(err, updateErr) {
			t.Fatalf("UpdateState(rejected) error = %v, want update error", err)
		}
		got, err = s.GetState(ctx, "updates", "key")
		if err != nil || string(got) != "after" {
			t.Fatalf("state after rejected update = %q, %v; want after", got, err)
		}
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("key-%d", id)
				_ = s.SaveState(ctx, "concurrent", key, []byte(key))
				_, _ = s.GetState(ctx, "concurrent", key)
			}(i)
		}
		wg.Wait()
	})

	t.Run("context cancellation", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		err := s.SaveState(canceledCtx, "pets", "key", []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("SaveState: got %v, want %v", err, context.Canceled)
		}

		_, err = s.GetState(canceledCtx, "pets", "key")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("GetState: got %v, want %v", err, context.Canceled)
		}

		err = s.DeleteState(canceledCtx, "pets", "key")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("DeleteState: got %v, want %v", err, context.Canceled)
		}

		err = s.UpdateState(canceledCtx, "pets", "key", func(current []byte) ([]byte, error) {
			return current, nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("UpdateState: got %v, want %v", err, context.Canceled)
		}
	})

	t.Run("list state", func(t *testing.T) {
		_ = s.SaveState(ctx, "list_test", "k1", []byte("v1"))
		_ = s.SaveState(ctx, "list_test", "k2", []byte("v2"))

		items, err := s.ListState(ctx, "list_test")
		if err != nil {
			t.Fatalf("ListState failed: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("got %d items, want 2", len(items))
		}
		if string(items["k1"]) != "v1" || string(items["k2"]) != "v2" {
			t.Errorf("unexpected list state output: %v", items)
		}
	})
}

func TestMemoryStoreUpdatesMatchAndParticipantsAtomically(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	if err := state.SaveState(ctx, store.MatchesCollection, "match-101", []byte("match-before")); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(
		ctx, store.MatchParticipantsCollection, "match-101", []byte("participants-before"),
	); err != nil {
		t.Fatal(err)
	}

	if err := state.UpdateMatchAndParticipants(ctx, "match-101", func(match, participants []byte) ([]byte, []byte, error) {
		match[0] = 'X'
		participants[0] = 'Y'
		return []byte("match-after"), []byte("participants-after"), nil
	}); err != nil {
		t.Fatalf("UpdateMatchAndParticipants() error = %v", err)
	}
	assertStoredValue(t, state, store.MatchesCollection, "match-101", "match-after")
	assertStoredValue(t, state, store.MatchParticipantsCollection, "match-101", "participants-after")

	rejected := errors.New("decision rejected")
	if err := state.UpdateMatchAndParticipants(ctx, "match-101", func([]byte, []byte) ([]byte, []byte, error) {
		return []byte("discarded-match"), []byte("discarded-participants"), rejected
	}); !errors.Is(err, rejected) {
		t.Fatalf("rejected update error = %v, want %v", err, rejected)
	}
	assertStoredValue(t, state, store.MatchesCollection, "match-101", "match-after")
	assertStoredValue(t, state, store.MatchParticipantsCollection, "match-101", "participants-after")
}

func assertStoredValue(t *testing.T, state store.StateStore, collection, key, want string) {
	t.Helper()
	got, err := state.GetState(context.Background(), collection, key)
	if err != nil || string(got) != want {
		t.Fatalf("%s/%s = %q, %v; want %q", collection, key, got, err, want)
	}
}
