package webfrontend_test

import (
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

func TestShelterAnalyticsEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	t.Run("GET /shelters/analytics serves HTML page or fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/shelters/analytics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("expected text/html content type, got %s", rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Shelter Analytics") {
			t.Errorf("expected Shelter Analytics in HTML, got %s", body)
		}
	})

	t.Run("GET /api/v1/shelters/analytics returns JSON report", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("expected application/json content type, got %s", rec.Header().Get("Content-Type"))
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		if resp["overallKpis"] == nil {
			t.Error("expected overallKpis key in response")
		}
	})

	t.Run("GET /api/v1/shelters/analytics/export.csv streams CSV file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.csv", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
			t.Errorf("expected text/csv content type, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
			t.Errorf("expected attachment Content-Disposition, got %s", rec.Header().Get("Content-Disposition"))
		}
		if !strings.Contains(rec.Body.String(), "IntakeID,ShelterID") {
			t.Errorf("expected CSV header row, got %s", rec.Body.String())
		}
	})

	t.Run("GET /api/v1/shelters/analytics/export.geojson streams GeoJSON FeatureCollection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.geojson", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/geo+json") {
			t.Errorf("expected application/geo+json, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), `"type":"FeatureCollection"`) {
			t.Errorf("expected FeatureCollection, got %s", rec.Body.String())
		}
	})
}

