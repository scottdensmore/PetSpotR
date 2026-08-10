package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config, err := runtimeconfig.LoadStateConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid runtime configuration: %v", err)
	}
	stateRuntime, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize state runtime: %v", err)
	}
	defer func() {
		if err := stateRuntime.Close(); err != nil {
			log.Printf("Failed to close state runtime: %v", err)
		}
	}()

	ps := pubsub.NewMemoryPubSub()
	oc := ollama.NewClient()

	worker := NewWorker(stateRuntime.Store, ps, oc)

	if err := worker.Start(ctx); err != nil {
		log.Fatalf("Failed to start Pet Matcher worker: %v", err)
	}

	log.Println("Pet Matcher Worker running and listening for foundPet events...")
	<-ctx.Done()
	log.Println("Pet Matcher Worker shutting down gracefully.")
}
