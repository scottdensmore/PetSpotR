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

	"github.com/scottdensmore/petspotr/pkg/biometrics"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastForensicNano atomic.Int64
	forensicSeedMu   sync.Mutex
)

func nextForensicNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastForensicNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastForensicNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// BiometricForensicsPageData represents template data passed to templates/biometric_forensics.html.
type BiometricForensicsPageData struct {
	Cases  []domain.BiometricForensicCase
	Locale string
}

type createBiometricForensicCaseRequest struct {
	CaseID              string `json:"caseId"`
	PetID               string `json:"petId"`
	PetName             string `json:"petName"`
	Species             string `json:"species"`
	AgeYears            int    `json:"ageYears"`
	SensoryCondition    string `json:"sensoryCondition"`
	BiometricModality   string `json:"biometricModality"`
	NearestHazardType   string `json:"nearestHazardType"`
	NasalPrintMatched   bool   `json:"nasalPrintMatched"`
	WaterCulvertCleared bool   `json:"waterCulvertCleared"`
}

type verifyBiometricForensicCaseRequest struct {
	CorrelateBiometricAndSweepHazard bool   `json:"correlateBiometricAndSweepHazard"`
	ConfirmForensicExtraction        bool   `json:"confirmForensicExtraction"`
	CompleteStepID                   string `json:"completeStepId"`
	NasalPrintMatched                *bool  `json:"nasalPrintMatched"`
	WaterCulvertCleared              *bool  `json:"waterCulvertCleared"`
}

func (s *Server) registerBiometricForensicsRoutes() {
	s.mux.HandleFunc("GET /biometric-forensics", s.handleBiometricForensicsPage)
	s.mux.HandleFunc("GET /api/v1/biometric-forensics/cases", s.handleListBiometricForensicCases)
	s.mux.HandleFunc("POST /api/v1/biometric-forensics/cases", s.handleCreateBiometricForensicCase)
	s.mux.HandleFunc("POST /api/v1/biometric-forensics/cases/{id}/verify", s.handleVerifyBiometricForensicCase)
	s.mux.HandleFunc("GET /api/v1/biometric-forensics/stream", s.handleBiometricForensicsStream)
}

func defaultBiometricForensicCases(now time.Time) []domain.BiometricForensicCase {
	cases := make([]domain.BiometricForensicCase, 0, 3)

	if c1, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              "forensic-case-101",
		PetID:               "pet-2026-001",
		PetName:             "Barnaby",
		Species:             "DOG",
		AgeYears:            14,
		SensoryCondition:    "CCD_SUNDOWNING_DEMENTIA",
		BiometricModality:   "NASAL_PLANUM_RIDGE_PRINT",
		NearestHazardType:   "RETENTION_POND_CULVERT",
		NasalPrintMatched:   false,
		WaterCulvertCleared: false,
	}, now.Add(-1*time.Minute)); err == nil && c1 != nil {
		cases = append(cases, *c1)
	}

	if c2, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              "forensic-case-102",
		PetID:               "pet-2026-002",
		PetName:             "Clementine",
		Species:             "CAT",
		AgeYears:            12,
		SensoryCondition:    "BILATERAL_CATARACT_BLIND",
		BiometricModality:   "IRIS_MELANIN_TOPOLOGY",
		NearestHazardType:   "THICK_BLACKBERRY_BRAMBLE",
		NasalPrintMatched:   false,
		WaterCulvertCleared: false,
	}, now.Add(-3*time.Minute)); err == nil && c2 != nil {
		_ = biometrics.ApplyForensicCaseUpdate(c2, biometrics.ForensicCaseUpdateInput{
			CorrelateBiometricAndSweepHazard: true,
		}, now.Add(-2*time.Minute))
		cases = append(cases, *c2)
	}

	if c3, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              "forensic-case-103",
		PetID:               "pet-2026-003",
		PetName:             "Jasper",
		Species:             "DOG",
		AgeYears:            8,
		SensoryCondition:    "INTACT_SENSORY",
		BiometricModality:   "BUCCAL_SWAB_STR_DNA",
		NearestHazardType:   "POOL_UNFENCED_DECK",
		NasalPrintMatched:   true,
		WaterCulvertCleared: true,
	}, now.Add(-5*time.Minute)); err == nil && c3 != nil {
		_ = biometrics.ApplyForensicCaseUpdate(c3, biometrics.ForensicCaseUpdateInput{
			ConfirmForensicExtraction: true,
		}, now.Add(-4*time.Minute))
		cases = append(cases, *c3)
	}

	return cases
}

