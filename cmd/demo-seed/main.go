// Command demo-seed writes fixed-ID match fixtures to a local Firestore emulator.
package main

import (
	"context"
	"log"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func main() {
	ctx := context.Background()
	config, err := runtimeconfig.LoadStateConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid runtime configuration: %v", err)
	}
	if config.Mode != runtimeconfig.ModeLocalEmulator {
		log.Fatalf("Demo seeding requires %s runtime mode, got %s", runtimeconfig.ModeLocalEmulator, config.Mode)
	}
	runtime, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize state runtime: %v", err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			log.Printf("Failed to close state runtime: %v", err)
		}
	}()

	if err := webfrontend.SeedDemoMatches(ctx, runtime.Store); err != nil {
		log.Fatalf("Failed to seed demo matches: %v", err)
	}
	log.Print("Seeded demo matches in the local Firestore emulator")
}
