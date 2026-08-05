package main

import (
	"log"
	"net/http"
	"os"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	svc := NewService(st, ps)

	mux := http.NewServeMux()
	mux.HandleFunc("/lostPet", svc.HandleLostPet)

	log.Printf("LostPet Service listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
