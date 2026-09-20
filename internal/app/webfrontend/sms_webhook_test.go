package webfrontend_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ratelimit"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
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

	// Verify trajectory recalculated and persisted in store.TrajectoriesCollection
	rawTraj, err := st.GetState(context.Background(), store.TrajectoriesCollection, petID)
	if err != nil {
		t.Fatalf("failed to retrieve trajectory from store: %v", err)
	}
	var analysis sighting.TrajectoryAnalysis
	if err := json.Unmarshal(rawTraj, &analysis); err != nil {
		t.Fatalf("failed to unmarshal trajectory analysis: %v", err)
	}
	if analysis.LostPetID != petID {
		t.Errorf("expected trajectory pet ID %s, got %s", petID, analysis.LostPetID)
	}
	if len(analysis.OrderedSightings) != 1 {
		t.Errorf("expected 1 ordered sighting in trajectory, got %d", len(analysis.OrderedSightings))
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

func TestSMSWebhook_SignatureValidation(t *testing.T) {
	st := store.NewMemoryStore()
	secret := "secret-sms-auth-token-123"
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
		SMSWebhookSecret:         secret,
	})

	payload := map[string]string{
		"From": "+12065550199",
		"Body": "STATUS",
	}
	bodyBytes, _ := json.Marshal(payload)

	// 1. Missing signature header -> 401 Unauthorized
	reqNoSig := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	reqNoSig.Header.Set("Content-Type", "application/json")
	wNoSig := httptest.NewRecorder()
	srv.ServeHTTP(wNoSig, reqNoSig)
	if wNoSig.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing signature, got %d", wNoSig.Code)
	}

	// 2. Invalid signature header -> 401 Unauthorized
	reqBadSig := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	reqBadSig.Header.Set("Content-Type", "application/json")
	reqBadSig.Header.Set("X-PetSpotR-Signature", "sha256=invalidhexsignature00000000")
	wBadSig := httptest.NewRecorder()
	srv.ServeHTTP(wBadSig, reqBadSig)
	if wBadSig.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for bad signature, got %d", wBadSig.Code)
	}

	// 3. Valid X-PetSpotR-Signature -> 200 OK
	validPetSpotRSig := webhook.GenerateSignature(bodyBytes, secret)
	reqValidSig := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	reqValidSig.Header.Set("Content-Type", "application/json")
	reqValidSig.Header.Set("X-PetSpotR-Signature", validPetSpotRSig)
	wValidSig := httptest.NewRecorder()
	srv.ServeHTTP(wValidSig, reqValidSig)
	if wValidSig.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PetSpotR signature, got %d", wValidSig.Code)
	}

	// 4. Valid X-Twilio-Signature (HMAC-SHA1 base64) -> 200 OK
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(bodyBytes)
	twilioSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	reqTwilio := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
	reqTwilio.Header.Set("Content-Type", "application/json")
	reqTwilio.Header.Set("X-Twilio-Signature", twilioSig)
	wTwilio := httptest.NewRecorder()
	srv.ServeHTTP(wTwilio, reqTwilio)
	if wTwilio.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid Twilio signature, got %d", wTwilio.Code)
	}
}

func TestSMSWebhook_RateLimiting(t *testing.T) {
	st := store.NewMemoryStore()
	limiter := ratelimit.New()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		RateLimiter:              limiter,
	})

	payload := map[string]string{
		"From": "+12065550199",
		"Body": "STATUS",
	}
	bodyBytes, _ := json.Marshal(payload)

	var got429 bool
	for i := 0; i < 25; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sms/inbound", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}

	if !got429 {
		t.Errorf("expected 429 Too Many Requests after exceeding rate limit burst")
	}
}