func TestShelterAnalyticsFilteringAndData(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	ctx := context.Background()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	intakeTime1 := now.Add(-48 * time.Hour)
	intakeTime2 := now.Add(-24 * time.Hour)
	intakeTime3 := now.Add(-10 * 24 * time.Hour) // 10 days ago

	pet1 := domain.FoundPetRecord{
		PetID:             "found-sea-1",
		ShelterID:         "shelter-sea-01",
		ShelterName:       "Seattle Animal Shelter",
		IntakeID:          "INT-101",
		Species:           "dog",
		Breed:             "Labrador",
		PrimaryColor:      "Yellow",
		CustodyStatus:     domain.CustodyShelterCare,
		Status:            domain.FoundPetStatusFound,
		FoundAt:           intakeTime1,
		MicrochipID:       "985141000123456",
		MicrochipRegistry: "HomeAgain",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.648,
			Longitude: -122.378,
		},
	}
	pet1Data, _ := json.Marshal(pet1)
	_ = st.SaveState(ctx, store.FoundPetsCollection, pet1.PetID, pet1Data)

	pet2 := domain.FoundPetRecord{
		PetID:             "found-bel-2",
		ShelterID:         "shelter-bel-02",
		ShelterName:       "Bellevue Humane Society",
		IntakeID:          "BEL-501",
		Species:           "cat",
		Breed:             "Siamese",
		PrimaryColor:      "Seal Point",
		CustodyStatus:     domain.CustodyShelterCare,
		Status:            domain.FoundPetStatusResolved,
		FoundAt:           intakeTime2,
		MicrochipID:       "981010000999999",
		MicrochipRegistry: "AKC Reunite",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.610,
			Longitude: -122.200,
		},
	}
	pet2Data, _ := json.Marshal(pet2)
	_ = st.SaveState(ctx, store.FoundPetsCollection, pet2.PetID, pet2Data)

	// Old pet beyond 7 days
	pet3 := domain.FoundPetRecord{
		PetID:         "found-sea-3",
		ShelterID:     "shelter-sea-01",
		ShelterName:   "Seattle Animal Shelter",
		IntakeID:      "INT-099",
		Species:       "dog",
		Breed:         "Poodle",
		PrimaryColor:  "White",
		CustodyStatus: domain.CustodyShelterCare,
		Status:        domain.FoundPetStatusFound,
		FoundAt:       intakeTime3,
	}
	pet3Data, _ := json.Marshal(pet3)
	_ = st.SaveState(ctx, store.FoundPetsCollection, pet3.PetID, pet3Data)

	// Match for pet2
	match2 := domain.MatchRecord{
		MatchID:            "match-2",
		FoundPetID:         "found-bel-2",
		MatchedPetID:       "lost-2",
		Status:             domain.MatchStatusReunited,
		DeterministicMatch: true,
		MatchType:          "deterministic_microchip",
		Score:              1.0,
		MatchedAt:          intakeTime2.Add(3 * time.Hour),
	}
	match2Data, _ := json.Marshal(match2)
	_ = st.SaveState(ctx, store.MatchesCollection, match2.MatchID, match2Data)

	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	t.Run("Filters JSON report by shelterId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics?shelterId=shelter-bel-02", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp struct {
			OverallKPIs struct {
				TotalIntakes  int `json:"totalIntakes"`
				ReunitedCount int `json:"reunitedCount"`
			} `json:"overallKpis"`
			ShelterBreakdown []struct {
				ShelterID string `json:"shelterId"`
			} `json:"shelterBreakdown"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if resp.OverallKPIs.TotalIntakes != 1 {
			t.Errorf("expected 1 intake for Bellevue, got %d", resp.OverallKPIs.TotalIntakes)
		}
		if resp.OverallKPIs.ReunitedCount != 1 {
			t.Errorf("expected 1 reunited for Bellevue, got %d", resp.OverallKPIs.ReunitedCount)
		}
		if len(resp.ShelterBreakdown) != 1 || resp.ShelterBreakdown[0].ShelterID != "shelter-bel-02" {
			t.Errorf("unexpected breakdown: %+v", resp.ShelterBreakdown)
		}
	})

	t.Run("Filters CSV export by shelterId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.csv?shelterId=shelter-bel-02", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) != 2 { // Header + 1 row
			t.Fatalf("expected 2 lines in CSV (header + 1 row), got %d lines:\n%s", len(lines), body)
		}
		if !strings.Contains(lines[1], "BEL-501") {
			t.Errorf("expected BEL-501 in CSV, got: %s", lines[1])
		}
		if strings.Contains(body, "INT-101") {
			t.Errorf("expected INT-101 to be excluded, got: %s", body)
		}
	})

	t.Run("Filters GeoJSON export by shelterId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.geojson?shelterId=shelter-sea-01", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var fc struct {
			Type     string `json:"type"`
			Features []struct {
				Type     string `json:"type"`
				Geometry struct {
					Coordinates []float64 `json:"coordinates"`
				} `json:"geometry"`
				Properties struct {
					IntakeID  string `json:"intakeId"`
					ShelterID string `json:"shelterId"`
				} `json:"properties"`
			} `json:"features"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		// pet1 and pet3 belong to shelter-sea-01, but only pet1 has coordinates
		if len(fc.Features) != 1 {
			t.Fatalf("expected 1 feature with coordinates, got %d", len(fc.Features))
		}
		if fc.Features[0].Properties.IntakeID != "INT-101" {
			t.Errorf("expected INT-101, got %s", fc.Features[0].Properties.IntakeID)
		}
	})

	t.Run("Date filtering with RFC3339 and DateOnly", func(t *testing.T) {
		// Filter with startDate using DateOnly
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics?startDate=2026-09-18", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp struct {
			OverallKPIs struct {
				TotalIntakes int `json:"totalIntakes"`
			} `json:"overallKpis"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		// pet2 is -24h (2026-09-18 10:00), pet1 is -48h (2026-09-17 10:00), pet3 is -10d
		if resp.OverallKPIs.TotalIntakes != 1 {
			t.Errorf("expected 1 intake on or after 2026-09-18, got %d", resp.OverallKPIs.TotalIntakes)
		}
	})

	t.Run("Date filtering with DateOnly endDate includes noon intake on that date", func(t *testing.T) {
		noonPet := domain.FoundPetRecord{
			PetID:         "found-sea-noon",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-NOON",
			Species:       "dog",
			Breed:         "Terrier",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
		}
		noonData, _ := json.Marshal(noonPet)
		_ = st.SaveState(ctx, store.FoundPetsCollection, noonPet.PetID, noonData)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics?startDate=2026-09-18&endDate=2026-09-18", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp struct {
			OverallKPIs struct {
				TotalIntakes int `json:"totalIntakes"`
			} `json:"overallKpis"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		// Both pet2 (2026-09-18 10:00:00Z) and noonPet (2026-09-18 12:00:00Z) occur on 2026-09-18.
		// If endDate were not adjusted to 23:59:59.999999999Z, noonPet would be excluded because 12:00:00 > 00:00:00.
		if resp.OverallKPIs.TotalIntakes != 2 {
			t.Errorf("expected 2 intakes on 2026-09-18 (including noon intake), got %d", resp.OverallKPIs.TotalIntakes)
		}
	})

	t.Run("Rejects invalid date format with 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics?startDate=invalid-date", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Rejects non-GET methods with 405 Method Not Allowed", func(t *testing.T) {
		endpoints := []string{
			"/shelters/analytics",
			"/api/v1/shelters/analytics",
			"/api/v1/shelters/analytics/export.csv",
			"/api/v1/shelters/analytics/export.geojson",
		}
		for _, ep := range endpoints {
			req := httptest.NewRequest(http.MethodPost, ep, nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for POST %s, got %d", ep, rec.Code)
			}
		}
	})
}
