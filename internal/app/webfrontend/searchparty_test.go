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
