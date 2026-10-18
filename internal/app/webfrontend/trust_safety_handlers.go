package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/trustsafety"
)

var (
	lastTrustNano atomic.Int64
	trustSeedMu   sync.Mutex
)

func nextTrustNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastTrustNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastTrustNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// TrustSafetyPageData represents template data passed to templates/trust_safety.html.
type TrustSafetyPageData struct {
	Screenings []domain.SightingTrustScreening
	Locale     string
}

type createTrustScreeningRequest struct {
	ScreeningID        string  `json:"screeningId"`
	PetID              string  `json:"petId"`
	PetName            string  `json:"petName"`
	ReporterHandle     string  `json:"reporterHandle"`
	ReporterTrustScore *int    `json:"reporterTrustScore"`
	PrevRefLat         float64 `json:"prevRefLat"`
	PrevRefLng         float64 `json:"prevRefLng"`
	SightingLat        float64 `json:"sightingLat"`
	SightingLng        float64 `json:"sightingLng"`
	ElapsedMinutes     float64 `json:"elapsedMinutes"`
	SubmittedNotes     string  `json:"submittedNotes"`
	ImageIntegrityTag  string  `json:"imageIntegrityTag"`
}

type adjudicateTrustScreeningRequest struct {
	Verdict        domain.TrustScreeningVerdict `json:"verdict"`
	ModeratorNotes string                       `json:"moderatorNotes"`
}

type trustScreeningEnvelope struct {
	domain.SightingTrustScreening
	Screening domain.SightingTrustScreening `json:"screening"`
	Sighting  *domain.Sighting              `json:"sighting,omitempty"`
}

func (s *Server) registerTrustSafetyRoutes() {
	s.mux.HandleFunc("GET /trust-safety", s.handleTrustSafetyPage)
	s.mux.HandleFunc("GET /api/v1/trust-safety/screenings", s.handleListTrustScreenings)
	s.mux.HandleFunc("POST /api/v1/trust-safety/screenings", s.handleCreateTrustScreening)
	s.mux.HandleFunc("POST /api/v1/trust-safety/screenings/{id}/decision", s.handleAdjudicateTrustScreening)
	s.mux.HandleFunc("GET /api/v1/trust-safety/stream", s.handleTrustSafetyStream)
}

func defaultTrustScreenings(now time.Time) []domain.SightingTrustScreening {
	s1 := trustsafety.EvaluateSightingTrust(domain.SightingTrustScreening{
		ScreeningID:        "screening-ts-101",
		PetID:              "pet-trust-max",
		PetName:            "Max",
		ReporterHandle:     "anon-Burner882",
		ReporterTrustScore: 12,
		PrevRefLat:         47.6062,
		PrevRefLng:         -122.3321,
		SightingLat:        47.6065,
		SightingLng:        -122.3315,
		ElapsedMinutes:     20,
		SubmittedNotes:     "I have your dog Max! Send $300 via Zelle or Bitcoin upfront and send me the 6-digit Google Voice verification code to prove you are the owner.",
		ImageIntegrityTag:  "SYNTHETIC_GAN_SIGNATURE",
		CreatedAt:          now.Add(-15 * time.Minute),
		UpdatedAt:          now.Add(-15 * time.Minute),
	})

	s2 := trustsafety.EvaluateSightingTrust(domain.SightingTrustScreening{
		ScreeningID:        "screening-ts-102",
		PetID:              "pet-trust-luna",
		PetName:            "Luna",
		ReporterHandle:     "highway-commuter-44",
		ReporterTrustScore: 48,
		PrevRefLat:         47.6062,
		PrevRefLng:         -122.3321,
		SightingLat:        47.2529,
		SightingLng:        -122.4443,
		ElapsedMinutes:     18,
		SubmittedNotes:     "Spotted a husky matching Luna near Tacoma Dome transit station",
		ImageIntegrityTag:  "STRIPPED_EXIF",
		CreatedAt:          now.Add(-10 * time.Minute),
		UpdatedAt:          now.Add(-10 * time.Minute),
	})

	s3 := trustsafety.EvaluateSightingTrust(domain.SightingTrustScreening{
		ScreeningID:        "screening-ts-103",
		PetID:              "pet-trust-bella",
		PetName:            "Bella",
		ReporterHandle:     "volunteer-maya-lin",
		ReporterTrustScore: 94,
		PrevRefLat:         47.6062,
		PrevRefLng:         -122.3321,
		SightingLat:        47.6105,
		SightingLng:        -122.3340,
		ElapsedMinutes:     35,
		SubmittedNotes:     "Spotted Bella drinking from creek near Olympic Sculpture Park; calm and wearing red collar",
		ImageIntegrityTag:  "CAMERA_EXIF_INTACT",
		PromotedSightingID: "sighting-trust-seed-103",
		CreatedAt:          now.Add(-5 * time.Minute),
		UpdatedAt:          now.Add(-5 * time.Minute),
	})

	return []domain.SightingTrustScreening{s1, s2, s3}
}

