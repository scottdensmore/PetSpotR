package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const defaultDeliveryLease = time.Minute

// Worker subscribes to matchFound and lostPet events, dispatching multi-channel owner and community broadcast alerts.
type Worker struct {
	broker        pubsub.Broker
	deliveryStore store.DeliveryOperationStore
	dispatcher    *MultiChannelDispatcher
	geoEngine     *GeoBroadcastEngine
	now           func() time.Time
	deliveryLease time.Duration
}

// NewWorker constructs a Worker instance with default multi-channel senders and geo broadcast engine.
func NewWorker(br pubsub.Broker) *Worker {
	return NewWorkerWithStore(store.NewMemoryStore(), br)
}

// NewWorkerWithStore constructs a Worker with durable delivery operations.
func NewWorkerWithStore(stateStore store.DeliveryOperationStore, br pubsub.Broker) *Worker {
	dispatcher := NewMultiChannelDispatcher(
		NewMockEmailSender(),
		NewMockSMSSender(),
		NewMockWebPushSender(),
	)
	return NewWorkerWithStoreAndDispatcher(stateStore, br, dispatcher)
}

// NewWorkerWithDispatcher constructs a Worker with a custom MultiChannelDispatcher.
func NewWorkerWithDispatcher(br pubsub.Broker, dispatcher *MultiChannelDispatcher) *Worker {
	return NewWorkerWithStoreAndDispatcher(store.NewMemoryStore(), br, dispatcher)
}

// NewWorkerWithStoreAndDispatcher constructs a Worker with custom durable
// state and provider dispatch dependencies.
func NewWorkerWithStoreAndDispatcher(
	stateStore store.DeliveryOperationStore,
	br pubsub.Broker,
	dispatcher *MultiChannelDispatcher,
) *Worker {
	geoEngine := NewGeoBroadcastEngine(DefaultSubscribers(), dispatcher)
	return &Worker{
		broker:        br,
		deliveryStore: stateStore,
		dispatcher:    dispatcher,
		geoEngine:     geoEngine,
		now:           time.Now,
		deliveryLease: defaultDeliveryLease,
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
	envelope, err := domain.DecodeEventPayload(matchResultData, domain.EventTypeMatchFound, &res)
	if err != nil {
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

	if w.dispatcher == nil || w.deliveryStore == nil {
		return nil, errors.New("notification-service: delivery dependencies are not configured")
	}
	envelopeID := ""
	if envelope != nil {
		envelopeID = envelope.ID
	}
	eventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeMatchFound, matchResultData)
	if err != nil {
		return nil, fmt.Errorf("notification-service: resolve matchFound identity: %w", err)
	}
	msg := &NotificationMessage{
		RecipientID: res.MatchedPetID,
		Email:       notif.ToEmail,
		Phone:       "+12065550199",
		PushToken:   "push-token-default",
		Subject:     notif.Subject,
		Body:        notif.Body,
		Channels:    []Channel{ChannelEmail, ChannelSMS, ChannelPush},
	}
	results, err := w.dispatchNotification(ctx, eventID, msg)
	if err != nil {
		return nil, err
	}
	log.Printf("[Notification Service] Dispatched %d owner-notification channels successfully", len(results))

	return notif, nil
}

func (w *Worker) dispatchNotification(
	ctx context.Context,
	eventID string,
	message *NotificationMessage,
) ([]DispatchResult, error) {
	channels := message.Channels
	if len(channels) == 0 {
		channels = []Channel{ChannelEmail}
	}
	allResults := make([]DispatchResult, 0, len(channels))
	for _, channel := range channels {
		now := w.now().UTC()
		operation, err := delivery.NewOperation(eventID, message.RecipientID, string(channel), now)
		if err != nil {
			return allResults, fmt.Errorf("notification-service: create %s delivery operation: %w", channel, err)
		}
		claim, err := w.deliveryStore.ClaimDeliveryOperation(ctx, operation, now, w.deliveryLease)
		if err != nil {
			return allResults, fmt.Errorf("notification-service: claim %s delivery: %w", channel, err)
		}
		switch claim.State {
		case delivery.ClaimCompleted:
			allResults = append(allResults, DispatchResult{Channel: channel, Success: true})
			continue
		case delivery.ClaimInProgress:
			return allResults, fmt.Errorf("notification-service: %s delivery: %w", channel, delivery.ErrOperationInProgress)
		case delivery.ClaimAcquired:
		default:
			return allResults, fmt.Errorf("notification-service: unexpected %s delivery claim %q", channel, claim.State)
		}

		channelMessage := *message
		channelMessage.Channels = []Channel{channel}
		channelMessage.IdempotencyKey = operation.IdempotencyKey
		results, dispatchErr := w.dispatcher.Dispatch(ctx, &channelMessage)
		if dispatchErr == nil {
			if len(results) != 1 || results[0].Channel != channel || !results[0].Success {
				if len(results) == 1 && results[0].Error != "" {
					dispatchErr = errors.New(results[0].Error)
				} else {
					dispatchErr = fmt.Errorf("unexpected dispatch result %#v", results)
				}
			}
		}
		if dispatchErr != nil {
			failureAt := w.now().UTC()
			persistErr := w.deliveryStore.FailDeliveryOperation(
				ctx,
				operation.ID,
				claim.Attempt,
				failureAt,
				dispatchErr.Error(),
			)
			return allResults, fmt.Errorf("notification-service: dispatch %s delivery: %w", channel, errors.Join(dispatchErr, persistErr))
		}
		if err := w.deliveryStore.CompleteDeliveryOperation(
			ctx,
			operation.ID,
			claim.Attempt,
			w.now().UTC(),
		); err != nil {
			return allResults, fmt.Errorf("notification-service: complete %s delivery: %w", channel, err)
		}
		allResults = append(allResults, results[0])
	}
	return allResults, nil
}

// ProcessLostPetBroadcast processes a lostPet event payload and dispatches radius-based community broadcast alerts.
func (w *Worker) ProcessLostPetBroadcast(ctx context.Context, lostPetData []byte) ([]DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var evt domain.LostPetEvent
	envelope, err := domain.DecodeEventPayload(lostPetData, domain.EventTypeLostPetReported, &evt)
	if err != nil {
		return nil, fmt.Errorf("notification-service: failed to unmarshal LostPetEvent: %w", err)
	}

	if err := evt.Validate(); err != nil {
		return nil, fmt.Errorf("notification-service: invalid LostPetEvent payload: %w", err)
	}

	if w.geoEngine == nil {
		return nil, fmt.Errorf("notification-service: geoEngine not configured")
	}
	if w.deliveryStore == nil {
		return nil, fmt.Errorf("notification-service: delivery store not configured")
	}
	envelopeID := ""
	if envelope != nil {
		envelopeID = envelope.ID
	}
	eventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeLostPetReported, lostPetData)
	if err != nil {
		return nil, fmt.Errorf("notification-service: resolve lostPet identity: %w", err)
	}

	return w.geoEngine.broadcastLostPetAlert(
		ctx,
		&evt,
		5.0,
		func(dispatchCtx context.Context, message *NotificationMessage) ([]DispatchResult, error) {
			return w.dispatchNotification(dispatchCtx, eventID, message)
		},
	)
}
