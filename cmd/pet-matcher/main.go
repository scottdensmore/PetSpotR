package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	oc := ollama.NewClient()

	worker := NewWorker(st, ps, oc)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		log.Fatalf("Failed to start Pet Matcher worker: %v", err)
	}

	log.Println("Pet Matcher Worker running and listening for foundPet events...")
	<-ctx.Done()
	log.Println("Pet Matcher Worker shutting down gracefully.")
}
