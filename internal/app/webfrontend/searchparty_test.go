package webfrontend_test

import (
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
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestSearchPartyEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	// Seed lost pet with coordinates
	lostPet := domain.LostPetRecord{
		PetID:      "lost-target-2",
		PetName:    "Luna",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.620,
			Longitude: -122.320,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	// Seed lost pet without coordinates
	lostPetNoCoords := domain.LostPetRecord{
		PetID:      "lost-no-coords",
		PetName:    "Shadow",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Status:     domain.LostPetStatusLost,
	}
	noCoordsBytes, _ := json.Marshal(lostPetNoCoords)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPetNoCoords.PetID, noCoordsBytes)

	var createdPartyID string

	t.Run("POST /api/v1/lost-pets/{id}/search-party creates party", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var party searchparty.SearchParty
		if err := json.Unmarshal(rec.Body.Bytes(), &party); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if party.PartyID == "" {
			t.Fatal("expected non-empty partyId in response")
		}
		if party.LostPetID != "lost-target-2" {
			t.Errorf("expected lostPetId 'lost-target-2', got %q", party.LostPetID)
		}
		if len(party.Sectors) != 4 {
			t.Fatalf("expected 4 sectors by default, got %d", len(party.Sectors))
		}
		for _, s := range party.Sectors {
			if s.Status != searchparty.SectorStatusUnassigned {
				t.Errorf("expected initial sector status 'unassigned', got %s", s.Status)
			}
		}
		if party.CoveragePercentage != 0.0 {
			t.Errorf("expected initial coverage 0.0, got %f", party.CoveragePercentage)
		}
		if party.ActiveVolunteersCount != 0 {
			t.Errorf("expected 0 active volunteers, got %d", party.ActiveVolunteersCount)
		}

		createdPartyID = party.PartyID
	})

	t.Run("POST /api/v1/lost-pets/{id}/search-party idempotent returns existing party", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("expected 200 or 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var party searchparty.SearchParty
		if err := json.Unmarshal(rec.Body.Bytes(), &party); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if party.PartyID != createdPartyID {
			t.Errorf("expected partyId %s, got %s", createdPartyID, party.PartyID)
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/search-party returns party", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var party searchparty.SearchParty
		if err := json.Unmarshal(rec.Body.Bytes(), &party); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if party.PartyID != createdPartyID {
			t.Errorf("expected partyId %s, got %s", createdPartyID, party.PartyID)
		}
		if len(party.Sectors) != 4 {
			t.Errorf("expected 4 sectors, got %d", len(party.Sectors))
		}
	})

	t.Run("GET /api/v1/lost-pets/non-existent/search-party returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/non-existent/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/lost-pets/non-existent/search-party returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/non-existent/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/lost-pets/lost-no-coords/search-party returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-no-coords/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/{sectorId}/claim claims sector", func(t *testing.T) {
		body := map[string]string{
			"volunteerAlias": "Volunteer #1",
		}
		data, _ := json.Marshal(body)
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/sector-1/claim"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["status"] != string(searchparty.SectorStatusActiveSearch) {
			t.Errorf("expected status 'active_search', got %v", resp["status"])
		}
		if resp["volunteerAlias"] != "Volunteer #1" {
			t.Errorf("expected volunteerAlias 'Volunteer #1', got %v", resp["volunteerAlias"])
		}

		// Verify party state updated
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)

		var updatedParty searchparty.SearchParty
		_ = json.Unmarshal(getRec.Body.Bytes(), &updatedParty)
		if updatedParty.ActiveVolunteersCount != 1 {
			t.Errorf("expected 1 active volunteer, got %d", updatedParty.ActiveVolunteersCount)
		}
		if updatedParty.Sectors[0].Status != searchparty.SectorStatusActiveSearch {
			t.Errorf("expected sector-1 status 'active_search', got %s", updatedParty.Sectors[0].Status)
		}
		if len(updatedParty.ActiveAssignments) != 1 {
			t.Fatalf("expected 1 active assignment, got %d", len(updatedParty.ActiveAssignments))
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/{sectorId}/claim Zero-PII sanitization", func(t *testing.T) {
		// Provide an email address as alias; system must sanitize to pseudonym
		body := map[string]string{
			"volunteerAlias": "jane.volunteer@example.com",
		}
		data, _ := json.Marshal(body)
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/sector-2/claim"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		alias, _ := resp["volunteerAlias"].(string)
		if strings.Contains(alias, "@") || strings.Contains(alias, "example.com") {
			t.Fatalf("Zero-PII violation: alias contains email: %s", alias)
		}
		if !strings.HasPrefix(alias, "Volunteer #") {
			t.Errorf("expected pseudonym prefix 'Volunteer #', got %s", alias)
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/{sectorId}/claim already claimed returns 409", func(t *testing.T) {
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/sector-1/claim"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/{sectorId}/status updates status and coverage", func(t *testing.T) {
		body := map[string]string{
			"status":         string(searchparty.SectorStatusCleared),
			"clearanceNotes": "Checked park and alleys; no sightings.",
		}
		data, _ := json.Marshal(body)
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/sector-1/status"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["status"] != string(searchparty.SectorStatusCleared) {
			t.Errorf("expected status 'cleared', got %v", resp["status"])
		}
		cov, ok := resp["coveragePercentage"].(float64)
		if !ok || cov != 25.0 {
			t.Errorf("expected 25%% coverage (1 of 4 sectors cleared), got %v", resp["coveragePercentage"])
		}

		// Verify GET reflection
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)

		var updatedParty searchparty.SearchParty
		_ = json.Unmarshal(getRec.Body.Bytes(), &updatedParty)
		if updatedParty.CoveragePercentage != 25.0 {
			t.Errorf("expected 25%% coverage on party, got %f", updatedParty.CoveragePercentage)
		}
		if updatedParty.Sectors[0].Status != searchparty.SectorStatusCleared {
			t.Errorf("expected sector-1 status 'cleared', got %s", updatedParty.Sectors[0].Status)
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/{sectorId}/status rejects invalid status", func(t *testing.T) {
		body := map[string]string{
			"status": "bogus_status",
		}
		data, _ := json.Marshal(body)
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/sector-1/status"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid status, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/search-parties/non-existent/sectors/sector-1/claim returns 404", func(t *testing.T) {
		url := "/api/v1/search-parties/non-existent-party/sectors/sector-1/claim"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/search-parties/{partyId}/sectors/nonexistent-sector/claim returns 404", func(t *testing.T) {
		url := "/api/v1/search-parties/" + createdPartyID + "/sectors/non-existent-sector/claim"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}

func TestSearchPartyRealtimeBroadcast(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		ReunionHub:               hub,
	})

	lostPet := domain.LostPetRecord{
		PetID:      "lost-broadcast-pet",
		PetName:    "Cooper",
		ReportedAt: time.Now().UTC().Add(-2 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.609,
			Longitude: -122.333,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	// Seed an active match associated with the lost pet
	match := domain.MatchRecord{
		MatchID:    "match-party-broadcast-1",
		LostPetID:  "lost-broadcast-pet",
		FoundPetID: "found-party-broadcast-1",
		Status:     domain.MatchStatusPendingReview,
	}
	matchBytes, _ := json.Marshal(match)
	_ = st.SaveState(context.Background(), store.MatchesCollection, match.MatchID, matchBytes)

	// Create search party
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-broadcast-pet/search-party", nil)
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("failed to create search party, status %d: %s", createRec.Code, createRec.Body.String())
	}
	var party searchparty.SearchParty
	if err := json.Unmarshal(createRec.Body.Bytes(), &party); err != nil {
		t.Fatalf("failed to parse search party: %v", err)
	}
	partyID := party.PartyID

	// Subscribe to lostPetID, matchID, and partyID
	petSubCh, petUnsub := hub.Subscribe("lost-broadcast-pet")
	defer petUnsub()
	matchSubCh, matchUnsub := hub.Subscribe("match-party-broadcast-1")
	defer matchUnsub()
	partySubCh, partyUnsub := hub.Subscribe(partyID)
	defer partyUnsub()

	// 1. Test that claiming a sector emits search_party_updated on ReunionHub
	t.Run("claim sector emits search_party_updated", func(t *testing.T) {
		claimBody, _ := json.Marshal(map[string]string{
			"volunteerAlias": "Volunteer Alice",
		})
		claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+partyID+"/sectors/sector-1/claim", bytes.NewReader(claimBody))
		claimReq.Header.Set("Content-Type", "application/json")
		claimRec := httptest.NewRecorder()
		srv.ServeHTTP(claimRec, claimReq)

		if claimRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", claimRec.Code, claimRec.Body.String())
		}

		select {
		case evt := <-partySubCh:
			if evt.Type != domain.ReunionEventSearchPartyUpdated {
				t.Errorf("expected event Type %q, got %q", domain.ReunionEventSearchPartyUpdated, evt.Type)
			}
			payload, ok := evt.Payload.(domain.SearchPartyEventPayload)
			if !ok {
				t.Fatalf("expected payload domain.SearchPartyEventPayload, got %T", evt.Payload)
			}
			if payload.Type != "search_party_updated" {
				t.Errorf("expected payload type search_party_updated, got %q", payload.Type)
			}
			if payload.PartyID != partyID {
				t.Errorf("expected partyId %q, got %q", partyID, payload.PartyID)
			}
			if payload.SectorID != "sector-1" {
				t.Errorf("expected sectorId 'sector-1', got %q", payload.SectorID)
			}
			if payload.Status != string(searchparty.SectorStatusActiveSearch) {
				t.Errorf("expected status %q, got %q", searchparty.SectorStatusActiveSearch, payload.Status)
			}
			if payload.ActiveVolunteersCount != 1 {
				t.Errorf("expected 1 active volunteer, got %d", payload.ActiveVolunteersCount)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated event on partySubCh")
		}

		// Also check that petSubCh and matchSubCh receive the event
		select {
		case evt := <-petSubCh:
			if evt.Type != domain.ReunionEventSearchPartyUpdated {
				t.Errorf("expected event Type %q on petSubCh, got %q", domain.ReunionEventSearchPartyUpdated, evt.Type)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated event on petSubCh")
		}

		select {
		case evt := <-matchSubCh:
			if evt.Type != domain.ReunionEventSearchPartyUpdated {
				t.Errorf("expected event Type %q on matchSubCh, got %q", domain.ReunionEventSearchPartyUpdated, evt.Type)
			}
			if evt.MatchID != "match-party-broadcast-1" {
				t.Errorf("expected MatchID 'match-party-broadcast-1', got %q", evt.MatchID)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated event on matchSubCh")
		}
	})

	// 2. Test that updating sector status emits search_party_updated on ReunionHub
	t.Run("update sector status emits search_party_updated", func(t *testing.T) {
		statusBody, _ := json.Marshal(map[string]string{
			"status":         string(searchparty.SectorStatusCleared),
			"clearanceNotes": "Sector thoroughly checked",
		})
		statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+partyID+"/sectors/sector-1/status", bytes.NewReader(statusBody))
		statusReq.Header.Set("Content-Type", "application/json")
		statusRec := httptest.NewRecorder()
		srv.ServeHTTP(statusRec, statusReq)

		if statusRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", statusRec.Code, statusRec.Body.String())
		}

		select {
		case evt := <-partySubCh:
			if evt.Type != domain.ReunionEventSearchPartyUpdated {
				t.Errorf("expected event Type %q, got %q", domain.ReunionEventSearchPartyUpdated, evt.Type)
			}
			payload, ok := evt.Payload.(domain.SearchPartyEventPayload)
			if !ok {
				t.Fatalf("expected payload domain.SearchPartyEventPayload, got %T", evt.Payload)
			}
			if payload.PartyID != partyID {
				t.Errorf("expected partyId %q, got %q", partyID, payload.PartyID)
			}
			if payload.SectorID != "sector-1" {
				t.Errorf("expected sectorId 'sector-1', got %q", payload.SectorID)
			}
			if payload.Status != string(searchparty.SectorStatusCleared) {
				t.Errorf("expected status %q, got %q", searchparty.SectorStatusCleared, payload.Status)
			}
			if payload.CoveragePercentage != 25.0 {
				t.Errorf("expected 25%% coverage, got %f", payload.CoveragePercentage)
			}
			if payload.ActiveVolunteersCount != 0 {
				t.Errorf("expected 0 active volunteers, got %d", payload.ActiveVolunteersCount)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated event on partySubCh")
		}

		// Drain petSubCh and matchSubCh
		select {
		case <-petSubCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated on petSubCh")
		}
		select {
		case <-matchSubCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for search_party_updated on matchSubCh")
		}
	})

	// 3. Test that volunteers in sighting_reported sectors are counted as active volunteers
	t.Run("volunteers in sighting_reported sectors are counted as active", func(t *testing.T) {
		// Claim sector-2
		claimBody, _ := json.Marshal(map[string]string{
			"volunteerAlias": "Volunteer Bob",
		})
		claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+partyID+"/sectors/sector-2/claim", bytes.NewReader(claimBody))
		claimReq.Header.Set("Content-Type", "application/json")
		claimRec := httptest.NewRecorder()
		srv.ServeHTTP(claimRec, claimReq)
		if claimRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", claimRec.Code)
		}
		// Drain the claim event
		select {
		case <-partySubCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for claim event on partySubCh")
		}
		select {
		case <-petSubCh:
		case <-time.After(500 * time.Millisecond):
		}
		select {
		case <-matchSubCh:
		case <-time.After(500 * time.Millisecond):
		}

		// Update sector-2 to sighting_reported
		statusBody, _ := json.Marshal(map[string]string{
			"status":         string(searchparty.SectorStatusSightingReported),
			"clearanceNotes": "Saw dog barking near fence",
		})
		statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+partyID+"/sectors/sector-2/status", bytes.NewReader(statusBody))
		statusReq.Header.Set("Content-Type", "application/json")
		statusRec := httptest.NewRecorder()
		srv.ServeHTTP(statusRec, statusReq)
		if statusRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", statusRec.Code)
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(statusRec.Body.Bytes(), &resp)
		if count, ok := resp["activeVolunteersCount"].(float64); !ok || int(count) != 1 {
			t.Errorf("expected 1 active volunteer in response for sighting_reported, got %v", resp["activeVolunteersCount"])
		}

		// Check broadcast event
		select {
		case evt := <-partySubCh:
			payload, ok := evt.Payload.(domain.SearchPartyEventPayload)
			if !ok {
				t.Fatalf("expected domain.SearchPartyEventPayload, got %T", evt.Payload)
			}
			if payload.Status != string(searchparty.SectorStatusSightingReported) {
				t.Errorf("expected status %s, got %s", searchparty.SectorStatusSightingReported, payload.Status)
			}
			if payload.ActiveVolunteersCount != 1 {
				t.Errorf("expected 1 active volunteer in payload, got %d", payload.ActiveVolunteersCount)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for sighting_reported event on partySubCh")
		}

		// Drain pet and match
		select {
		case <-petSubCh:
		case <-time.After(500 * time.Millisecond):
		}
		select {
		case <-matchSubCh:
		case <-time.After(500 * time.Millisecond):
		}

		// Verify GET returns activeVolunteersCount == 1
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-broadcast-pet/search-party", nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)
		var p searchparty.SearchParty
		_ = json.Unmarshal(getRec.Body.Bytes(), &p)
		if p.ActiveVolunteersCount != 1 {
			t.Errorf("expected GET search party activeVolunteersCount=1, got %d", p.ActiveVolunteersCount)
		}
	})

	// 4. Test that claiming a sector in sighting_reported returns 409 Conflict
	t.Run("claim sector in sighting_reported returns 409", func(t *testing.T) {
		claimBody, _ := json.Marshal(map[string]string{
			"volunteerAlias": "Volunteer Charlie",
		})
		claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+partyID+"/sectors/sector-2/claim", bytes.NewReader(claimBody))
		claimReq.Header.Set("Content-Type", "application/json")
		claimRec := httptest.NewRecorder()
		srv.ServeHTTP(claimRec, claimReq)

		if claimRec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict when claiming sector in sighting_reported, got %d: %s", claimRec.Code, claimRec.Body.String())
		}
	})
}

