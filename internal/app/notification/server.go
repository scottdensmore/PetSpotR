package notification

import (
	"context"
	"log"
	"net/http"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

// NewHTTPHandler constructs the notification push and health HTTP handler.
func NewHTTPHandler(
	worker *Worker,
	authorizer pubsub.PushAuthorizer,
	expectedMatchSubscription string,
	expectedLostPetSubscription string,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/pubsub/match-found", pubsub.NewPushHandler(
		authorizer,
		expectedMatchSubscription,
		func(ctx context.Context, data []byte) error {
			if _, err := worker.ProcessMatchFound(ctx, data); err != nil {
				log.Printf("Notification Service matchFound processing failed: %v", err)
				return err
			}
			return nil
		},
	))
	mux.Handle("/pubsub/lost-pet", pubsub.NewPushHandler(
		authorizer,
		expectedLostPetSubscription,
		func(ctx context.Context, data []byte) error {
			if _, err := worker.ProcessLostPetBroadcast(ctx, data); err != nil {
				log.Printf("Notification Service lostPet processing failed: %v", err)
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
