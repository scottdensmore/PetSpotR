// Command demo-seed writes fixed-ID match and pet fixtures to a local Firestore emulator.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func run(ctx context.Context) error {
	config, err := runtimeconfig.LoadStateConfigFromEnv()
	if err != nil {
		return fmt.Errorf("invalid runtime configuration: %w", err)
	}
	if config.Mode != runtimeconfig.ModeLocalEmulator {
		return fmt.Errorf("demo seeding requires %s runtime mode, got %s", runtimeconfig.ModeLocalEmulator, config.Mode)
	}
	runtime, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to initialize state runtime: %w", err)
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Printf("Failed to close state runtime: %v", closeErr)
		}
	}()

	return webfrontend.SeedDemoData(ctx, runtime.Store)
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
	log.Print("Seeded demo matches and pets in the local Firestore emulator")
}
