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
	"github.com/scottdensmore/petspotr/pkg/tetanusbotulism"
)

var lastTetanusBotulismNano atomic.Int64

func nextTetanusBotulismNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastTetanusBotulismNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastTetanusBotulismNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// TetanusBotulismPageData represents template data passed to templates/tetanus_botulism_triage.html.
type TetanusBotulismPageData struct {
	Operations                     []domain.TetanusBotulismOperation
	ActiveOperation                *domain.TetanusBotulismOperation
	Overview                       domain.TetanusBotulismOverview
	TotalOperations                int
	MetronidazoleTATStagedCount    int
	SensoryIsolationMonitoredCount int
	NormoreflexicClearedCount      int
	SevereBurkittClassIIIIVCount   int
	BotulismLMNParalysisCount      int
	Locale                         string
}

type createTetanusBotulismOperationPayload struct {
	CaseID                                  string                     `json:"case_id"`
	PetName                                 string                     `json:"pet_name"`
	Species                                 string                     `json:"species"`
	Breed                                   string                     `json:"breed"`
	WeightKg                                float64                    `json:"weight_kg"`
	Syndrome                                domain.ClostridialSyndrome `json:"syndrome"`
	Protocol                                domain.ClostridialProtocol `json:"protocol"`
	BurkittSeverityClass                    int                        `json:"burkitt_severity_class"`
	GagReflexIntact                         bool                       `json:"gag_reflex_intact"`
	SpasmFrequencyPerHr                     int                        `json:"spasm_frequency_per_hr"`
	CoreTempCelsius                         float64                    `json:"core_temp_celsius"`
	PaCO2mmHg                               float64                    `json:"paco2_mmhg"`
	HeartRateBPM                            int                        `json:"heart_rate_bpm"`
	MetronidazoleMgKgBID                    float64                    `json:"metronidazole_mg_kg_bid"`
	EquineTATUnitsKg                        float64                    `json:"equine_tat_units_kg"`
	MethocarbamolMgKgIV                     float64                    `json:"methocarbamol_mg_kg_iv"`
	IntradermalSkinTestNegativeVerified     bool                       `json:"intradermal_skin_test_negative_verified"`
	PenicillinGABAWarningAcknowledged       bool                       `json:"penicillin_gaba_warning_acknowledged"`
	DarkQuietCottonEarplugsActive           bool                       `json:"dark_quiet_cotton_earplugs_active"`
	AminoglycosideContraindicatedInBotulism bool                       `json:"aminoglycoside_contraindicated_in_botulism"`
}

type stageTetanusBotulismTherapyPayload struct {
	Protocol                                domain.ClostridialProtocol `json:"protocol"`
	MetronidazoleMgKgBID                    float64                    `json:"metronidazole_mg_kg_bid"`
	EquineTATUnitsKg                        *float64                   `json:"equine_tat_units_kg"`
	MethocarbamolMgKgIV                     *float64                   `json:"methocarbamol_mg_kg_iv"`
	IntradermalSkinTestNegativeVerified     *bool                      `json:"intradermal_skin_test_negative_verified"`
	PenicillinGABAWarningAcknowledged       *bool                      `json:"penicillin_gaba_warning_acknowledged"`
	DarkQuietCottonEarplugsActive           *bool                      `json:"dark_quiet_cotton_earplugs_active"`
	AminoglycosideContraindicatedInBotulism *bool                      `json:"aminoglycoside_contraindicated_in_botulism"`
	WoundInfiltrationPriorToDebridementNote string                     `json:"wound_infiltration_prior_to_debridement_note"`
	WoundInfiltrationNote                   string                     `json:"wound_infiltration_note"`
	StagedBy                                string                     `json:"staged_by"`
}

type recordTetanusBotulismNeuroCheckPayload struct {
	BurkittSeverityClass          int     `json:"burkitt_severity_class"`
	SpasmFrequencyPerHr           int     `json:"spasm_frequency_per_hr"`
	GagReflexIntact               bool    `json:"gag_reflex_intact"`
	CoreTempCelsius               float64 `json:"core_temp_celsius"`
	PaCO2mmHg                     float64 `json:"paco2_mmhg"`
	HeartRateBPM                  int     `json:"heart_rate_bpm"`
	DarkQuietCottonEarplugsActive *bool   `json:"dark_quiet_cotton_earplugs_active"`
	RecordedBy                    string  `json:"recorded_by"`
	ClinicalNote                  string  `json:"clinical_note"`
}

