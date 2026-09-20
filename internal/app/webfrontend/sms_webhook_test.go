package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func setupTestServerWithSearchParty(t *testing.T) (*webfrontend.Server, store.StateStore, *webfrontend.ReunionHub, string) {
	t.Helper()
	st := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()

	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
		ReunionHub:               hub,
	})

	ctx := context.Background()
	petID := "lost-pet-sms-101"
	lostPet := domain.LostPetRecord{
		PetID:           petID,
		PetName:         "Buddy",
		Species:         "dog",
		Status:          domain.LostPetStatusLost,
		ReportedAt:      time.Now().UTC(),
		Location:        "Capitol Hill, Seattle",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200},
	}
	petBytes, err := json.Marshal(lostPet)
	if err != nil {
		t.Fatalf("marshal pet: %v", err)
	}
	if err := st.SaveState(ctx, store.LostPetsCollection, petID, petBytes); err != nil {
		t.Fatalf("save pet: %v", err)
	}

	poly := []domain.LocationPoint{
		{Latitude: 47.615, Longitude: -122.320},
		{Latitude: 47.616, Longitude: -122.320},
		{Latitude: 47.616, Longitude: -122.319},
	}

	partyID := "party-" + petID
	party := searchparty.SearchParty{
		PartyID:           partyID,
		LostPetID:         petID,
		CreatedAt:         time.Now().UTC(),
		CenterCoordinates: *lostPet.Coordinates,
		RadiusMeters:      3200.0,
		Sectors: []searchparty.SearchSector{
			{
				SectorID:      "SEC-01",
				Name:          "Sector 1 (North)",
				PolygonPoints: poly,
				Status:        searchparty.SectorStatusUnassigned,
			},
			{
				SectorID:      "SEC-02",
				Name:          "Sector 2 (East)",
				PolygonPoints: poly,
				Status:        searchparty.SectorStatusUnassigned,
			},
		},
		ActiveAssignments:     []searchparty.SectorAssignment{},
		CoveragePercentage:    0,
		ActiveVolunteersCount: 0,
	}
	partyBytes, err := json.Marshal(party)
	if err != nil {
		t.Fatalf("marshal party: %v", err)
	}
	if err := st.SaveState(ctx, store.SearchPartiesCollection, partyID, partyBytes); err != nil {
		t.Fatalf("save party: %v", err)
	}

	return srv, st, hub, petID
}

func TestSMSWebhook_ClaimSector(t *testing.T) {
	srv, st, hub, petID := setupTestServerWithSearchParty(t)

	// Subscribe to SSE hub to verify broadcast
	subCh, unsub := hub.Subscribe(petID)
	defer unsub()

	// Send CLAIM SEC-01
	payload := map[string]string{
		"From": "+12065550199",
		"Body": "CLAIM SEC-01",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status  string `json:"status"`
		Reply   string `json:"reply"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Reply, "SEC-01") || !strings.Contains(strings.ToLower(resp.Reply), "claimed") {
		t.Errorf("unexpected reply: %s", resp.Reply)
	}

	// Verify sector assignment saved in store
	assignments, err := st.ListState(context.Background(), store.SectorAssignmentsCollection)
	if err != nil {
		t.Fatalf("failed to list sector assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 sector assignment, got %d", len(assignments))
	}

	// Verify SSE broadcast event received
	select {
	case evt := <-subCh:
		if evt.Type != domain.ReunionEventSearchPartyUpdated {
			t.Errorf("expected event type %s, got %s", domain.ReunionEventSearchPartyUpdated, evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE search_party_updated event")
	}

	// Claiming already claimed sector should return conflict message
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 for SMS reply, got %d", w2.Code)
	}
	var resp2 struct {
		Reply string `json:"reply"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if !strings.Contains(strings.ToLower(resp2.Reply), "already claimed") {
		t.Errorf("expected already claimed reply, got: %s", resp2.Reply)
	}
}

func TestSMSWebhook_Sighted(t *testing.T) {
	srv, st, hub, petID := setupTestServerWithSearchParty(t)

	subCh, unsub := hub.Subscribe(petID)
	defer unsub()

	payload := map[string]string{
		"From": "+12065550199",
		"Body": "SIGHTED Near Broadway & Pike running west",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Verify sighting persisted in store.SightingsCollection
	rawSightings, err := st.ListState(context.Background(), store.SightingsCollection)
	if err != nil {
		t.Fatalf("failed to list sightings: %v", err)
	}
	if len(rawSightings) != 1 {
		t.Fatalf("expected 1 sighting in store, got %d", len(rawSightings))
	}

	// Verify SSE broadcast event received
	select {
	case evt := <-subCh:
		if evt.Type != domain.ReunionEventSighting {
			t.Errorf("expected event type %s, got %s", domain.ReunionEventSighting, evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE sighting event")
	}
}

func TestSMSWebhook_Status(t *testing.T) {
	srv, _, _, _ := setupTestServerWithSearchParty(t)

	payload := map[string]string{
		"From": "+12065550199",
		"Body": "STATUS",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Reply string `json:"reply"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Reply, "SEC-01") || !strings.Contains(resp.Reply, "SEC-02") {
		t.Errorf("expected status to list open sectors, got: %s", resp.Reply)
	}
}

func TestSMSWebhook_StopAndStart(t *testing.T) {
	srv, st, _, _ := setupTestServerWithSearchParty(t)
	phone := "+12065550199"

	// 1. Send STOP
	payload := map[string]string{
		"From": phone,
		"Body": "STOP",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify opt-out state
	optMgr := sms.NewOptOutManager(st, nil)
	optedOut, err := optMgr.IsOptedOut(context.Background(), phone)
	if err != nil || !optedOut {
		t.Fatalf("expected phone to be opted out, got %v (err: %v)", optedOut, err)
	}

	// 2. Send START
	payloadStart := map[string]string{
		"From": phone,
		"Body": "START",
	}
	bodyStart, _ := json.Marshal(payloadStart)
	reqStart := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyStart))
	reqStart.Header.Set("Content-Type", "application/json")
	wStart := httptest.NewRecorder()

	srv.ServeHTTP(wStart, reqStart)
	if wStart.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wStart.Code)
	}

	// Verify opted-out is now false
	optedOut, err = optMgr.IsOptedOut(context.Background(), phone)
	if err != nil || optedOut {
		t.Fatalf("expected phone to NOT be opted out after START, got %v (err: %v)", optedOut, err)
	}
}

func TestSMSWebhook_UrlEncoded(t *testing.T) {
	srv, _, _, _ := setupTestServerWithSearchParty(t)

	formData := url.Values{}
	formData.Set("From", "+12065550199")
	formData.Set("Body", "STATUS")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestSMSWebhook_MethodNotAllowedAndInvalidRequests(t *testing.T) {
	srv, _, _, _ := setupTestServerWithSearchParty(t)

	// GET method not allowed
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/sms/inbound", nil)
	wGet := httptest.NewRecorder()
	srv.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", wGet.Code)
	}

	// Missing From
	payload := map[string]string{"Body": "STATUS"}
	b, _ := json.Marshal(payload)
	reqNoFrom := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(b))
	reqNoFrom.Header.Set("Content-Type", "application/json")
	wNoFrom := httptest.NewRecorder()
	srv.ServeHTTP(wNoFrom, reqNoFrom)
	if wNoFrom.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing From, got %d", wNoFrom.Code)
	}
}
