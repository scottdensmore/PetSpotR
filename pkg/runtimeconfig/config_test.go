package runtimeconfig_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestLoadStateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    runtimeconfig.StateConfig
		wantErr bool
	}{
		{
			name: "defaults to memory outside Cloud Run",
			want: runtimeconfig.StateConfig{Mode: runtimeconfig.ModeMemory},
		},
		{
			name: "accepts explicit memory mode outside Cloud Run",
			env:  map[string]string{"PETSPOTR_RUNTIME_MODE": "memory"},
			want: runtimeconfig.StateConfig{Mode: runtimeconfig.ModeMemory},
		},
		{
			name: "selects GCP mode with ADC project detection on Cloud Run",
			env:  map[string]string{"K_SERVICE": "lostpet-service"},
			want: runtimeconfig.StateConfig{
				Mode:            runtimeconfig.ModeGCP,
				DetectProjectID: true,
			},
		},
		{
			name: "rejects explicit memory mode on Cloud Run",
			env: map[string]string{
				"K_SERVICE":             "lostpet-service",
				"PETSPOTR_RUNTIME_MODE": "memory",
			},
			wantErr: true,
		},
		{
			name: "loads emulator mode",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE":   "local-emulator",
				"GOOGLE_CLOUD_PROJECT":    "petspotr-local",
				"FIRESTORE_EMULATOR_HOST": "127.0.0.1:8085",
			},
			want: runtimeconfig.StateConfig{
				Mode:                  runtimeconfig.ModeLocalEmulator,
				ProjectID:             "petspotr-local",
				FirestoreEmulatorHost: "127.0.0.1:8085",
			},
		},
		{
			name: "rejects emulator mode on Cloud Run",
			env: map[string]string{
				"K_SERVICE":               "lostpet-service",
				"PETSPOTR_RUNTIME_MODE":   "local-emulator",
				"GOOGLE_CLOUD_PROJECT":    "petspotr-local",
				"FIRESTORE_EMULATOR_HOST": "127.0.0.1:8085",
			},
			wantErr: true,
		},
		{
			name: "requires emulator host in emulator mode",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-local",
			},
			wantErr: true,
		},
		{
			name: "loads GCP mode",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "gcp",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-production",
			},
			want: runtimeconfig.StateConfig{
				Mode:      runtimeconfig.ModeGCP,
				ProjectID: "petspotr-production",
			},
		},
		{
			name: "allows GCP project detection on Cloud Run",
			env: map[string]string{
				"K_SERVICE":             "lostpet-service",
				"PETSPOTR_RUNTIME_MODE": "gcp",
			},
			want: runtimeconfig.StateConfig{
				Mode:            runtimeconfig.ModeGCP,
				DetectProjectID: true,
			},
		},
		{
			name: "rejects emulator host in GCP mode",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE":   "gcp",
				"GOOGLE_CLOUD_PROJECT":    "petspotr-production",
				"FIRESTORE_EMULATOR_HOST": "127.0.0.1:8085",
			},
			wantErr: true,
		},
		{
			name:    "rejects unknown mode",
			env:     map[string]string{"PETSPOTR_RUNTIME_MODE": "automatic"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := runtimeconfig.LoadStateConfig(func(key string) string {
				return tt.env[key]
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadStateConfig() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadStateConfig() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("LoadStateConfig() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewStateRuntimeUsesMemoryOnlyInMemoryMode(t *testing.T) {
	t.Parallel()

	runtime, err := runtimeconfig.NewStateRuntime(context.Background(), runtimeconfig.StateConfig{
		Mode: runtimeconfig.ModeMemory,
	})
	if err != nil {
		t.Fatalf("NewStateRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	if _, ok := runtime.Store.(*store.MemoryStore); !ok {
		t.Fatalf("NewStateRuntime().Store = %T, want *store.MemoryStore", runtime.Store)
	}
}

func TestNewStateRuntimeRejectsInvalidManagedConfigBeforeConnecting(t *testing.T) {
	t.Parallel()

	tests := []runtimeconfig.StateConfig{
		{Mode: "automatic"},
		{Mode: runtimeconfig.ModeLocalEmulator},
		{Mode: runtimeconfig.ModeGCP},
		{
			Mode:                  runtimeconfig.ModeGCP,
			ProjectID:             "petspotr-production",
			FirestoreEmulatorHost: "127.0.0.1:8085",
		},
	}

	for _, config := range tests {
		config := config
		t.Run(string(config.Mode), func(t *testing.T) {
			t.Parallel()
			if _, err := runtimeconfig.NewStateRuntime(context.Background(), config); err == nil {
				t.Fatalf("NewStateRuntime(%#v) error = nil, want non-nil", config)
			}
		})
	}
}

func TestStateRuntimeSharesAndRetainsStateWithFirestoreEmulator(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := runtimeconfig.StateConfig{
		Mode:                  runtimeconfig.ModeLocalEmulator,
		ProjectID:             "petspotr-runtime-test",
		FirestoreEmulatorHost: host,
	}
	writer, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewStateRuntime(writer) error = %v", err)
	}

	reader, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		_ = writer.Close()
		t.Fatalf("NewStateRuntime(reader) error = %v", err)
	}

	key := "https://push.example.test/send/runtime-contract-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	payload := []byte(`{"petId":"shared-101"}`)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = reader.Store.DeleteState(cleanupCtx, store.LostPetsCollection, key)
		_ = reader.Close()
	})

	if err := writer.Store.SaveState(ctx, store.LostPetsCollection, key, payload); err != nil {
		_ = writer.Close()
		t.Fatalf("writer SaveState() error = %v", err)
	}
	payload[0] = 'X'
	wantPayload := []byte(`{"petId":"shared-101"}`)

	got, err := reader.Store.GetState(ctx, store.LostPetsCollection, key)
	if err != nil {
		_ = writer.Close()
		t.Fatalf("reader GetState() error = %v", err)
	}
	if string(got) != string(wantPayload) {
		_ = writer.Close()
		t.Fatalf("reader GetState() = %s, want %s", got, wantPayload)
	}

	listed, err := reader.Store.ListState(ctx, store.LostPetsCollection)
	if err != nil {
		_ = writer.Close()
		t.Fatalf("reader ListState() error = %v", err)
	}
	if string(listed[key]) != string(wantPayload) {
		_ = writer.Close()
		t.Fatalf("reader ListState()[%q] = %s, want %s", key, listed[key], wantPayload)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer Close() error = %v", err)
	}

	restarted, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewStateRuntime(restarted) error = %v", err)
	}
	defer func() {
		if err := restarted.Close(); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("restarted Close() error = %v", err)
		}
	}()

	got, err = restarted.Store.GetState(ctx, store.LostPetsCollection, key)
	if err != nil {
		t.Fatalf("restarted GetState() error = %v", err)
	}
	if string(got) != string(wantPayload) {
		t.Fatalf("restarted GetState() = %s, want %s", got, wantPayload)
	}

	cancelledCtx, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	if _, err := restarted.Store.GetState(cancelledCtx, store.LostPetsCollection, key); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetState(cancelled context) error = %v, want context.Canceled", err)
	}

	if err := restarted.Store.DeleteState(ctx, store.LostPetsCollection, key); err != nil {
		t.Fatalf("DeleteState() error = %v", err)
	}
	if _, err := restarted.Store.GetState(ctx, store.LostPetsCollection, key); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetState(deleted key) error = %v, want store.ErrNotFound", err)
	}
}
