package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
)

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

// StateStore defines key-value state store persistence operations.
type StateStore interface {
	SaveState(ctx context.Context, storeName, key string, data []byte) error
	CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error)
	GetState(ctx context.Context, storeName, key string) ([]byte, error)
	DeleteState(ctx context.Context, storeName, key string) error
	ListState(ctx context.Context, storeName string) (map[string][]byte, error)
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
