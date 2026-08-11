package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestLostPetService_HandleLostPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	svc := NewService(st, ps)

	var publishedEvent domain.LostPetEvent
	var publishedEnvelope *domain.EventEnvelope
	var published bool
	_ = ps.Subscribe("lostPet", func(ctx context.Context, data []byte) error {
		published = true
		var err error
		publishedEnvelope, err = domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &publishedEvent)
		return err
	})

	t.Run("valid lost pet submission saves state, publishes event, and returns 201 JSON", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "pet-123",
			ReporterEmail: "owner@example.com",
			ReportedAt:    time.Now().UTC(),
			Location:      "Seattle, WA",
		}

		body, _ := json.Marshal(evt)
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 status code, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}
		if resp["status"] != "success" || resp["petId"] != "pet-123" {
			t.Errorf("unexpected response body: %+v", resp)
		}

		// Verify state persistence
		data, err := st.GetState(context.Background(), store.LostPetsCollection, "pet-123")
		if err != nil {
			t.Fatalf("failed to retrieve saved state: %v", err)
		}

		var savedEvt domain.LostPetEvent
		_ = json.Unmarshal(data, &savedEvt)
		if savedEvt.PetID != "pet-123" {
			t.Errorf("saved pet ID mismatch: got %s, want pet-123", savedEvt.PetID)
		}

		// Verify pubsub event publication
		if !published {
			t.Fatal("expected lostPet event to be published")
		}
		if publishedEvent.PetID != "pet-123" {
			t.Errorf("published pet ID mismatch: got %s, want pet-123", publishedEvent.PetID)
		}
		if publishedEnvelope == nil || publishedEnvelope.AggregateVersion != 1 || publishedEnvelope.PayloadVersion != 1 {
			t.Fatalf("published envelope = %#v", publishedEnvelope)
		}

		// An exact HTTP retry must not republish the same event.
		published = false
		retry := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
		retryRecorder := httptest.NewRecorder()
		svc.HandleLostPet(retryRecorder, retry)
		if retryRecorder.Code != http.StatusCreated {
			t.Fatalf("retry status = %d, want 201", retryRecorder.Code)
		}
		if published {
			t.Fatal("exact retry republished an already completed event")
		}

		competing := evt
		competing.Location = "Portland, OR"
		competingBody, _ := json.Marshal(competing)
		competingRequest := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(competingBody))
		competingRecorder := httptest.NewRecorder()
		svc.HandleLostPet(competingRecorder, competingRequest)
		if competingRecorder.Code != http.StatusConflict {
			t.Fatalf("competing create status = %d, want 409", competingRecorder.Code)
		}
		preservedData, err := st.GetState(context.Background(), store.LostPetsCollection, evt.PetID)
		if err != nil {
			t.Fatal(err)
		}
		var preserved domain.LostPetEvent
		if err := json.Unmarshal(preservedData, &preserved); err != nil {
			t.Fatal(err)
		}
		if preserved.Location != evt.Location {
			t.Fatalf("competing create replaced location = %q, want %q", preserved.Location, evt.Location)
		}
	})

	t.Run("non-POST method returns 405 Method Not Allowed JSON error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lostPet", nil)
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 status code, got %d", rec.Code)
		}

		var errResp ErrorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
		if !strings.Contains(errResp.Error, "Method not allowed") {
			t.Errorf("expected Method not allowed error message, got %s", errResp.Error)
		}
	})

	t.Run("malformed JSON returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader([]byte(`{bad-json`)))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}
	})

	t.Run("invalid domain payload returns 400 Bad Request JSON error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader([]byte(`{"petId":""}`)))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}

		var errResp ErrorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
		if !strings.Contains(errResp.Error, "validation failed") {
			t.Errorf("expected validation failed error message, got %s", errResp.Error)
		}
	})
}

type unavailableBroker struct{}

func (*unavailableBroker) Publish(context.Context, string, []byte) error {
	return errors.New("broker unavailable")
}

func (*unavailableBroker) Subscribe(string, pubsub.Handler) error { return nil }

func TestLostPetService_PublishFailureLeavesRecoverableOutbox(t *testing.T) {
	stateStore := store.NewMemoryStore()
	svc := NewService(stateStore, &unavailableBroker{})
	evt := domain.LostPetEvent{
		PetID:         "lost-recovery-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	body, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	svc.HandleLostPet(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	records, err := stateStore.ListState(context.Background(), store.OutboxCollection)
	if err != nil || len(records) != 1 {
		t.Fatalf("outbox records = %d, %v; want 1", len(records), err)
	}
	for _, data := range records {
		var record outbox.Record
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		if record.Status != outbox.StatusPending || record.Attempts != 1 {
			t.Fatalf("outbox record = %#v", record)
		}
	}
}

func TestLostPetService_NoSubscriberDoesNotScanOrRewriteBacklog(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	backlog := outbox.NewRecord("evt-backlog", "lostPet", []byte(`{"id":"evt-backlog"}`), time.Now().UTC())
	if err := outbox.SaveRecord(ctx, stateStore, backlog); err != nil {
		t.Fatal(err)
	}
	svc := NewService(stateStore, pubsub.NewMemoryPubSub())
	evt := domain.LostPetEvent{
		PetID:         "lost-no-subscriber",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	body, _ := json.Marshal(evt)
	req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	svc.HandleLostPet(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	records, err := stateStore.ListState(ctx, store.OutboxCollection)
	if err != nil || len(records) != 2 {
		t.Fatalf("outbox records = %d, %v; want 2", len(records), err)
	}
	for id, data := range records {
		var record outbox.Record
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		if record.Attempts != 0 {
			t.Fatalf("record %s attempts = %d, want 0", id, record.Attempts)
		}
	}
}

func TestLostPetService_RecoverPendingOutbox(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	published := make(chan []byte, 1)
	if err := broker.Subscribe("lostPet", func(_ context.Context, data []byte) error {
		published <- append([]byte(nil), data...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	record := outbox.NewRecord("evt-lost-recovery", "lostPet", []byte(`{"petId":"lost-recovery"}`), time.Now().UTC())
	if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
		t.Fatal(err)
	}
	svc := NewService(stateStore, broker)

	count, err := svc.recoverOutbox(ctx)
	if err != nil || count != 1 {
		t.Fatalf("recoverOutbox() = %d, %v; want 1, nil", count, err)
	}
	select {
	case got := <-published:
		if string(got) != string(record.Payload) {
			t.Fatalf("published payload = %s, want %s", got, record.Payload)
		}
	default:
		t.Fatal("recoverOutbox() did not publish the pending lostPet event")
	}
	got, err := outbox.GetRecord(ctx, stateStore, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != outbox.StatusPublished {
		t.Fatalf("recovered status = %q, want %q", got.Status, outbox.StatusPublished)
	}
}
