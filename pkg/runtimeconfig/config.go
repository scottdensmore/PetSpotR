// Package runtimeconfig defines the runtime modes and managed-service
// configuration shared by PetSpotR service entrypoints.
package runtimeconfig

import (
	"fmt"
	"os"
	"strings"
)

// Mode selects the backing services used by a PetSpotR process.
type Mode string

const (
	// ModeMemory uses process-local adapters and is only safe for tests and demos.
	ModeMemory Mode = "memory"
	// ModeLocalEmulator uses Google Cloud client libraries against local emulators.
	ModeLocalEmulator Mode = "local-emulator"
	// ModeGCP uses Google Cloud managed services with Application Default Credentials.
	ModeGCP Mode = "gcp"
)

// StateConfig contains the configuration required to construct a StateStore.
type StateConfig struct {
	Mode                  Mode
	ProjectID             string
	DetectProjectID       bool
	FirestoreEmulatorHost string
}

// LoadStateConfigFromEnv loads StateConfig from the process environment.
func LoadStateConfigFromEnv() (StateConfig, error) {
	return LoadStateConfig(os.Getenv)
}

// LoadStateConfig loads StateConfig with a caller-provided environment lookup.
// Supplying the lookup keeps configuration validation deterministic in tests.
func LoadStateConfig(lookup func(string) string) (StateConfig, error) {
	rawMode := strings.TrimSpace(lookup("PETSPOTR_RUNTIME_MODE"))
	cloudRunService := strings.TrimSpace(lookup("K_SERVICE"))
	if rawMode == "" {
		if cloudRunService != "" {
			rawMode = string(ModeGCP)
		} else {
			rawMode = string(ModeMemory)
		}
	}

	config := StateConfig{
		Mode:                  Mode(rawMode),
		ProjectID:             strings.TrimSpace(lookup("GOOGLE_CLOUD_PROJECT")),
		FirestoreEmulatorHost: strings.TrimSpace(lookup("FIRESTORE_EMULATOR_HOST")),
	}

	switch config.Mode {
	case ModeMemory:
		if cloudRunService != "" {
			return StateConfig{}, fmt.Errorf("runtime mode %q is not allowed on Cloud Run", config.Mode)
		}
		return StateConfig{Mode: ModeMemory}, nil
	case ModeLocalEmulator:
		if cloudRunService != "" {
			return StateConfig{}, fmt.Errorf("runtime mode %q is not allowed on Cloud Run", config.Mode)
		}
		if config.ProjectID == "" {
			return StateConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q mode", config.Mode)
		}
		if config.FirestoreEmulatorHost == "" {
			return StateConfig{}, fmt.Errorf("FIRESTORE_EMULATOR_HOST is required in %q mode", config.Mode)
		}
	case ModeGCP:
		if config.ProjectID == "" && cloudRunService == "" {
			return StateConfig{}, fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q mode", config.Mode)
		}
		config.DetectProjectID = config.ProjectID == ""
		if config.FirestoreEmulatorHost != "" {
			return StateConfig{}, fmt.Errorf("FIRESTORE_EMULATOR_HOST must not be set in %q mode", config.Mode)
		}
	default:
		return StateConfig{}, fmt.Errorf("unsupported PETSPOTR_RUNTIME_MODE %q", rawMode)
	}

	return config, nil
}
