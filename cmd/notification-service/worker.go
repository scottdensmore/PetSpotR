package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

// Worker subscribes to matchFound and lostPet events, dispatching multi-channel owner and community broadcast alerts.
type Worker struct {
	broker     pubsub.Broker
	dispatcher *MultiChannelDispatcher
	geoEngine  *GeoBroadcastEngine
}

// NewWorker constructs a Worker instance with default multi-channel senders and geo broadcast engine.
func NewWorker(br pubsub.Broker) *Worker {
	dispatcher := NewMultiChannelDispatcher(
		NewMockEmailSender(),
		NewMockSMSSender(),
		NewMockWebPushSender(),
	)
	geoEngine := NewGeoBroadcastEngine(DefaultSubscribers(), dispatcher)
	return &Worker{
		broker:     br,
		dispatcher: dispatcher,
		geoEngine:  geoEngine,
	}
}

// NewWorkerWithDispatcher constructs a Worker with a custom MultiChannelDispatcher.
func NewWorkerWithDispatcher(br pubsub.Broker, dispatcher *MultiChannelDispatcher) *Worker {
	geoEngine := NewGeoBroadcastEngine(DefaultSubscribers(), dispatcher)
	return &Worker{
		broker:     br,
		dispatcher: dispatcher,
		geoEngine:  geoEngine,
	}
}

// Start registers subscriptions for matchFound and lostPet topics.
func (w *Worker) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := w.broker.Subscribe("matchFound", func(handlerCtx context.Context, data []byte) error {
		_, err := w.ProcessMatchFound(handlerCtx, data)
		return err
	}); err != nil {
		return err
	}

	return w.broker.Subscribe("lostPet", func(handlerCtx context.Context, data []byte) error {
		_, err := w.ProcessLostPetBroadcast(handlerCtx, data)
		return err
	})
}

// ProcessMatchFound converts a matchFound event payload into an OwnerNotification and dispatches multi-channel alerts.
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

	// Multi-Channel Dispatch Execution
	if w.dispatcher != nil {
		msg := &NotificationMessage{
			RecipientID: res.MatchedPetID,
			Email:       notif.ToEmail,
			Phone:       "+12065550199",
			PushToken:   "push-token-default",
			Subject:     notif.Subject,
			Body:        notif.Body,
			Channels:    []Channel{ChannelEmail, ChannelSMS, ChannelPush},
		}

		results, err := w.dispatcher.Dispatch(ctx, msg)
		if err != nil {
			log.Printf("[Notification Service] Dispatch error: %v", err)
		} else {
			log.Printf("[Notification Service] Dispatched across %d channels successfully", len(results))
		}
	}

	return notif, nil
}

// ProcessLostPetBroadcast processes a lostPet event payload and dispatches radius-based community broadcast alerts.
func (w *Worker) ProcessLostPetBroadcast(ctx context.Context, lostPetData []byte) ([]DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var evt domain.LostPetEvent
	if err := json.Unmarshal(lostPetData, &evt); err != nil {
		return nil, fmt.Errorf("notification-service: failed to unmarshal LostPetEvent: %w", err)
	}

	if err := evt.Validate(); err != nil {
		return nil, fmt.Errorf("notification-service: invalid LostPetEvent payload: %w", err)
	}

	if w.geoEngine == nil {
		return nil, fmt.Errorf("notification-service: geoEngine not configured")
	}

	return w.geoEngine.BroadcastLostPetAlert(ctx, &evt, 5.0)
}
