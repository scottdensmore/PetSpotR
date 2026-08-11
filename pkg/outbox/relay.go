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
	MaxPublishBatch = 100
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

// Relay publishes pending outbox records and marks successful deliveries.
// Its process-local lock avoids duplicate publication by concurrent flushes;
// managed multi-instance claiming belongs with the Pub/Sub consumer slice.
type Relay struct {
	store  store.StateStore
	broker pubsub.Publisher
	mu     sync.Mutex
	now    func() time.Time
}

// NewRelay constructs an outbox relay.
func NewRelay(stateStore store.StateStore, broker pubsub.Publisher) *Relay {
	return &Relay{store: stateStore, broker: broker, now: time.Now}
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
		record, err := GetRecord(ctx, r.store, id)
		if err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}
		if record.Status == StatusPublished {
			continue
		}
		attemptedAt := r.now().UTC()
		record.Attempts++
		record.LastAttemptAt = &attemptedAt
		if err := r.broker.Publish(ctx, record.Topic, append([]byte(nil), record.Payload...)); err != nil {
			record.LastError = err.Error()
			if saveErr := SaveRecord(ctx, r.store, record); saveErr != nil {
				publishErrors = append(publishErrors, errors.Join(err, saveErr))
			} else {
				publishErrors = append(publishErrors, err)
			}
			continue
		}
		record.Status = StatusPublished
		record.PublishedAt = &attemptedAt
		record.LastError = ""
		if err := SaveRecord(ctx, r.store, record); err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}
		published++
	}
	return published, errors.Join(publishErrors...)
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
