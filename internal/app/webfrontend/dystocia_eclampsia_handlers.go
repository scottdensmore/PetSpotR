package webfrontend

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/dystociaeclampsia"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastDystociaEclampsiaNano atomic.Int64

func nextDystociaEclampsiaNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastDystociaEclampsiaNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastDystociaEclampsiaNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// DystociaEclampsiaPageData represents template data passed to templates/dystocia_eclampsia_triage.html.
type DystociaEclampsiaPageData struct {
	Operations          []domain.DystociaEclampsiaOperation
	ActiveOperation     *domain.DystociaEclampsiaOperation
	TotalOperations     int
	CriticalFetalCount  int
	StagedWhelpingCount int
	DeliveredSafeCount  int
	Locale              string
}

type dystociaStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureDystociaEclampsiaStore() *dystociaeclampsia.Store {
	if s.dystociaEclampsiaStore == nil {
		s.dystociaEclampsiaStore = dystociaeclampsia.NewStore()
	}
	return s.dystociaEclampsiaStore
}

func (s *Server) registerDystociaEclampsiaRoutes() {
	s.mux.HandleFunc("GET /dystocia-eclampsia-triage", s.handleDystociaEclampsiaTriagePage)
	s.mux.HandleFunc("GET /api/v1/dystocia-eclampsia/operations", s.handleListDystociaEclampsiaOperations)
	s.mux.HandleFunc("POST /api/v1/dystocia-eclampsia/evaluate", s.handleEvaluateDystociaEclampsiaOperation)
	s.mux.HandleFunc("POST /api/v1/dystocia-eclampsia/operations/{id}/steps", s.handleUpdateDystociaEclampsiaStep)
	s.mux.HandleFunc("POST /api/v1/dystocia-eclampsia/operations/{id}/step", s.handleUpdateDystociaEclampsiaStep)
	s.mux.HandleFunc("POST /api/v1/dystocia-eclampsia/operations/{id}/clearance", s.handleIssueDystociaEclampsiaClearance)
	s.mux.HandleFunc("GET /api/v1/dystocia-eclampsia/stream", s.handleDystociaEclampsiaStream)
}

func computeDystociaEclampsiaSummary(ops []domain.DystociaEclampsiaOperation) (total, criticalFetal, stagedWhelping, deliveredSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.DystociaStatusFetalDistressObstructiveCritical:
			criticalFetal++
		case domain.DystociaStatusCalciumECGWhelpingStaged:
			stagedWhelping++
		case domain.DystociaStatusDeliveredNormocalcemicSafe:
			deliveredSafe++
		}
	}
	return total, criticalFetal, stagedWhelping, deliveredSafe
}

func writeDystociaEclampsiaJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleDystociaEclampsiaTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deStore := s.ensureDystociaEclampsiaStore()
	ops := deStore.List()
	total, criticalFetal, stagedWhelping, deliveredSafe := computeDystociaEclampsiaSummary(ops)

	var activeOp *domain.DystociaEclampsiaOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("dystocia_eclampsia_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/dystocia_eclampsia_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Dystocia, Maternal C-Section & Eclampsia Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := DystociaEclampsiaPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalFetalCount:  criticalFetal,
		StagedWhelpingCount: stagedWhelping,
		DeliveredSafeCount:  deliveredSafe,
		Locale:              locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListDystociaEclampsiaOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deStore := s.ensureDystociaEclampsiaStore()
	allOps := deStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	presentationFilter := strings.TrimSpace(r.URL.Query().Get("presentation_type"))
	if presentationFilter == "" {
		presentationFilter = strings.TrimSpace(r.URL.Query().Get("presentation"))
	}

	filtered := make([]domain.DystociaEclampsiaOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if presentationFilter != "" && !strings.EqualFold(string(op.PresentationType), presentationFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalFetal, stagedWhelping, deliveredSafe := computeDystociaEclampsiaSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":     total,
			"critical_fetal_count": criticalFetal,
			"staged_whelping":      stagedWhelping,
			"delivered_safe":       deliveredSafe,
		},
	})
}

func (s *Server) handleEvaluateDystociaEclampsiaOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.DystociaEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	deStore := s.ensureDystociaEclampsiaStore()
	created, err := deStore.EvaluateAndSave(req)
	if err != nil {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDystociaEclampsiaNano()).UTC()
	s.broadcastDystociaEclampsiaEvent("dystocia_eclampsia_updated", map[string]any{
		"type":      "dystocia_eclampsia_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateDystociaEclampsiaStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload dystociaStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	deStore := s.ensureDystociaEclampsiaStore()
	if _, exists := deStore.Get(opID); !exists {
		writeDystociaEclampsiaJSONError(w, http.StatusNotFound, "Dystocia & eclampsia triage operation not found")
		return
	}

	updated, err := deStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDystociaEclampsiaNano()).UTC()
	s.broadcastDystociaEclampsiaEvent("dystocia_eclampsia_updated", map[string]any{
		"type":      "dystocia_eclampsia_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueDystociaEclampsiaClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	deStore := s.ensureDystociaEclampsiaStore()
	if _, exists := deStore.Get(opID); !exists {
		writeDystociaEclampsiaJSONError(w, http.StatusNotFound, "Dystocia & eclampsia triage operation not found")
		return
	}

	updated, err := deStore.IssueClearance(opID)
	if err != nil {
		writeDystociaEclampsiaJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDystociaEclampsiaNano()).UTC()
	s.broadcastDystociaEclampsiaEvent("dystocia_eclampsia_updated", map[string]any{
		"type":      "dystocia_eclampsia_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleDystociaEclampsiaStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("dystocia_eclampsia")
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

func (s *Server) broadcastDystociaEclampsiaEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextDystociaEclampsiaNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "dystocia_eclampsia",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "dystocia_eclampsia",
			SenderNodeID:  "dystocia-eclampsia-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