func (s *Server) ensureDefaultTrustScreenings(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	trustSeedMu.Lock()
	defer trustSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionSightingTrustScreening)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultTrustScreenings(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionSightingTrustScreening, item.ScreeningID, data)
		}
	}
}

func (s *Server) loadTrustScreenings(ctx context.Context, petIDFilter, verdictFilter string) []domain.SightingTrustScreening {
	screenings := make([]domain.SightingTrustScreening, 0)
	if s.stateStore == nil {
		return screenings
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionSightingTrustScreening)
	if err != nil {
		return screenings
	}
	for _, b := range raw {
		var item domain.SightingTrustScreening
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if verdictFilter != "" && !strings.EqualFold(string(item.Verdict), verdictFilter) {
				continue
			}
			if item.AnomalyFlags == nil {
				item.AnomalyFlags = []domain.ThreatAnomalyFlag{}
			}
			screenings = append(screenings, item)
		}
	}
	sort.Slice(screenings, func(i, j int) bool {
		if !screenings[i].UpdatedAt.Equal(screenings[j].UpdatedAt) {
			return screenings[i].UpdatedAt.After(screenings[j].UpdatedAt)
		}
		return screenings[i].ScreeningID < screenings[j].ScreeningID
	})
	return screenings
}

func writeTrustSafetyJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) promoteScreeningToSighting(ctx context.Context, item domain.SightingTrustScreening, now time.Time) (domain.Sighting, error) {
	sightingID := fmt.Sprintf("sighting-trust-%d", nextTrustNano())
	coords := &domain.LocationPoint{
		Latitude:  item.SightingLat,
		Longitude: item.SightingLng,
	}
	sighting := domain.Sighting{
		ID:                  sightingID,
		SightingID:          sightingID,
		PetID:               item.PetID,
		LostPetID:           item.PetID,
		ReportedAt:          now,
		SightedAt:           now,
		LocationDescription: fmt.Sprintf("Trust-Verified Sighting (%s)", item.ReporterHandle),
		Coordinates:         coords,
		Notes:               item.SubmittedNotes,
		Status:              domain.SightingStatusActive,
	}
	if s.stateStore != nil {
		sBytes, err := json.Marshal(sighting)
		if err != nil {
			return domain.Sighting{}, err
		}
		if err := s.stateStore.SaveState(ctx, store.SightingsCollection, sighting.ID, sBytes); err != nil {
			return domain.Sighting{}, err
		}
	}
	return sighting, nil
}

func (s *Server) handleTrustSafetyPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultTrustScreenings(r.Context())
	screenings := s.loadTrustScreenings(r.Context(), "", "")

	tmpl, err := template.New("trust_safety.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/trust_safety.html")
	if err != nil {
		http.Error(w, "Failed to load Trust & Safety template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := TrustSafetyPageData{
		Screenings: screenings,
		Locale:     locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListTrustScreenings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultTrustScreenings(r.Context())
	q := r.URL.Query()
	screenings := s.loadTrustScreenings(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("verdict")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"screenings": screenings,
	})
}

