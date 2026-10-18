package webfrontend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/scottdensmore/petspotr/pkg/humanetrap"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastHumaneTrapNano atomic.Int64
	humaneTrapSeedMu   sync.Mutex
)

func nextHumaneTrapNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHumaneTrapNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHumaneTrapNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

func randomHex8() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%08x", uint32(nextHumaneTrapNano()))
	}
	return hex.EncodeToString(b[:])
}

// HumaneTrapsPageData represents template data passed to templates/humane_traps.html.
type HumaneTrapsPageData struct {
	Traps  []domain.HumaneTrapDeployment
	Locale string
}

type createHumaneTrapRequest struct {
	TrapID         string   `json:"trapId"`
	PetID          string   `json:"petId"`
	PetName        string   `json:"petName"`
	TargetSpecies  string   `json:"targetSpecies"`
	TargetWeightKg float64  `json:"targetWeightKg"`
	TrapLabel      string   `json:"trapLabel"`
	TrapType       string   `json:"trapType"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	SectorID       string   `json:"sectorId"`
	BaitType       string   `json:"baitType"`
	InternalTempC  *float64 `json:"internalTempC"`
	BatteryPct     *int     `json:"batteryPct"`
	RssiDbm        *int     `json:"rssiDbm"`
}

type updateHumaneTrapTelemetryRequest struct {
	DoorClosed               bool     `json:"doorClosed"`
	InternalTempC            *float64 `json:"internalTempC"`
	BatteryPct               *int     `json:"batteryPct"`
	RssiDbm                  *int     `json:"rssiDbm"`
	AccelerometerPeakG       float64  `json:"accelerometerPeakG"`
	EstimatedCaptureWeightKg float64  `json:"estimatedCaptureWeightKg"`
	BaitReset                bool     `json:"baitReset"`
	BaitType                 string   `json:"baitType"`
}

type dispatchOrResolveHumaneTrapRequest struct {
	Action        string `json:"action"`
	ResponderID   string `json:"responderId"`
	ResponderName string `json:"responderName"`
	ETAMinutes    int    `json:"etaMinutes"`
	Outcome       string `json:"outcome"`
	Notes         string `json:"notes"`
}

func (s *Server) registerHumaneTrapRoutes() {
	s.mux.HandleFunc("GET /humane-traps", s.handleHumaneTrapsPage)
	s.mux.HandleFunc("GET /api/v1/humane-traps", s.handleListHumaneTraps)
	s.mux.HandleFunc("POST /api/v1/humane-traps", s.handleCreateHumaneTrap)
	s.mux.HandleFunc("POST /api/v1/humane-traps/{id}/telemetry", s.handleUpdateHumaneTrapTelemetry)
	s.mux.HandleFunc("POST /api/v1/humane-traps/{id}/dispatch", s.handleDispatchOrResolveHumaneTrap)
	s.mux.HandleFunc("GET /api/v1/humane-traps/stream", s.handleHumaneTrapsStream)
}

func defaultHumaneTrapDeployments(now time.Time) []domain.HumaneTrapDeployment {
	trippedAt := now.Add(-3 * time.Minute)
	slaDeadline := now.Add(12 * time.Minute)

	t1 := domain.HumaneTrapDeployment{
		TrapID:                   "trap-ht-101",
		PetID:                    "pet-2026-002",
		PetName:                  "Luna",
		TargetSpecies:            "CAT",
		TargetWeightKg:           4.2,
		TrapLabel:                "Tomahawk Tru-Catch #404 — Discovery Park Cedar Culvert",
		TrapType:                 domain.TrapTypeBoxGravity,
		Status:                   domain.TrapStatusDoorTrippedUrgent,
		Location:                 domain.GeoPoint{Latitude: 47.6612, Longitude: -122.4135},
		SectorID:                 "SEC-NW-04",
		BaitType:                 "SARDINES_IN_OIL",
		BaitPlacedAt:             now.Add(-1 * time.Hour),
		BaitFreshnessPct:         88,
		BaitState:                domain.BaitStateFreshPeak,
		DoorClosed:               true,
		InternalTempC:            16.4,
		BatteryPct:               92,
		RssiDbm:                  -68,
		AccelerometerPeakG:       2.85,
		EstimatedCaptureWeightKg: 4.1,
		TrippedAt:                &trippedAt,
		CheckSLADeadline:         &slaDeadline,
		AlertFlags:               []domain.HumaneTrapAlertFlag{domain.TrapAlertUrgentCheckSLA},
		TelemetrySummary:         "URGENT DOOR TRIPPED: Reed switch closed (2.85G spike, 4.1 kg match for Luna). 15-min Humane Check SLA active.",
		CreatedAt:                now.Add(-1 * time.Hour),
		UpdatedAt:                now.Add(-1 * time.Minute),
	}

	t2 := domain.HumaneTrapDeployment{
		TrapID:             "trap-ht-102",
		PetID:              "pet-2026-001",
		PetName:            "Buster",
		TargetSpecies:      "DOG",
		TargetWeightKg:     14.5,
		TrapLabel:          "Havahart Large Collar-Safe Drop Trap #108 — Green Lake Meadow",
		TrapType:           domain.TrapTypeDropRemote,
		Status:             domain.TrapStatusArmedActive,
		Location:           domain.GeoPoint{Latitude: 47.6805, Longitude: -122.3412},
		SectorID:           "SEC-NE-02",
		BaitType:           "ROTISSERIE_CHICKEN",
		BaitPlacedAt:       now.Add(-6 * time.Hour),
		BaitFreshnessPct:   42,
		BaitState:          domain.BaitStateDegradingRefresh,
		DoorClosed:         false,
		InternalTempC:      19.2,
		BatteryPct:         84,
		RssiDbm:            -74,
		AccelerometerPeakG: 0.08,
		AlertFlags:         nil,
		TelemetrySummary:   "Armed & monitoring (19.2°C, bait freshness 42% DEGRADING_REFRESH_SOON).",
		CreatedAt:          now.Add(-6 * time.Hour),
		UpdatedAt:          now.Add(-2 * time.Minute),
	}

	t3 := domain.HumaneTrapDeployment{
		TrapID:             "trap-ht-103",
		PetID:              "pet-2026-002",
		PetName:            "Luna",
		TargetSpecies:      "CAT",
		TargetWeightKg:     4.2,
		TrapLabel:          "Cellular Smart-Enclosure #212 — Ballard Locks South Bank",
		TrapType:           domain.TrapTypeEnclosureCellular,
		Status:             domain.TrapStatusThermalLockout,
		Location:           domain.GeoPoint{Latitude: 47.6655, Longitude: -122.3970},
		SectorID:           "SEC-SW-01",
		BaitType:           "WET_RECOVERY_PATE",
		BaitPlacedAt:       now.Add(-5 * time.Hour),
		BaitFreshnessPct:   12,
		BaitState:          domain.BaitStateSpoiledExpired,
		DoorClosed:         false,
		InternalTempC:      34.6,
		BatteryPct:         78,
		RssiDbm:            -71,
		AccelerometerPeakG: 0.05,
		AlertFlags: []domain.HumaneTrapAlertFlag{
			domain.TrapAlertHyperthermiaHazard,
			domain.TrapAlertBaitSpoiled,
		},
		TelemetrySummary: "THERMAL LOCKOUT (34.6°C >= 32.0°C): Auto-disarmed to prevent hyperthermia; bait spoiled (12%).",
		CreatedAt:        now.Add(-5 * time.Hour),
		UpdatedAt:        now.Add(-3 * time.Minute),
	}

	return []domain.HumaneTrapDeployment{t1, t2, t3}
}

func (s *Server) ensureDefaultHumaneTraps(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	humaneTrapSeedMu.Lock()
	defer humaneTrapSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionHumaneTrapDeployments)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultHumaneTrapDeployments(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionHumaneTrapDeployments, item.TrapID, data)
		}
	}
}

func (s *Server) loadHumaneTraps(ctx context.Context, petIDFilter, statusFilter string) []domain.HumaneTrapDeployment {
	traps := make([]domain.HumaneTrapDeployment, 0)
	if s.stateStore == nil {
		return traps
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionHumaneTrapDeployments)
	if err != nil {
		return traps
	}
	for _, b := range raw {
		var item domain.HumaneTrapDeployment
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.Status), statusFilter) {
				continue
			}
			traps = append(traps, item)
		}
	}
	sort.Slice(traps, func(i, j int) bool {
		if !traps[i].UpdatedAt.Equal(traps[j].UpdatedAt) {
			return traps[i].UpdatedAt.After(traps[j].UpdatedAt)
		}
		return traps[i].TrapID < traps[j].TrapID
	})
	return traps
}

func writeHumaneTrapJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) promoteTrapCaptureToSighting(ctx context.Context, trap domain.HumaneTrapDeployment, now time.Time) (domain.Sighting, error) {
	sightingID := "sighting-trap-" + randomHex8()
	reporterID := strings.TrimSpace(trap.AssignedResponderID)
	if reporterID == "" {
		reporterID = "trap-telemetry-iot"
	}
	coords := &domain.LocationPoint{
		Latitude:  trap.Location.Latitude,
		Longitude: trap.Location.Longitude,
	}
	sighting := domain.Sighting{
		ID:                  sightingID,
		SightingID:          sightingID,
		PetID:               trap.PetID,
		LostPetID:           trap.PetID,
		ReportedAt:          now,
		SightedAt:           now,
		LocationDescription: fmt.Sprintf("Humane Trap Capture (%s — %s)", trap.TrapLabel, reporterID),
		Coordinates:         coords,
		Notes:               trap.TelemetrySummary,
		Status:              domain.SightingStatusActive,
	}
	if s.stateStore != nil {
		sBytes, err := json.Marshal(sighting)
		if err != nil {
			return domain.Sighting{}, err
		}
		if err := s.stateStore.SaveState(ctx, store.SightingsCollection, sighting.SightingID, sBytes); err != nil {
			return domain.Sighting{}, err
		}
	}
	return sighting, nil
}

func (s *Server) handleHumaneTrapsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultHumaneTraps(r.Context())
	traps := s.loadHumaneTraps(r.Context(), "", "")

	tmpl, err := template.New("humane_traps.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/humane_traps.html")
	if err != nil {
		http.Error(w, "Failed to load Humane Traps template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HumaneTrapsPageData{
		Traps:  traps,
		Locale: locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHumaneTraps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultHumaneTraps(r.Context())
	q := r.URL.Query()
	traps := s.loadHumaneTraps(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"traps": traps,
	})
}

func (s *Server) handleCreateHumaneTrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createHumaneTrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultHumaneTraps(r.Context())

	seqNano := nextHumaneTrapNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	trapID := strings.TrimSpace(req.TrapID)
	if trapID == "" {
		trapID = "trap-ht-" + randomHex8()
	}

	internalTempC := 18.5
	if req.InternalTempC != nil {
		internalTempC = *req.InternalTempC
	}
	batteryPct := 98
	if req.BatteryPct != nil {
		batteryPct = *req.BatteryPct
	}
	rssiDbm := -64
	if req.RssiDbm != nil {
		rssiDbm = *req.RssiDbm
	}

	trap := domain.HumaneTrapDeployment{
		TrapID:         trapID,
		PetID:          strings.TrimSpace(req.PetID),
		PetName:        strings.TrimSpace(req.PetName),
		TargetSpecies:  strings.ToUpper(strings.TrimSpace(req.TargetSpecies)),
		TargetWeightKg: req.TargetWeightKg,
		TrapLabel:      strings.TrimSpace(req.TrapLabel),
		TrapType:       domain.HumaneTrapType(strings.ToUpper(strings.TrimSpace(req.TrapType))),
		Status:         domain.TrapStatusArmedActive,
		Location: domain.GeoPoint{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
		},
		SectorID:     strings.TrimSpace(req.SectorID),
		BaitType:     strings.ToUpper(strings.TrimSpace(req.BaitType)),
		BaitPlacedAt: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := humanetrap.ApplyTelemetryUpdate(&trap, humanetrap.TelemetryUpdateInput{
		DoorClosed:         false,
		InternalTempC:      internalTempC,
		BatteryPct:         batteryPct,
		RssiDbm:            rssiDbm,
		AccelerometerPeakG: 0.05,
	}, now); err != nil {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := trap.Validate(); err != nil {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	trapBytes, err := json.Marshal(trap)
	if err != nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "Failed to encode humane trap deployment")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionHumaneTrapDeployments, trap.TrapID, trapBytes); err != nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "Failed to save humane trap deployment")
		return
	}

	s.broadcastHumaneTrapEvent("humane_trap_updated", map[string]any{
		"type": "humane_trap_updated",
		"trap": trap,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(trap)
}

func (s *Server) handleUpdateHumaneTrapTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	trapID := strings.TrimSpace(r.PathValue("id"))
	if trapID == "" {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, "trap id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req updateHumaneTrapTelemetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultHumaneTraps(r.Context())

	seqNano := nextHumaneTrapNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedTrap domain.HumaneTrapDeployment
		updateErr   error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionHumaneTrapDeployments, trapID, func(current []byte) ([]byte, error) {
		var existing domain.HumaneTrapDeployment
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt humane trap deployment: %w", err)
		}

		internalTempC := existing.InternalTempC
		if req.InternalTempC != nil {
			internalTempC = *req.InternalTempC
		}
		batteryPct := existing.BatteryPct
		if req.BatteryPct != nil {
			batteryPct = *req.BatteryPct
		}
		rssiDbm := existing.RssiDbm
		if req.RssiDbm != nil {
			rssiDbm = *req.RssiDbm
		}

		if tErr := humanetrap.ApplyTelemetryUpdate(&existing, humanetrap.TelemetryUpdateInput{
			DoorClosed:               req.DoorClosed,
			InternalTempC:            internalTempC,
			BatteryPct:               batteryPct,
			RssiDbm:                  rssiDbm,
			AccelerometerPeakG:       req.AccelerometerPeakG,
			EstimatedCaptureWeightKg: req.EstimatedCaptureWeightKg,
			BaitReset:                req.BaitReset,
			BaitType:                 req.BaitType,
		}, now); tErr != nil {
			updateErr = tErr
			return nil, tErr
		}
		if vErr := existing.Validate(); vErr != nil {
			updateErr = vErr
			return nil, vErr
		}
		updatedTrap = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeHumaneTrapJSONError(w, http.StatusNotFound, "Humane trap deployment not found")
			return
		}
		if updateErr != nil {
			writeHumaneTrapJSONError(w, http.StatusBadRequest, updateErr.Error())
			return
		}
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update humane trap telemetry: %v", err))
		return
	}

	s.broadcastHumaneTrapEvent("humane_trap_updated", map[string]any{
		"type": "humane_trap_updated",
		"trap": updatedTrap,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedTrap)
}

func (s *Server) handleDispatchOrResolveHumaneTrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	trapID := strings.TrimSpace(r.PathValue("id"))
	if trapID == "" {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, "trap id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req dispatchOrResolveHumaneTrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHumaneTrapJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultHumaneTraps(r.Context())

	seqNano := nextHumaneTrapNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	action := strings.ToUpper(strings.TrimSpace(req.Action))
	outcome := humanetrap.ResolutionOutcome(strings.ToUpper(strings.TrimSpace(req.Outcome)))

	var (
		updatedTrap domain.HumaneTrapDeployment
		opErr       error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionHumaneTrapDeployments, trapID, func(current []byte) ([]byte, error) {
		var existing domain.HumaneTrapDeployment
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt humane trap deployment: %w", err)
		}

		if outcome != "" {
			if rErr := humanetrap.ResolveTrapInspection(&existing, outcome, req.Notes, now); rErr != nil {
				opErr = rErr
				return nil, rErr
			}
		} else if action == "DISPATCH" || strings.TrimSpace(req.ResponderID) != "" {
			if dErr := humanetrap.DispatchTrapResponder(&existing, humanetrap.DispatchInput{
				ResponderID:   req.ResponderID,
				ResponderName: req.ResponderName,
				ETAMinutes:    req.ETAMinutes,
			}, now); dErr != nil {
				opErr = dErr
				return nil, dErr
			}
		} else {
			opErr = errors.New("humanetrap: either dispatch responder details or resolution outcome is required")
			return nil, opErr
		}

		if vErr := existing.Validate(); vErr != nil {
			opErr = vErr
			return nil, vErr
		}
		updatedTrap = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeHumaneTrapJSONError(w, http.StatusNotFound, "Humane trap deployment not found")
			return
		}
		if opErr != nil {
			writeHumaneTrapJSONError(w, http.StatusBadRequest, opErr.Error())
			return
		}
		writeHumaneTrapJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to dispatch or resolve humane trap: %v", err))
		return
	}

	var promotedSighting *domain.Sighting
	if outcome == humanetrap.OutcomeTargetPetCaptured && updatedTrap.PromotedSightingID == "" {
		sighting, promErr := s.promoteTrapCaptureToSighting(r.Context(), updatedTrap, now)
		if promErr == nil {
			updatedTrap.PromotedSightingID = sighting.SightingID
			promotedSighting = &sighting
			if updatedBytes, mErr := json.Marshal(updatedTrap); mErr == nil {
				_ = s.stateStore.SaveState(r.Context(), store.CollectionHumaneTrapDeployments, updatedTrap.TrapID, updatedBytes)
			}
		}
	}

	s.broadcastHumaneTrapEvent("humane_trap_updated", map[string]any{
		"type":     "humane_trap_updated",
		"trap":     updatedTrap,
		"sighting": promotedSighting,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedTrap)
}

func (s *Server) handleHumaneTrapsStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("humane_traps")
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

func (s *Server) broadcastHumaneTrapEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHumaneTrapNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "humane_traps",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "humane_traps",
			SenderNodeID:  "humane-trap-telemetry-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
