package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/swiftwater"
)

var (
	lastSwiftwaterNano atomic.Int64
	swiftwaterSeedMu   sync.Mutex
)

func nextSwiftwaterNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastSwiftwaterNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastSwiftwaterNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// SwiftwaterRescuePageData represents template data passed to templates/swiftwater_rescue.html.
type SwiftwaterRescuePageData struct {
	Ops    []domain.SwiftwaterRescueOp
	Locale string
}

type createSwiftwaterRescueOpRequest struct {
	OpID                 string  `json:"opId"`
	PetID                string  `json:"petId"`
	PetName              string  `json:"petName"`
	Species              string  `json:"species"`
	WaterwayName         string  `json:"waterwayName"`
	HazardEnvironment    string  `json:"hazardEnvironment"`
	WaterTempFahrenheit  int     `json:"waterTempFahrenheit"`
	CurrentVelocityKnots float64 `json:"currentVelocityKnots"`
	PFDThrowBagDeployed  bool    `json:"pfdThrowBagDeployed"`
	WarmingBlanketReady  bool    `json:"warmingBlanketReady"`
}

type extractSwiftwaterRescueOpRequest struct {
	DeployRiggingAndFlotation   bool   `json:"deployRiggingAndFlotation"`
	CompleteShorelineExtraction bool   `json:"completeShorelineExtraction"`
	CompleteStepID              string `json:"completeStepId"`
	PFDThrowBagDeployed         *bool  `json:"pfdThrowBagDeployed"`
	WarmingBlanketReady         *bool  `json:"warmingBlanketReady"`
}

func (s *Server) registerSwiftwaterRescueRoutes() {
	s.mux.HandleFunc("GET /swiftwater-rescue", s.handleSwiftwaterRescuePage)
	s.mux.HandleFunc("GET /api/v1/swiftwater-rescue/ops", s.handleListSwiftwaterRescueOps)
	s.mux.HandleFunc("POST /api/v1/swiftwater-rescue/ops", s.handleCreateSwiftwaterRescueOp)
	s.mux.HandleFunc("POST /api/v1/swiftwater-rescue/ops/{id}/extract", s.handleExtractSwiftwaterRescueOp)
	s.mux.HandleFunc("GET /api/v1/swiftwater-rescue/stream", s.handleSwiftwaterRescueStream)
}

func defaultSwiftwaterRescueOps(now time.Time) []domain.SwiftwaterRescueOp {
	ops := make([]domain.SwiftwaterRescueOp, 0, 3)

	if o1, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-op-101",
		PetID:                "pet-2026-001",
		PetName:              "Yukon",
		Species:              "DOG",
		WaterwayName:         "Green Lake North Ice Shelf",
		HazardEnvironment:    "FRACTURED_LAKE_ICE_SHELF",
		WaterTempFahrenheit:  34,
		CurrentVelocityKnots: 1.5,
		PFDThrowBagDeployed:  false,
		WarmingBlanketReady:  false,
	}, now.Add(-1*time.Minute)); err == nil && o1 != nil {
		ops = append(ops, *o1)
	}

	if o2, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-op-102",
		PetID:                "pet-2026-002",
		PetName:              "Misty",
		Species:              "CAT",
		WaterwayName:         "Thornton Creek Flood Weir",
		HazardEnvironment:    "FLASH_FLOOD_STORM_CHANNEL",
		WaterTempFahrenheit:  47,
		CurrentVelocityKnots: 5.5,
		PFDThrowBagDeployed:  false,
		WarmingBlanketReady:  false,
	}, now.Add(-3*time.Minute)); err == nil && o2 != nil {
		_ = swiftwater.ApplySwiftwaterOpUpdate(o2, swiftwater.SwiftwaterOpUpdateInput{
			DeployRiggingAndFlotation: true,
		}, now.Add(-2*time.Minute))
		ops = append(ops, *o2)
	}

	if o3, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-op-103",
		PetID:                "pet-2026-003",
		PetName:              "Harbor",
		Species:              "DOG",
		WaterwayName:         "Duwamish Turning Basin Mudflat",
		HazardEnvironment:    "TIDAL_MUDFLAT_ESTUARY",
		WaterTempFahrenheit:  52,
		CurrentVelocityKnots: 2.0,
		PFDThrowBagDeployed:  true,
		WarmingBlanketReady:  true,
	}, now.Add(-5*time.Minute)); err == nil && o3 != nil {
		_ = swiftwater.ApplySwiftwaterOpUpdate(o3, swiftwater.SwiftwaterOpUpdateInput{
			CompleteShorelineExtraction: true,
		}, now.Add(-4*time.Minute))
		ops = append(ops, *o3)
	}

	return ops
}

