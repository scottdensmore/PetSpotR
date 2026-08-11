package runtimeconfig_test

import (
	"context"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func TestLoadMessagingConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		env     map[string]string
		want    runtimeconfig.MessagingConfig
		wantErr bool
	}{
		{name: "memory default", want: runtimeconfig.MessagingConfig{Mode: runtimeconfig.ModeMemory}},
		{
			name: "local emulator",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-local",
				"PUBSUB_EMULATOR_HOST":  "127.0.0.1:8086",
			},
			want: runtimeconfig.MessagingConfig{Mode: runtimeconfig.ModeLocalEmulator, ProjectID: "petspotr-local", PubSubEmulatorHost: "127.0.0.1:8086"},
		},
		{
			name: "Cloud Run detects project",
			env:  map[string]string{"K_SERVICE": "foundpet-service"},
			want: runtimeconfig.MessagingConfig{Mode: runtimeconfig.ModeGCP, DetectProjectID: true},
		},
		{
			name: "Cloud Run rejects local emulator",
			env: map[string]string{
				"K_SERVICE":             "foundpet-service",
				"PETSPOTR_RUNTIME_MODE": "local-emulator",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-local",
				"PUBSUB_EMULATOR_HOST":  "127.0.0.1:8086",
			},
			wantErr: true,
		},
		{
			name: "rejects emulator host in GCP",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "gcp",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-prod",
				"PUBSUB_EMULATOR_HOST":  "127.0.0.1:8086",
			},
			wantErr: true,
		},
		{
			name: "requires emulator host",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator",
				"GOOGLE_CLOUD_PROJECT":  "petspotr-local",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := runtimeconfig.LoadMessagingConfig(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadMessagingConfig() error = nil, want non-nil")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("LoadMessagingConfig() = %#v, %v; want %#v, nil", got, err, tt.want)
			}
		})
	}
}

func TestNewMessagingRuntimeUsesMemoryOnlyInMemoryMode(t *testing.T) {
	runtime, err := runtimeconfig.NewMessagingRuntime(context.Background(), runtimeconfig.MessagingConfig{Mode: runtimeconfig.ModeMemory})
	if err != nil {
		t.Fatalf("NewMessagingRuntime() error = %v", err)
	}
	if _, ok := runtime.Publisher.(*pubsub.MemoryPubSub); !ok {
		t.Fatalf("Publisher = %T, want *pubsub.MemoryPubSub", runtime.Publisher)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
