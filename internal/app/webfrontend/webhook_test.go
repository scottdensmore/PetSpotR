package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestWebhookEndpoints_CRUD_And_Test(t *testing.T) {
	t.Parallel()

	// Mock receiver for test ping
	var mu sync.Mutex
	var receivedHeaders http.Header
	var receivedBody []byte
	receivedCalls := 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedCalls++
		receivedHeaders = r.Header.Clone()
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer mockServer.Close()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowLocalhostWebhooks: true,
		DisableRateLimiting:    true,
	})

	// 1. POST /api/v1/webhooks with invalid target URL should fail validation
	t.Run("POST invalid URL returns 400", func(t *testing.T) {
		body := map[string]any{
			"targetUrl": "ftp://invalid-scheme.com/hook",
		}
		jsonBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	// 2. POST /api/v1/webhooks with valid payload returns 201 Created and generates 32-char hex secret
	var createdSub webhook.WebhookSubscription
	t.Run("POST valid webhook returns 201 Created", func(t *testing.T) {
		body := map[string]any{
			"targetUrl":    mockServer.URL + "/events",
			"filterEvents": []string{"pet_lost", "sighting_reported"},
			"geoFence": map[string]any{
				"centerLat":   47.6062,
				"centerLng":   -122.3321,
				"radiusMiles": 15.0,
			},
		}
		jsonBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		if err := json.Unmarshal(rec.Body.Bytes(), &createdSub); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}

		if createdSub.ID == "" {
			t.Errorf("expected non-empty webhook ID")
		}
		if createdSub.TargetURL != mockServer.URL+"/events" {
			t.Errorf("expected targetUrl %q, got %q", mockServer.URL+"/events", createdSub.TargetURL)
		}
		if !createdSub.Active {
			t.Errorf("expected active=true")
		}
		// Secret must be a 32-character hex string (16 bytes)
		if len(createdSub.Secret) != 32 {
			t.Errorf("expected 32-character hex secret, got len=%d (%q)", len(createdSub.Secret), createdSub.Secret)
		}
		if _, err := hex.DecodeString(createdSub.Secret); err != nil {
			t.Errorf("expected valid hex secret: %v", err)
		}
		if len(createdSub.FilterEvents) != 2 {
			t.Errorf("expected 2 filterEvents, got %d", len(createdSub.FilterEvents))
		}
		if createdSub.GeoFence == nil || createdSub.GeoFence.RadiusMiles != 15.0 {
			t.Errorf("expected geoFence with radius 15.0, got %+v", createdSub.GeoFence)
		}
	})

	// 3. GET /api/v1/webhooks returns list of active subscriptions with secret masked
	t.Run("GET /api/v1/webhooks lists subscriptions with masked secret", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var subs []webhook.WebhookSubscription
		if err := json.Unmarshal(rec.Body.Bytes(), &subs); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}

		if len(subs) != 1 {
			t.Fatalf("expected 1 subscription, got %d", len(subs))
		}
		if subs[0].ID != createdSub.ID {
			t.Errorf("expected id %q, got %q", createdSub.ID, subs[0].ID)
		}
		if subs[0].Secret != "••••••••" {
			t.Errorf("expected masked secret '••••••••', got %q", subs[0].Secret)
		}
	})

	// 4. POST /api/v1/webhooks/{id}/test triggers synthetic ping delivery
	t.Run("POST /api/v1/webhooks/{id}/test delivers ping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/"+createdSub.ID+"/test", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var record webhook.WebhookDeliveryRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
			t.Fatalf("failed to decode delivery record: %v", err)
		}
		if !record.Success {
			t.Errorf("expected record.Success=true, error was %q", record.Error)
		}
		if record.StatusCode != http.StatusOK {
			t.Errorf("expected record.StatusCode=200, got %d", record.StatusCode)
		}

		mu.Lock()
		defer mu.Unlock()
		if receivedCalls == 0 {
			t.Fatal("mock server did not receive ping request")
		}

		sigHeader := receivedHeaders.Get(webhook.SignatureHeader)
		if sigHeader == "" {
			t.Fatalf("missing %s header in ping request", webhook.SignatureHeader)
		}
		if !webhook.VerifySignature(receivedBody, createdSub.Secret, sigHeader) {
			t.Errorf("signature verification failed using created secret")
		}
		if !strings.Contains(string(receivedBody), `"ping"`) {
			t.Errorf("expected body to contain ping event, got %s", string(receivedBody))
		}
	})

	// 5. POST /api/v1/webhooks/nonexistent/test returns 404
	t.Run("POST /test on nonexistent webhook returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/nonexistent-id/test", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	// 5b. POST /api/v1/webhooks/{id}/test on inactive webhook returns 400
	t.Run("POST /test on inactive webhook returns 400", func(t *testing.T) {
		inactiveSub := createdSub
		inactiveSub.ID = "inactive-hook-123"
		inactiveSub.Active = false
		subData, _ := json.Marshal(inactiveSub)
		_ = st.SaveState(context.Background(), store.WebhooksCollection, inactiveSub.ID, subData)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/"+inactiveSub.ID+"/test", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "webhook subscription is inactive") {
			t.Errorf("expected inactive error message, got: %s", rec.Body.String())
		}
	})

	// 6. DELETE /api/v1/webhooks/{id} removes subscription
	t.Run("DELETE /api/v1/webhooks/{id} removes subscription", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/"+createdSub.ID, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d: %s", rec.Code, rec.Body.String())
		}

		// Subsequent GET should return empty list
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)

		var subs []webhook.WebhookSubscription
		_ = json.Unmarshal(getRec.Body.Bytes(), &subs)
		if len(subs) != 0 {
			t.Errorf("expected 0 subscriptions after delete, got %d", len(subs))
		}

		// Subsequent DELETE should return 404
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/"+createdSub.ID, nil)
		delRec := httptest.NewRecorder()
		srv.ServeHTTP(delRec, delReq)
		if delRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 on deleting absent webhook, got %d", delRec.Code)
		}
	})
}