func (s *Server) ensureDefaultBiometricForensicCases(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	forensicSeedMu.Lock()
	defer forensicSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionBiometricForensicCases)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultBiometricForensicCases(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionBiometricForensicCases, item.CaseID, data)
		}
	}
}

func (s *Server) loadBiometricForensicCases(ctx context.Context, petIDFilter, statusFilter string) []domain.BiometricForensicCase {
	cases := make([]domain.BiometricForensicCase, 0)
	if s.stateStore == nil {
		return cases
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionBiometricForensicCases)
	if err != nil {
		return cases
	}
	for _, b := range raw {
		var item domain.BiometricForensicCase
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.ResolutionStatus), statusFilter) {
				continue
			}
			cases = append(cases, item)
		}
	}
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].UpdatedAt != cases[j].UpdatedAt {
			return cases[i].UpdatedAt > cases[j].UpdatedAt
		}
		return cases[i].CaseID < cases[j].CaseID
	})
	return cases
}

func (s *Server) handleBiometricForensicsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultBiometricForensicCases(r.Context())
	cases := s.loadBiometricForensicCases(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := BiometricForensicsPageData{
		Cases:  cases,
		Locale: locale,
	}

	s.renderCockpitTemplate(w, r, "biometric_forensics.html", "Biometric Forensics", data)
}

func (s *Server) handleListBiometricForensicCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultBiometricForensicCases(r.Context())
	q := r.URL.Query()
	cases := s.loadBiometricForensicCases(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"cases": cases,
	})
}

func (s *Server) handleCreateBiometricForensicCase(w http.ResponseWriter, r *http.Request) {
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
	var req createBiometricForensicCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultBiometricForensicCases(r.Context())

	seqNano := nextForensicNano()
	now := time.Unix(0, seqNano).UTC()

	caseID := strings.TrimSpace(req.CaseID)
	if caseID == "" {
		caseID = "forensic-case-" + randomHex8()
	}

	forensicCase, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              caseID,
		PetID:               req.PetID,
		PetName:             req.PetName,
		Species:             req.Species,
		AgeYears:            req.AgeYears,
		SensoryCondition:    req.SensoryCondition,
		BiometricModality:   req.BiometricModality,
		NearestHazardType:   req.NearestHazardType,
		NasalPrintMatched:   req.NasalPrintMatched,
		WaterCulvertCleared: req.WaterCulvertCleared,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	caseBytes, err := json.Marshal(forensicCase)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode biometric forensic case")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionBiometricForensicCases, forensicCase.CaseID, caseBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save biometric forensic case")
		return
	}

	s.broadcastBiometricForensicsEvent("forensic_case_updated", map[string]any{
		"type":         "forensic_case_updated",
		"forensicCase": forensicCase,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, forensicCase)
}

func (s *Server) handleVerifyBiometricForensicCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	caseID := strings.TrimSpace(r.PathValue("id"))
	if caseID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "case id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req verifyBiometricForensicCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultBiometricForensicCases(r.Context())

	seqNano := nextForensicNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedCase domain.BiometricForensicCase
		verifyErr   error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionBiometricForensicCases, caseID, func(current []byte) ([]byte, error) {
		var existing domain.BiometricForensicCase
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt biometric forensic case: %w", err)
		}

		if vErr := biometrics.ApplyForensicCaseUpdate(&existing, biometrics.ForensicCaseUpdateInput{
			CorrelateBiometricAndSweepHazard: req.CorrelateBiometricAndSweepHazard,
			ConfirmForensicExtraction:        req.ConfirmForensicExtraction,
			CompleteStepID:                   req.CompleteStepID,
			NasalPrintMatched:                req.NasalPrintMatched,
			WaterCulvertCleared:              req.WaterCulvertCleared,
		}, now); vErr != nil {
			verifyErr = vErr
			return nil, vErr
		}
		updatedCase = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Biometric forensic case not found")
			return
		}
		if verifyErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, verifyErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to verify biometric forensic case: %v", err))
		return
	}

	s.broadcastBiometricForensicsEvent("forensic_case_updated", map[string]any{
		"type":         "forensic_case_updated",
		"forensicCase": updatedCase,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedCase)
}

func (s *Server) handleBiometricForensicsStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "biometric_forensics")
}

func (s *Server) broadcastBiometricForensicsEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("biometric_forensics", "biometric-forensics-hub", eventType, payload, now)
}
