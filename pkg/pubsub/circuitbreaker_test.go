package pubsub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	t.Run("transitions Closed -> Open after FailureThreshold failures", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Cooldown:         100 * time.Millisecond,
		})

		if s := cb.State(); s != StateClosed {
			t.Fatalf("initial state = %v, want %v", s, StateClosed)
		}

		failErr := errors.New("rpc failure")
		failingOp := func() error { return failErr }

		// Attempt 1 fails
		if err := cb.Execute(context.Background(), failingOp); !errors.Is(err, failErr) {
			t.Fatalf("attempt 1 err = %v, want %v", err, failErr)
		}
		if s := cb.State(); s != StateClosed {
			t.Fatalf("after 1 fail state = %v, want %v", s, StateClosed)
		}

		// Attempt 2 fails
		if err := cb.Execute(context.Background(), failingOp); !errors.Is(err, failErr) {
			t.Fatalf("attempt 2 err = %v, want %v", err, failErr)
		}
		if s := cb.State(); s != StateClosed {
			t.Fatalf("after 2 fails state = %v, want %v", s, StateClosed)
		}

		// Attempt 3 fails -> reaches threshold, opens circuit
		if err := cb.Execute(context.Background(), failingOp); !errors.Is(err, failErr) {
			t.Fatalf("attempt 3 err = %v, want %v", err, failErr)
		}
		if s := cb.State(); s != StateOpen {
			t.Fatalf("after 3 fails state = %v, want %v", s, StateOpen)
		}

		// Attempt 4 should immediately return ErrCircuitOpen without executing op
		var executed bool
		err := cb.Execute(context.Background(), func() error {
			executed = true
			return nil
		})
		if !errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("err = %v, want %v", err, ErrCircuitOpen)
		}
		if executed {
			t.Fatal("op was executed while circuit breaker was open")
		}
	})

	t.Run("transitions Open -> Half-Open after Cooldown", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 2,
			Cooldown:         20 * time.Millisecond,
		})

		_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
		if s := cb.State(); s != StateOpen {
			t.Fatalf("state = %v, want %v", s, StateOpen)
		}

		// Wait for cooldown to expire
		time.Sleep(30 * time.Millisecond)

		if s := cb.State(); s != StateHalfOpen {
			t.Fatalf("after cooldown state = %v, want %v", s, StateHalfOpen)
		}
	})

	t.Run("transitions Half-Open -> Closed after SuccessThreshold successes", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 2,
			Cooldown:         10 * time.Millisecond,
		})

		// Open circuit
		_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
		time.Sleep(15 * time.Millisecond)

		if s := cb.State(); s != StateHalfOpen {
			t.Fatalf("state = %v, want %v", s, StateHalfOpen)
		}

		// Success 1 in Half-Open
		err := cb.Execute(context.Background(), func() error { return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s := cb.State(); s != StateHalfOpen {
			t.Fatalf("after 1 success in half-open state = %v, want %v", s, StateHalfOpen)
		}

		// Success 2 in Half-Open -> reaches SuccessThreshold 2, closes circuit
		err = cb.Execute(context.Background(), func() error { return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s := cb.State(); s != StateClosed {
			t.Fatalf("after 2 successes in half-open state = %v, want %v", s, StateClosed)
		}
	})

	t.Run("transitions Half-Open -> Open on any failure", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Cooldown:         10 * time.Millisecond,
		})

		// Force open by 5 failures
		for i := 0; i < 5; i++ {
			_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
		}
		if s := cb.State(); s != StateOpen {
			t.Fatalf("state = %v, want %v", s, StateOpen)
		}

		time.Sleep(15 * time.Millisecond)
		if s := cb.State(); s != StateHalfOpen {
			t.Fatalf("state = %v, want %v", s, StateHalfOpen)
		}

		// 1 success in Half-Open
		_ = cb.Execute(context.Background(), func() error { return nil })
		if s := cb.State(); s != StateHalfOpen {
			t.Fatalf("state = %v, want %v", s, StateHalfOpen)
		}

		// 1 failure in Half-Open immediately trips back to Open
		tripErr := errors.New("trip back to open")
		err := cb.Execute(context.Background(), func() error { return tripErr })
		if !errors.Is(err, tripErr) {
			t.Fatalf("err = %v, want %v", err, tripErr)
		}
		if s := cb.State(); s != StateOpen {
			t.Fatalf("after failure in half-open state = %v, want %v", s, StateOpen)
		}
	})

	t.Run("success in Closed state resets consecutive failure count", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 2,
			Cooldown:         100 * time.Millisecond,
		})

		// 1 failure
		_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
		// 1 success resets counter
		_ = cb.Execute(context.Background(), func() error { return nil })
		// 1 failure should not trip because counter was reset
		_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
		if s := cb.State(); s != StateClosed {
			t.Fatalf("state = %v, want %v", s, StateClosed)
		}
	})
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var executed bool
	err := cb.Execute(ctx, func() error {
		executed = true
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
	if executed {
		t.Fatal("op executed with canceled context")
	}
}

func TestCircuitBreaker_ConcurrencySafety(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         5 * time.Millisecond,
	})

	const numGoroutines = 50
	const iterations = 50
	var wg sync.WaitGroup

	var successes int64
	var openErrors int64
	var opErrors int64

	customErr := errors.New("transient error")

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternately check state and execute
				_ = cb.State()

				err := cb.Execute(context.Background(), func() error {
					if (id+j)%3 == 0 {
						return customErr
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&successes, 1)
				} else if errors.Is(err, ErrCircuitOpen) {
					atomic.AddInt64(&openErrors, 1)
				} else if errors.Is(err, customErr) {
					atomic.AddInt64(&opErrors, 1)
				}
				time.Sleep(time.Microsecond * 100)
			}
		}(i)
	}

	wg.Wait()

	total := successes + openErrors + opErrors
	if total != numGoroutines*iterations {
		t.Fatalf("total operations %d, want %d", total, numGoroutines*iterations)
	}
}

func TestWithCircuitBreaker(t *testing.T) {
	t.Run("nil circuit breaker returns handler as is", func(t *testing.T) {
		var called bool
		handler := func(_ context.Context, _ []byte) error {
			called = true
			return nil
		}
		wrapped := WithCircuitBreaker(nil, handler)
		_ = wrapped(context.Background(), nil)
		if !called {
			t.Fatal("handler was not called")
		}
	})

	t.Run("nil handler returns nil", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{})
		wrapped := WithCircuitBreaker(cb, nil)
		if wrapped != nil {
			t.Fatal("expected nil wrapped handler")
		}
	})

	t.Run("wrapped handler blocks calls when circuit opens", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 2,
			Cooldown:         50 * time.Millisecond,
		})

		var calls int
		handler := func(_ context.Context, _ []byte) error {
			calls++
			return errors.New("backend down")
		}

		wrapped := WithCircuitBreaker(cb, handler)

		// 2 failures
		_ = wrapped(context.Background(), []byte("1"))
		_ = wrapped(context.Background(), []byte("2"))

		if cb.State() != StateOpen {
			t.Fatalf("state = %v, want %v", cb.State(), StateOpen)
		}

		// 3rd call should return ErrCircuitOpen without invoking handler
		err := wrapped(context.Background(), []byte("3"))
		if !errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("err = %v, want %v", err, ErrCircuitOpen)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})
}
