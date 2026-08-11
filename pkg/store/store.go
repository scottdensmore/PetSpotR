package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
)

type outboxIndexRecord struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Sentinel errors for StateStore operations.
var (
	ErrStoreNotFound = errors.New("store: state store not found")
	ErrNotFound      = errors.New("store: key not found")
	ErrConflict      = errors.New("store: conflicting aggregate state")
)

// StateWrite describes one state document written as part of an atomic unit.
type StateWrite struct {
	StoreName string
	Key       string
	Data      []byte
}

// StateUpdater computes the next value from an isolated copy of current state.
// Managed stores may invoke it more than once when a transaction is retried.
type StateUpdater func(current []byte) (next []byte, err error)

// StateStore defines key-value state store persistence operations.
type StateStore interface {
	SaveState(ctx context.Context, storeName, key string, data []byte) error
	UpdateState(ctx context.Context, storeName, key string, update StateUpdater) error
	CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error)
	GetState(ctx context.Context, storeName, key string) ([]byte, error)
	DeleteState(ctx context.Context, storeName, key string) error
	ListState(ctx context.Context, storeName string) (map[string][]byte, error)
	ListPendingOutbox(ctx context.Context, topic string, limit int) ([]string, error)
}

// DeliveryOperationStore provides transactional leases and fenced results for
// side effects performed by at-least-once event consumers.
type DeliveryOperationStore interface {
	ClaimDeliveryOperation(ctx context.Context, operation delivery.Operation, now time.Time, leaseDuration time.Duration) (delivery.Claim, error)
	CompleteDeliveryOperation(ctx context.Context, id string, attempt int, completedAt time.Time) error
	FailDeliveryOperation(ctx context.Context, id string, attempt int, failedAt time.Time, failure string) error
	GetDeliveryOperation(ctx context.Context, id string) (delivery.Operation, error)
}

// OutboxIndexBackfiller upgrades legacy Firestore outbox documents in bounded
// batches before indexed recovery. Memory stores do not need this migration.
type OutboxIndexBackfiller interface {
	BackfillOutboxIndexes(ctx context.Context, limit int) (migrated int, complete bool, err error)
}

// CreateStateAndOutbox atomically creates aggregate state and its outbox
// record. An exact retry is a successful no-op; a competing aggregate create
// returns ErrConflict.
func (m *MemoryStore) CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateAtomicWrites(state, outbox); err != nil {
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stateData, stateExists := m.items[state.StoreName][state.Key]
	_, outboxExists := m.items[outbox.StoreName][outbox.Key]
	if stateExists || outboxExists {
		if stateExists && outboxExists && bytes.Equal(stateData, state.Data) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %s/%s", ErrConflict, state.StoreName, state.Key)
	}

	if _, exists := m.items[state.StoreName]; !exists {
		m.items[state.StoreName] = make(map[string][]byte)
	}
	if _, exists := m.items[outbox.StoreName]; !exists {
		m.items[outbox.StoreName] = make(map[string][]byte)
	}
	m.items[state.StoreName][state.Key] = bytes.Clone(state.Data)
	m.items[outbox.StoreName][outbox.Key] = bytes.Clone(outbox.Data)
	return true, nil
}

// MemoryStore implements StateStore in memory for testing and local dev.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]map[string][]byte
}

// NewMemoryStore constructs a thread-safe MemoryStore instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]map[string][]byte),
	}
}

// SaveState saves data under storeName and key.
func (m *MemoryStore) SaveState(ctx context.Context, storeName, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.items[storeName]; !exists {
		m.items[storeName] = make(map[string][]byte)
	}
	m.items[storeName][key] = bytes.Clone(data)
	return nil
}

// UpdateState atomically replaces one existing value using the caller's
// retry-safe update function.
func (m *MemoryStore) UpdateState(ctx context.Context, storeName, key string, update StateUpdater) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if update == nil {
		return errors.New("store: state updater is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	storeMap, exists := m.items[storeName]
	if !exists {
		return fmt.Errorf("%w: %s", ErrStoreNotFound, storeName)
	}
	current, exists := storeMap[key]
	if !exists {
		return fmt.Errorf("%w: %s in store %s", ErrNotFound, key, storeName)
	}
	next, err := update(bytes.Clone(current))
	if err != nil {
		return err
	}
	storeMap[key] = bytes.Clone(next)
	return nil
}

// GetState retrieves data for storeName and key.
func (m *MemoryStore) GetState(ctx context.Context, storeName, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	storeMap, exists := m.items[storeName]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrStoreNotFound, storeName)
	}

	val, exists := storeMap[key]
	if !exists {
		return nil, fmt.Errorf("%w: %s in store %s", ErrNotFound, key, storeName)
	}

	return bytes.Clone(val), nil
}

// DeleteState removes a key from storeName.
func (m *MemoryStore) DeleteState(ctx context.Context, storeName, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if storeMap, exists := m.items[storeName]; exists {
		delete(storeMap, key)
	}
	return nil
}

// ListState returns all keys and byte clones for a storeName.
func (m *MemoryStore) ListState(ctx context.Context, storeName string) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string][]byte)
	storeMap, exists := m.items[storeName]
	if !exists {
		return res, nil
	}

	for k, v := range storeMap {
		res[k] = bytes.Clone(v)
	}
	return res, nil
}

// ListPendingOutbox returns a bounded, deterministic set of pending event IDs
// for one topic without including published history.
func (m *MemoryStore) ListPendingOutbox(ctx context.Context, topic string, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topic == "" || limit < 1 {
		return nil, errors.New("store: pending outbox topic and positive limit are required")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	records := make([]outboxIndexRecord, 0)
	var decodeErrors []error
	for key, data := range m.items[OutboxCollection] {
		var record outboxIndexRecord
		if err := json.Unmarshal(data, &record); err != nil {
			decodeErrors = append(decodeErrors, fmt.Errorf("store: decode outbox %s: %w", key, err))
			continue
		}
		if record.ID != key {
			decodeErrors = append(decodeErrors, fmt.Errorf("store: outbox ID %q does not match key %q", record.ID, key))
			continue
		}
		if record.Topic == topic && record.Status == "pending" {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID < records[j].ID
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	if len(records) > limit {
		records = records[:limit]
	}
	ids := make([]string, len(records))
	for index := range records {
		ids[index] = records[index].ID
	}
	return ids, errors.Join(decodeErrors...)
}

func validateAtomicWrites(state, outbox StateWrite) error {
	for _, write := range []StateWrite{state, outbox} {
		if write.StoreName == "" || write.Key == "" {
			return errors.New("store: atomic write store name and key are required")
		}
	}
	if outbox.StoreName != OutboxCollection {
		return fmt.Errorf("store: outbox write must target %s", OutboxCollection)
	}
	return nil
}
