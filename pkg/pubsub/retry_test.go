package pubsub

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryPolicy_NextBackoff(t *testing.T) {
	t.Run("exponential growth without jitter", func(t *testing.T) {
		p := RetryPolicy{
			InitialInterval: 50 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          false,
		}

		if got := p.NextBackoff(1); got != 50*time.Millisecond {
			t.Fatalf("attempt 1 backoff = %v, want %v", got, 50*time.Millisecond)
		}
		if got := p.NextBackoff(2); got != 100*time.Millisecond {
			t.Fatalf("attempt 2 backoff = %v, want %v", got, 100*time.Millisecond)
		}
		if got := p.NextBackoff(3); got != 200*time.Millisecond {
			t.Fatalf("attempt 3 backoff = %v, want %v", got, 200*time.Millisecond)
		}
		if got := p.NextBackoff(4); got != 400*time.Millisecond {
			t.Fatalf("attempt 4 backoff = %v, want %v", got, 400*time.Millisecond)
		}
		// Exceeding MaxInterval gets clamped to 500ms
		if got := p.NextBackoff(5); got != 500*time.Millisecond {
			t.Fatalf("attempt 5 backoff = %v, want %v", got, 500*time.Millisecond)
		}
	})

	t.Run("default multiplier", func(t *testing.T) {
		p := RetryPolicy{
			InitialInterval: 10 * time.Millisecond,
			Multiplier:      0, // should default to 2.0
		}
		if got := p.NextBackoff(2); got != 20*time.Millisecond {
			t.Fatalf("attempt 2 backoff with default multiplier = %v, want %v", got, 20*time.Millisecond)
		}
	})

	t.Run("zero initial interval or invalid attempt", func(t *testing.T) {
		p := RetryPolicy{}
		if got := p.NextBackoff(1); got != 0 {
			t.Fatalf("zero policy backoff = %v, want 0", got)
		}
		p.InitialInterval = 10 * time.Millisecond
		if got := p.NextBackoff(0); got != 0 {
			t.Fatalf("attempt 0 backoff = %v, want 0", got)
		}
		if got := p.NextBackoff(-1); got != 0 {
			t.Fatalf("negative attempt backoff = %v, want 0", got)
		}
	})

	t.Run("with jitter enabled", func(t *testing.T) {
		p := RetryPolicy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
			Jitter:          true,
		}

		for i := 0; i < 20; i++ {
			got := p.NextBackoff(1)
			if got < 0 || got > 100*time.Millisecond {
				t.Fatalf("attempt 1 jittered backoff %v out of bounds [0, 100ms]", got)
			}
		}
	})

	t.Run("default retry policy", func(t *testing.T) {
		dp := DefaultRetryPolicy()
		if dp.MaxAttempts != 3 {
			t.Fatalf("default MaxAttempts = %d, want 3", dp.MaxAttempts)
		}
		if dp.InitialInterval != 100*time.Millisecond {
			t.Fatalf("default InitialInterval = %v, want 100ms", dp.InitialInterval)
		}
		if dp.MaxInterval != 2*time.Second {
			t.Fatalf("default MaxInterval = %v, want 2s", dp.MaxInterval)
		}
		if dp.Multiplier != 2.0 {
			t.Fatalf("default Multiplier = %v, want 2.0", dp.Multiplier)
		}
		if !dp.Jitter {
			t.Fatal("default Jitter should be true")
		}
	})
}

func TestWithRetry(t *testing.T) {
	t.Run("nil handler returns nil", func(t *testing.T) {
		wrapped := WithRetry(DefaultRetryPolicy(), nil)
		if wrapped != nil {
			t.Fatal("expected nil wrapped handler for nil input")
		}
	})

	t.Run("succeeds on first attempt", func(t *testing.T) {
		var calls int32
		handler := func(ctx context.Context, data []byte) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}

		wrapped := WithRetry(RetryPolicy{MaxAttempts: 3}, handler)
		err := wrapped(context.Background(), []byte("data"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("calls = %d, want 1", got)
		}
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		var calls int32
		handler := func(ctx context.Context, data []byte) error {
			c := atomic.AddInt32(&calls, 1)
			if c < 3 {
				return errors.New("transient error")
			}
			return nil
		}

		policy := RetryPolicy{
			MaxAttempts:     3,
			InitialInterval: time.Millisecond,
			Multiplier:      1.5,
		}
		wrapped := WithRetry(policy, handler)
		err := wrapped(context.Background(), []byte("data"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := atomic.LoadInt32(&calls); got != 3 {
			t.Fatalf("calls = %d, want 3", got)
		}
	})

	t.Run("exhausts max attempts and returns last error", func(t *testing.T) {
		var calls int32
		sentinelErr := errors.New("permanent failure")
		handler := func(ctx context.Context, data []byte) error {
			atomic.AddInt32(&calls, 1)
			return sentinelErr
		}

		policy := RetryPolicy{
			MaxAttempts:     4,
			InitialInterval: time.Millisecond,
		}
		wrapped := WithRetry(policy, handler)
		err := wrapped(context.Background(), []byte("data"))
		if !errors.Is(err, sentinelErr) {
			t.Fatalf("err = %v, want %v", err, sentinelErr)
		}
		if got := atomic.LoadInt32(&calls); got != 4 {
			t.Fatalf("calls = %d, want 4", got)
		}
	})

	t.Run("defaults to 3 max attempts when zero", func(t *testing.T) {
		var calls int32
		handler := func(ctx context.Context, data []byte) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("fail")
		}

		wrapped := WithRetry(RetryPolicy{InitialInterval: time.Millisecond}, handler)
		_ = wrapped(context.Background(), []byte("data"))
		if got := atomic.LoadInt32(&calls); got != 3 {
			t.Fatalf("calls = %d, want 3", got)
		}
	})

	t.Run("context cancelled before start", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		var calls int32
		handler := func(ctx context.Context, data []byte) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}

		wrapped := WithRetry(RetryPolicy{MaxAttempts: 3}, handler)
		err := wrapped(ctx, []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want %v", err, context.Canceled)
		}
		if got := atomic.LoadInt32(&calls); got != 0 {
			t.Fatalf("calls = %d, want 0", got)
		}
	})

	t.Run("context cancelled stopping retries early", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var calls int32
		handler := func(ctx context.Context, data []byte) error {
			c := atomic.AddInt32(&calls, 1)
			if c == 1 {
				cancel()
			}
			return errors.New("err")
		}

		policy := RetryPolicy{
			MaxAttempts:     5,
			InitialInterval: 100 * time.Millisecond,
		}
		wrapped := WithRetry(policy, handler)
		err := wrapped(ctx, []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want %v", err, context.Canceled)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("calls = %d, want 1", got)
		}
	})
}
