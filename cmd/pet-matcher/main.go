package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
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
	messagingConfig, err := runtimeconfig.LoadMessagingConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid messaging configuration: %v", err)
	}
	messagingRuntime, err := runtimeconfig.NewMessagingRuntime(ctx, messagingConfig)
	if err != nil {
		log.Fatalf("Failed to initialize messaging runtime: %v", err)
	}
	defer func() {
		if err := messagingRuntime.Close(); err != nil {
			log.Printf("Failed to close messaging runtime: %v", err)
		}
	}()
	storageConfig, err := runtimeconfig.LoadStorageConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid storage configuration: %v", err)
	}
	storageRuntime, err := runtimeconfig.NewStorageRuntime(ctx, storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage runtime: %v", err)
	}
	defer func() {
		if err := storageRuntime.Close(); err != nil {
			log.Printf("Failed to close storage runtime: %v", err)
		}
	}()
	pushConfig, err := runtimeconfig.LoadPushConsumerConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid push consumer configuration: %v", err)
	}

	oc := ollama.NewClient()
	matcherStateStore, ok := stateRuntime.Store.(matcherStore)
	if !ok {
		log.Fatal("State runtime does not support durable matcher operations")
	}
	worker := NewWorkerWithImageStore(matcherStateStore, messagingRuntime.Publisher, oc, storageRuntime.Images)

	var authorizer pubsub.PushAuthorizer
	if pushConfig.Mode == runtimeconfig.ModeGCP {
		authorizer = pubsub.NewOIDCPushAuthorizer(pushConfig.ExpectedServiceAccount, nil)
	} else {
		authorizer = pubsub.NewStaticPushAuthorizer(pushConfig.StaticToken)
	}
	pushHandler := pubsub.NewPushHandler(authorizer, pushConfig.ExpectedSubscription, func(handlerCtx context.Context, data []byte) error {
		if err := worker.ProcessFoundPet(handlerCtx, data); err != nil {
			log.Printf("Pet Matcher foundPet processing failed: %v", err)
			return err
		}
		return nil
	})
	mux := http.NewServeMux()
	mux.Handle("/pubsub/found-pet", pushHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Pet Matcher push service listening on port %s...", port)
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown failed: %v", err)
		}
	}
}
