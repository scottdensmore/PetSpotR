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
	"github.com/scottdensmore/petspotr/pkg/hepaticencephalopathy"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHepaticEncephalopathyNano atomic.Int64

func nextHepaticEncephalopathyNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHepaticEncephalopathyNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHepaticEncephalopathyNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// HepaticEncephalopathyPageData represents template data passed to templates/hepatic_encephalopathy_triage.html.
type HepaticEncephalopathyPageData struct {
	Operations                        []domain.HepaticOperation
	ActiveOperation                   *domain.HepaticOperation
	Metrics                           domain.HepaticOverviewMetrics
	TotalOperations                   int
	FulminantHETriageCount            int
	NACLactuloseStagedCount           int
	ClearedHepaticCount               int
	SagoPalmMicrocystinCount          int
	HyperammonemicEncephalopathyCount int
	Locale                            string
}

type createHepaticOperationPayload struct {
	CaseID                      string                         `json:"case_id"`
	PetName                     string                         `json:"pet_name"`
	Species                     string                         `json:"species"`
	Breed                       string                         `json:"breed"`
	WeightKg                    float64                        `json:"weight_kg"`
	EtiologyType                domain.HepatotoxinEtiologyType `json:"etiology_type"`
	ALTIUL                      int                            `json:"alt_iu_l"`
	TotalBilirubinMgDL          float64                        `json:"total_bilirubin_mg_dl"`
	BloodAmmoniaUmolL           float64                        `json:"blood_ammonia_umol_l"`
	BloodGlucoseMgDL            float64                        `json:"blood_glucose_mg_dl"`
	PTSeconds                   float64                        `json:"pt_seconds"`
	EncephalopathyGrade         int                            `json:"encephalopathy_grade"`
	HeadPressingSeizuresPresent bool                           `json:"head_pressing_seizures_present"`
}

type stageHepaticSupportPayload struct {
	NACGlutathioneLoadingProtocol  string `json:"nac_glutathione_loading_protocol"`
	NACProtocol                    string `json:"nac_protocol"`
	SAMeSilybinHepatoprotectant    string `json:"same_silybin_hepatoprotectant"`
	SAMeSilybinPlan                string `json:"same_silybin_plan"`
	LactuloseEnemaOralProtocol     string `json:"lactulose_enema_oral_protocol"`
	LactuloseProtocol              string `json:"lactulose_protocol"`
	LevetiracetamNeuroProtocol     string `json:"levetiracetam_neuro_protocol"`
	LevetiracetamProtocol          string `json:"levetiracetam_protocol"`
	DextroseVitaminK1FFPPlan       string `json:"dextrose_vitamin_k1_ffp_plan"`
	DextroseVitK1Plan              string `json:"dextrose_vit_k1_plan"`
	PotassiumPhosphateThiaminePlan string `json:"potassium_phosphate_thiamine_plan"`
	ElectrolyteThiaminePlan        string `json:"electrolyte_thiamine_plan"`
}

type recordHepaticMetabolicCheckPayload struct {
	ALTIUL             int     `json:"alt_iu_l"`
	TotalBilirubinMgDL float64 `json:"total_bilirubin_mg_dl"`
	BloodAmmoniaUmolL  float64 `json:"blood_ammonia_umol_l"`
	BloodGlucoseMgDL   float64 `json:"blood_glucose_mg_dl"`
	PTSeconds          float64 `json:"pt_seconds"`
	RecordedBy         string  `json:"recorded_by"`
	ClinicalNote       string  `json:"clinical_note"`
}

type clearHepaticDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureHepaticService() *hepaticencephalopathy.HepaticService {
	if s.hepaticService == nil {
		s.hepaticService = hepaticencephalopathy.NewHepaticService()
	}
	return s.hepaticService
}

