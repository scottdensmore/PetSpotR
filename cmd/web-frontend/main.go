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
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := NewServer()
	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("PetSpotR Web Frontend Server listening on http://localhost:%s", port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
	}()

	<-ctx.Done()
	cancel()
	log.Println("Shutting down Web Frontend Server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		shutdownCancel()
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	shutdownCancel()
	log.Println("Web Frontend Server exited cleanly.")
}
