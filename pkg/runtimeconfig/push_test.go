package runtimeconfig_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func TestLoadPushConsumerConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		env     map[string]string
		want    runtimeconfig.PushConsumerConfig
		wantErr bool
	}{
		{
			name: "memory uses static local token",
			env: map[string]string{
				"PUBSUB_PUSH_SUBSCRIPTION": "projects/local/subscriptions/match-found-notification",
				"PUBSUB_PUSH_DEV_TOKEN":    "local-secret",
			},
			want: runtimeconfig.PushConsumerConfig{
				Mode:                 runtimeconfig.ModeMemory,
				ExpectedSubscription: "projects/local/subscriptions/match-found-notification",
				StaticToken:          "local-secret",
			},
		},
		{
			name: "legacy matcher subscription remains supported",
			env: map[string]string{
				"PUBSUB_FOUND_SUBSCRIPTION": "projects/local/subscriptions/found-pet-matcher",
				"PUBSUB_PUSH_DEV_TOKEN":     "local-secret",
			},
			want: runtimeconfig.PushConsumerConfig{
				Mode:                 runtimeconfig.ModeMemory,
				ExpectedSubscription: "projects/local/subscriptions/found-pet-matcher",
				StaticToken:          "local-secret",
			},
		},
		{
			name: "GCP uses exact OIDC identity",
			env: map[string]string{
				"K_SERVICE":                   "pet-matcher",
				"PUBSUB_PUSH_SUBSCRIPTION":    "projects/prod/subscriptions/found-pet-matcher",
				"PUBSUB_PUSH_SERVICE_ACCOUNT": "pubsub-pet-matcher-invoker@prod.iam.gserviceaccount.com",
			},
			want: runtimeconfig.PushConsumerConfig{
				Mode:                   runtimeconfig.ModeGCP,
				ExpectedSubscription:   "projects/prod/subscriptions/found-pet-matcher",
				ExpectedServiceAccount: "pubsub-pet-matcher-invoker@prod.iam.gserviceaccount.com",
			},
		},
		{
			name: "rejects conflicting generic and legacy subscriptions",
			env: map[string]string{
				"PUBSUB_PUSH_SUBSCRIPTION":  "projects/local/subscriptions/match-found-notification",
				"PUBSUB_FOUND_SUBSCRIPTION": "projects/local/subscriptions/found-pet-matcher",
				"PUBSUB_PUSH_DEV_TOKEN":     "local-secret",
			},
			wantErr: true,
		},
		{
			name: "GCP rejects development token",
			env: map[string]string{
				"K_SERVICE":                   "pet-matcher",
				"PUBSUB_PUSH_SUBSCRIPTION":    "projects/prod/subscriptions/found-pet-matcher",
				"PUBSUB_PUSH_SERVICE_ACCOUNT": "pubsub-pet-matcher-invoker@prod.iam.gserviceaccount.com",
				"PUBSUB_PUSH_DEV_TOKEN":       "unsafe",
			},
			wantErr: true,
		},
		{
			name: "Cloud Run rejects emulator static token",
			env: map[string]string{
				"K_SERVICE":                "pet-matcher",
				"PETSPOTR_RUNTIME_MODE":    "local-emulator",
				"PUBSUB_PUSH_SUBSCRIPTION": "projects/local/subscriptions/found-pet-matcher",
				"PUBSUB_PUSH_DEV_TOKEN":    "local-secret",
			},
			wantErr: true,
		},
		{name: "requires subscription and identity", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := runtimeconfig.LoadPushConsumerConfig(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadPushConsumerConfig() error = nil, want non-nil")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("LoadPushConsumerConfig() = %#v, %v; want %#v, nil", got, err, tt.want)
			}
		})
	}
}

func TestLoadNotificationPushConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		env     map[string]string
		want    runtimeconfig.NotificationPushConfig
		wantErr bool
	}{
		{
			name: "binds distinct match and lost pet subscriptions",
			env: map[string]string{
				"PUBSUB_PUSH_SUBSCRIPTION": "projects/local/subscriptions/match-found-notification",
				"PUBSUB_LOST_SUBSCRIPTION": "projects/local/subscriptions/lost-pet-notification",
				"PUBSUB_PUSH_DEV_TOKEN":    "local-secret",
			},
			want: runtimeconfig.NotificationPushConfig{
				PushConsumerConfig: runtimeconfig.PushConsumerConfig{
					Mode:                 runtimeconfig.ModeMemory,
					ExpectedSubscription: "projects/local/subscriptions/match-found-notification",
					StaticToken:          "local-secret",
				},
				ExpectedLostPetSubscription: "projects/local/subscriptions/lost-pet-notification",
			},
		},
		{
			name: "requires lost pet subscription",
			env: map[string]string{
				"PUBSUB_PUSH_SUBSCRIPTION": "projects/local/subscriptions/match-found-notification",
				"PUBSUB_PUSH_DEV_TOKEN":    "local-secret",
			},
			wantErr: true,
		},
		{
			name: "rejects one subscription bound to two handlers",
			env: map[string]string{
				"PUBSUB_PUSH_SUBSCRIPTION": "projects/local/subscriptions/shared-notification",
				"PUBSUB_LOST_SUBSCRIPTION": "projects/local/subscriptions/shared-notification",
				"PUBSUB_PUSH_DEV_TOKEN":    "local-secret",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := runtimeconfig.LoadNotificationPushConfig(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadNotificationPushConfig() error = nil, want non-nil")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("LoadNotificationPushConfig() = %#v, %v; want %#v, nil", got, err, tt.want)
			}
		})
	}
}