func TestSearchPartySectorCountCap(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	lostPet := domain.LostPetRecord{
		PetID:      "lost-cap-pet",
		PetName:    "Cap",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.60,
			Longitude: -122.33,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	body, _ := json.Marshal(map[string]interface{}{
		"sectorCount": 64,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-cap-pet/search-party", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	var party searchparty.SearchParty
	if err := json.Unmarshal(rec.Body.Bytes(), &party); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(party.Sectors) != 32 {
		t.Errorf("expected sector count capped at 32, got %d", len(party.Sectors))
	}
}

func TestClaimSectorInSightingReportedReturns409(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	lostPet := domain.LostPetRecord{
		PetID:      "lost-sr-pet",
		PetName:    "Buddy",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.60,
			Longitude: -122.33,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	// Create party
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-sr-pet/search-party", nil)
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)

	var party searchparty.SearchParty
	_ = json.Unmarshal(createRec.Body.Bytes(), &party)

	// Update sector-1 to sighting_reported directly
	statusBody, _ := json.Marshal(map[string]string{
		"status": string(searchparty.SectorStatusSightingReported),
	})
	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+party.PartyID+"/sectors/sector-1/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")
	statusRec := httptest.NewRecorder()
	srv.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", statusRec.Code, statusRec.Body.String())
	}

	// Now try to claim sector-1 (which is currently in sighting_reported)
	claimBody, _ := json.Marshal(map[string]string{
		"volunteerAlias": "Volunteer Dave",
	})
	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+party.PartyID+"/sectors/sector-1/claim", bytes.NewReader(claimBody))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	srv.ServeHTTP(claimRec, claimReq)

	if claimRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when claiming sector in sighting_reported status, got %d: %s", claimRec.Code, claimRec.Body.String())
	}
}

