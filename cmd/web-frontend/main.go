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
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/internal/app/outboxrecovery"
	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
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
	identityConfig, err := runtimeconfig.LoadIdentityConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid identity configuration: %v", err)
	}
	identityRuntime, err := runtimeconfig.NewIdentityRuntime(ctx, identityConfig)
	if err != nil {
		log.Fatalf("Failed to initialize identity runtime: %v", err)
	}

	lostReports := lostpet.NewService(stateRuntime.Store, messagingRuntime.Publisher)
	foundReports := foundpet.NewReportService(stateRuntime.Store, messagingRuntime.Publisher)
	backfiller, _ := stateRuntime.Store.(store.OutboxIndexBackfiller)
	recoveryRunner, err := outboxrecovery.New(outboxrecovery.Config{
		Service:    "WebFrontend",
		Backfiller: backfiller,
		Recover: func(ctx context.Context) (int, error) {
			lostCount, lostErr := lostReports.RecoverOutbox(ctx)
			foundCount, foundErr := foundReports.RecoverOutbox(ctx)
			return lostCount + foundCount, errors.Join(lostErr, foundErr)
		},
	})
	if err != nil {
		log.Fatalf("Invalid outbox recovery configuration: %v", err)
	}
	go recoveryRunner.Run(ctx)

	srv := webfrontend.NewServerWithOptions(stateRuntime.Store, webfrontend.ServerOptions{
		AllowPrivilegedMutations: config.Mode == runtimeconfig.ModeMemory,
		FoundPetReporter:         foundReports,
		LostPetReporter:          lostReports,
		IdentitySessions:         identityRuntime.Sessions,
		SecureSessionCookie:      identityRuntime.SecureCookies,
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
