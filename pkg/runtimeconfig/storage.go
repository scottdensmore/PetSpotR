package runtimeconfig

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/scottdensmore/petspotr/pkg/blob"
)

// StorageConfig contains the configuration for private pet images.
type StorageConfig struct {
	Mode                Mode
	BucketName          string
	StorageEmulatorHost string
	MemoryBaseURL       string
}

// StorageRuntime owns the selected image store and its client lifecycle.
type StorageRuntime struct {
	Images    blob.ImageStore
	close     func() error
	closeOnce sync.Once
	closeErr  error
}

// LoadStorageConfigFromEnv loads image-storage configuration from the process.
func LoadStorageConfigFromEnv() (StorageConfig, error) {
	return LoadStorageConfig(os.Getenv)
}

// LoadStorageConfig loads and validates image-storage configuration.
func LoadStorageConfig(lookup func(string) string) (StorageConfig, error) {
	mode, _, err := resolveComponentMode(lookup, "PETSPOTR_STORAGE_MODE")
	if err != nil {
		return StorageConfig{}, err
	}
	config := StorageConfig{
		Mode:                mode,
		BucketName:          strings.TrimSpace(lookup("PETSPOTR_IMAGE_BUCKET")),
		StorageEmulatorHost: strings.TrimSpace(lookup("STORAGE_EMULATOR_HOST")),
		MemoryBaseURL:       strings.TrimSpace(lookup("PETSPOTR_IMAGE_BASE_URL")),
	}
	switch config.Mode {
	case ModeMemory:
		return StorageConfig{Mode: ModeMemory, MemoryBaseURL: config.MemoryBaseURL}, nil
	case ModeLocalEmulator:
		if config.BucketName == "" || config.StorageEmulatorHost == "" {
			return StorageConfig{}, fmt.Errorf("PETSPOTR_IMAGE_BUCKET and STORAGE_EMULATOR_HOST are required in %q mode", config.Mode)
		}
	case ModeGCP:
		if config.BucketName == "" {
			return StorageConfig{}, fmt.Errorf("PETSPOTR_IMAGE_BUCKET is required in %q mode", config.Mode)
		}
		if config.StorageEmulatorHost != "" {
			return StorageConfig{}, fmt.Errorf("STORAGE_EMULATOR_HOST must not be set in %q mode", config.Mode)
		}
	}
	return config, nil
}

// NewStorageRuntime selects memory, emulator, or managed private image storage.
func NewStorageRuntime(ctx context.Context, config StorageConfig) (*StorageRuntime, error) {
	if err := validateStorageConfig(config); err != nil {
		return nil, err
	}
	switch config.Mode {
	case ModeMemory:
		baseURL := strings.TrimSpace(config.MemoryBaseURL)
		if baseURL == "" {
			baseURL = "http://localhost/images"
		}
		return &StorageRuntime{
			Images: blob.NewMemoryBlobStore(baseURL), close: func() error { return nil },
		}, nil
	case ModeLocalEmulator:
		privateKey, err := localSigningKey()
		if err != nil {
			return nil, err
		}
		images, err := blob.NewGCSImageStore(ctx, blob.GCSConfig{
			BucketName: config.BucketName, Endpoint: config.StorageEmulatorHost,
			GoogleAccessID: "local-emulator@petspotr.invalid", PrivateKey: privateKey,
		})
		if err != nil {
			return nil, err
		}
		return &StorageRuntime{Images: images, close: images.Close}, nil
	case ModeGCP:
		images, err := blob.NewGCSImageStore(ctx, blob.GCSConfig{BucketName: config.BucketName})
		if err != nil {
			return nil, err
		}
		return &StorageRuntime{Images: images, close: images.Close}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime mode %q", config.Mode)
	}
}

// Close releases the image-store client once.
func (r *StorageRuntime) Close() error {
	r.closeOnce.Do(func() { r.closeErr = r.close() })
	return r.closeErr
}

func validateStorageConfig(config StorageConfig) error {
	switch config.Mode {
	case ModeMemory:
		return nil
	case ModeLocalEmulator:
		if config.BucketName == "" || config.StorageEmulatorHost == "" {
			return fmt.Errorf("PETSPOTR_IMAGE_BUCKET and STORAGE_EMULATOR_HOST are required in %q mode", config.Mode)
		}
		if !strings.HasPrefix(config.StorageEmulatorHost, "http://") &&
			!strings.HasPrefix(config.StorageEmulatorHost, "https://") {
			return fmt.Errorf("STORAGE_EMULATOR_HOST must be an HTTP(S) URL in %q mode", config.Mode)
		}
		return nil
	case ModeGCP:
		if config.BucketName == "" {
			return fmt.Errorf("PETSPOTR_IMAGE_BUCKET is required in %q mode", config.Mode)
		}
		if config.StorageEmulatorHost != "" {
			return fmt.Errorf("STORAGE_EMULATOR_HOST must not be set in %q mode", config.Mode)
		}
		return nil
	default:
		return fmt.Errorf("unsupported runtime mode %q", config.Mode)
	}
}

func localSigningKey() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate local image signing key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), nil
}
