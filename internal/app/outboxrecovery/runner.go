// Package outboxrecovery runs the managed outbox maintenance lifecycle shared
// by HTTP service entrypoints.
package outboxrecovery

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	defaultRecoveryInterval          = 5 * time.Second
	defaultCompletedBackfillInterval = time.Minute
)

// RecoverFunc publishes a bounded batch of pending outbox records.
type RecoverFunc func(context.Context) (int, error)

// BeforeCycleFunc runs service-specific maintenance before backfill and
// recovery. The supplied time is UTC and is shared by the complete cycle.
type BeforeCycleFunc func(context.Context, time.Time)

// LogFunc records deferred work and successful legacy migrations.
type LogFunc func(string, ...any)

// Config supplies the service-specific operations and lifecycle dependencies.
// Zero cadence values select the production defaults.
type Config struct {
	Service                   string
	Backfiller                store.OutboxIndexBackfiller
	Recover                   RecoverFunc
	BeforeCycle               BeforeCycleFunc
	Logf                      LogFunc
	Now                       func() time.Time
	RecoveryInterval          time.Duration
	CompletedBackfillInterval time.Duration
}

// Runner coordinates one service's background outbox maintenance. Run must be
// called by at most one goroutine.
type Runner struct {
	service                   string
	backfiller                store.OutboxIndexBackfiller
	recover                   RecoverFunc
	beforeCycle               BeforeCycleFunc
	logf                      LogFunc
	now                       func() time.Time
	recoveryInterval          time.Duration
	completedBackfillInterval time.Duration
	nextBackfillAt            time.Time
}

// New validates config and returns a managed recovery runner.
func New(config Config) (*Runner, error) {
	if config.Service == "" {
		return nil, errors.New("outbox recovery: service is required")
	}
	if config.Recover == nil {
		return nil, errors.New("outbox recovery: recovery callback is required")
	}
	if config.RecoveryInterval < 0 {
		return nil, errors.New("outbox recovery: recovery interval must not be negative")
	}
	if config.CompletedBackfillInterval < 0 {
		return nil, errors.New("outbox recovery: completed backfill interval must not be negative")
	}

	if config.Logf == nil {
		config.Logf = log.Printf
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.RecoveryInterval == 0 {
		config.RecoveryInterval = defaultRecoveryInterval
	}
	if config.CompletedBackfillInterval == 0 {
		config.CompletedBackfillInterval = defaultCompletedBackfillInterval
	}

	return &Runner{
		service:                   config.Service,
		backfiller:                config.Backfiller,
		recover:                   config.Recover,
		beforeCycle:               config.BeforeCycle,
		logf:                      config.Logf,
		now:                       config.Now,
		recoveryInterval:          config.RecoveryInterval,
		completedBackfillInterval: config.CompletedBackfillInterval,
	}, nil
}

// Run performs an immediate maintenance cycle, repeats it at the configured
// cadence, and returns when ctx is cancelled.
func (r *Runner) Run(ctx context.Context) {
	r.runCycle(ctx)
	ticker := time.NewTicker(r.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runCycle(ctx)
		}
	}
}

func (r *Runner) runCycle(ctx context.Context) {
	now := r.now().UTC()
	if r.beforeCycle != nil {
		r.beforeCycle(ctx, now)
	}

	if r.backfiller != nil && !now.Before(r.nextBackfillAt) {
		migrated, complete, err := r.backfiller.BackfillOutboxIndexes(ctx, outbox.MaxPublishBatch)
		if err != nil && ctx.Err() == nil {
			r.logf("%s legacy outbox index backfill deferred: %v", r.service, err)
		} else if complete {
			r.nextBackfillAt = now.Add(r.completedBackfillInterval)
		}
		if migrated > 0 {
			r.logf("%s legacy outbox index backfill migrated %d records (complete=%t)", r.service, migrated, complete)
		}
	}

	if _, err := r.recover(ctx); err != nil && ctx.Err() == nil {
		r.logf("%s outbox recovery deferred: %v", r.service, err)
	}
}
