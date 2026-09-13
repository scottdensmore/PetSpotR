package pubsub

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// RetryPolicy defines the parameters for exponential backoff retries.
type RetryPolicy struct {
	MaxAttempts     int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	Jitter          bool
}

// DefaultRetryPolicy returns a RetryPolicy configured with sensible defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
	}
}

// NextBackoff calculates the backoff duration for a given 1-based attempt.
// For attempt 1, the interval is InitialInterval. Subsequent attempts scale
// exponentially by Multiplier up to MaxInterval. When Jitter is enabled,
// the returned duration is randomized between 0 and the calculated interval.
func (p RetryPolicy) NextBackoff(attempt int) time.Duration {
	if p.InitialInterval <= 0 || attempt < 1 {
		return 0
	}

	multiplier := p.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	backoff := float64(p.InitialInterval) * math.Pow(multiplier, float64(attempt-1))
	if p.MaxInterval > 0 && backoff > float64(p.MaxInterval) {
		backoff = float64(p.MaxInterval)
	}

	d := time.Duration(backoff)
	if p.Jitter && d > 0 {
		d = time.Duration(rand.Float64() * float64(d))
	}
	return d
}

// WithRetry wraps a Handler, retrying on error up to policy.MaxAttempts with
// exponential backoff while respecting context cancellation.
func WithRetry(policy RetryPolicy, handler Handler) Handler {
	if handler == nil {
		return nil
	}
	maxAttempts := policy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	return func(ctx context.Context, data []byte) error {
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if tracker, ok := ctx.Value(attemptsTrackerKey).(*attemptsTracker); ok {
				tracker.count = attempt
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			lastErr = handler(ctx, data)
			if lastErr == nil {
				return nil
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if attempt == maxAttempts {
				break
			}

			backoff := policy.NextBackoff(attempt)
			if backoff > 0 {
				timer := time.NewTimer(backoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
		return lastErr
	}
}
