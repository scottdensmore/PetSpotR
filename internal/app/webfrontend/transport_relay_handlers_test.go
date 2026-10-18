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

func TestTransportRelayHandlers_SeedingCreateAdvanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	relaySSECh, unsubSSE := server.ReunionHub().Subscribe("transport_relay")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("transport_relay", "transport-relay-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /transport-relay should render HTML and seed default missions
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/transport-relay", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /transport-relay, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Cross-Jurisdiction Pet Airlift") {
		t.Errorf("expected /transport-relay HTML to contain Cross-Jurisdiction Pet Airlift heading")
	}

	// 2. GET /api/v1/transport-relay/missions should return the 3 seeded missions
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/transport-relay/missions", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/transport-relay/missions, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Missions []domain.TransportRelayMission `json:"missions"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal transport relay missions response: %v", err)
	}
	if len(listResp.Missions) != 3 {
		t.Fatalf("expected 3 seeded transport relay missions, got %d", len(listResp.Missions))
	}

	byID := make(map[string]domain.TransportRelayMission)
	for _, m := range listResp.Missions {
		byID[m.MissionID] = m
	}

	m101, ok := byID["relay-msn-101"]
	if !ok {
		t.Fatalf("expected seeded relay-msn-101")
	}
	if m101.PetName != "Winston" || m101.RelayStatus != domain.RelayStatusPreFlightStaging || m101.ThermalRisk != domain.ThermalRiskBrachycephalicHypoxia || !m101.HypoxiaCabinAlert {
		t.Errorf("expected relay-msn-101 Winston in PRE_FLIGHT_STAGING / BRACHYCEPHALIC_CABIN_RISK with HypoxiaCabinAlert=true, got %+v", m101)
	}

	m102, ok := byID["relay-msn-102"]
	if !ok {
		t.Fatalf("expected seeded relay-msn-102")
	}
	if m102.PetName != "Saffron" || m102.RelayStatus != domain.RelayStatusInTransitActiveRelay || m102.ThermalRisk != domain.ThermalRiskTarmacExcursionWatch {
		t.Errorf("expected relay-msn-102 Saffron in IN_TRANSIT_ACTIVE_RELAY / TARMAC_THERMAL_WATCH, got %+v", m102)
	}

	m103, ok := byID["relay-msn-103"]
	if !ok {
		t.Fatalf("expected seeded relay-msn-103")
	}
	if m103.PetName != "Kodiak" || m103.RelayStatus != domain.RelayStatusDeliveredDestination || m103.ThermalRisk != domain.ThermalRiskPressurizedSafeEnvelope || m103.ManifestCompletionCode != "RELAY-MANIFEST-RELAY-MSN-103" {
		t.Errorf("expected relay-msn-103 Kodiak in DELIVERED_DESTINATION_VERIFIED / PRESSURIZED_SAFE_ENVELOPE with ManifestCompletionCode, got %+v", m103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/transport-relay/missions?petId=pet-2026-001&status=PRE_FLIGHT_STAGING", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/transport-relay/missions, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Missions []domain.TransportRelayMission `json:"missions"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered missions: %v", err)
	}
	if len(filteredResp.Missions) != 1 || filteredResp.Missions[0].MissionID != "relay-msn-101" {
		t.Errorf("expected 1 filtered mission (relay-msn-101), got %+v", filteredResp.Missions)
	}

	// 3. POST /api/v1/transport-relay/missions with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/transport-relay/missions", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid transport relay mission, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/transport-relay/missions creates a new mission (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":              "pet-2026-001",
		"petName":            "Winston",
		"species":            "DOG",
		"originHub":          "KBOI Boise Air Terminal FBO",
		"destinationHub":     "KBFI Boeing Field Seattle",
		"transportMode":      "GENERAL_AVIATION_AIRLIFT",
		"totalDistanceMiles": 410,
		"cruiseAltitudeFeet": 8500,
		"brachycephalicFlag": true,
		"usdaCviCertified":   false,
		"iataCrateVerified":  false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/transport-relay/missions", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/transport-relay/missions, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.TransportRelayMission
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created mission: %v", err)
	}
	if !strings.HasPrefix(created.MissionID, "relay-msn-") {
		t.Errorf("expected MissionID prefix 'relay-msn-', got %q", created.MissionID)
	}
	if !created.HypoxiaCabinAlert || created.RelayStatus != domain.RelayStatusPreFlightStaging {
		t.Errorf("expected new mission HypoxiaCabinAlert=true and PRE_FLIGHT_STAGING, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-relaySSECh:
		if evt.Type != domain.ReunionEventType("transport_relay_updated") {
			t.Errorf("expected transport_relay_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for transport_relay_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "transport_relay_updated" {
			t.Errorf("expected transport_relay_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for transport_relay_updated mesh event")
	}

	// 5. POST /api/v1/transport-relay/missions/relay-msn-101/advance completes relay delivery
	advPayload, _ := json.Marshal(map[string]any{
		"completeRelayDelivery": true,
	})
	recAdv := httptest.NewRecorder()
	reqAdv := httptest.NewRequest(http.MethodPost, "/api/v1/transport-relay/missions/relay-msn-101/advance", bytes.NewReader(advPayload))
	reqAdv.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recAdv, reqAdv)
	if recAdv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST advance, got %d: %s", recAdv.Code, recAdv.Body.String())
	}

	var updated domain.TransportRelayMission
	if err := json.Unmarshal(recAdv.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated mission: %v", err)
	}
	if updated.RelayStatus != domain.RelayStatusDeliveredDestination {
		t.Errorf("expected DELIVERED_DESTINATION_VERIFIED, got %s", updated.RelayStatus)
	}
	if updated.ThermalRisk != domain.ThermalRiskPressurizedSafeEnvelope {
		t.Errorf("expected PRESSURIZED_SAFE_ENVELOPE, got %s", updated.ThermalRisk)
	}
	if updated.ManifestCompletionCode != "RELAY-MANIFEST-RELAY-MSN-101" {
		t.Errorf("expected RELAY-MANIFEST-RELAY-MSN-101, got %s", updated.ManifestCompletionCode)
	}

	// 6. Unknown mission ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/transport-relay/missions/unknown-mission-id/advance", bytes.NewReader(advPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown mission ID, got %d", rec404.Code)
	}
}

func TestTransportRelayHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/transport-relay/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/transport-relay/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/transport-relay/stream, got %d", resp.StatusCode)
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
