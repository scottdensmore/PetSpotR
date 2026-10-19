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
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/transfusionbloodbank"
)

var lastTransfusionBloodbankNano atomic.Int64

func nextTransfusionBloodbankNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastTransfusionBloodbankNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastTransfusionBloodbankNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// TransfusionBloodbankPageData represents template data passed to templates/transfusion_bloodbank_triage.html.
type TransfusionBloodbankPageData struct {
	Operations             []domain.TransfusionBloodbankOperation
	ActiveOperation        *domain.TransfusionBloodbankOperation
	TotalOperations        int
	CriticalHemolyticCount int
	StagedCrossmatchCount  int
	ClearedSafeCount       int
	Locale                 string
}

type transfusionStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureTransfusionBloodbankStore() *transfusionbloodbank.Store {
	if s.transfusionBloodbankStore == nil {
		s.transfusionBloodbankStore = transfusionbloodbank.NewStore()
	}
	return s.transfusionBloodbankStore
}

func (s *Server) registerTransfusionBloodbankRoutes() {
	s.mux.HandleFunc("GET /transfusion-bloodbank-triage", s.handleTransfusionBloodbankTriagePage)
	s.mux.HandleFunc("GET /api/v1/transfusion-bloodbank/operations", s.handleListTransfusionBloodbankOperations)
	s.mux.HandleFunc("POST /api/v1/transfusion-bloodbank/evaluate", s.handleEvaluateTransfusionBloodbankOperation)
	s.mux.HandleFunc("POST /api/v1/transfusion-bloodbank/operations/{id}/steps", s.handleUpdateTransfusionBloodbankStep)
	s.mux.HandleFunc("POST /api/v1/transfusion-bloodbank/operations/{id}/step", s.handleUpdateTransfusionBloodbankStep)
	s.mux.HandleFunc("POST /api/v1/transfusion-bloodbank/operations/{id}/clearance", s.handleIssueTransfusionBloodbankClearance)
	s.mux.HandleFunc("GET /api/v1/transfusion-bloodbank/stream", s.handleTransfusionBloodbankStream)
}

func computeTransfusionBloodbankSummary(ops []domain.TransfusionBloodbankOperation) (total, criticalHemolytic, stagedCrossmatch, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.TransfusionStatusCriticalHemolyticAnaphylaxis:
			criticalHemolytic++
		case domain.TransfusionStatusStagedCrossmatchedMicronFilter:
			stagedCrossmatch++
		case domain.TransfusionStatusSafePCVRestoredNormothermic:
			clearedSafe++
		}
	}
	return total, criticalHemolytic, stagedCrossmatch, clearedSafe
}

func writeTransfusionBloodbankJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleTransfusionBloodbankTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tbStore := s.ensureTransfusionBloodbankStore()
	ops := tbStore.List()
	total, criticalHemolytic, stagedCrossmatch, clearedSafe := computeTransfusionBloodbankSummary(ops)

	var activeOp *domain.TransfusionBloodbankOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("transfusion_bloodbank_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/transfusion_bloodbank_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := TransfusionBloodbankPageData{
		Operations:             ops,
		ActiveOperation:        activeOp,
		TotalOperations:        total,
		CriticalHemolyticCount: criticalHemolytic,
		StagedCrossmatchCount:  stagedCrossmatch,
		ClearedSafeCount:       clearedSafe,
		Locale:                 locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListTransfusionBloodbankOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tbStore := s.ensureTransfusionBloodbankStore()
	allOps := tbStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	bgFilter := strings.TrimSpace(r.URL.Query().Get("blood_group_type"))
	if bgFilter == "" {
		bgFilter = strings.TrimSpace(r.URL.Query().Get("blood_group"))
	}

	filtered := make([]domain.TransfusionBloodbankOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if bgFilter != "" && !strings.EqualFold(string(op.BloodGroupType), bgFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalHemolytic, stagedCrossmatch, clearedSafe := computeTransfusionBloodbankSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":         total,
			"critical_hemolytic_count": criticalHemolytic,
			"staged_crossmatch":        stagedCrossmatch,
			"cleared_safe":             clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateTransfusionBloodbankOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.TransfusionEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	tbStore := s.ensureTransfusionBloodbankStore()
	created, err := tbStore.EvaluateAndSave(req)
	if err != nil {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTransfusionBloodbankNano()).UTC()
	s.broadcastTransfusionBloodbankEvent("transfusion_bloodbank_updated", map[string]any{
		"type":      "transfusion_bloodbank_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateTransfusionBloodbankStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload transfusionStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	tbStore := s.ensureTransfusionBloodbankStore()
	if _, exists := tbStore.Get(opID); !exists {
		writeTransfusionBloodbankJSONError(w, http.StatusNotFound, "Transfusion bloodbank triage operation not found")
		return
	}

	updated, err := tbStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTransfusionBloodbankNano()).UTC()
	s.broadcastTransfusionBloodbankEvent("transfusion_bloodbank_updated", map[string]any{
		"type":      "transfusion_bloodbank_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueTransfusionBloodbankClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	tbStore := s.ensureTransfusionBloodbankStore()
	if _, exists := tbStore.Get(opID); !exists {
		writeTransfusionBloodbankJSONError(w, http.StatusNotFound, "Transfusion bloodbank triage operation not found")
		return
	}

	updated, err := tbStore.IssueClearance(opID)
	if err != nil {
		writeTransfusionBloodbankJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTransfusionBloodbankNano()).UTC()
	s.broadcastTransfusionBloodbankEvent("transfusion_bloodbank_updated", map[string]any{
		"type":      "transfusion_bloodbank_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleTransfusionBloodbankStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("transfusion_bloodbank")
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

func (s *Server) broadcastTransfusionBloodbankEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextTransfusionBloodbankNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "transfusion_bloodbank",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "transfusion_bloodbank",
			SenderNodeID:  "transfusion-bloodbank-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
