package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrNotFound indicates the requested blob image was not found.
var ErrNotFound = errors.New("blob: image not found")

// PresignedURLResponse encapsulates direct-to-cloud upload pre-signed parameters.
type PresignedURLResponse struct {
	UploadURL string    `json:"uploadUrl"`
	PublicURL string    `json:"publicUrl"`
	FileName  string    `json:"fileName"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// BlobStore defines image/blob storage operations.
type BlobStore interface {
	UploadImage(ctx context.Context, fileName string, data []byte) (string, error)
	GetImage(ctx context.Context, fileName string) ([]byte, error)
	GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string, expiry time.Duration) (*PresignedURLResponse, error)
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

// GeneratePresignedUploadURL constructs a pre-signed HTTP PUT upload URL for direct cloud storage uploads.
func (m *MemoryBlobStore) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string, expiry time.Duration) (*PresignedURLResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	cleanFileName := strings.TrimSpace(fileName)
	if cleanFileName == "" {
		cleanFileName = fmt.Sprintf("upload-%d.jpg", time.Now().UnixNano())
	}

	expiresAt := time.Now().UTC().Add(expiry)
	uploadURL := fmt.Sprintf("%s/upload/presigned/%s?expires=%d&sig=mock-sig", m.baseURL, cleanFileName, expiresAt.Unix())
	publicURL := fmt.Sprintf("%s/%s", m.baseURL, cleanFileName)

	return &PresignedURLResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		FileName:  cleanFileName,
		ExpiresAt: expiresAt,
	}, nil
}
