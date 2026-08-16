package runtimeconfig

import (
	"fmt"
	"strings"
)

func resolveRuntimeMode(lookup func(string) string) (Mode, bool, error) {
	rawMode := strings.TrimSpace(lookup("PETSPOTR_RUNTIME_MODE"))
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
