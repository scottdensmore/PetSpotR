package webfrontend_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
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

func TestICS_VolunteersSeedingAndRegistration(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. GET /ics-command should render HTML and seed default volunteers
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/ics-command", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /ics-command, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}

	// 2. GET /api/v1/ics/volunteers should return 3 seeded SAR volunteers sorted by TotalXP descending
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/ics/volunteers", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/ics/volunteers, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Volunteers []domain.VolunteerProfile `json:"volunteers"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal volunteers response: %v", err)
	}
	if len(listResp.Volunteers) != 3 {
		t.Fatalf("expected 3 seeded volunteers, got %d", len(listResp.Volunteers))
	}

	// Verify descending XP sort order: vol-sar-101 > vol-sar-102 > vol-sar-103
	if listResp.Volunteers[0].VolunteerID != "vol-sar-101" {
		t.Errorf("expected highest XP volunteer to be vol-sar-101, got %s", listResp.Volunteers[0].VolunteerID)
	}
	if listResp.Volunteers[0].TotalXP < listResp.Volunteers[1].TotalXP || listResp.Volunteers[1].TotalXP < listResp.Volunteers[2].TotalXP {
		t.Errorf("expected volunteers sorted by TotalXP descending: %d, %d, %d",
			listResp.Volunteers[0].TotalXP, listResp.Volunteers[1].TotalXP, listResp.Volunteers[2].TotalXP)
	}
	if len(listResp.Volunteers[0].Credentials) < 2 {
		t.Errorf("expected vol-sar-101 to have 2 signed credentials, got %d", len(listResp.Volunteers[0].Credentials))
	}

	// 3. POST /api/v1/ics/volunteers creates a new volunteer
	createBody, _ := json.Marshal(map[string]any{
		"name":     "Jordan Vance",
		"callsign": "EAGLE-7",
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/ics/volunteers", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/ics/volunteers, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var createResp struct {
		Volunteer domain.VolunteerProfile `json:"volunteer"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create volunteer response: %v", err)
	}
	if !strings.HasPrefix(createResp.Volunteer.VolunteerID, "vol-") {
		t.Errorf("expected generated volunteerId starting with 'vol-', got %q", createResp.Volunteer.VolunteerID)
	}
	if createResp.Volunteer.PrimaryRole != domain.RoleFieldSearcher {
		t.Errorf("expected default primaryRole %q, got %q", domain.RoleFieldSearcher, createResp.Volunteer.PrimaryRole)
	}
	if len(createResp.Volunteer.Credentials) != 1 || createResp.Volunteer.Credentials[0].Role != domain.RoleFieldSearcher {
		t.Errorf("expected signed FIELD_SEARCHER credential on new volunteer, got %+v", createResp.Volunteer.Credentials)
	}

	// 4. Invalid volunteer registration returns 400 Bad Request
	invalidBody, _ := json.Marshal(map[string]any{
		"name":     "",
		"callsign": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/ics/volunteers", bytes.NewReader(invalidBody))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid volunteer, got %d", recInvalid.Code)
	}
}

func TestICS_RoleGateShiftProgressionAndCSVExport(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "ics" topic and Mesh signaling hub
	icsSSECh, unsubSSE := server.ReunionHub().Subscribe("ics")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("ics", "ics-command-post")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. Register an uncredentialed FIELD_SEARCHER volunteer
	volReq, _ := json.Marshal(map[string]any{
		"name":        "Samira Khan",
		"callsign":    "HAWK-4",
		"primaryRole": "FIELD_SEARCHER",
	})
	recVol := httptest.NewRecorder()
	reqVol := httptest.NewRequest(http.MethodPost, "/api/v1/ics/volunteers", bytes.NewReader(volReq))
	reqVol.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recVol, reqVol)
	if recVol.Code != http.StatusCreated {
		t.Fatalf("failed to create volunteer: %d %s", recVol.Code, recVol.Body.String())
	}
	var volResp struct {
		Volunteer domain.VolunteerProfile `json:"volunteer"`
	}
	_ = json.Unmarshal(recVol.Body.Bytes(), &volResp)
	volunteerID := volResp.Volunteer.VolunteerID

	// Drain initial volunteer creation SSE event if any
	select {
	case <-icsSSECh:
	case <-time.After(50 * time.Millisecond):
	}

	// 2. Attempt to check into UAV_PART107_PILOT shift without credential -> 403 Forbidden
	shiftDenyReq, _ := json.Marshal(map[string]any{
		"incidentId":       "INC-2026-WILDFIRE-01",
		"volunteerId":      volunteerID,
		"assignedRole":     "UAV_PART107_PILOT",
		"assignedSectorId": "SECTOR-BRAVO-2",
	})
	recDeny := httptest.NewRecorder()
	reqDeny := httptest.NewRequest(http.MethodPost, "/api/v1/ics/shifts", bytes.NewReader(shiftDenyReq))
	reqDeny.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recDeny, reqDeny)
	if recDeny.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for uncredentialed UAV_PART107_PILOT shift check-in, got %d: %s", recDeny.Code, recDeny.Body.String())
	}
	var errResp map[string]string
	_ = json.Unmarshal(recDeny.Body.Bytes(), &errResp)
	if !strings.Contains(errResp["error"], "UAV_PART107_PILOT") {
		t.Errorf("expected error message to mention missing UAV_PART107_PILOT credential, got %q", errResp["error"])
	}

	// 3. Issue signed UAV_PART107_PILOT credential via POST /api/v1/ics/volunteers/{id}/credentials
	credReq, _ := json.Marshal(map[string]any{
		"role":          "UAV_PART107_PILOT",
		"issuedBy":      "FAA / NASAR Command",
		"licenseNumber": "FAA-107-994812",
	})
	recCred := httptest.NewRecorder()
	reqCred := httptest.NewRequest(http.MethodPost, "/api/v1/ics/volunteers/"+volunteerID+"/credentials", bytes.NewReader(credReq))
	reqCred.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCred, reqCred)
	if recCred.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for credential issuance, got %d: %s", recCred.Code, recCred.Body.String())
	}
	var credResp struct {
		Volunteer  domain.VolunteerProfile `json:"volunteer"`
		Credential domain.SignedCredential `json:"credential"`
	}
	if err := json.Unmarshal(recCred.Body.Bytes(), &credResp); err != nil {
		t.Fatalf("failed to unmarshal credential response: %v", err)
	}
	if credResp.Credential.SignatureHex == "" || credResp.Credential.Role != domain.RoleUAVPart107Pilot {
		t.Errorf("expected signed UAV_PART107_PILOT credential, got %+v", credResp.Credential)
	}

	// Verify SSE event "ics_volunteer_updated"
	select {
	case evt := <-icsSSECh:
		if evt.Type != domain.ReunionEventType("ics_volunteer_updated") {
			t.Errorf("expected ics_volunteer_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ics_volunteer_updated SSE event")
	}

	// 4. Retry UAV_PART107_PILOT shift check-in -> 201 Created
	recShift := httptest.NewRecorder()
	reqShift := httptest.NewRequest(http.MethodPost, "/api/v1/ics/shifts", bytes.NewReader(shiftDenyReq))
	reqShift.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recShift, reqShift)
	if recShift.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created after credential issuance, got %d: %s", recShift.Code, recShift.Body.String())
	}
	var shiftResp struct {
		Shift domain.ICSShiftEntry `json:"shift"`
	}
	if err := json.Unmarshal(recShift.Body.Bytes(), &shiftResp); err != nil {
		t.Fatalf("failed to unmarshal shift response: %v", err)
	}
	shiftID := shiftResp.Shift.ShiftID
	if shiftResp.Shift.FatigueLevel != domain.FatigueLevelNormal || shiftResp.Shift.Status != domain.ShiftStatusOnSectorPatrol {
		t.Errorf("unexpected initial shift fatigue/status: %+v", shiftResp.Shift)
	}

	// Drain shift creation SSE event
	select {
	case evt := <-icsSSECh:
		if evt.Type != domain.ReunionEventType("ics_shift_updated") {
			t.Errorf("expected ics_shift_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ics_shift_updated SSE event")
	}

	// 5. Update shift progress to 6.5 hours -> FATIGUE_WARNING
	progWarnReq, _ := json.Marshal(map[string]any{
		"addHours":          6.5,
		"addDistanceKm":     19.5,
		"verifiedSightings": 1,
	})
	recWarn := httptest.NewRecorder()
	reqWarn := httptest.NewRequest(http.MethodPost, "/api/v1/ics/shifts/"+shiftID+"/progress", bytes.NewReader(progWarnReq))
	reqWarn.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recWarn, reqWarn)
	if recWarn.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shift progress update, got %d: %s", recWarn.Code, recWarn.Body.String())
	}
	var progWarnResp struct {
		Shift     domain.ICSShiftEntry    `json:"shift"`
		Volunteer domain.VolunteerProfile `json:"volunteer"`
	}
	_ = json.Unmarshal(recWarn.Body.Bytes(), &progWarnResp)
	if progWarnResp.Shift.FatigueLevel != domain.FatigueLevelWarning {
		t.Errorf("expected FATIGUE_WARNING at 6.5h, got %q", progWarnResp.Shift.FatigueLevel)
	}
	if progWarnResp.Shift.Status != domain.ShiftStatusOnSectorPatrol {
		t.Errorf("expected ON_SECTOR_PATROL at 6.5h, got %q", progWarnResp.Shift.Status)
	}

	// 6. Update shift progress to 8.5 hours and 52.0 km with nightShift -> MANDATORY_REST
	progRestReq, _ := json.Marshal(map[string]any{
		"addHours":      2.0,
		"addDistanceKm": 32.5,
		"nightShift":    true,
	})
	recRest := httptest.NewRecorder()
	reqRest := httptest.NewRequest(http.MethodPost, "/api/v1/ics/shifts/"+shiftID+"/progress", bytes.NewReader(progRestReq))
	reqRest.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recRest, reqRest)
	if recRest.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for mandatory rest progress update, got %d: %s", recRest.Code, recRest.Body.String())
	}
	var progRestResp struct {
		Shift     domain.ICSShiftEntry    `json:"shift"`
		Volunteer domain.VolunteerProfile `json:"volunteer"`
	}
	_ = json.Unmarshal(recRest.Body.Bytes(), &progRestResp)
	if progRestResp.Shift.FatigueLevel != domain.FatigueLevelMandatoryRest {
		t.Errorf("expected MANDATORY_REST fatigue level at 8.5h, got %q", progRestResp.Shift.FatigueLevel)
	}
	if progRestResp.Shift.Status != domain.ShiftStatusMandatoryRest {
		t.Errorf("expected MANDATORY_REST status at 8.5h, got %q", progRestResp.Shift.Status)
	}

	// 7. Checkout shift (checkout: true, reunionsAssisted: 1) -> CHECKED_OUT & badges awarded
	checkoutReq, _ := json.Marshal(map[string]any{
		"reunionsAssisted": 1,
		"nightShift":       true,
		"checkout":         true,
	})
	recCheckout := httptest.NewRecorder()
	reqCheckout := httptest.NewRequest(http.MethodPost, "/api/v1/ics/shifts/"+shiftID+"/progress", bytes.NewReader(checkoutReq))
	reqCheckout.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheckout, reqCheckout)
	if recCheckout.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shift checkout, got %d: %s", recCheckout.Code, recCheckout.Body.String())
	}
	var checkoutResp struct {
		Shift     domain.ICSShiftEntry    `json:"shift"`
		Volunteer domain.VolunteerProfile `json:"volunteer"`
	}
	_ = json.Unmarshal(recCheckout.Body.Bytes(), &checkoutResp)
	if checkoutResp.Shift.Status != domain.ShiftStatusCheckedOut {
		t.Errorf("expected CHECKED_OUT status after checkout, got %q", checkoutResp.Shift.Status)
	}
	if checkoutResp.Shift.CheckedOutAt == nil {
		t.Errorf("expected CheckedOutAt timestamp to be populated on checkout")
	}
	if checkoutResp.Volunteer.CompletedShifts != 1 {
		t.Errorf("expected CompletedShifts = 1, got %d", checkoutResp.Volunteer.CompletedShifts)
	}

	badgeSet := make(map[domain.SARMeritBadge]bool)
	for _, b := range checkoutResp.Volunteer.EarnedBadges {
		badgeSet[b] = true
	}
	for _, expectedBadge := range []domain.SARMeritBadge{
		domain.BadgeFirstResponder,
		domain.BadgeAerialSpotter,
		domain.BadgeNightOwlIR,
		domain.BadgeGridMaster50Km,
		domain.BadgeReunionHero,
	} {
		if !badgeSet[expectedBadge] {
			t.Errorf("expected volunteer to earn badge %s, got %v", expectedBadge, checkoutResp.Volunteer.EarnedBadges)
		}
	}

	// 8. Export ICS-204 CSV via GET /api/v1/ics/shifts?format=csv
	recCSV := httptest.NewRecorder()
	reqCSV := httptest.NewRequest(http.MethodGet, "/api/v1/ics/shifts?format=csv", nil)
	server.ServeHTTP(recCSV, reqCSV)
	if recCSV.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ICS-204 CSV export, got %d: %s", recCSV.Code, recCSV.Body.String())
	}
	if !strings.Contains(recCSV.Header().Get("Content-Type"), "text/csv") {
		t.Errorf("expected text/csv Content-Type, got %q", recCSV.Header().Get("Content-Type"))
	}
	if !strings.Contains(recCSV.Header().Get("Content-Disposition"), "ics-204-shift-roster.csv") {
		t.Errorf("expected attachment filename ics-204-shift-roster.csv, got %q", recCSV.Header().Get("Content-Disposition"))
	}

	csvReader := csv.NewReader(strings.NewReader(recCSV.Body.String()))
	rows, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse ICS-204 CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 CSV rows (header + 1 shift), got %d", len(rows))
	}
	if rows[0][0] != "shift_id" || rows[1][0] != shiftID {
		t.Errorf("unexpected CSV rows: %+v", rows)
	}
}

func TestICS_StreamSSEEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/ics/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/ics/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/ics/stream, got %d", resp.StatusCode)
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
