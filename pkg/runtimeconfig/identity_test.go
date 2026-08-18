package runtimeconfig_test

import (
	"context"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func TestLoadIdentityConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    runtimeconfig.IdentityConfig
		wantErr string
	}{
		{
			name: "defaults to disabled outside Cloud Run",
			want: runtimeconfig.IdentityConfig{Mode: runtimeconfig.IdentityModeDisabled},
		},
		{
			name: "defaults to disabled during consumer first Cloud Run rollout",
			env:  map[string]string{"K_SERVICE": "web-frontend"},
			want: runtimeconfig.IdentityConfig{Mode: runtimeconfig.IdentityModeDisabled},
		},
		{
			name: "loads local authentication emulator",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":      "local-emulator",
				"GOOGLE_CLOUD_PROJECT":        "petspotr-auth-test",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
			},
			want: runtimeconfig.IdentityConfig{
				Mode:             runtimeconfig.IdentityModeLocalEmulator,
				ProjectID:        "petspotr-auth-test",
				AuthEmulatorHost: "127.0.0.1:9099",
			},
		},
		{
			name: "enables the Google browser client for the local emulator",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":             "local-emulator",
				"GOOGLE_CLOUD_PROJECT":               "demo-petspotr-auth",
				"FIREBASE_AUTH_EMULATOR_HOST":        "127.0.0.1:9099",
				"PETSPOTR_IDENTITY_WEB_API_KEY":      "fake-api-key",
				"PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN":  "demo-petspotr-auth.firebaseapp.com",
				"PETSPOTR_IDENTITY_WEB_PROJECT_ID":   "demo-petspotr-auth",
				"PETSPOTR_IDENTITY_WEB_EMULATOR_URL": "http://127.0.0.1:9099",
			},
			want: runtimeconfig.IdentityConfig{
				Mode:             runtimeconfig.IdentityModeLocalEmulator,
				ProjectID:        "demo-petspotr-auth",
				AuthEmulatorHost: "127.0.0.1:9099",
				ClientConfig: identity.WebClientConfig{
					Enabled: true, Provider: identity.ProviderGoogle, APIKey: "fake-api-key",
					AuthDomain: "demo-petspotr-auth.firebaseapp.com", ProjectID: "demo-petspotr-auth",
					AuthEmulatorURL: "http://127.0.0.1:9099",
				},
			},
		},
		{
			name: "loads managed Identity Platform",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE": "gcp",
				"GOOGLE_CLOUD_PROJECT":   "petspotr-production",
			},
			want: runtimeconfig.IdentityConfig{
				Mode:      runtimeconfig.IdentityModeGCP,
				ProjectID: "petspotr-production",
			},
		},
		{
			name: "enables the managed Google browser client",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":            "gcp",
				"GOOGLE_CLOUD_PROJECT":              "petspotr-production",
				"PETSPOTR_IDENTITY_WEB_API_KEY":     "public-browser-key",
				"PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN": "auth.petspotr.example",
				"PETSPOTR_IDENTITY_WEB_PROJECT_ID":  "petspotr-production",
			},
			want: runtimeconfig.IdentityConfig{
				Mode: runtimeconfig.IdentityModeGCP, ProjectID: "petspotr-production",
				ClientConfig: identity.WebClientConfig{
					Enabled: true, Provider: identity.ProviderGoogle, APIKey: "public-browser-key",
					AuthDomain: "auth.petspotr.example", ProjectID: "petspotr-production",
				},
			},
		},
		{
			name: "detects the managed project on Cloud Run",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE": "gcp",
				"K_SERVICE":              "web-frontend",
			},
			want: runtimeconfig.IdentityConfig{
				Mode:            runtimeconfig.IdentityModeGCP,
				DetectProjectID: true,
			},
		},
		{
			name:    "rejects unknown mode",
			env:     map[string]string{"PETSPOTR_IDENTITY_MODE": "automatic"},
			wantErr: "unsupported PETSPOTR_IDENTITY_MODE",
		},
		{
			name: "requires a project for a local emulator",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":      "local-emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
			},
			wantErr: "GOOGLE_CLOUD_PROJECT is required",
		},
		{
			name: "requires an authentication emulator host",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE": "local-emulator",
				"GOOGLE_CLOUD_PROJECT":   "petspotr-auth-test",
			},
			wantErr: "FIREBASE_AUTH_EMULATOR_HOST is required",
		},
		{
			name: "rejects an emulator URL with a scheme",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":      "local-emulator",
				"GOOGLE_CLOUD_PROJECT":        "petspotr-auth-test",
				"FIREBASE_AUTH_EMULATOR_HOST": "http://127.0.0.1:9099",
			},
			wantErr: "must not include a URL scheme",
		},
		{
			name: "rejects local emulator mode on Cloud Run",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":      "local-emulator",
				"GOOGLE_CLOUD_PROJECT":        "petspotr-auth-test",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
				"K_SERVICE":                   "web-frontend",
			},
			wantErr: "not allowed on Cloud Run",
		},
		{
			name:    "requires a managed project outside Cloud Run",
			env:     map[string]string{"PETSPOTR_IDENTITY_MODE": "gcp"},
			wantErr: "GOOGLE_CLOUD_PROJECT is required",
		},
		{
			name: "rejects emulator configuration in managed mode",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":      "gcp",
				"GOOGLE_CLOUD_PROJECT":        "petspotr-production",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
			},
			wantErr: "must not be set",
		},
		{
			name: "rejects identity settings while disabled",
			env: map[string]string{
				"GOOGLE_CLOUD_PROJECT":        "petspotr-local",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
			},
			wantErr: "must not be set in disabled mode",
		},
		{
			name: "rejects partial browser configuration",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":        "gcp",
				"GOOGLE_CLOUD_PROJECT":          "petspotr-production",
				"PETSPOTR_IDENTITY_WEB_API_KEY": "public-browser-key",
			},
			wantErr: "browser identity configuration requires",
		},
		{
			name: "rejects a managed browser emulator URL",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":             "gcp",
				"GOOGLE_CLOUD_PROJECT":               "petspotr-production",
				"PETSPOTR_IDENTITY_WEB_API_KEY":      "public-browser-key",
				"PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN":  "auth.petspotr.example",
				"PETSPOTR_IDENTITY_WEB_PROJECT_ID":   "petspotr-production",
				"PETSPOTR_IDENTITY_WEB_EMULATOR_URL": "http://127.0.0.1:9099",
			},
			wantErr: "must not be set in \"gcp\" identity mode",
		},
		{
			name: "rejects a browser project that differs from the session project",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":            "gcp",
				"GOOGLE_CLOUD_PROJECT":              "petspotr-production",
				"PETSPOTR_IDENTITY_WEB_API_KEY":     "public-browser-key",
				"PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN": "auth.petspotr.example",
				"PETSPOTR_IDENTITY_WEB_PROJECT_ID":  "different-project",
			},
			wantErr: "must match GOOGLE_CLOUD_PROJECT",
		},
		{
			name: "requires an explicit session project for Cloud Run browser identity",
			env: map[string]string{
				"PETSPOTR_IDENTITY_MODE":            "gcp",
				"K_SERVICE":                         "web-frontend",
				"PETSPOTR_IDENTITY_WEB_API_KEY":     "public-browser-key",
				"PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN": "auth.petspotr.example",
				"PETSPOTR_IDENTITY_WEB_PROJECT_ID":  "petspotr-production",
			},
			wantErr: "GOOGLE_CLOUD_PROJECT is required for browser identity",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := runtimeconfig.LoadIdentityConfig(func(key string) string {
				return tt.env[key]
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadIdentityConfig() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadIdentityConfig() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("LoadIdentityConfig() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewIdentityRuntimeRejectsMismatchedProcessEmulatorHost(t *testing.T) {
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "127.0.0.1:9199")
	_, err := runtimeconfig.NewIdentityRuntime(context.Background(), runtimeconfig.IdentityConfig{
		Mode: runtimeconfig.IdentityModeLocalEmulator, ProjectID: "demo-petspotr-auth",
		AuthEmulatorHost: "127.0.0.1:9099",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("NewIdentityRuntime() error = %v, want mismatched host rejection", err)
	}
}