func TestAtomFeedEndpoints_LostPetsAndSightings(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{})

	ctx := context.Background()

	// Seed lost pets:
	// 1. Seattle pet with description
	seattlePet := domain.LostPetRecord{
		PetID:       "lost-seattle-1",
		PetName:     "Kona",
		Species:     "Dog",
		Breed:       "Husky",
		Description: "Energetic husky with bright blue eyes.",
		ReportedAt:  time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC),
		Location:    "Seattle, WA",
		Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		Status:      domain.LostPetStatusLost,
	}
	data1, _ := json.Marshal(seattlePet)
	_ = st.SaveState(ctx, store.LostPetsCollection, seattlePet.PetID, data1)

	// 2. NYC pet with empty description to verify Task 3 review recommendation fallback
	nycPet := domain.LostPetRecord{
		PetID:       "lost-nyc-2",
		PetName:     "Simba",
		Species:     "Cat",
		Breed:       "Persian",
		Description: "", // Empty description -> fallback "Lost pet reported"
		ReportedAt:  time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC),
		Location:    "New York, NY",
		Coordinates: &domain.LocationPoint{Latitude: 40.7128, Longitude: -74.0060},
		Status:      domain.LostPetStatusLost,
	}
	data2, _ := json.Marshal(nycPet)
	_ = st.SaveState(ctx, store.LostPetsCollection, nycPet.PetID, data2)

	// Seed sightings:
	// 1. Seattle sighting
	seattleSighting := domain.PetSightingRecord{
		SightingID:          "sight-seattle-1",
		LostPetID:           seattlePet.PetID,
		ReportedAt:          time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC),
		LocationDescription: "Near Pike Place Market",
		Coordinates:         &domain.LocationPoint{Latitude: 47.6097, Longitude: -122.3422},
		Status:              domain.SightingStatusActive,
	}
	sData1, _ := json.Marshal(seattleSighting)
	_ = st.SaveState(ctx, store.SightingsCollection, seattleSighting.SightingID, sData1)

	// 2. NYC sighting
	nycSighting := domain.PetSightingRecord{
		SightingID:          "sight-nyc-2",
		LostPetID:           nycPet.PetID,
		ReportedAt:          time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		LocationDescription: "Central Park West",
		Coordinates:         &domain.LocationPoint{Latitude: 40.7850, Longitude: -73.9682},
		Status:              domain.SightingStatusActive,
	}
	sData2, _ := json.Marshal(nycSighting)
	_ = st.SaveState(ctx, store.SightingsCollection, nycSighting.SightingID, sData2)

	t.Run("GET /feeds/lost-pets.atom without filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/feeds/lost-pets.atom", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/atom+xml; charset=utf-8" {
			t.Errorf("expected Content-Type application/atom+xml; charset=utf-8, got %q", contentType)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "<author>") || !strings.Contains(body, "<name>PetSpotR</name>") {
			t.Errorf("expected feed to contain author PetSpotR, got:\n%s", body)
		}
		if !strings.Contains(body, "Lost pet reported") {
			t.Errorf("expected feed to contain fallback description 'Lost pet reported' for empty description pet")
		}
		if !strings.Contains(body, "lost-seattle-1") {
			t.Errorf("expected feed to contain seattle pet")
		}
		if !strings.Contains(body, "lost-nyc-2") {
			t.Errorf("expected feed to contain nyc pet")
		}
	})

	t.Run("GET /feeds/lost-pets.atom with geofence query parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/feeds/lost-pets.atom?lat=47.6062&lng=-122.3321&radiusMiles=10", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, "lost-seattle-1") {
			t.Errorf("expected feed to contain seattle pet within 10 miles")
		}
		if strings.Contains(body, "lost-nyc-2") {
			t.Errorf("feed should NOT contain nyc pet outside geofence")
		}
	})

	t.Run("GET /feeds/lost-pets.atom with invalid query params returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/feeds/lost-pets.atom?lat=invalid&lng=-122.3321", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid lat, got %d", rec.Code)
		}
	})

	t.Run("GET /feeds/sightings.atom without filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/feeds/sightings.atom", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/atom+xml; charset=utf-8" {
			t.Errorf("expected Content-Type application/atom+xml; charset=utf-8, got %q", contentType)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "<author>") || !strings.Contains(body, "<name>PetSpotR</name>") {
			t.Errorf("expected feed to contain author PetSpotR, got:\n%s", body)
		}
		if !strings.Contains(body, "sight-seattle-1") {
			t.Errorf("expected feed to contain seattle sighting")
		}
		if !strings.Contains(body, "sight-nyc-2") {
			t.Errorf("expected feed to contain nyc sighting")
		}
	})

	t.Run("GET /feeds/sightings.atom with geofence query parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/feeds/sightings.atom?lat=47.6062&lng=-122.3321&radiusMiles=10", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "sight-seattle-1") {
			t.Errorf("expected feed to contain seattle sighting within 10 miles")
		}
		if strings.Contains(body, "sight-nyc-2") {
			t.Errorf("feed should NOT contain nyc sighting outside geofence")
		}
	})
}
