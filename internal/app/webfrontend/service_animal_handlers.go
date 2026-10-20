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
	"github.com/scottdensmore/petspotr/pkg/serviceanimal"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastServiceAnimalNano atomic.Int64
	serviceAnimalSeedMu   sync.Mutex
)

func nextServiceAnimalNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastServiceAnimalNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastServiceAnimalNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// ServiceAnimalPriorityPageData represents template data passed to templates/service_animal_priority.html.
type ServiceAnimalPriorityPageData struct {
	Cases  []domain.ServiceAnimalCase
	Locale string
}

type createServiceAnimalCaseRequest struct {
	CaseID               string `json:"caseId"`
	PetID                string `json:"petId"`
	PetName              string `json:"petName"`
	Species              string `json:"species"`
	HandlerName          string `json:"handlerName"`
	ServiceTaskType      string `json:"serviceTaskType"`
	VestEquipmentState   string `json:"vestEquipmentState"`
	HoursSeparated       int    `json:"hoursSeparated"`
	HandlerStandbyLinked bool   `json:"handlerStandbyLinked"`
	RecallCueBroadcasted bool   `json:"recallCueBroadcasted"`
}

type recoverServiceAnimalCaseRequest struct {
	StageTacticalIntercept bool   `json:"stageTacticalIntercept"`
	RestoreADATeamReunion  bool   `json:"restoreAdaTeamReunion"`
	CompleteStepID         string `json:"completeStepId"`
	HandlerStandbyLinked   *bool  `json:"handlerStandbyLinked"`
	RecallCueBroadcasted   *bool  `json:"recallCueBroadcasted"`
}

func (s *Server) registerServiceAnimalRoutes() {
	s.mux.HandleFunc("GET /service-animal-priority", s.handleServiceAnimalPriorityPage)
	s.mux.HandleFunc("GET /api/v1/service-animal-priority/cases", s.handleListServiceAnimalCases)
	s.mux.HandleFunc("POST /api/v1/service-animal-priority/cases", s.handleCreateServiceAnimalCase)
	s.mux.HandleFunc("POST /api/v1/service-animal-priority/cases/{id}/recover", s.handleRecoverServiceAnimalCase)
	s.mux.HandleFunc("GET /api/v1/service-animal-priority/stream", s.handleServiceAnimalStream)
}

func defaultServiceAnimalCases(now time.Time) []domain.ServiceAnimalCase {
	cases := make([]domain.ServiceAnimalCase, 0, 3)

	if c1, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               "sa-case-101",
		PetID:                "pet-2026-001",
		PetName:              "Apollo",
		Species:              "DOG",
		HandlerName:          "Maya Lin (T1D Handler)",
		ServiceTaskType:      "DIABETIC_HYPOGLYCEMIA_ALERT",
		VestEquipmentState:   "ADA_CAPE_ENTANGLED_HAZARD",
		HoursSeparated:       8,
		HandlerStandbyLinked: false,
		RecallCueBroadcasted: false,
	}, now.Add(-1*time.Minute)); err == nil && c1 != nil {
		cases = append(cases, *c1)
	}

	if c2, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               "sa-case-102",
		PetID:                "pet-2026-002",
		PetName:              "Pax",
		Species:              "DOG",
		HandlerName:          "Jordan Vance",
		ServiceTaskType:      "BLIND_GUIDE_NAVIGATION",
		VestEquipmentState:   "HIGH_VIS_WORKING_HARNESS",
		HoursSeparated:       3,
		HandlerStandbyLinked: false,
		RecallCueBroadcasted: false,
	}, now.Add(-3*time.Minute)); err == nil && c2 != nil {
		_ = serviceanimal.ApplyServiceCaseUpdate(c2, serviceanimal.ServiceCaseUpdateInput{
			StageTacticalIntercept: true,
		}, now.Add(-2*time.Minute))
		cases = append(cases, *c2)
	}

	if c3, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               "sa-case-103",
		PetID:                "pet-2026-003",
		PetName:              "Valor",
		Species:              "DOG",
		HandlerName:          "Capt. Samira Reyes",
		ServiceTaskType:      "PTSD_PSYCHIATRIC_DEEP_PRESSURE",
		VestEquipmentState:   "HIGH_VIS_WORKING_HARNESS",
		HoursSeparated:       1,
		HandlerStandbyLinked: true,
		RecallCueBroadcasted: true,
	}, now.Add(-5*time.Minute)); err == nil && c3 != nil {
		_ = serviceanimal.ApplyServiceCaseUpdate(c3, serviceanimal.ServiceCaseUpdateInput{
			RestoreADATeamReunion: true,
		}, now.Add(-4*time.Minute))
		cases = append(cases, *c3)
	}

	return cases
}

