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
	})
}
