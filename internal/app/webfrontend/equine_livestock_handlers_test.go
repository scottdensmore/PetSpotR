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

func TestEquineLivestockHandlers_SeedingCreateContainAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	equineSSECh, unsubSSE := server.ReunionHub().Subscribe("equine_livestock")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("equine_livestock", "equine-livestock-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /equine-livestock should render HTML and seed default dispatches
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/equine-livestock", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /equine-livestock, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Large-Animal Containment Cockpit") {
		t.Errorf("expected /equine-livestock HTML to contain Large-Animal Containment Cockpit heading")
	}

	// 2. GET /api/v1/equine-livestock/dispatches should return the 3 seeded dispatches
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/equine-livestock/dispatches", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/equine-livestock/dispatches, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Dispatches []domain.EquineLivestockDispatch `json:"dispatches"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal equine livestock dispatches response: %v", err)
	}
	if len(listResp.Dispatches) != 3 {
		t.Fatalf("expected 3 seeded equine livestock dispatches, got %d", len(listResp.Dispatches))
	}

	byID := make(map[string]domain.EquineLivestockDispatch)
	for _, d := range listResp.Dispatches {
		byID[d.DispatchID] = d
	}

	d101, ok := byID["eq-dsp-101"]
	if !ok {
		t.Fatalf("expected seeded eq-dsp-101")
	}
	if d101.AnimalName != "Valegro & Starlight" || d101.DispatchStatus != domain.LivestockStatusStampedeLoose || d101.CollisionRisk != domain.CollisionRiskCriticalHighwayHerd || !d101.HighwayStampedeAlert {
		t.Errorf("expected eq-dsp-101 Valegro & Starlight in LOOSE_HERD_CORRIDOR_ALERT / CRITICAL_HIGHWAY_STAMPEDE_RISK with HighwayStampedeAlert=true, got %+v", d101)
	}

	d102, ok := byID["eq-dsp-102"]
	if !ok {
		t.Fatalf("expected seeded eq-dsp-102")
	}
	if d102.AnimalName != "Andes Alpaca Trio" || d102.DispatchStatus != domain.LivestockStatusPanelsCorralled || d102.CollisionRisk != domain.CollisionRiskElevatedPastureBreach {
		t.Errorf("expected eq-dsp-102 Andes Alpaca Trio in PORTABLE_CORRAL_STAGED / ELEVATED_PASTURE_PERIMETER_RISK, got %+v", d102)
	}

	d103, ok := byID["eq-dsp-103"]
	if !ok {
		t.Fatalf("expected seeded eq-dsp-103")
	}
	if d103.AnimalName != "Clover & Bramble" || d103.DispatchStatus != domain.LivestockStatusLoadedSafeHaul || d103.CollisionRisk != domain.CollisionRiskContainedPaddockSafe || d103.PaddockClearanceCode != "PADDOCK-SAFE-EQ-DSP-103" {
		t.Errorf("expected eq-dsp-103 Clover & Bramble in LOADED_SAFE_PADDOCK_CUSTODY / CONTAINED_PADDOCK_SAFE with PaddockClearanceCode, got %+v", d103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/equine-livestock/dispatches?petId=pet-2026-001&status=LOOSE_HERD_CORRIDOR_ALERT", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/equine-livestock/dispatches, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Dispatches []domain.EquineLivestockDispatch `json:"dispatches"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered dispatches: %v", err)
	}
	if len(filteredResp.Dispatches) != 1 || filteredResp.Dispatches[0].DispatchID != "eq-dsp-101" {
		t.Errorf("expected 1 filtered dispatch (eq-dsp-101), got %+v", filteredResp.Dispatches)
	}

	// 3. POST /api/v1/equine-livestock/dispatches with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":      "",
		"animalName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/equine-livestock/dispatches", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid equine livestock dispatch, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/equine-livestock/dispatches creates a new dispatch (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":                  "pet-2026-001",
		"animalName":             "Secretariat II",
		"animalCategory":         "EQUINE_WARMBLOOD",
		"headCount":              1,
		"estimatedWeightKg":      520,
		"breachLocation":         "SR-202 Milepost 14 Highway Corridor",
		"stockTrailerDispatched": false,
		"corralPanelsErected":    false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/equine-livestock/dispatches", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/equine-livestock/dispatches, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.EquineLivestockDispatch
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created livestock dispatch: %v", err)
	}
	if !strings.HasPrefix(created.DispatchID, "eq-dsp-") {
		t.Errorf("expected DispatchID prefix 'eq-dsp-', got %q", created.DispatchID)
	}
	if !created.HighwayStampedeAlert || created.DispatchStatus != domain.LivestockStatusStampedeLoose {
		t.Errorf("expected new dispatch HighwayStampedeAlert=true and LOOSE_HERD_CORRIDOR_ALERT, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-equineSSECh:
		if evt.Type != domain.ReunionEventType("livestock_dispatch_updated") {
			t.Errorf("expected livestock_dispatch_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for livestock_dispatch_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "livestock_dispatch_updated" {
			t.Errorf("expected livestock_dispatch_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for livestock_dispatch_updated mesh event")
	}

	// 5. POST /api/v1/equine-livestock/dispatches/eq-dsp-101/contain completes safe trailer load
	containPayload, _ := json.Marshal(map[string]any{
		"completeSafeTrailerLoad": true,
	})
	recContain := httptest.NewRecorder()
	reqContain := httptest.NewRequest(http.MethodPost, "/api/v1/equine-livestock/dispatches/eq-dsp-101/contain", bytes.NewReader(containPayload))
	reqContain.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recContain, reqContain)
	if recContain.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST contain, got %d: %s", recContain.Code, recContain.Body.String())
	}

	var updated domain.EquineLivestockDispatch
	if err := json.Unmarshal(recContain.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated livestock dispatch: %v", err)
	}
	if updated.DispatchStatus != domain.LivestockStatusLoadedSafeHaul {
		t.Errorf("expected LOADED_SAFE_PADDOCK_CUSTODY, got %s", updated.DispatchStatus)
	}
	if updated.CollisionRisk != domain.CollisionRiskContainedPaddockSafe {
		t.Errorf("expected CONTAINED_PADDOCK_SAFE, got %s", updated.CollisionRisk)
	}
	if updated.PaddockClearanceCode != "PADDOCK-SAFE-EQ-DSP-101" {
		t.Errorf("expected PADDOCK-SAFE-EQ-DSP-101, got %s", updated.PaddockClearanceCode)
	}

	// 6. Unknown dispatch ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/equine-livestock/dispatches/unknown-dispatch-id/contain", bytes.NewReader(containPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown dispatch ID, got %d", rec404.Code)
	}
}

func TestEquineLivestockHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/equine-livestock/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/equine-livestock/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/equine-livestock/stream, got %d", resp.StatusCode)
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
