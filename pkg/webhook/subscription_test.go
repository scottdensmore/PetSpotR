package webhook_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestWebhookSubscription_JSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)
	sub := webhook.WebhookSubscription{
		ID:           "sub-123",
		PartnerID:    "partner-456",
		TargetURL:    "https://api.example.com/webhook",
		Secret:       "sec_xyz123",
		FilterEvents: []string{"pet_lost", "sighting_reported"},
		GeoFence: &webhook.GeoFence{
			CenterLat:   47.6062,
			CenterLng:   -122.3321,
			RadiusMiles: 15.5,
		},
		Active:    true,
		CreatedAt: now,
	}

	data, err := json.Marshal(sub)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded webhook.WebhookSubscription
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != sub.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, sub.ID)
	}
	if decoded.PartnerID != sub.PartnerID {
		t.Errorf("PartnerID = %q, want %q", decoded.PartnerID, sub.PartnerID)
	}
	if decoded.TargetURL != sub.TargetURL {
		t.Errorf("TargetURL = %q, want %q", decoded.TargetURL, sub.TargetURL)
	}
	if decoded.Secret != sub.Secret {
		t.Errorf("Secret = %q, want %q", decoded.Secret, sub.Secret)
	}
	if len(decoded.FilterEvents) != 2 || decoded.FilterEvents[0] != "pet_lost" || decoded.FilterEvents[1] != "sighting_reported" {
		t.Errorf("FilterEvents = %v, want %v", decoded.FilterEvents, sub.FilterEvents)
	}
	if decoded.GeoFence == nil {
		t.Fatal("GeoFence is nil, want non-nil")
	}
	if decoded.GeoFence.CenterLat != 47.6062 || decoded.GeoFence.CenterLng != -122.3321 || decoded.GeoFence.RadiusMiles != 15.5 {
		t.Errorf("GeoFence = %+v, want %+v", *decoded.GeoFence, *sub.GeoFence)
	}
	if !decoded.Active {
		t.Errorf("Active = false, want true")
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, now)
	}
}

func TestWebhookDeliveryRecord_JSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 12, 5, 0, 0, time.UTC)
	record := webhook.WebhookDeliveryRecord{
		ID:             "del-789",
		SubscriptionID: "sub-123",
		EventID:        "evt-001",
		EventType:      "pet_lost",
		TargetURL:      "https://api.example.com/webhook",
		StatusCode:     200,
		Success:        true,
		Error:          "",
		Attempt:        1,
		ResponseTimeMs: 142,
		CreatedAt:      now,
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded webhook.WebhookDeliveryRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != record.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, record.ID)
	}
	if decoded.SubscriptionID != record.SubscriptionID {
		t.Errorf("SubscriptionID = %q, want %q", decoded.SubscriptionID, record.SubscriptionID)
	}
	if decoded.EventID != record.EventID {
		t.Errorf("EventID = %q, want %q", decoded.EventID, record.EventID)
	}
	if decoded.EventType != record.EventType {
		t.Errorf("EventType = %q, want %q", decoded.EventType, record.EventType)
	}
	if decoded.TargetURL != record.TargetURL {
		t.Errorf("TargetURL = %q, want %q", decoded.TargetURL, record.TargetURL)
	}
	if decoded.StatusCode != record.StatusCode {
		t.Errorf("StatusCode = %d, want %d", decoded.StatusCode, record.StatusCode)
	}
	if !decoded.Success {
		t.Errorf("Success = false, want true")
	}
	if decoded.Attempt != record.Attempt {
		t.Errorf("Attempt = %d, want %d", decoded.Attempt, record.Attempt)
	}
	if decoded.ResponseTimeMs != record.ResponseTimeMs {
		t.Errorf("ResponseTimeMs = %d, want %d", decoded.ResponseTimeMs, record.ResponseTimeMs)
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, now)
	}
}

func TestCollectionConstants(t *testing.T) {
	t.Parallel()

	if webhook.WebhooksCollection != store.WebhooksCollection {
		t.Errorf("webhook.WebhooksCollection = %q, want %q", webhook.WebhooksCollection, store.WebhooksCollection)
	}
	if webhook.WebhookDeliveriesCollection != store.WebhookDeliveriesCollection {
		t.Errorf("webhook.WebhookDeliveriesCollection = %q, want %q", webhook.WebhookDeliveriesCollection, store.WebhookDeliveriesCollection)
	}
	if webhook.WebhooksCollection != "webhooks" {
		t.Errorf("webhook.WebhooksCollection = %q, want webhooks", webhook.WebhooksCollection)
	}
	if webhook.WebhookDeliveriesCollection != "webhookDeliveries" {
		t.Errorf("webhook.WebhookDeliveriesCollection = %q, want webhookDeliveries", webhook.WebhookDeliveriesCollection)
	}
}
