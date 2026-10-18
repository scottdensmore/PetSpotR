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
	"github.com/scottdensmore/petspotr/pkg/municipalcad"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastCADNano atomic.Int64
	cadSeedMu   sync.Mutex
)

func nextCADNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastCADNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastCADNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// MunicipalCADPageData represents template data passed to templates/municipal_cad.html.
type MunicipalCADPageData struct {
	Incidents []domain.MunicipalCADIncident
	Locale    string
}

type createCADIncidentRequest struct {
	IncidentID            string `json:"incidentId"`
	CADCallNumber         string `json:"cadCallNumber"`
	PetID                 string `json:"petId"`
	PetName               string `json:"petName"`
	Species               string `json:"species"`
	RoadwayName           string `json:"roadwayName"`
	SpeedLimitMPH         int    `json:"speedLimitMph"`
	LaneCount             int    `json:"laneCount"`
	PetBehaviorState      string `json:"petBehaviorState"`
	ACOUnitAssigned       string `json:"acoUnitAssigned"`
	DOTSignAdvisoryActive bool   `json:"dotSignAdvisoryActive"`
}

type dispatchCADIncidentRequest struct {
	DeployHighwayContainment bool   `json:"deployHighwayContainment"`
	ResolveSafeCustody       bool   `json:"resolveSafeCustody"`
	CompleteStepID           string `json:"completeStepId"`
	DOTSignAdvisoryActive    *bool  `json:"dotSignAdvisoryActive"`
	ACOUnitAssigned          string `json:"acoUnitAssigned"`
}

func (s *Server) registerMunicipalCADRoutes() {
	s.mux.HandleFunc("GET /municipal-cad", s.handleMunicipalCADPage)
	s.mux.HandleFunc("GET /api/v1/municipal-cad/incidents", s.handleListMunicipalCADIncidents)
	s.mux.HandleFunc("POST /api/v1/municipal-cad/incidents", s.handleCreateMunicipalCADIncident)
	s.mux.HandleFunc("POST /api/v1/municipal-cad/incidents/{id}/dispatch", s.handleDispatchMunicipalCADIncident)
	s.mux.HandleFunc("GET /api/v1/municipal-cad/stream", s.handleMunicipalCADStream)
}

func defaultMunicipalCADIncidents(now time.Time) []domain.MunicipalCADIncident {
	incidents := make([]domain.MunicipalCADIncident, 0, 3)

	if inc1, err := municipalcad.IngestMunicipal311Call(municipalcad.CreateCADIncidentInput{
		IncidentID:            "cad-inc-101",
		CADCallNumber:         "311-SEA-8841",
		PetID:                 "pet-2026-001",
		PetName:               "Cooper",
		Species:               "DOG",
		RoadwayName:           "I-5 Express Lanes Milepost 167 Median",
		SpeedLimitMPH:         60,
		LaneCount:             4,
		PetBehaviorState:      "BOLTING_INTO_TRAFFIC",
		DOTSignAdvisoryActive: false,
		ACOUnitAssigned:       "",
	}, now.Add(-1*time.Minute)); err == nil && inc1 != nil {
		incidents = append(incidents, *inc1)
	}

	if inc2, err := municipalcad.IngestMunicipal311Call(municipalcad.CreateCADIncidentInput{
		IncidentID:            "cad-inc-102",
		CADCallNumber:         "311-SEA-8835",
		PetID:                 "pet-2026-002",
		PetName:               "Hazel",
		Species:               "DOG",
		RoadwayName:           "SR-99 Aurora Bridge North Approach",
		SpeedLimitMPH:         45,
		LaneCount:             3,
		PetBehaviorState:      "HUDDLED_IN_MEDIAN",
		DOTSignAdvisoryActive: false,
		ACOUnitAssigned:       "",
	}, now.Add(-3*time.Minute)); err == nil && inc2 != nil {
		_ = municipalcad.ApplyCADDispatchAction(inc2, municipalcad.CADDispatchUpdateInput{
			DeployHighwayContainment: true,
		}, now.Add(-2*time.Minute))
		incidents = append(incidents, *inc2)
	}

	if inc3, err := municipalcad.IngestMunicipal311Call(municipalcad.CreateCADIncidentInput{
		IncidentID:            "cad-inc-103",
		CADCallNumber:         "311-SEA-8819",
		PetID:                 "pet-2026-003",
		PetName:               "Biscuit",
		Species:               "CAT",
		RoadwayName:           "NE 65th St Park-and-Ride Overpass",
		SpeedLimitMPH:         30,
		LaneCount:             2,
		PetBehaviorState:      "TROTTING_SHOULDER",
		DOTSignAdvisoryActive: false,
		ACOUnitAssigned:       "",
	}, now.Add(-5*time.Minute)); err == nil && inc3 != nil {
		_ = municipalcad.ApplyCADDispatchAction(inc3, municipalcad.CADDispatchUpdateInput{
			ResolveSafeCustody: true,
		}, now.Add(-4*time.Minute))
		incidents = append(incidents, *inc3)
	}

	return incidents
}

