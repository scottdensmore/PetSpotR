package runtimeconfig

import (
	"context"
	"fmt"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"github.com/scottdensmore/petspotr/pkg/identity"
)

// IdentityMode controls whether human-user authentication is disabled, local,
// or backed by managed Google Identity Platform.
type IdentityMode string

const (
	IdentityModeDisabled      IdentityMode = "disabled"
	IdentityModeLocalEmulator IdentityMode = "local-emulator"
	IdentityModeGCP           IdentityMode = "gcp"
)

// IdentityConfig contains the human identity provider configuration.
type IdentityConfig struct {
	Mode             IdentityMode
	ProjectID        string
	DetectProjectID  bool
	AuthEmulatorHost string
}

// IdentityRuntime owns the configured session boundary. Sessions is nil when
// identity is deliberately disabled for a consumer-first rollout.
type IdentityRuntime struct {
	Sessions      identity.SessionManager
	SecureCookies bool
}

// LoadIdentityConfigFromEnv loads human identity configuration.
func LoadIdentityConfigFromEnv() (IdentityConfig, error) {
	return LoadIdentityConfig(os.Getenv)
}

// LoadIdentityConfig validates human identity configuration without making
// network calls.
func LoadIdentityConfig(lookup func(string) string) (IdentityConfig, error) {
	rawMode := strings.TrimSpace(lookup("PETSPOTR_IDENTITY_MODE"))
	if rawMode == "" {
		rawMode = string(IdentityModeDisabled)
	}
	config := IdentityConfig{
		Mode:             IdentityMode(rawMode),
		ProjectID:        strings.TrimSpace(lookup("GOOGLE_CLOUD_PROJECT")),
		AuthEmulatorHost: strings.TrimSpace(lookup("FIREBASE_AUTH_EMULATOR_HOST")),
	}
	cloudRun := strings.TrimSpace(lookup("K_SERVICE")) != ""

	switch config.Mode {
	case IdentityModeDisabled:
		if config.AuthEmulatorHost != "" {
			return IdentityConfig{}, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST must not be set in disabled mode")
		}
		return IdentityConfig{Mode: IdentityModeDisabled}, nil
	case IdentityModeLocalEmulator:
		if cloudRun {
			return IdentityConfig{}, fmt.Errorf("identity mode %q is not allowed on Cloud Run", config.Mode)
		}
		if config.ProjectID == "" {
			return IdentityConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q identity mode", config.Mode)
		}
		if config.AuthEmulatorHost == "" {
			return IdentityConfig{}, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST is required in %q identity mode", config.Mode)
		}
		if strings.Contains(config.AuthEmulatorHost, "://") {
			return IdentityConfig{}, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST must not include a URL scheme")
		}
		return config, nil
	case IdentityModeGCP:
		if config.AuthEmulatorHost != "" {
			return IdentityConfig{}, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST must not be set in %q identity mode", config.Mode)
		}
		if config.ProjectID == "" && !cloudRun {
			return IdentityConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q identity mode", config.Mode)
		}
		config.DetectProjectID = config.ProjectID == ""
		return config, nil
	default:
		return IdentityConfig{}, fmt.Errorf("unsupported PETSPOTR_IDENTITY_MODE %q", rawMode)
	}
}

// NewIdentityRuntime constructs the selected Identity Platform session adapter.
func NewIdentityRuntime(ctx context.Context, config IdentityConfig) (*IdentityRuntime, error) {
	if config.Mode == IdentityModeDisabled {
		return &IdentityRuntime{}, nil
	}
	loaded, err := LoadIdentityConfig(func(key string) string {
		switch key {
		case "PETSPOTR_IDENTITY_MODE":
			return string(config.Mode)
		case "GOOGLE_CLOUD_PROJECT":
			return config.ProjectID
		case "FIREBASE_AUTH_EMULATOR_HOST":
			return config.AuthEmulatorHost
		case "K_SERVICE":
			if config.DetectProjectID {
				return "configured-cloud-run-service"
			}
		}
		return ""
	})
	if err != nil {
		return nil, err
	}
	processEmulatorHost := strings.TrimSpace(os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"))
	if loaded.Mode == IdentityModeLocalEmulator && processEmulatorHost != loaded.AuthEmulatorHost {
		return nil, fmt.Errorf(
			"FIREBASE_AUTH_EMULATOR_HOST process value %q does not match configured local emulator %q",
			processEmulatorHost,
			loaded.AuthEmulatorHost,
		)
	}
	if loaded.Mode == IdentityModeGCP && processEmulatorHost != "" {
		return nil, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST must not be set in %q identity mode", loaded.Mode)
	}

	var firebaseConfig *firebase.Config
	if !loaded.DetectProjectID {
		firebaseConfig = &firebase.Config{ProjectID: loaded.ProjectID}
	}
	app, err := firebase.NewApp(ctx, firebaseConfig)
	if err != nil {
		return nil, fmt.Errorf("initialize Identity Platform app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize Identity Platform auth client: %w", err)
	}
	sessions, err := identity.NewIdentityPlatformSessions(client)
	if err != nil {
		return nil, err
	}
	return &IdentityRuntime{
		Sessions:      sessions,
		SecureCookies: loaded.Mode == IdentityModeGCP,
	}, nil
}
