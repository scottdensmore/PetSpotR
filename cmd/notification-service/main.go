package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	stateConfig, err := runtimeconfig.LoadStateConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid runtime configuration: %v", err)
	}
	stateRuntime, err := runtimeconfig.NewStateRuntime(ctx, stateConfig)
	if err != nil {
		log.Fatalf("Failed to initialize state runtime: %v", err)
	}
	defer func() {
		if err := stateRuntime.Close(); err != nil {
			log.Printf("Failed to close state runtime: %v", err)
		}
	}()
	deliveryStore, ok := stateRuntime.Store.(store.DeliveryOperationStore)
	if !ok {
		log.Fatalf("Configured state runtime does not support delivery operations")
	}

	ps := pubsub.NewMemoryPubSub()
	worker := NewWorkerWithStore(deliveryStore, ps)
	if err := worker.Start(ctx); err != nil {
		log.Fatalf("Failed to start Notification Worker: %v", err)
	}

	log.Println("Notification Service Worker running and listening for matchFound events...")
	<-ctx.Done()
	log.Println("Notification Service Worker shutting down gracefully.")
}
