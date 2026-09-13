package runtimeconfig

import (
	"fmt"
	"strings"
)

func resolveRuntimeMode(lookup func(string) string) (Mode, bool, error) {
	rawMode := strings.TrimSpace(lookup("PETSPOTR_RUNTIME_MODE"))
	if rawMode == "" {
		env := strings.TrimSpace(lookup("PETSPOTR_ENV"))
		if env == "" {
			env = strings.TrimSpace(lookup("ENVIRONMENT"))
		}
		switch strings.ToLower(env) {
		case string(EnvironmentLocalEmulator), "local", "emulator":
			rawMode = string(ModeLocalEmulator)
		}
	}
	cloudRun := strings.TrimSpace(lookup("K_SERVICE")) != ""
	if rawMode == "" {
		if cloudRun {
			rawMode = string(ModeGCP)
		} else {
			rawMode = string(ModeMemory)
		}
	}

	mode := Mode(rawMode)
	switch mode {
	case ModeMemory, ModeLocalEmulator, ModeGCP:
	default:
		return mode, cloudRun, fmt.Errorf("unsupported PETSPOTR_RUNTIME_MODE %q", rawMode)
	}
	if cloudRun && mode != ModeGCP {
		return mode, true, fmt.Errorf("runtime mode %q is not allowed on Cloud Run", mode)
	}
	return mode, cloudRun, nil
}

func resolveComponentMode(lookup func(string) string, environmentKey string) (Mode, bool, error) {
	rawMode := strings.TrimSpace(lookup(environmentKey))
	if rawMode == "" {
		mode, cloudRun, err := resolveRuntimeMode(lookup)
		if err != nil {
			return mode, cloudRun, err
		}
		if environmentKey == "PETSPOTR_STORAGE_MODE" && mode == ModeLocalEmulator && strings.TrimSpace(lookup("PETSPOTR_RUNTIME_MODE")) == "" && strings.TrimSpace(lookup("STORAGE_EMULATOR_HOST")) == "" {
			return ModeMemory, cloudRun, nil
		}
		return mode, cloudRun, nil
	}
	cloudRun := strings.TrimSpace(lookup("K_SERVICE")) != ""
	mode := Mode(rawMode)
	switch mode {
	case ModeMemory, ModeLocalEmulator, ModeGCP:
	default:
		return mode, cloudRun, fmt.Errorf("unsupported %s %q", environmentKey, rawMode)
	}
	if cloudRun && mode != ModeGCP {
		return mode, true, fmt.Errorf("runtime mode %q is not allowed on Cloud Run", mode)
	}
	return mode, cloudRun, nil
}
