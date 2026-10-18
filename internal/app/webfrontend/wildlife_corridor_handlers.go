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
	"github.com/scottdensmore/petspotr/pkg/wildlife"
)

var (
	lastCorridorNano atomic.Int64
	corridorSeedMu   sync.Mutex
)

func nextCorridorNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastCorridorNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastCorridorNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// WildlifeCorridorsPageData represents template data passed to templates/wildlife_corridors.html.
type WildlifeCorridorsPageData struct {
	Reports []domain.WildlifeCorridorReport
	Locale  string
}

type createWildlifeCorridorRequest struct {
	ReportID              string  `json:"reportId"`
	PetID                 string  `json:"petId"`
	PetName               string  `json:"petName"`
	PetSpecies            string  `json:"petSpecies"`
	PetWeightKg           float64 `json:"petWeightKg"`
	PredatorSpecies       string  `json:"predatorSpecies"`
	PackCount             int     `json:"packCount"`
	SolarPhase            string  `json:"solarPhase"`
	CorridorTerrain       string  `json:"corridorTerrain"`
	DistanceMeters        float64 `json:"distanceMeters"`
	StrobeDeterrentActive bool    `json:"strobeDeterrentActive"`
	AttractantsCleared    bool    `json:"attractantsCleared"`
}

type mitigateWildlifeCorridorRequest struct {
	CompleteAllMitigations bool   `json:"completeAllMitigations"`
	DeployDeterrents       bool   `json:"deployDeterrents"`
	CompleteItemID         string `json:"completeItemId"`
	StrobeDeterrentActive  *bool  `json:"strobeDeterrentActive"`
	AttractantsCleared     *bool  `json:"attractantsCleared"`
}

func (s *Server) registerWildlifeCorridorRoutes() {
	s.mux.HandleFunc("GET /wildlife-corridors", s.handleWildlifeCorridorsPage)
	s.mux.HandleFunc("GET /api/v1/wildlife-corridors/reports", s.handleListWildlifeCorridorReports)
	s.mux.HandleFunc("POST /api/v1/wildlife-corridors/reports", s.handleCreateWildlifeCorridorReport)
	s.mux.HandleFunc("POST /api/v1/wildlife-corridors/reports/{id}/mitigate", s.handleMitigateWildlifeCorridorReport)
	s.mux.HandleFunc("GET /api/v1/wildlife-corridors/stream", s.handleWildlifeCorridorStream)
}

func defaultWildlifeCorridorReports(now time.Time) []domain.WildlifeCorridorReport {
	reports := make([]domain.WildlifeCorridorReport, 0, 3)

	if r1, err := wildlife.EvaluateWildlifeCorridor(wildlife.CreateCorridorInput{
		ReportID:              "corridor-rep-101",
		PetID:                 "pet-2026-002",
		PetName:               "Mochi",
		PetSpecies:            "CAT",
		PetWeightKg:           4.2,
		PredatorSpecies:       "COYOTE_PACK",
		PackCount:             3,
		SolarPhase:            "NIGHT",
		CorridorTerrain:       "URBAN_RAVINE_GREENBELT",
		DistanceMeters:        85,
		StrobeDeterrentActive: false,
		AttractantsCleared:    false,
	}, now.Add(-1*time.Minute)); err == nil && r1 != nil {
		reports = append(reports, *r1)
	}

	if r2, err := wildlife.EvaluateWildlifeCorridor(wildlife.CreateCorridorInput{
		ReportID:              "corridor-rep-102",
		PetID:                 "pet-2026-001",
		PetName:               "Pippin",
		PetSpecies:            "DOG",
		PetWeightKg:           8.5,
		PredatorSpecies:       "BOBCAT",
		PackCount:             1,
		SolarPhase:            "DAWN_DUSK_CREPUSCULAR",
		CorridorTerrain:       "BRUSH_CANYON_INTERFACE",
		DistanceMeters:        220,
		StrobeDeterrentActive: false,
		AttractantsCleared:    false,
	}, now.Add(-3*time.Minute)); err == nil && r2 != nil {
		_ = wildlife.ApplyCorridorMitigation(r2, wildlife.MitigationUpdateInput{
			DeployDeterrents: true,
		}, now.Add(-2*time.Minute))
		reports = append(reports, *r2)
	}

	if r3, err := wildlife.EvaluateWildlifeCorridor(wildlife.CreateCorridorInput{
		ReportID:              "corridor-rep-103",
		PetID:                 "pet-2026-003",
		PetName:               "Shadow",
		PetSpecies:            "DOG",
		PetWeightKg:           24.0,
		PredatorSpecies:       "COYOTE_PACK",
		PackCount:             2,
		SolarPhase:            "DAYLIGHT",
		CorridorTerrain:       "SUBURBAN_CUL_DE_SAC",
		DistanceMeters:        450,
		StrobeDeterrentActive: true,
		AttractantsCleared:    true,
	}, now.Add(-5*time.Minute)); err == nil && r3 != nil {
		_ = wildlife.ApplyCorridorMitigation(r3, wildlife.MitigationUpdateInput{
			CompleteAllMitigations: true,
		}, now.Add(-4*time.Minute))
		reports = append(reports, *r3)
	}

	return reports
}

