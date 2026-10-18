package webfrontend_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestTrustSafetyHandlers_SeedingCreateQuarantineAndAdjudicate(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "trust_safety" topic and Mesh signaling hub
	trustSSECh, unsubSSE := server.ReunionHub().Subscribe("trust_safety")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("trust_safety", "trust-safety-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /trust-safety should render HTML and seed default screenings
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/trust-safety", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /trust-safety, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Trust &amp; Safety") && !strings.Contains(recPage.Body.String(), "Trust & Safety") {
		t.Errorf("expected /trust-safety HTML to contain Trust & Safety heading")
	}

	// 2. GET /api/v1/trust-safety/screenings should return the 3 seeded screenings
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/trust-safety/screenings", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/trust-safety/screenings, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Screenings []domain.SightingTrustScreening `json:"screenings"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal trust screenings response: %v", err)
	}
	if len(listResp.Screenings) != 3 {
		t.Fatalf("expected 3 seeded trust screenings, got %d", len(listResp.Screenings))
	}

	byID := make(map[string]domain.SightingTrustScreening)
	for _, sc := range listResp.Screenings {
		byID[sc.ScreeningID] = sc
	}

	s101, ok := byID["screening-ts-101"]
	if !ok {
		t.Fatalf("expected seeded screening-ts-101")
	}
	if s101.PetName != "Max" || s101.Verdict != domain.TrustVerdictQuarantinedScam {
		t.Errorf("expected screening-ts-101 Max to be QUARANTINED_SCAM_RISK, got %s / %s", s101.PetName, s101.Verdict)
	}
	if s101.PromotedSightingID != "" {
		t.Errorf("expected quarantined screening-ts-101 to have empty PromotedSightingID, got %q", s101.PromotedSightingID)
	}

	s102, ok := byID["screening-ts-102"]
	if !ok {
		t.Fatalf("expected seeded screening-ts-102")
	}
	if s102.PetName != "Luna" || s102.Verdict != domain.TrustVerdictHoldModeration {
		t.Errorf("expected screening-ts-102 Luna to be HOLD_FOR_MODERATION, got %s / %s", s102.PetName, s102.Verdict)
	}

	s103, ok := byID["screening-ts-103"]
	if !ok {
		t.Fatalf("expected seeded screening-ts-103")
	}
	if s103.PetName != "Bella" || s103.Verdict != domain.TrustVerdictAuthentic {
		t.Errorf("expected screening-ts-103 Bella to be VERIFIED_AUTHENTIC, got %s / %s", s103.PetName, s103.Verdict)
	}
	if s103.PromotedSightingID != "sighting-trust-seed-103" {
		t.Errorf("expected screening-ts-103 PromotedSightingID 'sighting-trust-seed-103', got %q", s103.PromotedSightingID)
	}

	// 3. POST /api/v1/trust-safety/screenings with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":          "",
		"reporterHandle": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid trust screening, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/trust-safety/screenings with extortion scam auto-quarantines and does NOT promote sighting
	scamPayload, _ := json.Marshal(map[string]any{
		"petId":              "pet-trust-rocky",
		"petName":            "Rocky",
		"reporterHandle":     "scam-actor-99",
		"reporterTrustScore": 15,
		"prevRefLat":         47.6062,
		"prevRefLng":         -122.3321,
		"sightingLat":        47.6080,
		"sightingLng":        -122.3300,
		"elapsedMinutes":     25,
		"submittedNotes":     "Pay upfront via Venmo or gift card before I tell you where Rocky is",
		"imageIntegrityTag":  "SYNTHETIC_GAN_SIGNATURE",
	})
	recScam := httptest.NewRecorder()
	reqScam := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings", bytes.NewReader(scamPayload))
	reqScam.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recScam, reqScam)
	if recScam.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for scam screening, got %d: %s", recScam.Code, recScam.Body.String())
	}

	var createdScam domain.SightingTrustScreening
	if err := json.Unmarshal(recScam.Body.Bytes(), &createdScam); err != nil {
		t.Fatalf("failed to unmarshal created scam screening: %v", err)
	}
	if createdScam.Verdict != domain.TrustVerdictQuarantinedScam {
		t.Errorf("expected Verdict QUARANTINED_SCAM_RISK, got %s", createdScam.Verdict)
	}
	if createdScam.PromotedSightingID != "" {
		t.Errorf("expected quarantined scam to NOT be promoted to sighting, got %q", createdScam.PromotedSightingID)
	}

	// Verify SSE event "trust_screening_updated" and Mesh event
	select {
	case evt := <-trustSSECh:
		if evt.Type != domain.ReunionEventType("trust_screening_updated") {
			t.Errorf("expected trust_screening_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for trust_screening_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "trust_screening_updated" {
			t.Errorf("expected trust_screening_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for trust_screening_updated mesh event")
	}

	// 5. POST /api/v1/trust-safety/screenings with authentic sighting auto-promotes to store.SightingsCollection
	authenticPayload, _ := json.Marshal(map[string]any{
		"petId":              "pet-trust-archie",
		"petName":            "Archie",
		"reporterHandle":     "sar-spotter-elena",
		"reporterTrustScore": 90,
		"prevRefLat":         47.6062,
		"prevRefLng":         -122.3321,
		"sightingLat":        47.6095,
		"sightingLng":        -122.3340,
		"elapsedMinutes":     30,
		"submittedNotes":     "Spotted Archie resting calmly near Myrtle Edwards Park trail",
		"imageIntegrityTag":  "CAMERA_EXIF_INTACT",
	})
	recAuth := httptest.NewRecorder()
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings", bytes.NewReader(authenticPayload))
	reqAuth.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for authentic screening, got %d: %s", recAuth.Code, recAuth.Body.String())
	}

	var createdAuth domain.SightingTrustScreening
	if err := json.Unmarshal(recAuth.Body.Bytes(), &createdAuth); err != nil {
		t.Fatalf("failed to unmarshal created authentic screening: %v", err)
	}
	if createdAuth.Verdict != domain.TrustVerdictAuthentic {
		t.Errorf("expected Verdict VERIFIED_AUTHENTIC, got %s", createdAuth.Verdict)
	}
	if !strings.HasPrefix(createdAuth.PromotedSightingID, "sighting-trust-") {
		t.Fatalf("expected auto-promoted sighting ID starting with 'sighting-trust-', got %q", createdAuth.PromotedSightingID)
	}

	// Verify sighting was saved to store.SightingsCollection
	sightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, createdAuth.PromotedSightingID)
	if err != nil {
		t.Fatalf("expected auto-promoted sighting %q in SightingsCollection: %v", createdAuth.PromotedSightingID, err)
	}
	var promotedSighting domain.Sighting
	if err := json.Unmarshal(sightingBytes, &promotedSighting); err != nil {
		t.Fatalf("failed to unmarshal promoted sighting: %v", err)
	}
	if promotedSighting.PetID != "pet-trust-archie" {
		t.Errorf("expected promoted sighting PetID 'pet-trust-archie', got %q", promotedSighting.PetID)
	}

	// 6. POST /api/v1/trust-safety/screenings/{id}/decision approves held screening-ts-102 and promotes to SightingsCollection
	approvePayload, _ := json.Marshal(map[string]any{
		"verdict":        "VERIFIED_AUTHENTIC",
		"moderatorNotes": "Confirmed reporter rode commuter rail; sighting verified by field unit",
	})
	recApprove := httptest.NewRecorder()
	reqApprove := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings/screening-ts-102/decision", bytes.NewReader(approvePayload))
	reqApprove.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recApprove, reqApprove)
	if recApprove.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for moderator approval, got %d: %s", recApprove.Code, recApprove.Body.String())
	}

	var approvedScreening domain.SightingTrustScreening
	if err := json.Unmarshal(recApprove.Body.Bytes(), &approvedScreening); err != nil {
		t.Fatalf("failed to unmarshal approved screening: %v", err)
	}
	if approvedScreening.Verdict != domain.TrustVerdictAuthentic {
		t.Errorf("expected Verdict VERIFIED_AUTHENTIC after approval, got %s", approvedScreening.Verdict)
	}
	if approvedScreening.ReporterTrustScore != 63 {
		t.Errorf("expected ReporterTrustScore boosted to 63 (48+15), got %d", approvedScreening.ReporterTrustScore)
	}
	if !strings.HasPrefix(approvedScreening.PromotedSightingID, "sighting-trust-") {
		t.Fatalf("expected PromotedSightingID starting with 'sighting-trust-' after moderator approval, got %q", approvedScreening.PromotedSightingID)
	}

	lunaSightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, approvedScreening.PromotedSightingID)
	if err != nil {
		t.Fatalf("expected moderator-promoted sighting %q in SightingsCollection: %v", approvedScreening.PromotedSightingID, err)
	}
	var lunaSighting domain.Sighting
	if err := json.Unmarshal(lunaSightingBytes, &lunaSighting); err != nil {
		t.Fatalf("failed to unmarshal Luna promoted sighting: %v", err)
	}
	if lunaSighting.PetID != "pet-trust-luna" {
		t.Errorf("expected Luna sighting PetID 'pet-trust-luna', got %q", lunaSighting.PetID)
	}

	// 7. POST /api/v1/trust-safety/screenings/{id}/decision rejects scam screening and zeroes reporter trust score
	rejectPayload, _ := json.Marshal(map[string]any{
		"verdict":        "REJECTED_CONFIRMED_HOAX",
		"moderatorNotes": "Confirmed extortion actor — permanently blocked",
	})
	recReject := httptest.NewRecorder()
	reqReject := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings/"+createdScam.ScreeningID+"/decision", bytes.NewReader(rejectPayload))
	reqReject.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recReject, reqReject)
	if recReject.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for moderator rejection, got %d: %s", recReject.Code, recReject.Body.String())
	}

	var rejectedScreening domain.SightingTrustScreening
	if err := json.Unmarshal(recReject.Body.Bytes(), &rejectedScreening); err != nil {
		t.Fatalf("failed to unmarshal rejected screening: %v", err)
	}
	if rejectedScreening.Verdict != domain.TrustVerdictRejectedHoax {
		t.Errorf("expected Verdict REJECTED_CONFIRMED_HOAX, got %s", rejectedScreening.Verdict)
	}
	if rejectedScreening.ReporterTrustScore != 0 {
		t.Errorf("expected ReporterTrustScore zeroed to 0, got %d", rejectedScreening.ReporterTrustScore)
	}
	if rejectedScreening.RiskScore != 100 || rejectedScreening.TrustConfidenceScore != 0 {
		t.Errorf("expected RiskScore=100 and TrustConfidenceScore=0, got %d / %d", rejectedScreening.RiskScore, rejectedScreening.TrustConfidenceScore)
	}

	// 8. Unknown screening ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/trust-safety/screenings/unknown-screening-id/decision", bytes.NewReader(rejectPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown screening ID, got %d", rec404.Code)
	}
}

func TestTrustSafetyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/trust-safety/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/trust-safety/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/trust-safety/stream, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("expected text/event-stream Content-Type, got %q", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed reading initial SSE comment: %v", err)
	}
	if !strings.Contains(line, ": connected") {
		t.Errorf("expected ': connected' prologue, got %q", line)
	}
}
