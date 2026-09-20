package sms_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestRadiusBroadcastWorker(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	mockProvider := sms.NewMockProvider()
	secret := []byte("broadcast-secret-key-salt")
	optMgr := sms.NewOptOutManager(st, secret)

	worker := sms.NewBroadcastWorker(mockProvider, optMgr, st, secret, sms.WithRadiusMiles(2.0))

	// Center point: Capitol Hill, Seattle (47.6150, -122.3200)
	capitolHill := domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}

	subscribers := []sms.Subscriber{
		{
			ID:          "sub-nearby-1",
			Phone:       "+12065550101",
			Coordinates: domain.LocationPoint{Latitude: 47.6160, Longitude: -122.3200}, // ~0.07 miles
			RadiusMiles: 2.0,
		},
		{
			ID:          "sub-opted-out",
			Phone:       "+12065550102",
			Coordinates: domain.LocationPoint{Latitude: 47.6170, Longitude: -122.3210}, // ~0.15 miles
			RadiusMiles: 2.0,
		},
		{
			ID:          "sub-far-ballard",
			Phone:       "+12065550103",
			Coordinates: domain.LocationPoint{Latitude: 47.6684, Longitude: -122.3847}, // ~5.2 miles
			RadiusMiles: 2.0,
		},
		{
			ID:          "sub-tight-radius",
			Phone:       "+12065550104",
			Coordinates: domain.LocationPoint{Latitude: 47.6250, Longitude: -122.3200}, // ~0.7 miles away
			RadiusMiles: 0.5,                                                           // Wants alerts <= 0.5 miles only
		},
	}

	// Opt out sub-opted-out
	if err := optMgr.OptOut(ctx, "+12065550102", "STOP"); err != nil {
		t.Fatalf("failed to opt out: %v", err)
	}

	pet := domain.LostPetRecord{
		PetID:       "lost-coco-123",
		PetName:     "Coco",
		Species:     "Dog",
		Location:    "Capitol Hill, Seattle",
		Coordinates: &capitolHill,
		ReportedAt:  time.Now().UTC(),
	}

	results, err := worker.BroadcastLostPet(ctx, pet, subscribers)
	if err != nil {
		t.Fatalf("BroadcastLostPet failed: %v", err)
	}

	if len(results) != len(subscribers) {
		t.Fatalf("expected %d results, got %d", len(subscribers), len(results))
	}

	// Verify sub-nearby-1 was delivered
	var sub1Result *sms.BroadcastResult
	var optedOutResult *sms.BroadcastResult
	var farResult *sms.BroadcastResult
	var tightResult *sms.BroadcastResult

	for i := range results {
		switch results[i].SubscriberID {
		case "sub-nearby-1":
			sub1Result = &results[i]
		case "sub-opted-out":
			optedOutResult = &results[i]
		case "sub-far-ballard":
			farResult = &results[i]
		case "sub-tight-radius":
			tightResult = &results[i]
		}
	}

	if sub1Result == nil || !sub1Result.Delivered {
		t.Errorf("sub-nearby-1 should be delivered: %+v", sub1Result)
	}

	if optedOutResult == nil || !optedOutResult.Skipped || !strings.Contains(optedOutResult.Reason, "opt") {
		t.Errorf("sub-opted-out should be skipped for opt-out: %+v", optedOutResult)
	}

	if farResult == nil || !farResult.Skipped || !strings.Contains(farResult.Reason, "radius") {
		t.Errorf("sub-far-ballard should be skipped for distance: %+v", farResult)
	}

	if tightResult == nil || !tightResult.Skipped || !strings.Contains(tightResult.Reason, "radius") {
		t.Errorf("sub-tight-radius should be skipped for preference radius: %+v", tightResult)
	}

	// Verify SMS message was sent only to sub-nearby-1
	sentMsgs := mockProvider.SentMessages()
	if len(sentMsgs) != 1 {
		t.Fatalf("expected 1 sent SMS message, got %d", len(sentMsgs))
	}
	if sentMsgs[0].To != "+12065550101" {
		t.Errorf("expected SMS sent to +12065550101, got %s", sentMsgs[0].To)
	}
	if !strings.Contains(sentMsgs[0].Body, "Coco") || !strings.Contains(sentMsgs[0].Body, "CLAIM") {
		t.Errorf("unexpected SMS body content: %s", sentMsgs[0].Body)
	}
	if len(sentMsgs[0].Body) > 160 {
		t.Errorf("SMS body exceeds 160 characters: len=%d (%s)", len(sentMsgs[0].Body), sentMsgs[0].Body)
	}

	// Verify delivery recorded in store.SMSDeliveriesCollection
	deliveries, err := st.ListState(ctx, store.SMSDeliveriesCollection)
	if err != nil {
		t.Fatalf("failed to list SMS deliveries: %v", err)
	}
	if len(deliveries) < 1 {
		t.Fatal("expected delivery records in store.SMSDeliveriesCollection")
	}

	// Verify zero-PII in stored deliveries (phone numbers must be tokenized)
	for _, b := range deliveries {
		if strings.Contains(string(b), "2065550101") || strings.Contains(string(b), "2065550102") {
			t.Errorf("delivery store leaked plaintext phone number: %s", string(b))
		}
	}
}
