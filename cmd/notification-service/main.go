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

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
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
	deliveryStore, ok := stateRuntime.Store.(store.DeliveryOperationStore)
	if !ok {
		log.Fatalf("Configured state runtime does not support delivery operations")
	}
	pushConfig, err := runtimeconfig.LoadNotificationPushConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid push consumer configuration: %v", err)
	}

	worker := NewWorkerWithStore(deliveryStore, nil)
	var authorizer pubsub.PushAuthorizer
	if pushConfig.Mode == runtimeconfig.ModeGCP {
		authorizer = pubsub.NewOIDCPushAuthorizer(pushConfig.ExpectedServiceAccount, nil)
	} else {
		authorizer = pubsub.NewStaticPushAuthorizer(pushConfig.StaticToken)
	}
	httpServer := &http.Server{
		Addr: ":" + port,
		Handler: newNotificationHTTPHandler(
			worker,
			authorizer,
			pushConfig.ExpectedSubscription,
			pushConfig.ExpectedLostPetSubscription,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Notification Service push endpoint listening on port %s...", port)
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
