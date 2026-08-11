package runtimeconfig_test

import (
	"context"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func TestLoadStorageConfigRejectsUnsafeModes(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name: "memory is forbidden on Cloud Run",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "memory", "K_SERVICE": "foundpet-service",
			},
			wantErr: "not allowed on Cloud Run",
		},
		{
			name: "emulator is forbidden on Cloud Run",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator", "K_SERVICE": "foundpet-service",
				"PETSPOTR_IMAGE_BUCKET": "bucket", "STORAGE_EMULATOR_HOST": "http://gcs:4443",
			},
			wantErr: "not allowed on Cloud Run",
		},
		{
			name: "managed mode requires a bucket",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "gcp", "GOOGLE_CLOUD_PROJECT": "project",
			},
			wantErr: "PETSPOTR_IMAGE_BUCKET is required",
		},
		{
			name: "managed mode rejects emulator host",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "gcp", "PETSPOTR_IMAGE_BUCKET": "bucket",
				"STORAGE_EMULATOR_HOST": "http://localhost:4443",
			},
			wantErr: "STORAGE_EMULATOR_HOST must not be set",
		},
		{
			name: "local emulator requires host and bucket",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator",
			},
			wantErr: "PETSPOTR_IMAGE_BUCKET and STORAGE_EMULATOR_HOST are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runtimeconfig.LoadStorageConfig(func(key string) string { return tt.env[key] })
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadStorageConfig() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewStorageRuntimeSelectsExplicitAdapter(t *testing.T) {
	memory, err := runtimeconfig.NewStorageRuntime(context.Background(), runtimeconfig.StorageConfig{
		Mode: runtimeconfig.ModeMemory, MemoryBaseURL: "http://localhost:8081/images",
	})
	if err != nil {
		t.Fatalf("NewStorageRuntime(memory) error = %v", err)
	}
	defer func() { _ = memory.Close() }()
	if _, ok := memory.Images.(*blob.MemoryBlobStore); !ok {
		t.Fatalf("memory Images = %T, want *blob.MemoryBlobStore", memory.Images)
	}

	emulator, err := runtimeconfig.NewStorageRuntime(context.Background(), runtimeconfig.StorageConfig{
		Mode: runtimeconfig.ModeLocalEmulator, BucketName: "petspotr-local-pet-images",
		StorageEmulatorHost: "http://127.0.0.1:4443",
	})
	if err != nil {
		t.Fatalf("NewStorageRuntime(local-emulator) error = %v", err)
	}
	defer func() { _ = emulator.Close() }()
	if _, ok := emulator.Images.(*blob.GCSImageStore); !ok {
		t.Fatalf("emulator Images = %T, want *blob.GCSImageStore", emulator.Images)
	}
	grant, err := emulator.Images.BeginImageUpload(context.Background(), blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("emulator BeginImageUpload() error = %v", err)
	}
	if !strings.HasPrefix(grant.UploadURL, "http://127.0.0.1:4443/") {
		t.Fatalf("emulator upload URL = %q, want configured local endpoint", grant.UploadURL)
	}
}
