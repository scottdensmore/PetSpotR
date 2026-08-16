package outboxrecovery

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/outbox"
)

type backfillResult struct {
	migrated int
	complete bool
	err      error
}

type recordingBackfiller struct {
	results []backfillResult
	limits  []int
	onCall  func()
}

func (b *recordingBackfiller) BackfillOutboxIndexes(_ context.Context, limit int) (int, bool, error) {
	if b.onCall != nil {
		b.onCall()
	}
	b.limits = append(b.limits, limit)
	result := b.results[0]
	b.results = b.results[1:]
	return result.migrated, result.complete, result.err
}

func TestRunnerRunExecutesImmediateCycleInOrderAndStopsOnCancellation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.FixedZone("PDT", -7*60*60))
	ctx, cancel := context.WithCancel(context.Background())
	var calls []string
	backfiller := &recordingBackfiller{
		results: []backfillResult{{complete: true}},
		onCall:  func() { calls = append(calls, "backfill") },
	}
	runner, err := New(Config{
		Service:    "LostPet",
		Backfiller: backfiller,
		BeforeCycle: func(_ context.Context, cycleTime time.Time) {
			calls = append(calls, "before:"+cycleTime.Format(time.RFC3339))
		},
		Recover: func(context.Context) (int, error) {
			calls = append(calls, "recover")
			cancel()
			return 0, nil
		},
		Now:              func() time.Time { return now },
		RecoveryInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runner.Run(ctx)

	wantCalls := []string{"before:2026-08-15T19:00:00Z", "backfill", "recover"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	if !reflect.DeepEqual(backfiller.limits, []int{outbox.MaxPublishBatch}) {
		t.Fatalf("backfill limits = %v, want [%d]", backfiller.limits, outbox.MaxPublishBatch)
	}
}

func TestRunnerDelaysCompletedBackfillWhileContinuingRecovery(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 15, 19, 0, 0, 0, time.UTC)
	backfiller := &recordingBackfiller{results: []backfillResult{
		{complete: true},
		{complete: true},
	}}
	recoveryCalls := 0
	runner, err := New(Config{
		Service:    "LostPet",
		Backfiller: backfiller,
		Recover: func(context.Context) (int, error) {
			recoveryCalls++
			return 0, nil
		},
		Now:                       func() time.Time { return now },
		CompletedBackfillInterval: time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runner.runCycle(context.Background())
	now = now.Add(5 * time.Second)
	runner.runCycle(context.Background())
	now = now.Add(55 * time.Second)
	runner.runCycle(context.Background())

	if got := len(backfiller.limits); got != 2 {
		t.Fatalf("backfill calls = %d, want 2", got)
	}
	if recoveryCalls != 3 {
		t.Fatalf("recovery calls = %d, want 3", recoveryCalls)
	}
}

func TestRunnerRetriesIncompleteAndFailedBackfillsOnNextCycle(t *testing.T) {
	t.Parallel()

	deferredErr := errors.New("temporarily unavailable")
	backfiller := &recordingBackfiller{results: []backfillResult{
		{migrated: 3, complete: false},
		{migrated: 2, err: deferredErr},
		{complete: true},
	}}
	var logs []string
	runner, err := New(Config{
		Service:    "FoundPet",
		Backfiller: backfiller,
		Recover:    func(context.Context) (int, error) { return 0, nil },
		Logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runner.runCycle(context.Background())
	runner.runCycle(context.Background())
	runner.runCycle(context.Background())

	if got := len(backfiller.limits); got != 3 {
		t.Fatalf("backfill calls = %d, want 3", got)
	}
	wantLogs := []string{
		"FoundPet legacy outbox index backfill migrated 3 records (complete=false)",
		"FoundPet legacy outbox index backfill deferred: temporarily unavailable",
		"FoundPet legacy outbox index backfill migrated 2 records (complete=false)",
	}
	if !reflect.DeepEqual(logs, wantLogs) {
		t.Fatalf("logs = %v, want %v", logs, wantLogs)
	}
}

func TestRunnerSuppressesCancellationErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backfiller := &recordingBackfiller{results: []backfillResult{{complete: true, err: context.Canceled}}}
	var logs []string
	runner, err := New(Config{
		Service:    "LostPet",
		Backfiller: backfiller,
		Recover:    func(context.Context) (int, error) { return 0, context.Canceled },
		Logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runner.runCycle(ctx)

	if len(logs) != 0 {
		t.Fatalf("logs = %v, want no cancellation logs", logs)
	}
}

func TestRunnerLogsRecoveryFailure(t *testing.T) {
	t.Parallel()

	var logs []string
	runner, err := New(Config{
		Service: "LostPet",
		Recover: func(context.Context) (int, error) {
			return 0, errors.New("publisher unavailable")
		},
		Logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runner.runCycle(context.Background())

	want := []string{"LostPet outbox recovery deferred: publisher unavailable"}
	if !reflect.DeepEqual(logs, want) {
		t.Fatalf("logs = %v, want %v", logs, want)
	}
}

func TestNewAppliesProductionCadenceDefaults(t *testing.T) {
	t.Parallel()

	runner, err := New(Config{
		Service: "LostPet",
		Recover: func(context.Context) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if runner.recoveryInterval != 5*time.Second {
		t.Fatalf("recovery interval = %s, want 5s", runner.recoveryInterval)
	}
	if runner.completedBackfillInterval != time.Minute {
		t.Fatalf("completed backfill interval = %s, want 1m", runner.completedBackfillInterval)
	}
}

func TestNewRejectsMissingRequiredConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config Config
	}{
		{name: "service", config: Config{Recover: func(context.Context) (int, error) { return 0, nil }}},
		{name: "recovery callback", config: Config{Service: "LostPet"}},
		{
			name: "negative recovery interval",
			config: Config{
				Service:          "LostPet",
				Recover:          func(context.Context) (int, error) { return 0, nil },
				RecoveryInterval: -time.Second,
			},
		},
		{
			name: "negative completed backfill interval",
			config: Config{
				Service:                   "LostPet",
				Recover:                   func(context.Context) (int, error) { return 0, nil },
				CompletedBackfillInterval: -time.Second,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(test.config); err == nil {
				t.Fatal("New() error = nil, want configuration error")
			}
		})
	}
}
