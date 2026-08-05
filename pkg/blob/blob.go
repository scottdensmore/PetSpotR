package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrNotFound indicates the requested blob image was not found.
var ErrNotFound = errors.New("blob: image not found")

// BlobStore defines image/blob storage operations.
type BlobStore interface {
	UploadImage(ctx context.Context, fileName string, data []byte) (string, error)
	GetImage(ctx context.Context, fileName string) ([]byte, error)
}

// MemoryBlobStore implements BlobStore in memory for testing and local dev.
type MemoryBlobStore struct {
	mu      sync.RWMutex
	baseURL string
	blobs   map[string][]byte
}

// NewMemoryBlobStore constructs a MemoryBlobStore with a base URL prefix.
func NewMemoryBlobStore(baseURL string) *MemoryBlobStore {
	return &MemoryBlobStore{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		blobs:   make(map[string][]byte),
	}
}

// UploadImage stores image bytes in memory and returns its public URL.
func (m *MemoryBlobStore) UploadImage(ctx context.Context, fileName string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.blobs[fileName] = bytes.Clone(data)
	return fmt.Sprintf("%s/%s", m.baseURL, fileName), nil
}

// GetImage retrieves image bytes by fileName.
func (m *MemoryBlobStore) GetImage(ctx context.Context, fileName string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.blobs[fileName]
	if !exists {
		return nil, ErrNotFound
	}
	return bytes.Clone(data), nil
}
