package webfrontend

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func parseGeoFenceParams(r *http.Request) (*webhook.GeoFence, error) {
	q := r.URL.Query()
	latStr := strings.TrimSpace(q.Get("lat"))
	lngStr := strings.TrimSpace(q.Get("lng"))

	if latStr == "" && lngStr == "" {
		return nil, nil
	}

	if latStr == "" || lngStr == "" {
		return nil, errors.New("both lat and lng parameters are required for proximity filtering")
	}

	lat, errLat := strconv.ParseFloat(latStr, 64)
	lng, errLng := strconv.ParseFloat(lngStr, 64)
	if errLat != nil || errLng != nil || math.IsNaN(lat) || math.IsNaN(lng) ||
		math.IsInf(lat, 0) || math.IsInf(lng, 0) ||
		lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, errors.New("invalid coordinates: lat must be [-90, 90] and lng [-180, 180]")
	}

	radiusMiles := 25.0
	radiusStr := strings.TrimSpace(q.Get("radiusMiles"))
	if radiusStr == "" {
		radiusStr = strings.TrimSpace(q.Get("radius"))
	}
	if radiusStr != "" {
		rVal, err := strconv.ParseFloat(radiusStr, 64)
		if err != nil || math.IsNaN(rVal) || math.IsInf(rVal, 0) || rVal <= 0 {
			return nil, errors.New("invalid radiusMiles: must be a positive number")
		}
		radiusMiles = rVal
	}

	return &webhook.GeoFence{
		CenterLat:   lat,
		CenterLng:   lng,
		RadiusMiles: radiusMiles,
	}, nil
}

func (s *Server) handleLostPetsFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	fence, err := parseGeoFenceParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rawLost, err := s.stateStore.ListState(r.Context(), store.LostPetsCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "failed to list lost pets", http.StatusInternalServerError)
		return
	}

	pets := make([]domain.LostPetRecord, 0)
	for _, b := range rawLost {
		var rec domain.LostPetRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			continue
		}
		rec = domain.NormalizeLostPetRecord(rec)
		if !rec.Status.IsActive() {
			continue
		}
		pets = append(pets, rec)
	}

	sort.Slice(pets, func(i, j int) bool {
		return pets[i].ReportedAt.After(pets[j].ReportedAt)
	})

	baseURL := determineRequestBaseURL(r)
	feedXML, err := webhook.GenerateLostPetsAtomFeed(pets, baseURL, fence)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to generate lost pets atom feed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feedXML))
}

func (s *Server) handleSightingsFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	fence, err := parseGeoFenceParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rawSightings, err := s.stateStore.ListState(r.Context(), store.SightingsCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "failed to list sightings", http.StatusInternalServerError)
		return
	}

	sightings := make([]domain.PetSightingRecord, 0)
	for _, b := range rawSightings {
		var sRec domain.PetSightingRecord
		if err := json.Unmarshal(b, &sRec); err != nil {
			continue
		}
		if sRec.Status == domain.SightingStatusDismissed || sRec.Status == domain.SightingStatusFlagged {
			continue
		}
		sightings = append(sightings, sRec)
	}

	sort.Slice(sightings, func(i, j int) bool {
		tI := sightings[i].ReportedAt
		if tI.IsZero() {
			tI = sightings[i].SightedAt
		}
		tJ := sightings[j].ReportedAt
		if tJ.IsZero() {
			tJ = sightings[j].SightedAt
		}
		return tI.After(tJ)
	})

	baseURL := determineRequestBaseURL(r)
	feedXML, err := webhook.GenerateSightingsAtomFeed(sightings, baseURL, fence)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to generate sightings atom feed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feedXML))
}
