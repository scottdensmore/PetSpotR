package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

func TestNotificationWorker_ProcessMatchFound(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	worker := NewWorker(ps)

	t.Run("valid matchFound event constructs and renders owner notification", func(t *testing.T) {
		matchRes := domain.MatchResult{
			FoundPetID:   "found-202",
			MatchedPetID: "lost-101",
			Score:        0.95,
			IsMatch:      true,
			Details:      "High visual match score",
		}
		data, _ := matchRes.ToJSON()

		notif, err := worker.ProcessMatchFound(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessMatchFound failed: %v", err)
		}

		if notif == nil {
			t.Fatal("expected non-nil OwnerNotification")
		}

		if notif.ToEmail == "" {
			t.Errorf("expected non-empty recipient email")
		}

		if notif.Body == "" {
			t.Errorf("expected rendered email body, got empty string")
		}
	})

	t.Run("invalid json event returns error", func(t *testing.T) {
		_, err := worker.ProcessMatchFound(context.Background(), []byte("{invalid-json"))
		if err == nil {
			t.Error("expected error for invalid json, got nil")
		}
	})

	t.Run("invalid domain payload returns error", func(t *testing.T) {
		invalidRes := domain.MatchResult{FoundPetID: ""}
		data, _ := invalidRes.ToJSON()
		_, err := worker.ProcessMatchFound(context.Background(), data)
		if err == nil {
			t.Error("expected error for invalid match result payload, got nil")
		}
	})

	t.Run("versioned matchFound envelope remains consumable", func(t *testing.T) {
		matchRes := domain.MatchResult{FoundPetID: "found-envelope", MatchedPetID: "lost-envelope", Score: 0.91, IsMatch: true}
		payload, _ := matchRes.ToJSON()
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeMatchFound,
			OccurredAt:       time.Now().UTC(),
			AggregateID:      "found-envelope:lost-envelope",
			AggregateVersion: 1,
			PayloadVersion:   1,
			Payload:          payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(envelope)
		if _, err := worker.ProcessMatchFound(context.Background(), data); err != nil {
			t.Fatalf("ProcessMatchFound(envelope) error = %v", err)
		}
	})
}

func TestNotificationWorker_Start(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	worker := NewWorker(ps)

	t.Run("Start registers matchFound subscription successfully", func(t *testing.T) {
		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("worker.Start failed: %v", err)
		}
	})

	t.Run("Start returns error on cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := worker.Start(ctx); err == nil {
			t.Error("expected error on cancelled context, got nil")
		}
	})
}
