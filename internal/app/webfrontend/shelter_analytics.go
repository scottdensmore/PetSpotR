package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/analytics"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func parseFilterDate(val string, endOfDay bool) (time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.DateOnly, val); err == nil {
		if endOfDay {
			return t.UTC().Add(24*time.Hour - time.Nanosecond), nil
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format: %q", val)
}

func parseAnalyticsFilter(r *http.Request) (analytics.FilterOptions, error) {
	q := r.URL.Query()
	filter := analytics.FilterOptions{
		ShelterID: strings.TrimSpace(q.Get("shelterId")),
	}

	rangeVal := strings.ToLower(strings.TrimSpace(q.Get("range")))
	switch rangeVal {
	case "7d":
		filter.StartDate = time.Now().UTC().AddDate(0, 0, -7)
	case "30d":
		filter.StartDate = time.Now().UTC().AddDate(0, 0, -30)
	case "90d":
		filter.StartDate = time.Now().UTC().AddDate(0, 0, -90)
	case "all", "":
		// no start date restriction
	}

	if startStr := strings.TrimSpace(q.Get("startDate")); startStr != "" {
		t, err := parseFilterDate(startStr, false)
		if err != nil {
			return filter, fmt.Errorf("invalid startDate: %w", err)
		}
		filter.StartDate = t
	}

	if endStr := strings.TrimSpace(q.Get("endDate")); endStr != "" {
		t, err := parseFilterDate(endStr, true)
		if err != nil {
			return filter, fmt.Errorf("invalid endDate: %w", err)
		}
		filter.EndDate = t
	}

	return filter, nil
}

func (s *Server) getShelterAnalyticsData(ctx context.Context) ([]domain.FoundPetRecord, []domain.MatchRecord, error) {
	if s.stateStore == nil {
		return nil, nil, errors.New("state store uninitialized")
	}

	rawPets, err := s.stateStore.ListState(ctx, store.FoundPetsCollection)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query found pets: %w", err)
	}
	foundPets := make([]domain.FoundPetRecord, 0, len(rawPets))
	for _, b := range rawPets {
		var pet domain.FoundPetRecord
		if err := json.Unmarshal(b, &pet); err == nil {
			foundPets = append(foundPets, domain.NormalizeFoundPetRecord(pet))
		}
	}

	rawMatches, err := s.stateStore.ListState(ctx, store.MatchesCollection)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query matches: %w", err)
	}
	matches := make([]domain.MatchRecord, 0, len(rawMatches))
	for _, b := range rawMatches {
		var m domain.MatchRecord
		if err := json.Unmarshal(b, &m); err == nil {
			matches = append(matches, m)
		}
	}

	return foundPets, matches, nil
}

func (s *Server) handleShelterAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	content, err := embeddedFiles.ReadFile("templates/shelter-analytics.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>Shelter Analytics</title></head><body><h1>Shelter Analytics</h1></body></html>"))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handleApiShelterAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filter, err := parseAnalyticsFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	foundPets, matches, err := s.getShelterAnalyticsData(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	report := analytics.ComputeAnalytics(foundPets, matches, filter)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

func (s *Server) handleApiShelterAnalyticsExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filter, err := parseAnalyticsFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	foundPets, matches, err := s.getShelterAnalyticsData(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="shelter-reconciliation-export.csv"`)
	w.WriteHeader(http.StatusOK)

	_ = analytics.ExportReconciliationCSV(w, foundPets, matches, filter)
}

func (s *Server) handleApiShelterAnalyticsExportGeoJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filter, err := parseAnalyticsFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	foundPets, matches, err := s.getShelterAnalyticsData(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="shelter-reconciliation-export.geojson"`)
	w.WriteHeader(http.StatusOK)

	_ = analytics.ExportReconciliationGeoJSON(w, foundPets, matches, filter)
}
