// Package outbox provides durable event records and an at-least-once relay.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Status describes the delivery state of an outbox record.
type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	// MaxPublishBatch bounds the explicit IDs processed by one relay call.
	MaxPublishBatch     = 100
	defaultPublishLease = 10 * time.Minute
)

// Record is the durable representation of an event awaiting publication.
type Record struct {
	ID            string     `json:"id"`
	Topic         string     `json:"topic"`
	Payload       []byte     `json:"payload"`
	Status        Status     `json:"status"`
	Attempts      int        `json:"attempts"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	LeaseUntil    *time.Time `json:"leaseUntil,omitempty"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
}

// NewRecord creates a pending outbox record.
func NewRecord(id, topic string, payload []byte, createdAt time.Time) Record {
	return Record{ID: id, Topic: topic, Payload: append([]byte(nil), payload...), Status: StatusPending, CreatedAt: createdAt.UTC()}
}

// MarshalRecord validates and serializes an outbox record.
func MarshalRecord(record Record) ([]byte, error) {
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	return json.Marshal(record)
}

func validateRecord(record Record) error {
	if record.ID == "" || record.Topic == "" || len(record.Payload) == 0 || record.CreatedAt.IsZero() {
		return errors.New("outbox: record ID, topic, payload, and creation time are required")
	}
	if record.Status != StatusPending && record.Status != StatusPublished {
		return fmt.Errorf("outbox: invalid record status %q", record.Status)
	}
	if record.Status == StatusPublished && record.LeaseUntil != nil {
		return errors.New("outbox: published record cannot retain a lease")
	}
	if record.LeaseUntil != nil && (record.Attempts < 1 || record.LastAttemptAt == nil) {
		return errors.New("outbox: leased record requires an attempt time")
	}
	return nil
}

// SaveRecord persists a record. Aggregate creation should instead use the
// store's atomic CreateStateAndOutbox method.
func SaveRecord(ctx context.Context, stateStore store.StateStore, record Record) error {
	data, err := MarshalRecord(record)
	if err != nil {
		return err
	}
	return stateStore.SaveState(ctx, store.OutboxCollection, record.ID, data)
}

// GetRecord retrieves and decodes one outbox record.
func GetRecord(ctx context.Context, stateStore store.StateStore, id string) (Record, error) {
	data, err := stateStore.GetState(ctx, store.OutboxCollection, id)
	if err != nil {
		return Record{}, err
	}
	return decodeRecord(id, data)
}

// Relay leases pending outbox records before publication and marks successful
// deliveries. Its process-local lock serializes flushes in one process, while
// the store transaction arbitrates claims across service instances.
type Relay struct {
	store  store.StateStore
	broker pubsub.Publisher
	mu     sync.Mutex
	now    func() time.Time
	lease  time.Duration
}

// RelayOption customizes relay timing for deterministic crash recovery tests.
type RelayOption func(*Relay)

// WithClock sets the relay clock.
func WithClock(now func() time.Time) RelayOption {
	return func(relay *Relay) {
		if now != nil {
			relay.now = now
		}
	}
}

// WithPublishLease sets how long another instance must wait before reclaiming
// a publication whose owner stopped before recording completion.
func WithPublishLease(duration time.Duration) RelayOption {
	return func(relay *Relay) {
		if duration > 0 {
			relay.lease = duration
		}
	}
}

// NewRelay constructs an outbox relay.
func NewRelay(stateStore store.StateStore, broker pubsub.Publisher, options ...RelayOption) *Relay {
	relay := &Relay{store: stateStore, broker: broker, now: time.Now, lease: defaultPublishLease}
	for _, option := range options {
		option(relay)
	}
	return relay
}

// CanPublish reports whether a broker with local topic awareness can currently
// deliver the topic. Managed brokers without this capability are assumed able
// to attempt publication and return a delivery error if unavailable.
func (r *Relay) CanPublish(topic string) bool {
	availability, ok := r.broker.(pubsub.TopicAvailability)
	return !ok || availability.HasSubscribers(topic)
}

