package pubsub_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

func TestMemoryPubSub(t *testing.T) {
	ctx := context.Background()
	ps := pubsub.NewMemoryPubSub()

	t.Run("publish and subscribe", func(t *testing.T) {
		var received []byte
		handler := func(ctx context.Context, data []byte) error {
			received = data
			return nil
		}

		err := ps.Subscribe("lostPet", handler)
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		err = ps.Publish(ctx, "lostPet", []byte("hello-pet"))
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}

		if string(received) != "hello-pet" {
			t.Errorf("received payload %s, want hello-pet", string(received))
		}
	})

	t.Run("multiple subscribers receive cloned payloads", func(t *testing.T) {
		var count int
		var mu sync.Mutex

		h1 := func(ctx context.Context, data []byte) error {
			data[0] = 'X' // Mutates local clone
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		}

		h2 := func(ctx context.Context, data []byte) error {
			if string(data) != "event-data" {
				t.Errorf("subscriber h2 got mutated data: %s", string(data))
			}
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		}

		_ = ps.Subscribe("multiTopic", h1)
		_ = ps.Subscribe("multiTopic", h2)

		_ = ps.Publish(ctx, "multiTopic", []byte("event-data"))

		if count != 2 {
			t.Errorf("expected 2 handler invocations, got %d", count)
		}
	})

	t.Run("subscriber error collection", func(t *testing.T) {
		errFail := errors.New("handler error")
		hFail := func(ctx context.Context, data []byte) error {
			return errFail
		}

		_ = ps.Subscribe("errTopic", hFail)

		err := ps.Publish(ctx, "errTopic", []byte("data"))
		if !errors.Is(err, errFail) {
			t.Fatalf("expected error containing errFail, got %v", err)
		}
	})

	t.Run("concurrent publish and subscribe", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				topic := fmt.Sprintf("topic-%d", id%5)
				_ = ps.Subscribe(topic, func(ctx context.Context, data []byte) error {
					return nil
				})
				_ = ps.Publish(ctx, topic, []byte("data"))
			}(i)
		}
		wg.Wait()
	})

	t.Run("context cancellation", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		err := ps.Publish(canceledCtx, "topic", []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Publish: got %v, want %v", err, context.Canceled)
		}
	})
}
