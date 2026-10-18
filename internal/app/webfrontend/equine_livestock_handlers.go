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
	"github.com/scottdensmore/petspotr/pkg/equinelivestock"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastEquineNano atomic.Int64
	equineSeedMu   sync.Mutex
)

func nextEquineNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastEquineNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastEquineNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// EquineLivestockPageData represents template data passed to templates/equine_livestock.html.
type EquineLivestockPageData struct {
	Dispatches []domain.EquineLivestockDispatch
	Locale     string
}

type createEquineLivestockDispatchRequest struct {
	DispatchID             string `json:"dispatchId"`
	PetID                  string `json:"petId"`
	AnimalName             string `json:"animalName"`
	AnimalCategory         string `json:"animalCategory"`
	HeadCount              int    `json:"headCount"`
	EstimatedWeightKg      int    `json:"estimatedWeightKg"`
	BreachLocation         string `json:"breachLocation"`
	StockTrailerDispatched bool   `json:"stockTrailerDispatched"`
	CorralPanelsErected    bool   `json:"corralPanelsErected"`
}

type containEquineLivestockDispatchRequest struct {
	StageCorralAndTrailer   bool   `json:"stageCorralAndTrailer"`
	CompleteSafeTrailerLoad bool   `json:"completeSafeTrailerLoad"`
	CompleteStepID          string `json:"completeStepId"`
	StockTrailerDispatched  *bool  `json:"stockTrailerDispatched"`
	CorralPanelsErected     *bool  `json:"corralPanelsErected"`
}

func (s *Server) registerEquineLivestockRoutes() {
	s.mux.HandleFunc("GET /equine-livestock", s.handleEquineLivestockPage)
	s.mux.HandleFunc("GET /api/v1/equine-livestock/dispatches", s.handleListEquineLivestockDispatches)
	s.mux.HandleFunc("POST /api/v1/equine-livestock/dispatches", s.handleCreateEquineLivestockDispatch)
	s.mux.HandleFunc("POST /api/v1/equine-livestock/dispatches/{id}/contain", s.handleContainEquineLivestockDispatch)
	s.mux.HandleFunc("GET /api/v1/equine-livestock/stream", s.handleEquineLivestockStream)
}

func defaultEquineLivestockDispatches(now time.Time) []domain.EquineLivestockDispatch {
	dispatches := make([]domain.EquineLivestockDispatch, 0, 3)

	if d1, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             "eq-dsp-101",
		PetID:                  "pet-2026-001",
		AnimalName:             "Valegro & Starlight",
		AnimalCategory:         "EQUINE_WARMBLOOD",
		HeadCount:              2,
		EstimatedWeightKg:      540,
		BreachLocation:         "SR-202 Snoqualmie Valley Bridle Corridor",
		StockTrailerDispatched: false,
		CorralPanelsErected:    false,
	}, now.Add(-1*time.Minute)); err == nil && d1 != nil {
		dispatches = append(dispatches, *d1)
	}

	if d2, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             "eq-dsp-102",
		PetID:                  "pet-2026-002",
		AnimalName:             "Andes Alpaca Trio",
		AnimalCategory:         "CAMELID_ALPACA_HERD",
		HeadCount:              3,
		EstimatedWeightKg:      195,
		BreachLocation:         "Vashon Island North Pasture Gate",
		StockTrailerDispatched: false,
		CorralPanelsErected:    false,
	}, now.Add(-3*time.Minute)); err == nil && d2 != nil {
		_ = equinelivestock.ApplyLivestockDispatchUpdate(d2, equinelivestock.LivestockDispatchUpdateInput{
			StageCorralAndTrailer: true,
		}, now.Add(-2*time.Minute))
		dispatches = append(dispatches, *d2)
	}

	if d3, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             "eq-dsp-103",
		PetID:                  "pet-2026-003",
		AnimalName:             "Clover & Bramble",
		AnimalCategory:         "CAPRINE_OVINE_FLOCK",
		HeadCount:              2,
		EstimatedWeightKg:      85,
		BreachLocation:         "Redmond Farmland 4-H Paddock",
		StockTrailerDispatched: true,
		CorralPanelsErected:    true,
	}, now.Add(-5*time.Minute)); err == nil && d3 != nil {
		_ = equinelivestock.ApplyLivestockDispatchUpdate(d3, equinelivestock.LivestockDispatchUpdateInput{
			CompleteSafeTrailerLoad: true,
		}, now.Add(-4*time.Minute))
		dispatches = append(dispatches, *d3)
	}

	return dispatches
}

func (s *Server) ensureDefaultEquineLivestockDispatches(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	equineSeedMu.Lock()
	defer equineSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionEquineLivestockDispatches)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultEquineLivestockDispatches(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionEquineLivestockDispatches, item.DispatchID, data)
		}
	}
}

