package webfrontend_test

import (
	"bytes"
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

func TestVerticalRescueHandlers_SeedEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	sseCh, unsubSSE := server.ReunionHub().Subscribe("vertical_rescue")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("vertical_rescue", "vertical-rescue-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /vertical-rope-rescue renders HTML with seeded ops and KPI counters
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/vertical-rope-rescue", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /vertical-rope-rescue, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	bodyStr := recPage.Body.String()
	for _, token := range []string{
		"stat-total-vertical-ops",
		"stat-critical-fall-alerts",
		"stat-rigged-twin-lines",
		"stat-lowered-safe-count",
		"VR-OP-101",
		"VR-OP-102",
		"VR-OP-103",
		"canopy-crown-gale",
		"cliff-ledge-crag",
		"highrise-balcony-ledge",
	} {
		if !strings.Contains(bodyStr, token) {
			t.Errorf("expected HTML to contain %q", token)
		}
	}

	// 2. GET /api/v1/vertical-rescue/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/vertical-rescue/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/vertical-rescue/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations        []domain.VerticalRescueOperation `json:"operations"`
		Total             int                              `json:"total"`
		CriticalFallCount int                              `json:"criticalFallCount"`
		RiggedTwinCount   int                              `json:"riggedTwinCount"`
		LoweredSafeCount  int                              `json:"loweredSafeCount"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal operations list: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Total != 3 {
		t.Fatalf("expected 3 seeded vertical rescue operations, got %d", len(listResp.Operations))
	}
	if listResp.CriticalFallCount != 1 || listResp.RiggedTwinCount != 1 || listResp.LoweredSafeCount != 1 {
		t.Errorf("unexpected summary counts: %+v", listResp)
	}

	// 2b. Filter by status & environment
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/vertical-rescue/operations?status=FALL_SHOCK_HAZARD_CRITICAL&environment=TREE_CANOPY_CROWN", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET, got %d", recFilter.Code)
	}

	// 3. POST /api/v1/vertical-rescue/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/vertical-rescue/evaluate creates a new operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                "PET-VR-950",
		"petName":              "Nimbus",
		"species":              "Feline",
		"weightKg":             4.0,
		"environmentType":      "TREE_CANOPY_CROWN",
		"elevationMeters":      25.0,
		"slopeAngleDeg":        86.0,
		"windGustKnots":        29.0,
		"hoursStranded":        32.0,
		"branchStabilityIndex": 20,
		"leadRiggerCallsign":   "ARBOR-12",
		"notes":                "Gale gusts in brittle crown",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/vertical-rescue/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.VerticalRescueOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "VR-OP-104" {
		t.Errorf("expected VR-OP-104, got %s", created.OperationID)
	}
	if !created.HighRiseFallShockAlert || !created.CanopyFailureAlert || created.Status != domain.VerticalStatusFallShockCritical {
		t.Errorf("expected critical fall shock and canopy failure alerts on VR-OP-104, got %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-sseCh:
		if evt.Type != domain.ReunionEventType("vertical_rescue_updated") {
			t.Errorf("expected vertical_rescue_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for vertical_rescue_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "vertical_rescue_updated" {
			t.Errorf("expected vertical_rescue_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for vertical_rescue_updated mesh event")
	}

	// 5. POST /api/v1/vertical-rescue/operations/VR-OP-101/step updates remaining steps
	for _, stepID := range []string{"vr-step-throwline-cambium", "vr-step-calming-bag", "vr-step-controlled-lower"} {
		stepBody, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/operations/VR-OP-101/step", bytes.NewReader(stepBody))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/vertical-rescue/operations/VR-OP-101/clearance issues certificate
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/operations/VR-OP-101/clearance", nil)
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance on VR-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.VerticalRescueOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.VerticalStatusLoweredGroundSafe {
		t.Errorf("expected LOWERED_GROUND_SAFE, got %s", cleared.Status)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "VERTICAL-SAFE-") {
		t.Errorf("expected VERTICAL-SAFE- certificate hash, got %q", cleared.ClearanceCertificateHash)
	}

	// 7. 404 Not Found on unknown operation ID for step & clearance
	stepBody, _ := json.Marshal(map[string]any{"stepId": "vr-step-dropzone-net", "completed": true})
	recStep404 := httptest.NewRecorder()
	reqStep404 := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/operations/VR-UNKNOWN/step", bytes.NewReader(stepBody))
	server.ServeHTTP(recStep404, reqStep404)
	if recStep404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown step op, got %d", recStep404.Code)
	}

	recClear404 := httptest.NewRecorder()
	reqClear404 := httptest.NewRequest(http.MethodPost, "/api/v1/vertical-rescue/operations/VR-UNKNOWN/clearance", nil)
	server.ServeHTTP(recClear404, reqClear404)
	if recClear404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown clearance op, got %d", recClear404.Code)
	}
}
