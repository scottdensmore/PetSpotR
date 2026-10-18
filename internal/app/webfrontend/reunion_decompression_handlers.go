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

	"github.com/scottdensmore/petspotr/pkg/decompression"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastDecompNano atomic.Int64
	decompSeedMu   sync.Mutex
)

func nextDecompNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastDecompNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastDecompNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// ReunionDecompressionPageData represents template data passed to templates/reunion_decompression.html.
type ReunionDecompressionPageData struct {
	Plans  []domain.ReunionDecompressionPlan
	Locale string
}

type createDecompressionPlanRequest struct {
	PlanID              string  `json:"planId"`
	PetID               string  `json:"petId"`
	PetName             string  `json:"petName"`
	Species             string  `json:"species"`
	DaysMissing         int     `json:"daysMissing"`
	WeightLossPct       float64 `json:"weightLossPct"`
	EscapeVector        string  `json:"escapeVector"`
	GpsCollarEquipped   bool    `json:"gpsCollarEquipped"`
	DoubleLeashVerified bool    `json:"doubleLeashVerified"`
}

type updateDecompressionProgressRequest struct {
	CompleteAllChecklist  bool   `json:"completeAllChecklist"`
	RemediateAllPerimeter bool   `json:"remediateAllPerimeter"`
	CompleteItemID        string `json:"completeItemId"`
	RemediateFindingID    string `json:"remediateFindingId"`
	GpsCollarEquipped     *bool  `json:"gpsCollarEquipped"`
	DoubleLeashVerified   *bool  `json:"doubleLeashVerified"`
	Notes                 string `json:"notes"`
}

func (s *Server) registerReunionDecompressionRoutes() {
	s.mux.HandleFunc("GET /reunion-decompression", s.handleReunionDecompressionPage)
	s.mux.HandleFunc("GET /api/v1/reunion-decompression/plans", s.handleListDecompressionPlans)
	s.mux.HandleFunc("POST /api/v1/reunion-decompression/plans", s.handleCreateDecompressionPlan)
	s.mux.HandleFunc("POST /api/v1/reunion-decompression/plans/{id}/progress", s.handleUpdateDecompressionProgress)
	s.mux.HandleFunc("GET /api/v1/reunion-decompression/stream", s.handleReunionDecompressionStream)
}

func defaultReunionDecompressionPlans(now time.Time) []domain.ReunionDecompressionPlan {
	plans := make([]domain.ReunionDecompressionPlan, 0, 3)

	if p1, err := decompression.GenerateDecompressionPlan(decompression.CreatePlanInput{
		PlanID:              "decomp-plan-101",
		PetID:               "pet-2026-002",
		PetName:             "Luna",
		Species:             "CAT",
		DaysMissing:         9,
		WeightLossPct:       14.5,
		EscapeVector:        "DOOR_DASH_BOLT",
		GpsCollarEquipped:   false,
		DoubleLeashVerified: false,
	}, now.Add(-1*time.Minute)); err == nil && p1 != nil {
		plans = append(plans, *p1)
	}

	if p2, err := decompression.GenerateDecompressionPlan(decompression.CreatePlanInput{
		PlanID:              "decomp-plan-102",
		PetID:               "pet-2026-001",
		PetName:             "Buster",
		Species:             "DOG",
		DaysMissing:         3,
		WeightLossPct:       4.0,
		EscapeVector:        "UNLATCHED_SIDE_GATE",
		GpsCollarEquipped:   true,
		DoubleLeashVerified: false,
	}, now.Add(-3*time.Minute)); err == nil && p2 != nil {
		doubleLeash := true
		_ = decompression.ApplyDecompressionProgress(p2, decompression.ProgressUpdateInput{
			CompleteItemID:      "chk-quiet-refuge",
			RemediateFindingID:  "aud-primary-vector",
			DoubleLeashVerified: &doubleLeash,
		}, now.Add(-2*time.Minute))
		plans = append(plans, *p2)
	}

	if p3, err := decompression.GenerateDecompressionPlan(decompression.CreatePlanInput{
		PlanID:              "decomp-plan-103",
		PetID:               "pet-2026-003",
		PetName:             "Cooper",
		Species:             "DOG",
		DaysMissing:         2,
		WeightLossPct:       2.5,
		EscapeVector:        "DIG_UNDER_FENCE",
		GpsCollarEquipped:   true,
		DoubleLeashVerified: true,
	}, now.Add(-5*time.Minute)); err == nil && p3 != nil {
		_ = decompression.ApplyDecompressionProgress(p3, decompression.ProgressUpdateInput{
			CompleteAllChecklist:  true,
			RemediateAllPerimeter: true,
			Notes:                 "All 14-day cortisol & perimeter fortification milestones verified.",
		}, now.Add(-4*time.Minute))
		plans = append(plans, *p3)
	}

	return plans
}

