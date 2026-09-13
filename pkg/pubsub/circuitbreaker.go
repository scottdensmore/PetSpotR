package pubsub

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when an operation is attempted while the circuit breaker is open.
var ErrCircuitOpen = errors.New("pubsub: circuit breaker is open")

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	StateClosed   CircuitState = "closed"
	StateHalfOpen CircuitState = "half-open"
	StateOpen     CircuitState = "open"
)

// CircuitBreakerConfig configures a CircuitBreaker.
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Cooldown         time.Duration
}

// CircuitConfig is an alias for CircuitBreakerConfig.
type CircuitConfig = CircuitBreakerConfig

// CircuitBreaker provides thread-safe protection against cascading external RPC/inference failures.
type CircuitBreaker struct {
	FailureThreshold int
	SuccessThreshold int
	Cooldown         time.Duration

	mu                 sync.Mutex
	state              CircuitState
	consecutiveFails   int
	consecutiveSuccess int
	lastStateChange    time.Time
	now                func() time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker with the given configuration.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	failureThreshold := cfg.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	successThreshold := cfg.SuccessThreshold
	if successThreshold <= 0 {
		successThreshold = 2
	}
	cooldown := cfg.Cooldown
	if cooldown <= 0 {
		cooldown = 5 * time.Second
	}

	return &CircuitBreaker{
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		Cooldown:         cooldown,
		state:            StateClosed,
	}
}

func (cb *CircuitBreaker) nowFunc() time.Time {
	if cb.now != nil {
		return cb.now()
	}
	return time.Now()
}

func (cb *CircuitBreaker) failureThresholdLocked() int {
	if cb.FailureThreshold <= 0 {
		return 3
	}
	return cb.FailureThreshold
}

func (cb *CircuitBreaker) successThresholdLocked() int {
	if cb.SuccessThreshold <= 0 {
		return 2
	}
	return cb.SuccessThreshold
}

func (cb *CircuitBreaker) cooldownLocked() time.Duration {
	if cb.Cooldown <= 0 {
		return 5 * time.Second
	}
	return cb.Cooldown
}

func (cb *CircuitBreaker) checkStateLocked(now time.Time) {
	if cb.state == "" {
		cb.state = StateClosed
	}
	if cb.state == StateOpen {
		cooldown := cb.cooldownLocked()
		if !cb.lastStateChange.IsZero() && now.Sub(cb.lastStateChange) >= cooldown {
			cb.state = StateHalfOpen
			cb.consecutiveSuccess = 0
			cb.consecutiveFails = 0
			cb.lastStateChange = now
		}
	}
}

// State returns the current CircuitState.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkStateLocked(cb.nowFunc())
	return cb.state
}

// Execute runs the operation protected by the circuit breaker.
func (cb *CircuitBreaker) Execute(ctx context.Context, op func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if op == nil {
		return nil
	}

	now := cb.nowFunc()
	cb.mu.Lock()
	cb.checkStateLocked(now)
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	cb.mu.Unlock()

	opErr := op()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	now = cb.nowFunc()
	cb.checkStateLocked(now)

	if opErr != nil {
		switch cb.state {
		case StateHalfOpen:
			// Half-Open -> Open on any failure.
			cb.state = StateOpen
			cb.lastStateChange = now
			cb.consecutiveFails = 0
			cb.consecutiveSuccess = 0
		case StateClosed:
			cb.consecutiveFails++
			if cb.consecutiveFails >= cb.failureThresholdLocked() {
				cb.state = StateOpen
				cb.lastStateChange = now
				cb.consecutiveFails = 0
				cb.consecutiveSuccess = 0
			}
		case StateOpen:
			// Already open.
		}
		return opErr
	}

	// Operation succeeded (opErr == nil)
	switch cb.state {
	case StateHalfOpen:
		cb.consecutiveSuccess++
		if cb.consecutiveSuccess >= cb.successThresholdLocked() {
			cb.state = StateClosed
			cb.lastStateChange = now
			cb.consecutiveSuccess = 0
			cb.consecutiveFails = 0
		}
	case StateClosed:
		cb.consecutiveFails = 0
	}

	return nil
}

// WithCircuitBreaker wraps a Handler with a CircuitBreaker.
func WithCircuitBreaker(cb *CircuitBreaker, handler Handler) Handler {
	if cb == nil || handler == nil {
		return handler
	}
	return func(ctx context.Context, data []byte) error {
		return cb.Execute(ctx, func() error {
			return handler(ctx, data)
		})
	}
}
