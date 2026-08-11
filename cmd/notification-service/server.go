package main

import (
	"context"
	"log"
	"net/http"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

func newNotificationHTTPHandler(
	worker *Worker,
	authorizer pubsub.PushAuthorizer,
	expectedSubscription string,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/pubsub/match-found", pubsub.NewPushHandler(
		authorizer,
		expectedSubscription,
		func(ctx context.Context, data []byte) error {
			if _, err := worker.ProcessMatchFound(ctx, data); err != nil {
				log.Printf("Notification Service matchFound processing failed: %v", err)
				return err
			}
			return nil
		},
	))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	return mux
}
