// Command demo-seed writes fixed-ID match and pet fixtures to a local Firestore emulator.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func shouldSeedFromEnv() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_DEMO_DATA")))
	return val != "false" && val != "0" && val != "no"
}

func seedStore(ctx context.Context, s store.StateStore, out io.Writer) error {
	if err := webfrontend.SeedDemoMatches(ctx, s); err != nil {
		return fmt.Errorf("failed to seed demo matches: %w", err)
	}
	if err := webfrontend.SeedDemoPets(ctx, s); err != nil {
		return fmt.Errorf("failed to seed demo pets: %w", err)
	}
	_, _ = fmt.Fprintln(out, "Seeded demo matches and pets in the local Firestore emulator")
	return nil
}

func run(ctx context.Context, out io.Writer) error {
	if !shouldSeedFromEnv() {
		_, _ = fmt.Fprintln(out, "Skipping demo data seed (SEED_DEMO_DATA is disabled)")
		return nil
	}

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

	return seedStore(ctx, runtime.Store, out)
}

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		log.Fatalf("%v", err)
	}
}