// PublishRecords attempts only the explicitly selected record IDs, bounded by
// MaxPublishBatch. Corrupt records are reported independently and do not stop
// valid records in the same batch. Failed records remain pending for retry.
func (r *Relay) PublishRecords(ctx context.Context, ids ...string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(ids) > MaxPublishBatch {
		return 0, fmt.Errorf("outbox: publish batch has %d records, maximum is %d", len(ids), MaxPublishBatch)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	published := 0
	var publishErrors []error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return published, errors.Join(append(publishErrors, err)...)
		}
		record, acquired, err := r.claimRecord(ctx, id)
		if err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}
		if !acquired {
			continue
		}
		if err := r.broker.Publish(ctx, record.Topic, append([]byte(nil), record.Payload...)); err != nil {
			publishErrors = append(publishErrors, errors.Join(err, r.failRecord(ctx, record.ID, record.Attempts, err)))
			continue
		}
		if err := r.completeRecord(ctx, record.ID, record.Attempts); err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}
		published++
	}
	return published, errors.Join(publishErrors...)
}

func (r *Relay) claimRecord(ctx context.Context, id string) (Record, bool, error) {
	now := r.now().UTC()
	var claimed Record
	acquired := false
	err := r.store.UpdateState(ctx, store.OutboxCollection, id, func(current []byte) ([]byte, error) {
		acquired = false
		record, err := decodeRecord(id, current)
		if err != nil {
			return nil, err
		}
		claimed = record
		if record.Status == StatusPublished {
			return current, nil
		}
		if record.LeaseUntil != nil && record.LeaseUntil.After(now) {
			return current, nil
		}
		leaseUntil := now.Add(r.lease)
		record.Attempts++
		record.LastAttemptAt = &now
		record.LeaseUntil = &leaseUntil
		record.LastError = ""
		data, err := MarshalRecord(record)
		if err != nil {
			return nil, err
		}
		claimed = record
		acquired = true
		return data, nil
	})
	if err != nil {
		return Record{}, false, err
	}
	return claimed, acquired, nil
}

func (r *Relay) failRecord(ctx context.Context, id string, attempt int, failure error) error {
	now := r.now().UTC()
	return r.store.UpdateState(ctx, store.OutboxCollection, id, func(current []byte) ([]byte, error) {
		record, err := decodeRecord(id, current)
		if err != nil {
			return nil, err
		}
		if record.Status == StatusPublished {
			return current, nil
		}
		if record.Attempts != attempt {
			return nil, fmt.Errorf("%w: outbox record %s attempt %d", store.ErrConflict, id, attempt)
		}
		record.LeaseUntil = nil
		record.LastAttemptAt = &now
		record.LastError = failure.Error()
		return MarshalRecord(record)
	})
}

func (r *Relay) completeRecord(ctx context.Context, id string, attempt int) error {
	now := r.now().UTC()
	return r.store.UpdateState(ctx, store.OutboxCollection, id, func(current []byte) ([]byte, error) {
		record, err := decodeRecord(id, current)
		if err != nil {
			return nil, err
		}
		if record.Status == StatusPublished {
			return current, nil
		}
		if record.Attempts != attempt {
			return nil, fmt.Errorf("%w: outbox record %s attempt %d", store.ErrConflict, id, attempt)
		}
		record.Status = StatusPublished
		record.LeaseUntil = nil
		record.PublishedAt = &now
		record.LastError = ""
		return MarshalRecord(record)
	})
}

func decodeRecord(id string, data []byte) (Record, error) {
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("outbox: decode record %s: %w", id, err)
	}
	if err := validateRecord(record); err != nil {
		return Record{}, fmt.Errorf("outbox: validate record %s: %w", id, err)
	}
	if record.ID != id {
		return Record{}, fmt.Errorf("outbox: record ID %q does not match storage key %q", record.ID, id)
	}
	return record, nil
}

// PublishPending publishes one bounded topic-specific recovery batch.
func (r *Relay) PublishPending(ctx context.Context, topic string) (int, error) {
	if !r.CanPublish(topic) {
		return 0, nil
	}
	ids, listErr := r.store.ListPendingOutbox(ctx, topic, MaxPublishBatch)
	published, publishErr := r.PublishRecords(ctx, ids...)
	return published, errors.Join(listErr, publishErr)
}
