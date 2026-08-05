package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

func main() {
	ps := pubsub.NewMemoryPubSub()
	worker := NewWorker(ps)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		log.Fatalf("Failed to start Notification Worker: %v", err)
	}

	log.Println("Notification Service Worker running and listening for matchFound events...")
	<-ctx.Done()
	log.Println("Notification Service Worker shutting down gracefully.")
}
