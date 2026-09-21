package webfrontend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestTrajectory_PredictiveModelIncluded(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	petID := "lost-pred-dog"
	now := time.Now().UTC()
	pet := domain.LostPetRecord{
		PetID:      petID,
		PetName:    "Barkley",
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

	s1 := domain.PetSightingRecord{
		SightingID: "sight-1",
		LostPetID:  petID,
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6080,
			Longitude: -122.3300,
		},
		SightedAt: now.Add(-90 * time.Minute),
		Status:    domain.SightingStatusActive,
	}
	s1Bytes, _ := json.Marshal(s1)
	_ = st.SaveState(context.Background(), store.SightingsCollection, s1.SightingID, s1Bytes)

	s2 := domain.PetSightingRecord{
		SightingID: "sight-2",
		LostPetID:  petID,
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6100,
			Longitude: -122.3280,
		},
		SightedAt: now.Add(-30 * time.Minute),
		Status:    domain.SightingStatusActive,
	}
	s2Bytes, _ := json.Marshal(s2)
	_ = st.SaveState(context.Background(), store.SightingsCollection, s2.SightingID, s2Bytes)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/"+petID+"/trajectory", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var analysis sighting.TrajectoryAnalysis
	if err := json.Unmarshal(rec.Body.Bytes(), &analysis); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if analysis.LostPetID != petID {
		t.Errorf("expected pet ID %q, got %q", petID, analysis.LostPetID)
	}
	if analysis.PredictiveModel == nil {
		t.Fatal("expected predictiveModel in trajectory response, got nil")
	}

	pred := analysis.PredictiveModel
	if pred.LostPetID != petID {
		t.Errorf("expected pred.LostPetID %q, got %q", petID, pred.LostPetID)
	}
	if pred.Species != "dog" {
		t.Errorf("expected pred.Species 'dog', got %q", pred.Species)
	}
	if len(pred.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone contours, got %d", len(pred.Isochrones))
	}
	expectedProbLevels := []float64{0.50, 0.75, 0.90}
	for i, expProb := range expectedProbLevels {
		if pred.Isochrones[i].ProbabilityLevel != expProb {
			t.Errorf("expected isochrone[%d] prob %f, got %f", i, expProb, pred.Isochrones[i].ProbabilityLevel)
		}
		if len(pred.Isochrones[i].PolygonCoordinates) == 0 || len(pred.Isochrones[i].PolygonCoordinates[0]) < 4 {
			t.Errorf("expected closed polygon in isochrone[%d], got %v", i, pred.Isochrones[i].PolygonCoordinates)
		}
	}
	if len(pred.BarriersEncountered) == 0 {
		t.Error("expected default barriers encountered to be present")
	}
}

func TestPredictiveTrajectoryScenario_CatCustomization(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	petID := "lost-pred-cat"
	pet := domain.LostPetRecord{
		PetID:      petID,
		PetName:    "Whiskers",
		Species:    "cat",
		ReportedAt: time.Now().UTC().Add(-4 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6062,
			Longitude: -122.3321,
		},
		Status: domain.LostPetStatusLost,
	}
	petBytes, _ := json.Marshal(pet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/"+petID+"/predictive-trajectory?elapsedHours=3.0&species=cat", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var pred domain.PredictiveTrajectoryResult
	if err := json.Unmarshal(rec.Body.Bytes(), &pred); err != nil {
		t.Fatalf("failed to decode predictive trajectory: %v", err)
	}

	if pred.LostPetID != petID {
		t.Errorf("expected petId %q, got %q", petID, pred.LostPetID)
	}
	if pred.Species != "cat" {
		t.Errorf("expected species 'cat', got %q", pred.Species)
	}
	if pred.ElapsedHours != 3.0 {
		t.Errorf("expected elapsedHours 3.0, got %f", pred.ElapsedHours)
	}
	if len(pred.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone contours, got %d", len(pred.Isochrones))
	}
}

func TestPredictiveTrajectoryScenario_MethodAndNotFound(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	petID := "lost-pred-test"
	pet := domain.LostPetRecord{
		PetID:      petID,
		PetName:    "Buddy",
		Species:    "dog",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6062,
			Longitude: -122.3321,
		},
		Status: domain.LostPetStatusLost,
	}
	petBytes, _ := json.Marshal(pet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes)

	// Non-GET method rejected
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/lost-pets/"+petID+"/predictive-trajectory", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /predictive-trajectory: expected 405 Method Not Allowed, got %d", method, rec.Code)
		}
	}

	// Non-existent pet returns 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/non-existent-pet/predictive-trajectory", nil)
	rec404 := httptest.NewRecorder()
	srv.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec404.Code)
	}
}
