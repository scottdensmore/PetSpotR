package runtimeconfig

import (
	"context"
	"fmt"
	"net"
	"net/url"
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
	ClientConfig     identity.WebClientConfig
}

// IdentityRuntime owns the configured session boundary. Sessions is nil when
// identity is deliberately disabled for a consumer-first rollout.
type IdentityRuntime struct {
	Sessions      identity.SessionManager
	ClientConfig  identity.WebClientConfig
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
	clientConfig, clientConfigured, err := loadIdentityWebClientConfig(lookup)
	if err != nil {
		return IdentityConfig{}, err
	}
	cloudRun := strings.TrimSpace(lookup("K_SERVICE")) != ""

	switch config.Mode {
	case IdentityModeDisabled:
		if config.AuthEmulatorHost != "" || clientConfigured {
			return IdentityConfig{}, fmt.Errorf("identity settings must not be set in disabled mode")
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
		if clientConfigured {
			if clientConfig.ProjectID != config.ProjectID {
				return IdentityConfig{}, fmt.Errorf("PETSPOTR_IDENTITY_WEB_PROJECT_ID must match GOOGLE_CLOUD_PROJECT")
			}
			if err := validateLocalIdentityWebClientConfig(clientConfig); err != nil {
				return IdentityConfig{}, err
			}
			config.ClientConfig = clientConfig
		}
		return config, nil
	case IdentityModeGCP:
		if config.AuthEmulatorHost != "" {
			return IdentityConfig{}, fmt.Errorf("FIREBASE_AUTH_EMULATOR_HOST must not be set in %q identity mode", config.Mode)
		}
		if config.ProjectID == "" && !cloudRun {
			return IdentityConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q identity mode", config.Mode)
		}
		if clientConfigured && config.ProjectID == "" {
			return IdentityConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required for browser identity")
		}
		config.DetectProjectID = config.ProjectID == ""
		if clientConfigured {
			if config.ProjectID != "" && clientConfig.ProjectID != config.ProjectID {
				return IdentityConfig{}, fmt.Errorf("PETSPOTR_IDENTITY_WEB_PROJECT_ID must match GOOGLE_CLOUD_PROJECT")
			}
			if clientConfig.AuthEmulatorURL != "" {
				return IdentityConfig{}, fmt.Errorf("PETSPOTR_IDENTITY_WEB_EMULATOR_URL must not be set in %q identity mode", config.Mode)
			}
			config.ClientConfig = clientConfig
		}
		return config, nil
	default:
		return IdentityConfig{}, fmt.Errorf("unsupported PETSPOTR_IDENTITY_MODE %q", rawMode)
	}
}

func loadIdentityWebClientConfig(lookup func(string) string) (identity.WebClientConfig, bool, error) {
	config := identity.WebClientConfig{
		APIKey:          strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_API_KEY")),
		AuthDomain:      strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN")),
		ProjectID:       strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_PROJECT_ID")),
		AuthEmulatorURL: strings.TrimSpace(lookup("PETSPOTR_IDENTITY_WEB_EMULATOR_URL")),
	}
	configured := config.APIKey != "" || config.AuthDomain != "" || config.ProjectID != "" || config.AuthEmulatorURL != ""
	if !configured {
		return identity.WebClientConfig{}, false, nil
	}
	if config.APIKey == "" || config.AuthDomain == "" || config.ProjectID == "" {
		return identity.WebClientConfig{}, false, fmt.Errorf(
			"browser identity configuration requires PETSPOTR_IDENTITY_WEB_API_KEY, PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN, and PETSPOTR_IDENTITY_WEB_PROJECT_ID",
		)
	}
	if strings.Contains(config.AuthDomain, "://") || strings.ContainsAny(config.AuthDomain, "/?#") {
		return identity.WebClientConfig{}, false, fmt.Errorf("PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN must be a hostname")
	}
	if parsed, err := url.Parse("https://" + config.AuthDomain); err != nil || parsed.Hostname() == "" {
		return identity.WebClientConfig{}, false, fmt.Errorf("PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN must be a hostname")
	}
	config.Enabled = true
	config.Provider = identity.ProviderGoogle
	if err := config.Validate(); err != nil {
		return identity.WebClientConfig{}, false, fmt.Errorf("invalid browser identity configuration: %w", err)
	}
	return config, true, nil
}

func validateLocalIdentityWebClientConfig(config identity.WebClientConfig) error {
	if config.AuthEmulatorURL == "" {
		return fmt.Errorf("PETSPOTR_IDENTITY_WEB_EMULATOR_URL is required for local browser identity")
	}
	parsed, err := url.Parse(config.AuthEmulatorURL)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("PETSPOTR_IDENTITY_WEB_EMULATOR_URL must be an HTTP loopback origin")
	}
	host := parsed.Hostname()
	if host != "localhost" {
		address := net.ParseIP(host)
		if address == nil || !address.IsLoopback() {
			return fmt.Errorf("PETSPOTR_IDENTITY_WEB_EMULATOR_URL must be an HTTP loopback origin")
		}
	}
	return nil
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
		case "PETSPOTR_IDENTITY_WEB_API_KEY":
			return config.ClientConfig.APIKey
		case "PETSPOTR_IDENTITY_WEB_AUTH_DOMAIN":
			return config.ClientConfig.AuthDomain
		case "PETSPOTR_IDENTITY_WEB_PROJECT_ID":
			return config.ClientConfig.ProjectID
		case "PETSPOTR_IDENTITY_WEB_EMULATOR_URL":
			return config.ClientConfig.AuthEmulatorURL
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
		ClientConfig:  loaded.ClientConfig,
		SecureCookies: loaded.Mode == IdentityModeGCP,
	}, nil
}