func (s *Server) ensureDefaultDecompressionPlans(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	decompSeedMu.Lock()
	defer decompSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionReunionDecompressionPlans)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultReunionDecompressionPlans(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionReunionDecompressionPlans, item.PlanID, data)
		}
	}
}

func (s *Server) loadDecompressionPlans(ctx context.Context, petIDFilter, phaseFilter string) []domain.ReunionDecompressionPlan {
	plans := make([]domain.ReunionDecompressionPlan, 0)
	if s.stateStore == nil {
		return plans
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionReunionDecompressionPlans)
	if err != nil {
		return plans
	}
	for _, b := range raw {
		var item domain.ReunionDecompressionPlan
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if phaseFilter != "" && !strings.EqualFold(string(item.Phase), phaseFilter) {
				continue
			}
			plans = append(plans, item)
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if !plans[i].UpdatedAt.Equal(plans[j].UpdatedAt) {
			return plans[i].UpdatedAt.After(plans[j].UpdatedAt)
		}
		return plans[i].PlanID < plans[j].PlanID
	})
	return plans
}

func writeDecompJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleReunionDecompressionPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultDecompressionPlans(r.Context())
	plans := s.loadDecompressionPlans(r.Context(), "", "")

	tmpl, err := template.New("reunion_decompression.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/reunion_decompression.html")
	if err != nil {
		http.Error(w, "Failed to load Reunion Decompression template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ReunionDecompressionPageData{
		Plans:  plans,
		Locale: locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListDecompressionPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeDecompJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultDecompressionPlans(r.Context())
	q := r.URL.Query()
	plans := s.loadDecompressionPlans(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("phase")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"plans": plans,
	})
}

func (s *Server) handleCreateDecompressionPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeDecompJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createDecompressionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecompJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultDecompressionPlans(r.Context())

	seqNano := nextDecompNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	planID := strings.TrimSpace(req.PlanID)
	if planID == "" {
		planID = "decomp-plan-" + randomHex8()
	}

	plan, err := decompression.GenerateDecompressionPlan(decompression.CreatePlanInput{
		PlanID:              planID,
		PetID:               req.PetID,
		PetName:             req.PetName,
		Species:             req.Species,
		DaysMissing:         req.DaysMissing,
		WeightLossPct:       req.WeightLossPct,
		EscapeVector:        req.EscapeVector,
		GpsCollarEquipped:   req.GpsCollarEquipped,
		DoubleLeashVerified: req.DoubleLeashVerified,
	}, now)
	if err != nil {
		writeDecompJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	planBytes, err := json.Marshal(plan)
	if err != nil {
		writeDecompJSONError(w, http.StatusInternalServerError, "Failed to encode decompression plan")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionReunionDecompressionPlans, plan.PlanID, planBytes); err != nil {
		writeDecompJSONError(w, http.StatusInternalServerError, "Failed to save decompression plan")
		return
	}

	s.broadcastReunionDecompressionEvent("decompression_plan_updated", map[string]any{
		"type": "decompression_plan_updated",
		"plan": plan,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(plan)
}

func (s *Server) handleUpdateDecompressionProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeDecompJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	planID := strings.TrimSpace(r.PathValue("id"))
	if planID == "" {
		writeDecompJSONError(w, http.StatusBadRequest, "plan id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req updateDecompressionProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecompJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultDecompressionPlans(r.Context())

	seqNano := nextDecompNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedPlan domain.ReunionDecompressionPlan
		progErr     error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionReunionDecompressionPlans, planID, func(current []byte) ([]byte, error) {
		var existing domain.ReunionDecompressionPlan
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt decompression plan: %w", err)
		}

		if pErr := decompression.ApplyDecompressionProgress(&existing, decompression.ProgressUpdateInput{
			CompleteAllChecklist:  req.CompleteAllChecklist,
			RemediateAllPerimeter: req.RemediateAllPerimeter,
			CompleteItemID:        req.CompleteItemID,
			RemediateFindingID:    req.RemediateFindingID,
			GpsCollarEquipped:     req.GpsCollarEquipped,
			DoubleLeashVerified:   req.DoubleLeashVerified,
			Notes:                 req.Notes,
		}, now); pErr != nil {
			progErr = pErr
			return nil, pErr
		}
		updatedPlan = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeDecompJSONError(w, http.StatusNotFound, "Decompression plan not found")
			return
		}
		if progErr != nil {
			writeDecompJSONError(w, http.StatusBadRequest, progErr.Error())
			return
		}
		writeDecompJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update decompression progress: %v", err))
		return
	}

	s.broadcastReunionDecompressionEvent("decompression_plan_updated", map[string]any{
		"type": "decompression_plan_updated",
		"plan": updatedPlan,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedPlan)
}

func (s *Server) handleReunionDecompressionStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("reunion_decompression")
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

func (s *Server) broadcastReunionDecompressionEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextDecompNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "reunion_decompression",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "reunion_decompression",
			SenderNodeID:  "reunion-decompression-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
