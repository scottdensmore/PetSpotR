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
)

// StateStore defines key-value state store persistence operations.
type StateStore interface {
	SaveState(ctx context.Context, storeName, key string, data []byte) error
	GetState(ctx context.Context, storeName, key string) ([]byte, error)
	DeleteState(ctx context.Context, storeName, key string) error
	ListState(ctx context.Context, storeName string) (map[string][]byte, error)
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
