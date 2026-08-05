package main

import (
	"log"
	"net/http"
	"os"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("http://localhost:" + port + "/images")
	svc := NewService(st, ps, bs)

	mux := http.NewServeMux()
	mux.HandleFunc("/foundPet", svc.HandleFoundPet)

	log.Printf("FoundPet Service listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