func (s *Server) ensureDefaultMunicipalCADIncidents(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	cadSeedMu.Lock()
	defer cadSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionMunicipalCADIncidents)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultMunicipalCADIncidents(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionMunicipalCADIncidents, item.IncidentID, data)
		}
	}
}

func (s *Server) loadMunicipalCADIncidents(ctx context.Context, petIDFilter, statusFilter string) []domain.MunicipalCADIncident {
	incidents := make([]domain.MunicipalCADIncident, 0)
	if s.stateStore == nil {
		return incidents
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionMunicipalCADIncidents)
	if err != nil {
		return incidents
	}
	for _, b := range raw {
		var item domain.MunicipalCADIncident
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.DispatchStatus), statusFilter) {
				continue
			}
			incidents = append(incidents, item)
		}
	}
	sort.Slice(incidents, func(i, j int) bool {
		if incidents[i].UpdatedAt != incidents[j].UpdatedAt {
			return incidents[i].UpdatedAt > incidents[j].UpdatedAt
		}
		return incidents[i].IncidentID < incidents[j].IncidentID
	})
	return incidents
}

func writeCADJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleMunicipalCADPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultMunicipalCADIncidents(r.Context())
	incidents := s.loadMunicipalCADIncidents(r.Context(), "", "")

	tmpl, err := template.New("municipal_cad.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/municipal_cad.html")
	if err != nil {
		http.Error(w, "Failed to load Municipal CAD template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MunicipalCADPageData{
		Incidents: incidents,
		Locale:    locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListMunicipalCADIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCADJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultMunicipalCADIncidents(r.Context())
	q := r.URL.Query()
	incidents := s.loadMunicipalCADIncidents(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"incidents": incidents,
	})
}

func (s *Server) handleCreateMunicipalCADIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCADJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createCADIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCADJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultMunicipalCADIncidents(r.Context())

	seqNano := nextCADNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	incidentID := strings.TrimSpace(req.IncidentID)
	if incidentID == "" {
		incidentID = "cad-inc-" + randomHex8()
	}

	incident, err := municipalcad.IngestMunicipal311Call(municipalcad.CreateCADIncidentInput{
		IncidentID:            incidentID,
		CADCallNumber:         req.CADCallNumber,
		PetID:                 req.PetID,
		PetName:               req.PetName,
		Species:               req.Species,
		RoadwayName:           req.RoadwayName,
		SpeedLimitMPH:         req.SpeedLimitMPH,
		LaneCount:             req.LaneCount,
		PetBehaviorState:      req.PetBehaviorState,
		ACOUnitAssigned:       req.ACOUnitAssigned,
		DOTSignAdvisoryActive: req.DOTSignAdvisoryActive,
	}, now)
	if err != nil {
		writeCADJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	incBytes, err := json.Marshal(incident)
	if err != nil {
		writeCADJSONError(w, http.StatusInternalServerError, "Failed to encode municipal CAD incident")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionMunicipalCADIncidents, incident.IncidentID, incBytes); err != nil {
		writeCADJSONError(w, http.StatusInternalServerError, "Failed to save municipal CAD incident")
		return
	}

	s.broadcastMunicipalCADEvent("municipal_cad_updated", map[string]any{
		"type":     "municipal_cad_updated",
		"incident": incident,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incident)
}

func (s *Server) handleDispatchMunicipalCADIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCADJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	incidentID := strings.TrimSpace(r.PathValue("id"))
	if incidentID == "" {
		writeCADJSONError(w, http.StatusBadRequest, "incident id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req dispatchCADIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCADJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultMunicipalCADIncidents(r.Context())

	seqNano := nextCADNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedIncident domain.MunicipalCADIncident
		dispErr         error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionMunicipalCADIncidents, incidentID, func(current []byte) ([]byte, error) {
		var existing domain.MunicipalCADIncident
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt municipal CAD incident: %w", err)
		}

		if dErr := municipalcad.ApplyCADDispatchAction(&existing, municipalcad.CADDispatchUpdateInput{
			DeployHighwayContainment: req.DeployHighwayContainment,
			ResolveSafeCustody:       req.ResolveSafeCustody,
			CompleteStepID:           req.CompleteStepID,
			DOTSignAdvisoryActive:    req.DOTSignAdvisoryActive,
			ACOUnitAssigned:          req.ACOUnitAssigned,
		}, now); dErr != nil {
			dispErr = dErr
			return nil, dErr
		}
		updatedIncident = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCADJSONError(w, http.StatusNotFound, "Municipal CAD incident not found")
			return
		}
		if dispErr != nil {
			writeCADJSONError(w, http.StatusBadRequest, dispErr.Error())
			return
		}
		writeCADJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update municipal CAD incident: %v", err))
		return
	}

	s.broadcastMunicipalCADEvent("municipal_cad_updated", map[string]any{
		"type":     "municipal_cad_updated",
		"incident": updatedIncident,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updatedIncident)
}

func (s *Server) handleMunicipalCADStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("municipal_cad")
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

func (s *Server) broadcastMunicipalCADEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextCADNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "municipal_cad",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "municipal_cad",
			SenderNodeID:  "municipal-cad-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
