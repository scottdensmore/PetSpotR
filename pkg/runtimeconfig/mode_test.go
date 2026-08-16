package runtimeconfig

import (
	"strings"
	"testing"
)

func TestResolveRuntimeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		env          map[string]string
		wantMode     Mode
		wantCloudRun bool
		wantErr      string
	}{
		{name: "defaults to memory outside Cloud Run", wantMode: ModeMemory},
		{
			name: "defaults to GCP on Cloud Run", env: map[string]string{"K_SERVICE": "lostpet-service"},
			wantMode: ModeGCP, wantCloudRun: true,
		},
		{
			name: "trims explicit mode and Cloud Run service", env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "  gcp  ", "K_SERVICE": "  lostpet-service  ",
			},
			wantMode: ModeGCP, wantCloudRun: true,
		},
		{
			name: "rejects memory on Cloud Run", env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "memory", "K_SERVICE": "lostpet-service",
			},
			wantMode: ModeMemory, wantCloudRun: true,
			wantErr: `runtime mode "memory" is not allowed on Cloud Run`,
		},
		{
			name: "rejects emulator on Cloud Run", env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "local-emulator", "K_SERVICE": "lostpet-service",
			},
			wantMode: ModeLocalEmulator, wantCloudRun: true,
			wantErr: `runtime mode "local-emulator" is not allowed on Cloud Run`,
		},
		{
			name: "rejects unknown mode", env: map[string]string{"PETSPOTR_RUNTIME_MODE": "automatic"},
			wantMode: "automatic", wantErr: `unsupported PETSPOTR_RUNTIME_MODE "automatic"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mode, cloudRun, err := resolveRuntimeMode(mapLookup(tt.env))
			if mode != tt.wantMode || cloudRun != tt.wantCloudRun {
				t.Fatalf("resolveRuntimeMode() = %q, %t; want %q, %t", mode, cloudRun, tt.wantMode, tt.wantCloudRun)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("resolveRuntimeMode() error = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("resolveRuntimeMode() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestConfigurationLoadersShareRuntimeModePolicy(t *testing.T) {
	t.Parallel()

	type loader struct {
		name string
		load func(map[string]string) (Mode, error)
	}
	loaders := []loader{
		{
			name: "state",
			load: func(env map[string]string) (Mode, error) {
				env = withAdapterConfig(env, "state")
				config, err := LoadStateConfig(mapLookup(env))
				return config.Mode, err
			},
		},
		{
			name: "messaging",
			load: func(env map[string]string) (Mode, error) {
				env = withAdapterConfig(env, "messaging")
				config, err := LoadMessagingConfig(mapLookup(env))
				return config.Mode, err
			},
		},
		{
			name: "storage",
			load: func(env map[string]string) (Mode, error) {
				env = withAdapterConfig(env, "storage")
				config, err := LoadStorageConfig(mapLookup(env))
				return config.Mode, err
			},
		},
		{
			name: "push",
			load: func(env map[string]string) (Mode, error) {
				env = withAdapterConfig(env, "push")
				config, err := LoadPushConsumerConfig(mapLookup(env))
				return config.Mode, err
			},
		},
	}

	policies := []struct {
		name     string
		env      map[string]string
		wantMode Mode
		wantErr  string
	}{
		{name: "default memory", wantMode: ModeMemory},
		{name: "explicit memory", env: map[string]string{"PETSPOTR_RUNTIME_MODE": "memory"}, wantMode: ModeMemory},
		{
			name: "explicit emulator", env: map[string]string{"PETSPOTR_RUNTIME_MODE": "local-emulator"},
			wantMode: ModeLocalEmulator,
		},
		{name: "explicit GCP", env: map[string]string{"PETSPOTR_RUNTIME_MODE": "gcp"}, wantMode: ModeGCP},
		{name: "Cloud Run default", env: map[string]string{"K_SERVICE": "service"}, wantMode: ModeGCP},
		{
			name: "Cloud Run rejects memory", env: map[string]string{
				"K_SERVICE": "service", "PETSPOTR_RUNTIME_MODE": "memory",
			},
			wantErr: `runtime mode "memory" is not allowed on Cloud Run`,
		},
		{
			name: "Cloud Run rejects emulator", env: map[string]string{
				"K_SERVICE": "service", "PETSPOTR_RUNTIME_MODE": "local-emulator",
			},
			wantErr: `runtime mode "local-emulator" is not allowed on Cloud Run`,
		},
		{
			name: "rejects unknown mode", env: map[string]string{"PETSPOTR_RUNTIME_MODE": "automatic"},
			wantErr: `unsupported PETSPOTR_RUNTIME_MODE "automatic"`,
		},
	}

	for _, policy := range policies {
		policy := policy
		t.Run(policy.name, func(t *testing.T) {
			t.Parallel()
			for _, loader := range loaders {
				loader := loader
				t.Run(loader.name, func(t *testing.T) {
					t.Parallel()

					mode, err := loader.load(policy.env)
					if policy.wantErr == "" {
						if err != nil {
							t.Fatalf("load() error = %v, want nil", err)
						}
						if mode != policy.wantMode {
							t.Fatalf("load() mode = %q, want %q", mode, policy.wantMode)
						}
						return
					}
					if err == nil || err.Error() != policy.wantErr {
						t.Fatalf("load() error = %v, want %q", err, policy.wantErr)
					}
				})
			}
		})
	}
}

func TestLoadPushConsumerConfigPreservesSubscriptionValidationPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing subscription precedes unsupported mode",
			env:     map[string]string{"PETSPOTR_RUNTIME_MODE": "automatic"},
			wantErr: `PUBSUB_PUSH_SUBSCRIPTION is required in "automatic" mode`,
		},
		{
			name: "missing subscription precedes Cloud Run prohibition",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE": "memory", "K_SERVICE": "pet-matcher",
			},
			wantErr: `PUBSUB_PUSH_SUBSCRIPTION is required in "memory" mode`,
		},
		{
			name: "conflicting subscriptions precede unsupported mode",
			env: map[string]string{
				"PETSPOTR_RUNTIME_MODE":     "automatic",
				"PUBSUB_PUSH_SUBSCRIPTION":  "projects/test/subscriptions/generic",
				"PUBSUB_FOUND_SUBSCRIPTION": "projects/test/subscriptions/legacy",
			},
			wantErr: "PUBSUB_PUSH_SUBSCRIPTION and PUBSUB_FOUND_SUBSCRIPTION must match when both are set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := LoadPushConsumerConfig(mapLookup(tt.env))
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("LoadPushConsumerConfig() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func mapLookup(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func withAdapterConfig(env map[string]string, adapter string) map[string]string {
	configured := make(map[string]string, len(env)+3)
	for key, value := range env {
		configured[key] = value
	}

	mode := Mode(strings.TrimSpace(configured["PETSPOTR_RUNTIME_MODE"]))
	if mode == "" {
		if strings.TrimSpace(configured["K_SERVICE"]) != "" {
			mode = ModeGCP
		} else {
			mode = ModeMemory
		}
	}
	switch adapter {
	case "state":
		if mode == ModeLocalEmulator || (mode == ModeGCP && strings.TrimSpace(configured["K_SERVICE"]) == "") {
			configured["GOOGLE_CLOUD_PROJECT"] = "petspotr-test"
		}
		if mode == ModeLocalEmulator {
			configured["FIRESTORE_EMULATOR_HOST"] = "127.0.0.1:8085"
		}
	case "messaging":
		if mode == ModeLocalEmulator || (mode == ModeGCP && strings.TrimSpace(configured["K_SERVICE"]) == "") {
			configured["GOOGLE_CLOUD_PROJECT"] = "petspotr-test"
		}
		if mode == ModeLocalEmulator {
			configured["PUBSUB_EMULATOR_HOST"] = "127.0.0.1:8086"
		}
	case "storage":
		configured["PETSPOTR_IMAGE_BUCKET"] = "petspotr-test-pet-images"
		if mode == ModeLocalEmulator {
			configured["STORAGE_EMULATOR_HOST"] = "http://127.0.0.1:4443"
		}
	case "push":
		configured["PUBSUB_PUSH_SUBSCRIPTION"] = "projects/petspotr-test/subscriptions/events"
		switch mode {
		case ModeGCP:
			configured["PUBSUB_PUSH_SERVICE_ACCOUNT"] = "invoker@petspotr-test.iam.gserviceaccount.com"
		case ModeMemory, ModeLocalEmulator:
			configured["PUBSUB_PUSH_DEV_TOKEN"] = "local-token"
		}
	}
	return configured
}
