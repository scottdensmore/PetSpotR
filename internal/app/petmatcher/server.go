package petmatcher

import (
	"context"
	"log"
	"net/http"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

// NewHTTPHandler constructs the authenticated matcher push and health routes.
func NewHTTPHandler(
	worker *Worker,
	authorizer pubsub.PushAuthorizer,
	expectedFoundSubscription string,
	expectedLostSubscription string,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/pubsub/found-pet", pubsub.NewPushHandler(
		authorizer,
		expectedFoundSubscription,
		func(ctx context.Context, data []byte) error {
			if err := worker.ProcessFoundPet(ctx, data); err != nil {
				log.Printf("Pet Matcher foundPet processing failed: %v", err)
				return err
			}
			return nil
		},
	))
	mux.Handle("/pubsub/lost-pet", pubsub.NewPushHandler(
		authorizer,
		expectedLostSubscription,
		func(ctx context.Context, data []byte) error {
			if err := worker.ProcessLostPet(ctx, data); err != nil {
				log.Printf("Pet Matcher lostPet image analysis failed: %v", err)
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
