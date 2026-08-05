package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

// Worker subscribes to matchFound events and dispatches owner notifications.
type Worker struct {
	broker pubsub.Broker
}

// NewWorker constructs a Worker instance.
func NewWorker(br pubsub.Broker) *Worker {
	return &Worker{
		broker: br,
	}
}

// Start registers the matchFound topic subscription.
func (w *Worker) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return w.broker.Subscribe("matchFound", func(handlerCtx context.Context, data []byte) error {
		_, err := w.ProcessMatchFound(handlerCtx, data)
		return err
	})
}

// ProcessMatchFound converts a matchFound event payload into an OwnerNotification.
func (w *Worker) ProcessMatchFound(ctx context.Context, matchResultData []byte) (*domain.OwnerNotification, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var res domain.MatchResult
	if err := json.Unmarshal(matchResultData, &res); err != nil {
		return nil, fmt.Errorf("notification-service: failed to unmarshal MatchResult: %w", err)
	}

	if err := res.Validate(); err != nil {
		return nil, fmt.Errorf("notification-service: invalid MatchResult payload: %w", err)
	}

	notif := &domain.OwnerNotification{
		FromEmail:  "alerts@petspotr.io",
		ToEmail:    "owner@example.com",
		Subject:    fmt.Sprintf("Match Found for Your Pet (%s)", res.MatchedPetID),
		PetName:    res.MatchedPetID,
		MatchScore: res.Score,
	}

	if err := notif.Validate(); err != nil {
		return nil, fmt.Errorf("notification-service: notification validation failed: %w", err)
	}

	notif.Body = notif.RenderEmailBody()

	log.Printf("[Notification Service] DISPATCHING NOTIFICATION to %s: %s (Score: %.2f)",
		notif.ToEmail, notif.Subject, notif.MatchScore)

	return notif, nil
}