func TestActiveVolunteersCountIncludesSightingReported(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	lostPet := domain.LostPetRecord{
		PetID:      "lost-sr-count-pet",
		PetName:    "Milo",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.60,
			Longitude: -122.33,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	// Create party
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-sr-count-pet/search-party", nil)
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)

	var party searchparty.SearchParty
	_ = json.Unmarshal(createRec.Body.Bytes(), &party)

	// Claim sector-1
	claimBody, _ := json.Marshal(map[string]string{"volunteerAlias": "Alice"})
	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+party.PartyID+"/sectors/sector-1/claim", bytes.NewReader(claimBody))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	srv.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", claimRec.Code)
	}

	// Update sector-1 to sighting_reported
	statusBody, _ := json.Marshal(map[string]string{"status": string(searchparty.SectorStatusSightingReported)})
	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+party.PartyID+"/sectors/sector-1/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")
	statusRec := httptest.NewRecorder()
	srv.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", statusRec.Code)
	}

	var statusResp map[string]interface{}
	_ = json.Unmarshal(statusRec.Body.Bytes(), &statusResp)
	if count, ok := statusResp["activeVolunteersCount"].(float64); !ok || int(count) != 1 {
		t.Fatalf("expected activeVolunteersCount=1 in status response, got %v", statusResp["activeVolunteersCount"])
	}

	// Also verify GET
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-sr-count-pet/search-party", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	var getParty searchparty.SearchParty
	_ = json.Unmarshal(getRec.Body.Bytes(), &getParty)
	if getParty.ActiveVolunteersCount != 1 {
		t.Errorf("expected GET search party activeVolunteersCount=1, got %d", getParty.ActiveVolunteersCount)
	}
}