func (s *Server) loadEquineLivestockDispatches(ctx context.Context, petIDFilter, statusFilter string) []domain.EquineLivestockDispatch {
	dispatches := make([]domain.EquineLivestockDispatch, 0)
	if s.stateStore == nil {
		return dispatches
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionEquineLivestockDispatches)
	if err != nil {
		return dispatches
	}
	for _, b := range raw {
		var item domain.EquineLivestockDispatch
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.DispatchStatus), statusFilter) {
				continue
			}
			dispatches = append(dispatches, item)
		}
	}
	sort.Slice(dispatches, func(i, j int) bool {
		if dispatches[i].UpdatedAt != dispatches[j].UpdatedAt {
			return dispatches[i].UpdatedAt > dispatches[j].UpdatedAt
		}
		return dispatches[i].DispatchID < dispatches[j].DispatchID
	})
	return dispatches
}

func writeEquineJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleEquineLivestockPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultEquineLivestockDispatches(r.Context())
	dispatches := s.loadEquineLivestockDispatches(r.Context(), "", "")

	tmpl, err := template.New("equine_livestock.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/equine_livestock.html")
	if err != nil {
		http.Error(w, "Failed to load Equine Livestock template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := EquineLivestockPageData{
		Dispatches: dispatches,
		Locale:     locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListEquineLivestockDispatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeEquineJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultEquineLivestockDispatches(r.Context())
	q := r.URL.Query()
	dispatches := s.loadEquineLivestockDispatches(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"dispatches": dispatches,
	})
}

func (s *Server) handleCreateEquineLivestockDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeEquineJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createEquineLivestockDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEquineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultEquineLivestockDispatches(r.Context())

	seqNano := nextEquineNano()
	now := time.Unix(0, seqNano).UTC()

	dispatchID := strings.TrimSpace(req.DispatchID)
	if dispatchID == "" {
		dispatchID = "eq-dsp-" + randomHex8()
	}

	dispatch, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             dispatchID,
		PetID:                  req.PetID,
		AnimalName:             req.AnimalName,
		AnimalCategory:         req.AnimalCategory,
		HeadCount:              req.HeadCount,
		EstimatedWeightKg:      req.EstimatedWeightKg,
		BreachLocation:         req.BreachLocation,
		StockTrailerDispatched: req.StockTrailerDispatched,
		CorralPanelsErected:    req.CorralPanelsErected,
	}, now)
	if err != nil {
		writeEquineJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	dispatchBytes, err := json.Marshal(dispatch)
	if err != nil {
		writeEquineJSONError(w, http.StatusInternalServerError, "Failed to encode equine livestock dispatch")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionEquineLivestockDispatches, dispatch.DispatchID, dispatchBytes); err != nil {
		writeEquineJSONError(w, http.StatusInternalServerError, "Failed to save equine livestock dispatch")
		return
	}

	s.broadcastEquineLivestockEvent("livestock_dispatch_updated", map[string]any{
		"type":     "livestock_dispatch_updated",
		"dispatch": dispatch,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dispatch)
}

func (s *Server) handleContainEquineLivestockDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeEquineJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	dispatchID := strings.TrimSpace(r.PathValue("id"))
	if dispatchID == "" {
		writeEquineJSONError(w, http.StatusBadRequest, "dispatch id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req containEquineLivestockDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEquineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultEquineLivestockDispatches(r.Context())

	seqNano := nextEquineNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedDispatch domain.EquineLivestockDispatch
		containErr      error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionEquineLivestockDispatches, dispatchID, func(current []byte) ([]byte, error) {
		var existing domain.EquineLivestockDispatch
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt equine livestock dispatch: %w", err)
		}

		if cErr := equinelivestock.ApplyLivestockDispatchUpdate(&existing, equinelivestock.LivestockDispatchUpdateInput{
			StageCorralAndTrailer:   req.StageCorralAndTrailer,
			CompleteSafeTrailerLoad: req.CompleteSafeTrailerLoad,
			CompleteStepID:          req.CompleteStepID,
			StockTrailerDispatched:  req.StockTrailerDispatched,
			CorralPanelsErected:     req.CorralPanelsErected,
		}, now); cErr != nil {
			containErr = cErr
			return nil, cErr
		}
		updatedDispatch = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeEquineJSONError(w, http.StatusNotFound, "Equine livestock dispatch not found")
			return
		}
		if containErr != nil {
			writeEquineJSONError(w, http.StatusBadRequest, containErr.Error())
			return
		}
		writeEquineJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to contain equine livestock dispatch: %v", err))
		return
	}

	s.broadcastEquineLivestockEvent("livestock_dispatch_updated", map[string]any{
		"type":     "livestock_dispatch_updated",
		"dispatch": updatedDispatch,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedDispatch)
}

func (s *Server) handleEquineLivestockStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("equine_livestock")
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

func (s *Server) broadcastEquineLivestockEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextEquineNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "equine_livestock",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "equine_livestock",
			SenderNodeID:  "equine-livestock-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