func (s *Server) ensureDefaultSwiftwaterRescueOps(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	swiftwaterSeedMu.Lock()
	defer swiftwaterSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionSwiftwaterRescueOps)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultSwiftwaterRescueOps(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionSwiftwaterRescueOps, item.OpID, data)
		}
	}
}

func (s *Server) loadSwiftwaterRescueOps(ctx context.Context, petIDFilter, statusFilter string) []domain.SwiftwaterRescueOp {
	ops := make([]domain.SwiftwaterRescueOp, 0)
	if s.stateStore == nil {
		return ops
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionSwiftwaterRescueOps)
	if err != nil {
		return ops
	}
	for _, b := range raw {
		var item domain.SwiftwaterRescueOp
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.ExtractionStatus), statusFilter) {
				continue
			}
			ops = append(ops, item)
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].UpdatedAt != ops[j].UpdatedAt {
			return ops[i].UpdatedAt > ops[j].UpdatedAt
		}
		return ops[i].OpID < ops[j].OpID
	})
	return ops
}

func (s *Server) handleSwiftwaterRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultSwiftwaterRescueOps(r.Context())
	ops := s.loadSwiftwaterRescueOps(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := SwiftwaterRescuePageData{
		Ops:    ops,
		Locale: locale,
	}

	s.renderCockpitTemplate(w, r, "swiftwater_rescue.html", "Swiftwater Rescue", data)
}

func (s *Server) handleListSwiftwaterRescueOps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultSwiftwaterRescueOps(r.Context())
	q := r.URL.Query()
	ops := s.loadSwiftwaterRescueOps(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"ops": ops,
	})
}

func (s *Server) handleCreateSwiftwaterRescueOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createSwiftwaterRescueOpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultSwiftwaterRescueOps(r.Context())

	seqNano := nextSwiftwaterNano()
	now := time.Unix(0, seqNano).UTC()

	opID := strings.TrimSpace(req.OpID)
	if opID == "" {
		opID = "sw-op-" + randomHex8()
	}

	rescueOp, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 opID,
		PetID:                req.PetID,
		PetName:              req.PetName,
		Species:              req.Species,
		WaterwayName:         req.WaterwayName,
		HazardEnvironment:    req.HazardEnvironment,
		WaterTempFahrenheit:  req.WaterTempFahrenheit,
		CurrentVelocityKnots: req.CurrentVelocityKnots,
		PFDThrowBagDeployed:  req.PFDThrowBagDeployed,
		WarmingBlanketReady:  req.WarmingBlanketReady,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	opBytes, err := json.Marshal(rescueOp)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode swiftwater rescue op")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionSwiftwaterRescueOps, rescueOp.OpID, opBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save swiftwater rescue op")
		return
	}

	s.broadcastSwiftwaterRescueEvent("swiftwater_op_updated", map[string]any{
		"type":         "swiftwater_op_updated",
		"swiftwaterOp": rescueOp,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, rescueOp)
}

func (s *Server) handleExtractSwiftwaterRescueOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "op id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req extractSwiftwaterRescueOpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultSwiftwaterRescueOps(r.Context())

	seqNano := nextSwiftwaterNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedOp  domain.SwiftwaterRescueOp
		extractErr error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionSwiftwaterRescueOps, opID, func(current []byte) ([]byte, error) {
		var existing domain.SwiftwaterRescueOp
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt swiftwater rescue op: %w", err)
		}

		if eErr := swiftwater.ApplySwiftwaterOpUpdate(&existing, swiftwater.SwiftwaterOpUpdateInput{
			DeployRiggingAndFlotation:   req.DeployRiggingAndFlotation,
			CompleteShorelineExtraction: req.CompleteShorelineExtraction,
			CompleteStepID:              req.CompleteStepID,
			PFDThrowBagDeployed:         req.PFDThrowBagDeployed,
			WarmingBlanketReady:         req.WarmingBlanketReady,
		}, now); eErr != nil {
			extractErr = eErr
			return nil, eErr
		}
		updatedOp = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Swiftwater rescue op not found")
			return
		}
		if extractErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, extractErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update swiftwater rescue op: %v", err))
		return
	}

	s.broadcastSwiftwaterRescueEvent("swiftwater_op_updated", map[string]any{
		"type":         "swiftwater_op_updated",
		"swiftwaterOp": updatedOp,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedOp)
}

func (s *Server) handleSwiftwaterRescueStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "swiftwater_rescue")
}

func (s *Server) broadcastSwiftwaterRescueEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("swiftwater_rescue", "swiftwater-rescue-hub", eventType, payload, now)
}
