package runtimeconfig

import (
	"fmt"
	"os"
	"strings"
)

// PushConsumerConfig protects one Pub/Sub push endpoint.
type PushConsumerConfig struct {
	Mode                   Mode
	ExpectedSubscription   string
	ExpectedServiceAccount string
	StaticToken            string
}

// LoadPushConsumerConfigFromEnv loads matcher push authentication settings.
func LoadPushConsumerConfigFromEnv() (PushConsumerConfig, error) {
	return LoadPushConsumerConfig(os.Getenv)
}

// LoadPushConsumerConfig requires OIDC identity in GCP and an explicit static
// token only for memory/emulator development.
func LoadPushConsumerConfig(lookup func(string) string) (PushConsumerConfig, error) {
	rawMode := strings.TrimSpace(lookup("PETSPOTR_RUNTIME_MODE"))
	cloudRunService := strings.TrimSpace(lookup("K_SERVICE"))
	if rawMode == "" {
		if cloudRunService != "" {
			rawMode = string(ModeGCP)
		} else {
			rawMode = string(ModeMemory)
		}
	}
	config := PushConsumerConfig{
		Mode:                   Mode(rawMode),
		ExpectedSubscription:   strings.TrimSpace(lookup("PUBSUB_FOUND_SUBSCRIPTION")),
		ExpectedServiceAccount: strings.TrimSpace(lookup("PUBSUB_PUSH_SERVICE_ACCOUNT")),
		StaticToken:            strings.TrimSpace(lookup("PUBSUB_PUSH_DEV_TOKEN")),
	}
	if config.ExpectedSubscription == "" {
		return PushConsumerConfig{}, fmt.Errorf("PUBSUB_FOUND_SUBSCRIPTION is required in %q mode", config.Mode)
	}
	switch config.Mode {
	case ModeMemory, ModeLocalEmulator:
		if cloudRunService != "" {
			return PushConsumerConfig{}, fmt.Errorf("runtime mode %q is not allowed on Cloud Run", config.Mode)
		}
		if config.StaticToken == "" {
			return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_DEV_TOKEN is required in %q mode", config.Mode)
		}
		if config.ExpectedServiceAccount != "" {
			return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_SERVICE_ACCOUNT must not be set in %q mode", config.Mode)
		}
	case ModeGCP:
		if config.ExpectedServiceAccount == "" {
			return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_SERVICE_ACCOUNT is required in %q mode", config.Mode)
		}
		if config.StaticToken != "" {
			return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_DEV_TOKEN must not be set in %q mode", config.Mode)
		}
	default:
		return PushConsumerConfig{}, fmt.Errorf("unsupported PETSPOTR_RUNTIME_MODE %q", rawMode)
	}
	return config, nil
}
