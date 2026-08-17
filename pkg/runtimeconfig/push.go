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

// MatcherPushConfig protects the matcher's independent foundPet matching and
// lostPet image-analysis subscriptions with one invocation identity.
type MatcherPushConfig struct {
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

// LoadMatcherPushConfigFromEnv loads matcher push settings.
func LoadMatcherPushConfigFromEnv() (MatcherPushConfig, error) {
	return LoadMatcherPushConfig(os.Getenv)
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

// LoadMatcherPushConfig requires a distinct subscription for each matcher
// route so a delivery cannot cross from matching into image analysis.
func LoadMatcherPushConfig(lookup func(string) string) (MatcherPushConfig, error) {
	base, err := LoadPushConsumerConfig(lookup)
	if err != nil {
		return MatcherPushConfig{}, err
	}
	lostPetSubscription := strings.TrimSpace(lookup("PUBSUB_LOST_SUBSCRIPTION"))
	if lostPetSubscription == "" {
		return MatcherPushConfig{}, fmt.Errorf("PUBSUB_LOST_SUBSCRIPTION is required in %q mode", base.Mode)
	}
	if lostPetSubscription == base.ExpectedSubscription {
		return MatcherPushConfig{}, fmt.Errorf("matcher push subscriptions must be distinct")
	}
	return MatcherPushConfig{
		PushConsumerConfig:          base,
		ExpectedLostPetSubscription: lostPetSubscription,
	}, nil
}

// LoadPushConsumerConfig requires OIDC identity in GCP and an explicit static
// token only for memory/emulator development.
func LoadPushConsumerConfig(lookup func(string) string) (PushConsumerConfig, error) {
	mode, _, modeErr := resolveRuntimeMode(lookup)
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
		Mode:                   mode,
		ExpectedSubscription:   pushSubscription,
		ExpectedServiceAccount: strings.TrimSpace(lookup("PUBSUB_PUSH_SERVICE_ACCOUNT")),
		StaticToken:            strings.TrimSpace(lookup("PUBSUB_PUSH_DEV_TOKEN")),
	}
	if config.ExpectedSubscription == "" {
		return PushConsumerConfig{}, fmt.Errorf("PUBSUB_PUSH_SUBSCRIPTION is required in %q mode", config.Mode)
	}
	// Preserve the push loader's existing subscription-validation precedence
	// while sharing the runtime-mode policy with the other configuration loaders.
	if modeErr != nil {
		return PushConsumerConfig{}, modeErr
	}
	switch config.Mode {
	case ModeMemory, ModeLocalEmulator:
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
	}
	return config, nil
}