type evaluateTetanusBotulismClearancePayload struct {
	SpasmFrequencyPerHr            int     `json:"spasm_frequency_per_hr"`
	GagReflexIntact                *bool   `json:"gag_reflex_intact"`
	PaCO2mmHg                      float64 `json:"paco2_mmhg"`
	AmbulatoryNormothermicVerified *bool   `json:"ambulatory_normothermic_verified"`
	ClearedBy                      string  `json:"cleared_by"`
	DischargedBy                   string  `json:"discharged_by"`
	ClearanceSummary               string  `json:"clearance_summary"`
	DischargeSummary               string  `json:"discharge_summary"`
	Summary                        string  `json:"summary"`
}

func (s *Server) ensureTetanusBotulismService() *tetanusbotulism.TetanusBotulismService {
	if s.tetanusBotulismService == nil {
		s.tetanusBotulismService = tetanusbotulism.NewTetanusBotulismService()
	}
	return s.tetanusBotulismService
}

func (s *Server) registerTetanusBotulismRoutes() {
	s.mux.HandleFunc("GET /tetanus-botulism-triage", s.handleTetanusBotulismTriagePage)

	s.mux.HandleFunc("GET /api/tetanus-botulism/overview", s.handleGetTetanusBotulismOverview)
	s.mux.HandleFunc("GET /api/tetanus-botulism/operations", s.handleListTetanusBotulismOperations)
	s.mux.HandleFunc("POST /api/tetanus-botulism/operations", s.handleCreateTetanusBotulismOperation)
	s.mux.HandleFunc("GET /api/tetanus-botulism/operations/{id}", s.handleGetTetanusBotulismOperation)
	s.mux.HandleFunc("POST /api/tetanus-botulism/operations/{id}/therapy", s.handleStageTetanusBotulismTherapy)
	s.mux.HandleFunc("POST /api/tetanus-botulism/operations/{id}/neuro-check", s.handleRecordTetanusBotulismNeuroCheck)
	s.mux.HandleFunc("POST /api/tetanus-botulism/operations/{id}/clearance", s.handleEvaluateTetanusBotulismClearance)
	s.mux.HandleFunc("GET /api/tetanus-botulism/stream", s.handleTetanusBotulismStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/tetanus-botulism/overview", s.handleGetTetanusBotulismOverview)
	s.mux.HandleFunc("GET /api/v1/tetanus-botulism/operations", s.handleListTetanusBotulismOperations)
	s.mux.HandleFunc("POST /api/v1/tetanus-botulism/operations", s.handleCreateTetanusBotulismOperation)
	s.mux.HandleFunc("GET /api/v1/tetanus-botulism/operations/{id}", s.handleGetTetanusBotulismOperation)
	s.mux.HandleFunc("POST /api/v1/tetanus-botulism/operations/{id}/therapy", s.handleStageTetanusBotulismTherapy)
	s.mux.HandleFunc("POST /api/v1/tetanus-botulism/operations/{id}/neuro-check", s.handleRecordTetanusBotulismNeuroCheck)
	s.mux.HandleFunc("POST /api/v1/tetanus-botulism/operations/{id}/clearance", s.handleEvaluateTetanusBotulismClearance)
	s.mux.HandleFunc("GET /api/v1/tetanus-botulism/stream", s.handleTetanusBotulismStream)
}

func writeTetanusBotulismJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleTetanusBotulismTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTetanusBotulismService()
	ops := svc.ListOperations("")
	overview := svc.CalculateOverview()

	var activeOp *domain.TetanusBotulismOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("tetanus_botulism_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/tetanus_botulism_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Tetanus & Botulism Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := TetanusBotulismPageData{
		Operations:                     ops,
		ActiveOperation:                activeOp,
		Overview:                       overview,
		TotalOperations:                overview.TotalOperations,
		MetronidazoleTATStagedCount:    overview.MetronidazoleTATStagedCount,
		SensoryIsolationMonitoredCount: overview.SensoryIsolationMonitoredCount,
		NormoreflexicClearedCount:      overview.NormoreflexicClearedCount,
		SevereBurkittClassIIIIVCount:   overview.SevereBurkittClassIIIIVCount,
		BotulismLMNParalysisCount:      overview.BotulismLMNParalysisCount,
		Locale:                         locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleGetTetanusBotulismOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTetanusBotulismService()
	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
}

func (s *Server) handleListTetanusBotulismOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTetanusBotulismService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	syndromeFilter := strings.TrimSpace(r.URL.Query().Get("syndrome"))

	ops := svc.ListOperations(statusFilter)
	if syndromeFilter != "" {
		filtered := make([]domain.TetanusBotulismOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.Syndrome), syndromeFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"overview":   overview,
		"metrics":    overview,
	})
}

func (s *Server) handleGetTetanusBotulismOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTetanusBotulismService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateTetanusBotulismOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createTetanusBotulismOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureTetanusBotulismService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-CLB-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.Syndrome,
		payload.Protocol,
		payload.BurkittSeverityClass,
		payload.GagReflexIntact,
		payload.SpasmFrequencyPerHr,
		payload.CoreTempCelsius,
		payload.PaCO2mmHg,
		payload.HeartRateBPM,
		payload.MetronidazoleMgKgBID,
		payload.EquineTATUnitsKg,
		payload.MethocarbamolMgKgIV,
		payload.IntradermalSkinTestNegativeVerified,
		payload.PenicillinGABAWarningAcknowledged,
		payload.DarkQuietCottonEarplugsActive,
		payload.AminoglycosideContraindicatedInBotulism,
	)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTetanusBotulismNano()).UTC()
	s.broadcastTetanusBotulismEvent("tetanus_botulism_updated", map[string]any{
		"type":      "tetanus_botulism_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageTetanusBotulismTherapy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTetanusBotulismService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageTetanusBotulismTherapyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = existing.Protocol
	}
	metro := payload.MetronidazoleMgKgBID
	if metro == 0 {
		metro = existing.MetronidazoleMgKgBID
	}
	tat := existing.EquineTATUnitsKg
	if payload.EquineTATUnitsKg != nil {
		tat = *payload.EquineTATUnitsKg
	}
	metho := existing.MethocarbamolMgKgIV
	if payload.MethocarbamolMgKgIV != nil {
		metho = *payload.MethocarbamolMgKgIV
	}
	idVerified := existing.IntradermalSkinTestNegativeVerified
	if payload.IntradermalSkinTestNegativeVerified != nil {
		idVerified = *payload.IntradermalSkinTestNegativeVerified
	}
	penAck := existing.PenicillinGABAWarningAcknowledged
	if payload.PenicillinGABAWarningAcknowledged != nil {
		penAck = *payload.PenicillinGABAWarningAcknowledged
	}
	earplugs := existing.DarkQuietCottonEarplugsActive
	if payload.DarkQuietCottonEarplugsActive != nil {
		earplugs = *payload.DarkQuietCottonEarplugsActive
	}
	aminoGuard := existing.AminoglycosideContraindicatedInBotulism
	if payload.AminoglycosideContraindicatedInBotulism != nil {
		aminoGuard = *payload.AminoglycosideContraindicatedInBotulism
	}
	note := firstNonEmpty(payload.WoundInfiltrationPriorToDebridementNote, payload.WoundInfiltrationNote)
	stagedBy := firstNonEmpty(payload.StagedBy, "Dr. Ingrid Vance, DACVIM (Neurology)")

	updated, err := svc.StageTherapy(
		id,
		protocol,
		metro,
		tat,
		metho,
		idVerified,
		penAck,
		earplugs,
		aminoGuard,
		note,
		stagedBy,
	)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTetanusBotulismNano()).UTC()
	s.broadcastTetanusBotulismEvent("tetanus_botulism_updated", map[string]any{
		"type":      "tetanus_botulism_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordTetanusBotulismNeuroCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTetanusBotulismService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordTetanusBotulismNeuroCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	earplugs := existing.DarkQuietCottonEarplugsActive
	if payload.DarkQuietCottonEarplugsActive != nil {
		earplugs = *payload.DarkQuietCottonEarplugsActive
	}

	updated, err := svc.RecordNeuroCheck(
		id,
		payload.BurkittSeverityClass,
		payload.SpasmFrequencyPerHr,
		payload.GagReflexIntact,
		payload.CoreTempCelsius,
		payload.PaCO2mmHg,
		payload.HeartRateBPM,
		earplugs,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTetanusBotulismNano()).UTC()
	s.broadcastTetanusBotulismEvent("tetanus_botulism_updated", map[string]any{
		"type":      "tetanus_botulism_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEvaluateTetanusBotulismClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTetanusBotulismService()
	if _, err := svc.GetOperation(id); err != nil {
		writeTetanusBotulismJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateTetanusBotulismClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	gagIntact := true
	if payload.GagReflexIntact != nil {
		gagIntact = *payload.GagReflexIntact
	}
	ambulatory := true
	if payload.AmbulatoryNormothermicVerified != nil {
		ambulatory = *payload.AmbulatoryNormothermicVerified
	}
	paCO2 := payload.PaCO2mmHg
	if paCO2 == 0 {
		paCO2 = 38.0
	}
	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		payload.SpasmFrequencyPerHr,
		gagIntact,
		paCO2,
		ambulatory,
		clearedBy,
		summary,
	)
	if err != nil {
		writeTetanusBotulismJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTetanusBotulismNano()).UTC()
	s.broadcastTetanusBotulismEvent("tetanus_botulism_updated", map[string]any{
		"type":      "tetanus_botulism_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleTetanusBotulismStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("tetanus_botulism")
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

func (s *Server) broadcastTetanusBotulismEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextTetanusBotulismNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "tetanus_botulism",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "tetanus_botulism",
			SenderNodeID:  "tetanus-botulism-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
