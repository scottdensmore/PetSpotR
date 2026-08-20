package runtimeconfig

import (
	"context"
	"fmt"
	"sync"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// StateRuntime owns a configured StateStore and its managed client lifecycle.
type StateRuntime struct {
	Store           store.StateStore
	RoleAssignments store.RoleAssignmentStore
	close           func() error
	closeErr        error
	closeOne        sync.Once
}

// NewStateRuntime selects an in-memory, emulator, or managed Firestore
// StateStore from an already-loaded StateConfig.
func NewStateRuntime(ctx context.Context, config StateConfig) (*StateRuntime, error) {
	if err := validateStateConfig(config); err != nil {
		return nil, err
	}

	switch config.Mode {
	case ModeMemory:
		stateStore := store.NewMemoryStore()
		return &StateRuntime{
			Store:           stateStore,
			RoleAssignments: stateStore,
			close:           func() error { return nil },
		}, nil
	case ModeLocalEmulator:
		stateStore, err := store.NewFirestoreEmulatorStore(
			ctx,
			config.ProjectID,
			config.FirestoreEmulatorHost,
		)
		if err != nil {
			return nil, err
		}
		return &StateRuntime{Store: stateStore, RoleAssignments: stateStore, close: stateStore.Close}, nil
	case ModeGCP:
		projectID := config.ProjectID
		if config.DetectProjectID {
			projectID = store.DetectFirestoreProjectID
		}
		stateStore, err := store.NewFirestoreStore(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return &StateRuntime{Store: stateStore, RoleAssignments: stateStore, close: stateStore.Close}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime mode %q", config.Mode)
	}
}

// Close releases any managed client held by the runtime. It is safe to call
// more than once.
func (r *StateRuntime) Close() error {
	r.closeOne.Do(func() {
		r.closeErr = r.close()
	})
	return r.closeErr
}

func validateStateConfig(config StateConfig) error {
	switch config.Mode {
	case ModeMemory:
		return nil
	case ModeLocalEmulator:
		if config.ProjectID == "" {
			return fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q mode", config.Mode)
		}
		if config.FirestoreEmulatorHost == "" {
			return fmt.Errorf("FIRESTORE_EMULATOR_HOST is required in %q mode", config.Mode)
		}
		return nil
	case ModeGCP:
		if config.ProjectID == "" && !config.DetectProjectID {
			return fmt.Errorf("GOOGLE_CLOUD_PROJECT is required in %q mode", config.Mode)
		}
		if config.ProjectID != "" && config.DetectProjectID {
			return fmt.Errorf("runtime mode %q cannot set both ProjectID and DetectProjectID", config.Mode)
		}
		if config.FirestoreEmulatorHost != "" {
			return fmt.Errorf("FIRESTORE_EMULATOR_HOST must not be set in %q mode", config.Mode)
		}
		return nil
	default:
		return fmt.Errorf("unsupported runtime mode %q", config.Mode)
	}
}
