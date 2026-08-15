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

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
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
	if storageConfig.Mode == runtimeconfig.ModeMemory && storageConfig.MemoryBaseURL == "" {
		storageConfig.MemoryBaseURL = "http://localhost:" + port + "/images"
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

	svc := foundpet.NewServiceWithOptions(
		stateRuntime.Store,
		messagingRuntime.Publisher,
		storageRuntime.Images,
		foundpet.ServiceOptions{RequireFinalizedImage: storageConfig.Mode != runtimeconfig.ModeMemory},
	)
	nextBackfillAt := time.Time{}
	nextImageCleanupAt := time.Time{}
	recoverOutbox := func() {
		now := time.Now().UTC()
		if !now.Before(nextImageCleanupAt) {
			deleted, err := svc.CleanupOrphanedImages(ctx, now)
			if err != nil && ctx.Err() == nil {
				log.Printf("FoundPet finalized-image cleanup deferred: %v", err)
				nextImageCleanupAt = now.Add(5 * time.Minute)
			} else {
				nextImageCleanupAt = now.Add(time.Hour)
			}
			if deleted > 0 {
				log.Printf("FoundPet finalized-image cleanup removed %d orphaned objects", deleted)
			}
		}
		if backfiller, ok := stateRuntime.Store.(store.OutboxIndexBackfiller); ok && !now.Before(nextBackfillAt) {
			migrated, complete, err := backfiller.BackfillOutboxIndexes(ctx, outbox.MaxPublishBatch)
			if err != nil && ctx.Err() == nil {
				log.Printf("FoundPet legacy outbox index backfill deferred: %v", err)
			} else if complete {
				nextBackfillAt = now.Add(time.Minute)
			}
			if migrated > 0 {
				log.Printf("FoundPet legacy outbox index backfill migrated %d records (complete=%t)", migrated, complete)
			}
		}
		if _, err := svc.RecoverOutbox(ctx); err != nil && ctx.Err() == nil {
			log.Printf("FoundPet outbox recovery deferred: %v", err)
		}
	}
	go func() {
		recoverOutbox()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recoverOutbox()
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/foundPet", svc.HandleFoundPet)
	mux.HandleFunc("/foundPet/uploads", svc.HandleBeginImageUpload)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("FoundPet Service listening on port %s...", port)
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