func (s *Server) ensureDefaultServiceAnimalCases(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	serviceAnimalSeedMu.Lock()
	defer serviceAnimalSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionServiceAnimalCases)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultServiceAnimalCases(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionServiceAnimalCases, item.CaseID, data)
		}
	}
}

func (s *Server) loadServiceAnimalCases(ctx context.Context, petIDFilter, statusFilter string) []domain.ServiceAnimalCase {
	cases := make([]domain.ServiceAnimalCase, 0)
	if s.stateStore == nil {
		return cases
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionServiceAnimalCases)
	if err != nil {
		return cases
	}
	for _, b := range raw {
		var item domain.ServiceAnimalCase
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.RecoveryStatus), statusFilter) {
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

func (s *Server) handleServiceAnimalPriorityPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultServiceAnimalCases(r.Context())
	cases := s.loadServiceAnimalCases(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ServiceAnimalPriorityPageData{
		Cases:  cases,
		Locale: locale,
	}

	s.renderCockpitTemplate(w, r, "service_animal_priority.html", "Service Animal Priority", data)
}

func (s *Server) handleListServiceAnimalCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultServiceAnimalCases(r.Context())
	q := r.URL.Query()
	cases := s.loadServiceAnimalCases(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"cases": cases,
	})
}

func (s *Server) handleCreateServiceAnimalCase(w http.ResponseWriter, r *http.Request) {
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
	var req createServiceAnimalCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultServiceAnimalCases(r.Context())

	seqNano := nextServiceAnimalNano()
	now := time.Unix(0, seqNano).UTC()

	caseID := strings.TrimSpace(req.CaseID)
	if caseID == "" {
		caseID = "sa-case-" + randomHex8()
	}

	serviceCase, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               caseID,
		PetID:                req.PetID,
		PetName:              req.PetName,
		Species:              req.Species,
		HandlerName:          req.HandlerName,
		ServiceTaskType:      req.ServiceTaskType,
		VestEquipmentState:   req.VestEquipmentState,
		HoursSeparated:       req.HoursSeparated,
		HandlerStandbyLinked: req.HandlerStandbyLinked,
		RecallCueBroadcasted: req.RecallCueBroadcasted,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	caseBytes, err := json.Marshal(serviceCase)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode service animal case")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionServiceAnimalCases, serviceCase.CaseID, caseBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save service animal case")
		return
	}

	s.broadcastServiceAnimalEvent("service_animal_case_updated", map[string]any{
		"type":              "service_animal_case_updated",
		"serviceAnimalCase": serviceCase,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, serviceCase)
}

func (s *Server) handleRecoverServiceAnimalCase(w http.ResponseWriter, r *http.Request) {
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
	var req recoverServiceAnimalCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultServiceAnimalCases(r.Context())

	seqNano := nextServiceAnimalNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedCase domain.ServiceAnimalCase
		recoverErr  error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionServiceAnimalCases, caseID, func(current []byte) ([]byte, error) {
		var existing domain.ServiceAnimalCase
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt service animal case: %w", err)
		}

		if rErr := serviceanimal.ApplyServiceCaseUpdate(&existing, serviceanimal.ServiceCaseUpdateInput{
			StageTacticalIntercept: req.StageTacticalIntercept,
			RestoreADATeamReunion:  req.RestoreADATeamReunion,
			CompleteStepID:         req.CompleteStepID,
			HandlerStandbyLinked:   req.HandlerStandbyLinked,
			RecallCueBroadcasted:   req.RecallCueBroadcasted,
		}, now); rErr != nil {
			recoverErr = rErr
			return nil, rErr
		}
		updatedCase = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Service animal case not found")
			return
		}
		if recoverErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, recoverErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to recover service animal case: %v", err))
		return
	}

	s.broadcastServiceAnimalEvent("service_animal_case_updated", map[string]any{
		"type":              "service_animal_case_updated",
		"serviceAnimalCase": updatedCase,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedCase)
}

func (s *Server) handleServiceAnimalStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "service_animal_priority")
}

func (s *Server) broadcastServiceAnimalEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("service_animal_priority", "service-animal-priority-hub", eventType, payload, now)
}