func (s *Server) handleCreateTrustScreening(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createTrustScreeningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeTrustSafetyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultTrustScreenings(r.Context())

	seqNano := nextTrustNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	screeningID := strings.TrimSpace(req.ScreeningID)
	if screeningID == "" {
		screeningID = fmt.Sprintf("screening-ts-%d", seqNano)
	}

	petID := strings.TrimSpace(req.PetID)
	petName := strings.TrimSpace(req.PetName)
	if petName == "" && petID != "" {
		petName = petID
	}

	reporterTrust := 50
	if req.ReporterTrustScore != nil {
		reporterTrust = *req.ReporterTrustScore
	}

	item := domain.SightingTrustScreening{
		ScreeningID:        screeningID,
		PetID:              petID,
		PetName:            petName,
		ReporterHandle:     strings.TrimSpace(req.ReporterHandle),
		ReporterTrustScore: reporterTrust,
		PrevRefLat:         req.PrevRefLat,
		PrevRefLng:         req.PrevRefLng,
		SightingLat:        req.SightingLat,
		SightingLng:        req.SightingLng,
		ElapsedMinutes:     req.ElapsedMinutes,
		SubmittedNotes:     strings.TrimSpace(req.SubmittedNotes),
		ImageIntegrityTag:  strings.TrimSpace(req.ImageIntegrityTag),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	item = trustsafety.EvaluateSightingTrust(item)
	if err := item.Validate(); err != nil {
		writeTrustSafetyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var promotedSighting *domain.Sighting
	if item.Verdict == domain.TrustVerdictAuthentic {
		sighting, err := s.promoteScreeningToSighting(r.Context(), item, now)
		if err == nil {
			item.PromotedSightingID = sighting.ID
			promotedSighting = &sighting
		}
	}

	itemBytes, err := json.Marshal(item)
	if err != nil {
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, "Failed to encode trust screening")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionSightingTrustScreening, item.ScreeningID, itemBytes); err != nil {
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, "Failed to save trust screening")
		return
	}

	s.broadcastTrustSafetyEvent("trust_screening_updated", map[string]any{
		"type":      "trust_screening_updated",
		"screening": item,
		"sighting":  promotedSighting,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(trustScreeningEnvelope{
		SightingTrustScreening: item,
		Screening:              item,
		Sighting:               promotedSighting,
	})
}

func (s *Server) handleAdjudicateTrustScreening(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	screeningID := strings.TrimSpace(r.PathValue("id"))
	if screeningID == "" {
		writeTrustSafetyJSONError(w, http.StatusBadRequest, "screening id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req adjudicateTrustScreeningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeTrustSafetyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultTrustScreenings(r.Context())

	verdict := domain.TrustScreeningVerdict(strings.ToUpper(strings.TrimSpace(string(req.Verdict))))
	seqNano := nextTrustNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedItem domain.SightingTrustScreening
		decisionErr error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionSightingTrustScreening, screeningID, func(current []byte) ([]byte, error) {
		var existing domain.SightingTrustScreening
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt trust screening: %w", err)
		}
		nextItem, dErr := trustsafety.ApplyModeratorDecision(existing, verdict, req.ModeratorNotes, now)
		if dErr != nil {
			decisionErr = dErr
			return nil, dErr
		}
		updatedItem = nextItem
		return json.Marshal(nextItem)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeTrustSafetyJSONError(w, http.StatusNotFound, "Trust screening not found")
			return
		}
		if decisionErr != nil {
			writeTrustSafetyJSONError(w, http.StatusBadRequest, decisionErr.Error())
			return
		}
		writeTrustSafetyJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to adjudicate trust screening: %v", err))
		return
	}

	var promotedSighting *domain.Sighting
	if updatedItem.Verdict == domain.TrustVerdictAuthentic && updatedItem.PromotedSightingID == "" {
		sighting, promErr := s.promoteScreeningToSighting(r.Context(), updatedItem, now)
		if promErr == nil {
			updatedItem.PromotedSightingID = sighting.ID
			promotedSighting = &sighting
			if updatedBytes, mErr := json.Marshal(updatedItem); mErr == nil {
				_ = s.stateStore.SaveState(r.Context(), store.CollectionSightingTrustScreening, updatedItem.ScreeningID, updatedBytes)
			}
		}
	}

	s.broadcastTrustSafetyEvent("trust_screening_updated", map[string]any{
		"type":      "trust_screening_updated",
		"screening": updatedItem,
		"sighting":  promotedSighting,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trustScreeningEnvelope{
		SightingTrustScreening: updatedItem,
		Screening:              updatedItem,
		Sighting:               promotedSighting,
	})
}

func (s *Server) handleTrustSafetyStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe("trust_safety")
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastTrustSafetyEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextTrustNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "trust_safety",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "trust_safety",
			SenderNodeID:  "trust-safety-shield-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
