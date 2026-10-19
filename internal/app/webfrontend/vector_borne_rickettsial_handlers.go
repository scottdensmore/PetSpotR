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
	"github.com/scottdensmore/petspotr/pkg/vectorbornerickettsial"
)

var lastVectorBorneRickettsialNano atomic.Int64

func nextVectorBorneRickettsialNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastVectorBorneRickettsialNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastVectorBorneRickettsialNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// VectorBorneRickettsialPageData represents template data passed to templates/vector_borne_rickettsial_triage.html.
type VectorBorneRickettsialPageData struct {
	Operations                      []domain.VectorBorneOperation
	ActiveOperation                 *domain.VectorBorneOperation
	Overview                        domain.VectorBorneOverview
	TotalOperations                 int
	AntimicrobialStagedCount        int
	ThrombocytopeniaMonitoredCount  int
	PCRNegativeClearedCount         int
	LymeNephritisProteinuriaCount   int
	SevereThrombocytopeniaCount     int
	BabesiaCytauxzoonProtozoalCount int
	Locale                          string
}

type createVectorBorneOperationPayload struct {
	CaseID                       string                          `json:"case_id"`
	PetName                      string                          `json:"pet_name"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	Syndrome                     domain.VectorBorneSyndrome      `json:"syndrome"`
	Assay                        domain.VectorBorneAssay         `json:"assay"`
	AntimicrobialProtocol        domain.VectorBorneAntimicrobial `json:"antimicrobial_protocol"`
	PlateletCountKUL             float64                         `json:"platelet_count_kul"`
	PCVPercent                   float64                         `json:"pcv_percent"`
	UrinePCRatio                 float64                         `json:"urine_pc_ratio"`
	SerumAlbuminGDL              float64                         `json:"serum_albumin_g_dl"`
	DoxycyclineMgKgDay           float64                         `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID            float64                         `json:"atovaquone_mg_kg_tid"`
	ImidocarbMgKgIM              float64                         `json:"imidocarb_mg_kg_im"`
	EsophagealWaterChaseVerified bool                            `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified       bool                            `json:"atropine_premed_verified"`
}

type stageVectorBorneAntimicrobialPayload struct {
	Protocol                          domain.VectorBorneAntimicrobial `json:"protocol"`
	AntimicrobialProtocol             domain.VectorBorneAntimicrobial `json:"antimicrobial_protocol"`
	DoxycyclineMgKgDay                *float64                        `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID                 *float64                        `json:"atovaquone_mg_kg_tid"`
	ImidocarbMgKgIM                   *float64                        `json:"imidocarb_mg_kg_im"`
	EsophagealWaterChaseVerified      *bool                           `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified            *bool                           `json:"atropine_premed_verified"`
	FattyMealBioavailabilityNote      string                          `json:"fatty_meal_bioavailability_note"`
	LymeNephritisImmunomodulationNote string                          `json:"lyme_nephritis_immunomodulation_note"`
	StagedBy                          string                          `json:"staged_by"`
	ClinicalRationale                 string                          `json:"clinical_rationale"`
}

type recordVectorBorneHematologyCheckPayload struct {
	PlateletCountKUL           float64 `json:"platelet_count_kul"`
	PCVPercent                 float64 `json:"pcv_percent"`
	UrinePCRatio               float64 `json:"urine_pc_ratio"`
	SerumAlbuminGDL            float64 `json:"serum_albumin_g_dl"`
	MorulaeOrPiroplasmsCleared bool    `json:"morulae_or_piroplasms_cleared"`
	RecordedBy                 string  `json:"recorded_by"`
	ClinicalNote               string  `json:"clinical_note"`
}

type clearVectorBorneOperationPayload struct {
	PlateletCountKUL           *float64 `json:"platelet_count_kul"`
	PCVPercent                 *float64 `json:"pcv_percent"`
	UrinePCRatio               *float64 `json:"urine_pc_ratio"`
	PCRNegativeVerified        *bool    `json:"pcr_negative_verified"`
	NormothermicEatingVerified *bool    `json:"normothermic_eating_verified"`
	ClearedBy                  string   `json:"cleared_by"`
	DischargedBy               string   `json:"discharged_by"`
	ClearanceSummary           string   `json:"clearance_summary"`
	DischargeSummary           string   `json:"discharge_summary"`
	Summary                    string   `json:"summary"`
}

func (s *Server) ensureVectorBorneService() *vectorbornerickettsial.VectorBorneService {
	if s.vectorBorneService == nil {
		s.vectorBorneService = vectorbornerickettsial.NewVectorBorneService()
	}
	return s.vectorBorneService
}

func (s *Server) registerVectorBorneRickettsialRoutes() {
	s.mux.HandleFunc("GET /vector-borne-rickettsial-triage", s.handleVectorBorneRickettsialTriagePage)

	s.mux.HandleFunc("GET /api/vector-borne-rickettsial/overview", s.handleGetVectorBorneRickettsialOverview)
	s.mux.HandleFunc("GET /api/vector-borne-rickettsial/operations", s.handleListVectorBorneRickettsialOperations)
	s.mux.HandleFunc("POST /api/vector-borne-rickettsial/operations", s.handleCreateVectorBorneRickettsialOperation)
	s.mux.HandleFunc("GET /api/vector-borne-rickettsial/operations/{id}", s.handleGetVectorBorneRickettsialOperation)
	s.mux.HandleFunc("POST /api/vector-borne-rickettsial/operations/{id}/antimicrobial", s.handleStageVectorBorneRickettsialAntimicrobial)
	s.mux.HandleFunc("POST /api/vector-borne-rickettsial/operations/{id}/hematology-check", s.handleRecordVectorBorneRickettsialHematologyCheck)
	s.mux.HandleFunc("POST /api/vector-borne-rickettsial/operations/{id}/clearance", s.handleClearVectorBorneRickettsialOperation)
	s.mux.HandleFunc("GET /api/vector-borne-rickettsial/stream", s.handleVectorBorneRickettsialStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/vector-borne-rickettsial/overview", s.handleGetVectorBorneRickettsialOverview)
	s.mux.HandleFunc("GET /api/v1/vector-borne-rickettsial/operations", s.handleListVectorBorneRickettsialOperations)
	s.mux.HandleFunc("POST /api/v1/vector-borne-rickettsial/operations", s.handleCreateVectorBorneRickettsialOperation)
	s.mux.HandleFunc("GET /api/v1/vector-borne-rickettsial/operations/{id}", s.handleGetVectorBorneRickettsialOperation)
	s.mux.HandleFunc("POST /api/v1/vector-borne-rickettsial/operations/{id}/antimicrobial", s.handleStageVectorBorneRickettsialAntimicrobial)
	s.mux.HandleFunc("POST /api/v1/vector-borne-rickettsial/operations/{id}/hematology-check", s.handleRecordVectorBorneRickettsialHematologyCheck)
	s.mux.HandleFunc("POST /api/v1/vector-borne-rickettsial/operations/{id}/clearance", s.handleClearVectorBorneRickettsialOperation)
	s.mux.HandleFunc("GET /api/v1/vector-borne-rickettsial/stream", s.handleVectorBorneRickettsialStream)
}

func writeVectorBorneRickettsialJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleVectorBorneRickettsialTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	ops := svc.ListOperations("")
	overview := svc.CalculateOverview()

	var activeOp *domain.VectorBorneOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("vector_borne_rickettsial_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/vector_borne_rickettsial_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Vector-Borne & Rickettsial Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := VectorBorneRickettsialPageData{
		Operations:                      ops,
		ActiveOperation:                 activeOp,
		Overview:                        overview,
		TotalOperations:                 overview.TotalOperations,
		AntimicrobialStagedCount:        overview.AntimicrobialStagedCount,
		ThrombocytopeniaMonitoredCount:  overview.ThrombocytopeniaMonitoredCount,
		PCRNegativeClearedCount:         overview.PCRNegativeClearedCount,
		LymeNephritisProteinuriaCount:   overview.LymeNephritisProteinuriaCount,
		SevereThrombocytopeniaCount:     overview.SevereThrombocytopeniaCount,
		BabesiaCytauxzoonProtozoalCount: overview.BabesiaCytauxzoonProtozoalCount,
		Locale:                          locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleGetVectorBorneRickettsialOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
}

func (s *Server) handleListVectorBorneRickettsialOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	syndromeFilter := strings.TrimSpace(r.URL.Query().Get("syndrome"))

	ops := svc.ListOperations(statusFilter)
	if syndromeFilter != "" {
		filtered := make([]domain.VectorBorneOperation, 0, len(ops))
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

func (s *Server) handleGetVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureVectorBorneService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createVectorBorneOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureVectorBorneService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-VBR-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.Syndrome,
		payload.Assay,
		payload.AntimicrobialProtocol,
		payload.PlateletCountKUL,
		payload.PCVPercent,
		payload.UrinePCRatio,
		payload.SerumAlbuminGDL,
		payload.DoxycyclineMgKgDay,
		payload.AtovaquoneMgKgTID,
		payload.ImidocarbMgKgIM,
		payload.EsophagealWaterChaseVerified,
		payload.AtropinePremedVerified,
	)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextVectorBorneRickettsialNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageVectorBorneRickettsialAntimicrobial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureVectorBorneService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageVectorBorneAntimicrobialPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = payload.AntimicrobialProtocol
	}
	if protocol == "" {
		protocol = existing.AntimicrobialProtocol
	}

	doxy := existing.DoxycyclineMgKgDay
	if payload.DoxycyclineMgKgDay != nil {
		doxy = *payload.DoxycyclineMgKgDay
	}
	atovaquone := existing.AtovaquoneMgKgTID
	if payload.AtovaquoneMgKgTID != nil {
		atovaquone = *payload.AtovaquoneMgKgTID
	}
	imidocarb := existing.ImidocarbMgKgIM
	if payload.ImidocarbMgKgIM != nil {
		imidocarb = *payload.ImidocarbMgKgIM
	}
	waterChase := existing.EsophagealWaterChaseVerified
	if payload.EsophagealWaterChaseVerified != nil {
		waterChase = *payload.EsophagealWaterChaseVerified
	}
	atropinePremed := existing.AtropinePremedVerified
	if payload.AtropinePremedVerified != nil {
		atropinePremed = *payload.AtropinePremedVerified
	}

	updated, err := svc.StageAntimicrobial(
		id,
		protocol,
		doxy, atovaquone, imidocarb,
		waterChase, atropinePremed,
		payload.FattyMealBioavailabilityNote,
		payload.LymeNephritisImmunomodulationNote,
		payload.StagedBy,
		payload.ClinicalRationale,
	)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextVectorBorneRickettsialNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordVectorBorneRickettsialHematologyCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureVectorBorneService()
	if _, err := svc.GetOperation(id); err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordVectorBorneHematologyCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHematologyCheck(
		id,
		payload.PlateletCountKUL,
		payload.PCVPercent,
		payload.UrinePCRatio,
		payload.SerumAlbuminGDL,
		payload.MorulaeOrPiroplasmsCleared,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextVectorBorneRickettsialNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureVectorBorneService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearVectorBorneOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	platelets := existing.PlateletCountKUL
	if payload.PlateletCountKUL != nil {
		platelets = *payload.PlateletCountKUL
	}
	pcv := existing.PCVPercent
	if payload.PCVPercent != nil {
		pcv = *payload.PCVPercent
	}
	upc := existing.UrinePCRatio
	if payload.UrinePCRatio != nil {
		upc = *payload.UrinePCRatio
	}
	pcrNeg := true
	if payload.PCRNegativeVerified != nil {
		pcrNeg = *payload.PCRNegativeVerified
	}
	normothermicEating := true
	if payload.NormothermicEatingVerified != nil {
		normothermicEating = *payload.NormothermicEatingVerified
	}

	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		platelets, pcv, upc,
		pcrNeg, normothermicEating,
		clearedBy, summary,
	)
	if err != nil {
		writeVectorBorneRickettsialJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextVectorBorneRickettsialNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleVectorBorneRickettsialStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("vector_borne_rickettsial")
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

func (s *Server) broadcastVectorBorneRickettsialEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextVectorBorneRickettsialNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "vector_borne_rickettsial",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "vector_borne_rickettsial",
			SenderNodeID:  "vector-borne-rickettsial-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
