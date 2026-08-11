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

// NotificationPushConfig protects the notification service's independent
// matchFound and lostPet push subscriptions with one invocation identity.
type NotificationPushConfig struct {
	PushConsumerConfig
	ExpectedLostPetSubscription string
}

// LoadPushConsumerConfigFromEnv loads push authentication settings.
func LoadPushConsumerConfigFromEnv() (PushConsumerConfig, error) {
	return LoadPushConsumerConfig(os.Getenv)
}

// LoadNotificationPushConfigFromEnv loads notification push settings.
func LoadNotificationPushConfigFromEnv() (NotificationPushConfig, error) {
	return LoadNotificationPushConfig(os.Getenv)
}

// LoadNotificationPushConfig requires a distinct subscription for each
// notification route so one delivery cannot be accepted by the wrong handler.
func LoadNotificationPushConfig(lookup func(string) string) (NotificationPushConfig, error) {
	base, err := LoadPushConsumerConfig(lookup)
	if err != nil {
		return NotificationPushConfig{}, err
	}
	lostPetSubscription := strings.TrimSpace(lookup("PUBSUB_LOST_SUBSCRIPTION"))
	if lostPetSubscription == "" {
		return NotificationPushConfig{}, fmt.Errorf("PUBSUB_LOST_SUBSCRIPTION is required in %q mode", base.Mode)
	}
	if lostPetSubscription == base.ExpectedSubscription {
		return NotificationPushConfig{}, fmt.Errorf("notification push subscriptions must be distinct")
	}
	return NotificationPushConfig{
		PushConsumerConfig:          base,
		ExpectedLostPetSubscription: lostPetSubscription,
	}, nil
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
	pushSubscription := strings.TrimSpace(lookup("PUBSUB_PUSH_SUBSCRIPTION"))
	legacyFoundSubscription := strings.TrimSpace(lookup("PUBSUB_FOUND_SUBSCRIPTION"))
	if pushSubscription != "" && legacyFoundSubscription != "" && pushSubscription != legacyFoundSubscription {
		return PushConsumerConfig{}, fmt.Errorf(
			"PUBSUB_PUSH_SUBSCRIPTION and PUBSUB_FOUND_SUBSCRIPTION must match when both are set",
		)
	}
	if pushSubscription == "" {
		// Preserve the first matcher deployment's variable while callers migrate
		// to the event-agnostic push consumer contract.
		pushSubscription = legacyFoundSubscription
	}
	config := PushConsumerConfig{
		Mode:                   Mode(rawMode),
		ExpectedSubscription:   pushSubscription,
		ExpectedServiceAccount: strings.TrimSpace(lookup("PUBSUB_PUSH_SERVICE_ACCOUNT")),
		StaticToken:            strings.TrimSpace(lookup("PUBSUB_PUSH_DEV_TOKEN")),
	}
	if config.ExpectedSubscription == "" {
		return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_SUBSCRIPTION is required in %q mode", config.Mode)
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