func (s *Server) registerHepaticEncephalopathyRoutes() {
	s.mux.HandleFunc("GET /hepatic-encephalopathy-triage", s.handleHepaticEncephalopathyTriagePage)

	s.mux.HandleFunc("GET /api/hepatic-encephalopathy/operations", s.handleListHepaticEncephalopathyOperations)
	s.mux.HandleFunc("POST /api/hepatic-encephalopathy/operations", s.handleCreateHepaticEncephalopathyOperation)
	s.mux.HandleFunc("GET /api/hepatic-encephalopathy/operations/{id}", s.handleGetHepaticEncephalopathyOperation)
	s.mux.HandleFunc("POST /api/hepatic-encephalopathy/operations/{id}/stage-support", s.handleStageHepaticSupport)
	s.mux.HandleFunc("POST /api/hepatic-encephalopathy/operations/{id}/record-check", s.handleRecordHepaticMetabolicCheck)
	s.mux.HandleFunc("POST /api/hepatic-encephalopathy/operations/{id}/clear-discharge", s.handleClearHepaticDischarge)
	s.mux.HandleFunc("GET /api/hepatic-encephalopathy/stream", s.handleHepaticEncephalopathyStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/hepatic-encephalopathy/operations", s.handleListHepaticEncephalopathyOperations)
	s.mux.HandleFunc("POST /api/v1/hepatic-encephalopathy/operations", s.handleCreateHepaticEncephalopathyOperation)
	s.mux.HandleFunc("GET /api/v1/hepatic-encephalopathy/operations/{id}", s.handleGetHepaticEncephalopathyOperation)
	s.mux.HandleFunc("POST /api/v1/hepatic-encephalopathy/operations/{id}/stage-support", s.handleStageHepaticSupport)
	s.mux.HandleFunc("POST /api/v1/hepatic-encephalopathy/operations/{id}/record-check", s.handleRecordHepaticMetabolicCheck)
	s.mux.HandleFunc("POST /api/v1/hepatic-encephalopathy/operations/{id}/clear-discharge", s.handleClearHepaticDischarge)
	s.mux.HandleFunc("GET /api/v1/hepatic-encephalopathy/stream", s.handleHepaticEncephalopathyStream)
}

func writeHepaticEncephalopathyJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleHepaticEncephalopathyTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHepaticService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.HepaticOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("hepatic_encephalopathy_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/hepatic_encephalopathy_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Hepatic Encephalopathy Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HepaticEncephalopathyPageData{
		Operations:                        ops,
		ActiveOperation:                   activeOp,
		Metrics:                           metrics,
		TotalOperations:                   metrics.TotalOperations,
		FulminantHETriageCount:            metrics.FulminantHETriageCount,
		NACLactuloseStagedCount:           metrics.NACLactuloseStagedCount,
		ClearedHepaticCount:               metrics.ClearedHepaticCount,
		SagoPalmMicrocystinCount:          metrics.SagoPalmMicrocystinCount,
		HyperammonemicEncephalopathyCount: metrics.HyperammonemicEncephalopathyCount,
		Locale:                            locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHepaticEncephalopathyOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHepaticService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	etiologyFilter := strings.TrimSpace(r.URL.Query().Get("etiology"))

	ops := svc.ListOperations(statusFilter)
	if etiologyFilter != "" {
		filtered := make([]domain.HepaticOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.EtiologyType), etiologyFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.GetOverviewMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetHepaticEncephalopathyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHepaticService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateHepaticEncephalopathyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createHepaticOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureHepaticService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-HEP-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.EtiologyType,
		payload.ALTIUL,
		payload.TotalBilirubinMgDL,
		payload.BloodAmmoniaUmolL,
		payload.BloodGlucoseMgDL,
		payload.PTSeconds,
		payload.EncephalopathyGrade,
		payload.HeadPressingSeizuresPresent,
	)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHepaticEncephalopathyNano()).UTC()
	s.broadcastHepaticEncephalopathyEvent("hepatic_encephalopathy_updated", map[string]any{
		"type":      "hepatic_encephalopathy_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageHepaticSupport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHepaticService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageHepaticSupportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	nac := firstNonEmpty(payload.NACGlutathioneLoadingProtocol, payload.NACProtocol, existing.SupportPlan.NACGlutathioneLoadingProtocol)
	sameSilybin := firstNonEmpty(payload.SAMeSilybinHepatoprotectant, payload.SAMeSilybinPlan, existing.SupportPlan.SAMeSilybinHepatoprotectant)
	lactulose := firstNonEmpty(payload.LactuloseEnemaOralProtocol, payload.LactuloseProtocol, existing.SupportPlan.LactuloseEnemaOralProtocol)
	levetiracetam := firstNonEmpty(payload.LevetiracetamNeuroProtocol, payload.LevetiracetamProtocol, existing.SupportPlan.LevetiracetamNeuroProtocol)
	dexVitK := firstNonEmpty(payload.DextroseVitaminK1FFPPlan, payload.DextroseVitK1Plan, existing.SupportPlan.DextroseVitaminK1FFPPlan)
	electrolyte := firstNonEmpty(payload.PotassiumPhosphateThiaminePlan, payload.ElectrolyteThiaminePlan, existing.SupportPlan.PotassiumPhosphateThiaminePlan)

	updated, err := svc.StageNACLactuloseSupport(id, nac, sameSilybin, lactulose, levetiracetam, dexVitK, electrolyte)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHepaticEncephalopathyNano()).UTC()
	s.broadcastHepaticEncephalopathyEvent("hepatic_encephalopathy_updated", map[string]any{
		"type":      "hepatic_encephalopathy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordHepaticMetabolicCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHepaticService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordHepaticMetabolicCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordMetabolicCheck(
		id,
		payload.ALTIUL,
		payload.TotalBilirubinMgDL,
		payload.BloodAmmoniaUmolL,
		payload.BloodGlucoseMgDL,
		payload.PTSeconds,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHepaticEncephalopathyNano()).UTC()
	s.broadcastHepaticEncephalopathyEvent("hepatic_encephalopathy_updated", map[string]any{
		"type":      "hepatic_encephalopathy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearHepaticDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHepaticService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearHepaticDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearHepaticDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeHepaticEncephalopathyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHepaticEncephalopathyNano()).UTC()
	s.broadcastHepaticEncephalopathyEvent("hepatic_encephalopathy_updated", map[string]any{
		"type":      "hepatic_encephalopathy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHepaticEncephalopathyStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("hepatic_encephalopathy")
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

func (s *Server) broadcastHepaticEncephalopathyEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHepaticEncephalopathyNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "hepatic_encephalopathy",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "hepatic_encephalopathy",
			SenderNodeID:  "hepatic-encephalopathy-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