func TestSearchParty_SectorUrgencyAndPriorityAnnotation(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	petID := "lost-party-urgency"
	now := time.Now().UTC()
	pet := domain.LostPetRecord{
		PetID:      petID,
		PetName:    "Cooper",
		Species:    "dog",
		ReportedAt: now.Add(-2 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6062,
			Longitude: -122.3321,
		},
		Status: domain.LostPetStatusLost,
	}
	petBytes, _ := json.Marshal(pet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes)

	// Create search party
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/"+petID+"/search-party", nil)
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", createRec.Code, createRec.Body.String())
	}

	// GET search party and check urgency annotations
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/"+petID+"/search-party", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var partyResp struct {
		searchparty.SearchParty
		CollarBeacon *domain.CollarBeaconConfig `json:"collarBeacon,omitempty"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &partyResp); err != nil {
		t.Fatalf("failed to decode search party response: %v", err)
	}

	if len(partyResp.Sectors) == 0 {
		t.Fatal("expected sectors in search party, got 0")
	}

	hasCriticalOrHigh := false
	for _, sec := range partyResp.Sectors {
		if sec.PriorityScore <= 0 {
			t.Errorf("expected sector %s priorityScore > 0, got %f", sec.SectorID, sec.PriorityScore)
		}
		if sec.UrgencyLevel == "" {
			t.Errorf("expected sector %s urgencyLevel to be populated", sec.SectorID)
		}
		if sec.UrgencyLevel == domain.SectorUrgencyCritical || sec.UrgencyLevel == domain.SectorUrgencyHigh {
			hasCriticalOrHigh = true
		}
	}

	if !hasCriticalOrHigh {
		t.Error("expected at least one sector with CRITICAL or HIGH urgency")
	}

	// Check raw JSON fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(getRec.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("failed to unmarshal raw map: %v", err)
	}
	sectorsRaw, ok := rawMap["sectors"].([]interface{})
	if !ok || len(sectorsRaw) == 0 {
		t.Fatal("expected sectors array in raw json")
	}
	firstSec := sectorsRaw[0].(map[string]interface{})
	if _, ok := firstSec["urgencyLevel"]; !ok {
		t.Errorf("expected urgencyLevel key in sector JSON: %v", firstSec)
	}
	if _, ok := firstSec["priorityScore"]; !ok {
		t.Errorf("expected priorityScore key in sector JSON: %v", firstSec)
	}
}