func (s *Server) ensureDefaultWildlifeCorridorReports(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	corridorSeedMu.Lock()
	defer corridorSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionWildlifeCorridorReports)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultWildlifeCorridorReports(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionWildlifeCorridorReports, item.ReportID, data)
		}
	}
}

func (s *Server) loadWildlifeCorridorReports(ctx context.Context, petIDFilter, phaseFilter string) []domain.WildlifeCorridorReport {
	reports := make([]domain.WildlifeCorridorReport, 0)
	if s.stateStore == nil {
		return reports
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionWildlifeCorridorReports)
	if err != nil {
		return reports
	}
	for _, b := range raw {
		var item domain.WildlifeCorridorReport
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if phaseFilter != "" && !strings.EqualFold(string(item.Phase), phaseFilter) {
				continue
			}
			reports = append(reports, item)
		}
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].UpdatedAt != reports[j].UpdatedAt {
			return reports[i].UpdatedAt > reports[j].UpdatedAt
		}
		return reports[i].ReportID < reports[j].ReportID
	})
	return reports
}

func writeCorridorJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleWildlifeCorridorsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultWildlifeCorridorReports(r.Context())
	reports := s.loadWildlifeCorridorReports(r.Context(), "", "")

	tmpl, err := template.New("wildlife_corridors.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/wildlife_corridors.html")
	if err != nil {
		http.Error(w, "Failed to load Wildlife Corridors template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := WildlifeCorridorsPageData{
		Reports: reports,
		Locale:  locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListWildlifeCorridorReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCorridorJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultWildlifeCorridorReports(r.Context())
	q := r.URL.Query()
	reports := s.loadWildlifeCorridorReports(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("phase")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"reports": reports,
	})
}

func (s *Server) handleCreateWildlifeCorridorReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCorridorJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createWildlifeCorridorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCorridorJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultWildlifeCorridorReports(r.Context())

	seqNano := nextCorridorNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	reportID := strings.TrimSpace(req.ReportID)
	if reportID == "" {
		reportID = "corridor-rep-" + randomHex8()
	}

	report, err := wildlife.EvaluateWildlifeCorridor(wildlife.CreateCorridorInput{
		ReportID:              reportID,
		PetID:                 req.PetID,
		PetName:               req.PetName,
		PetSpecies:            req.PetSpecies,
		PetWeightKg:           req.PetWeightKg,
		PredatorSpecies:       req.PredatorSpecies,
		PackCount:             req.PackCount,
		SolarPhase:            req.SolarPhase,
		CorridorTerrain:       req.CorridorTerrain,
		DistanceMeters:        req.DistanceMeters,
		StrobeDeterrentActive: req.StrobeDeterrentActive,
		AttractantsCleared:    req.AttractantsCleared,
	}, now)
	if err != nil {
		writeCorridorJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	reportBytes, err := json.Marshal(report)
	if err != nil {
		writeCorridorJSONError(w, http.StatusInternalServerError, "Failed to encode wildlife corridor report")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionWildlifeCorridorReports, report.ReportID, reportBytes); err != nil {
		writeCorridorJSONError(w, http.StatusInternalServerError, "Failed to save wildlife corridor report")
		return
	}

	s.broadcastWildlifeCorridorEvent("wildlife_corridor_updated", map[string]any{
		"type":   "wildlife_corridor_updated",
		"report": report,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(report)
}

func (s *Server) handleMitigateWildlifeCorridorReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCorridorJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	reportID := strings.TrimSpace(r.PathValue("id"))
	if reportID == "" {
		writeCorridorJSONError(w, http.StatusBadRequest, "report id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req mitigateWildlifeCorridorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCorridorJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultWildlifeCorridorReports(r.Context())

	seqNano := nextCorridorNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedReport domain.WildlifeCorridorReport
		mitErr        error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionWildlifeCorridorReports, reportID, func(current []byte) ([]byte, error) {
		var existing domain.WildlifeCorridorReport
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt wildlife corridor report: %w", err)
		}

		if mErr := wildlife.ApplyCorridorMitigation(&existing, wildlife.MitigationUpdateInput{
			CompleteAllMitigations: req.CompleteAllMitigations,
			DeployDeterrents:       req.DeployDeterrents,
			CompleteItemID:         req.CompleteItemID,
			StrobeDeterrentActive:  req.StrobeDeterrentActive,
			AttractantsCleared:     req.AttractantsCleared,
		}, now); mErr != nil {
			mitErr = mErr
			return nil, mErr
		}
		updatedReport = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCorridorJSONError(w, http.StatusNotFound, "Wildlife corridor report not found")
			return
		}
		if mitErr != nil {
			writeCorridorJSONError(w, http.StatusBadRequest, mitErr.Error())
			return
		}
		writeCorridorJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update corridor mitigation: %v", err))
		return
	}

	s.broadcastWildlifeCorridorEvent("wildlife_corridor_updated", map[string]any{
		"type":   "wildlife_corridor_updated",
		"report": updatedReport,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedReport)
}

func (s *Server) handleWildlifeCorridorStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("wildlife_corridors")
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

func (s *Server) broadcastWildlifeCorridorEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextCorridorNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "wildlife_corridors",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "wildlife_corridors",
			SenderNodeID:  "wildlife-corridors-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
