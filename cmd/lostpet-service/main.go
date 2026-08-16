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

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/internal/app/outboxrecovery"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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

	svc := lostpet.NewService(stateRuntime.Store, messagingRuntime.Publisher)
	backfiller, _ := stateRuntime.Store.(store.OutboxIndexBackfiller)
	recoveryRunner, err := outboxrecovery.New(outboxrecovery.Config{
		Service:    "LostPet",
		Backfiller: backfiller,
		Recover:    svc.RecoverOutbox,
	})
	if err != nil {
		log.Fatalf("Invalid outbox recovery configuration: %v", err)
	}
	go recoveryRunner.Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/lostPet", svc.HandleLostPet)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("LostPet Service listening on port %s...", port)
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
