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

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

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

	srv := webfrontend.NewServerWithOptions(stateRuntime.Store, webfrontend.ServerOptions{
		AllowPrivilegedMutations: config.Mode == runtimeconfig.ModeMemory,
	})
	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("PetSpotR Web Frontend Server listening on http://localhost:%s", port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down Web Frontend Server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		shutdownCancel()
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	shutdownCancel()
	log.Println("Web Frontend Server exited cleanly.")
}
