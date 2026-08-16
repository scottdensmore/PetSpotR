package runtimeconfig

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

// MessagingRuntime owns the selected publisher and its lifecycle.
type MessagingRuntime struct {
	Publisher pubsub.Publisher
	close     func() error
	closeOnce sync.Once
	closeErr  error
}

// MessagingConfig contains the configuration for a Pub/Sub publisher.
type MessagingConfig struct {
	Mode               Mode
	ProjectID          string
	DetectProjectID    bool
	PubSubEmulatorHost string
}

// LoadMessagingConfigFromEnv loads publisher configuration from the process.
func LoadMessagingConfigFromEnv() (MessagingConfig, error) {
	return LoadMessagingConfig(os.Getenv)
}

// LoadMessagingConfig loads and validates publisher configuration.
func LoadMessagingConfig(lookup func(string) string) (MessagingConfig, error) {
	mode, cloudRun, err := resolveRuntimeMode(lookup)
	if err != nil {
		return MessagingConfig{}, err
	}
	config := MessagingConfig{
		Mode:               mode,
		ProjectID:          strings.TrimSpace(lookup("GOOGLE_CLOUD_PROJECT")),
		PubSubEmulatorHost: strings.TrimSpace(lookup("PUBSUB_EMULATOR_HOST")),
	}
	switch config.Mode {
	case ModeMemory:
		return MessagingConfig{Mode: ModeMemory}, nil
	case ModeLocalEmulator:
		if config.ProjectID == "" || config.PubSubEmulatorHost == "" {
			return MessagingConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT and PUBSUB_EMULATOR_HOST are required in %q mode", config.Mode)
		}
	case ModeGCP:
		if config.PubSubEmulatorHost != "" {
			return MessagingConfig{}, fmt.Errorf("PUBSUB_EMULATOR_HOST must not be set in %q mode", config.Mode)
		}
		if config.ProjectID == "" && !cloudRun {
			return MessagingConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q mode", config.Mode)
		}
		config.DetectProjectID = config.ProjectID == ""
	}
	return config, nil
}

// NewMessagingRuntime selects memory, emulator, or managed Pub/Sub publishing.
func NewMessagingRuntime(ctx context.Context, config MessagingConfig) (*MessagingRuntime, error) {
	switch config.Mode {
	case ModeMemory:
		broker := pubsub.NewMemoryPubSub()
		return &MessagingRuntime{Publisher: broker, close: func() error { return nil }}, nil
	case ModeLocalEmulator:
		publisher, err := pubsub.NewGooglePublisher(ctx, config.ProjectID, config.PubSubEmulatorHost)
		if err != nil {
			return nil, err
		}
		return &MessagingRuntime{Publisher: publisher, close: publisher.Close}, nil
	case ModeGCP:
		projectID := config.ProjectID
		if config.DetectProjectID {
			projectID = pubsub.DetectGoogleProjectID
		}
		publisher, err := pubsub.NewGooglePublisher(ctx, projectID, "")
		if err != nil {
			return nil, err
		}
		return &MessagingRuntime{Publisher: publisher, close: publisher.Close}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime mode %q", config.Mode)
	}
}

// Close releases the publisher once.
func (r *MessagingRuntime) Close() error {
	r.closeOnce.Do(func() { r.closeErr = r.close() })
	return r.closeErr
}
