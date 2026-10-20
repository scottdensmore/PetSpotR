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
	"github.com/scottdensmore/petspotr/pkg/microchip"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastMicrochipNano atomic.Int64
	microchipSeedMu   sync.Mutex
)

func nextMicrochipNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastMicrochipNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastMicrochipNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// MicrochipFederationPageData represents template data passed to templates/microchip_federation.html.
type MicrochipFederationPageData struct {
	Cases  []domain.MicrochipScanCase
	Locale string
}

type createMicrochipCaseRequest struct {
	CaseID          string  `json:"caseId"`
	PetID           string  `json:"petId"`
	PetName         string  `json:"petName"`
	Species         string  `json:"species"`
	RawChipInput    string  `json:"rawChipInput"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	ScannedByAgency string  `json:"scannedByAgency"`
	SimulateMode    string  `json:"simulateMode"`
}

type traceMicrochipBatchLotRequest struct {
	ImplantingClinicName    string `json:"implantingClinicName"`
	ImplantingClinicPhone   string `json:"implantingClinicPhone"`
	ImplantPatientChartID   string `json:"implantPatientChartId"`
	RecoveredOwnerName      string `json:"recoveredOwnerName"`
	RecoveredOwnerPhoneMask string `json:"recoveredOwnerPhoneMask"`
}

type resolveMicrochipCaseRequest struct {
	VerifiedOwnerName      string `json:"verifiedOwnerName"`
	VerifiedOwnerPhoneMask string `json:"verifiedOwnerPhoneMask"`
	VerificationMethod     string `json:"verificationMethod"`
	Notes                  string `json:"notes"`
}

func (s *Server) registerMicrochipFederationRoutes() {
	s.mux.HandleFunc("GET /microchip-federation", s.handleMicrochipFederationPage)
	s.mux.HandleFunc("GET /api/v1/microchip-federation/cases", s.handleListMicrochipCases)
	s.mux.HandleFunc("POST /api/v1/microchip-federation/cases", s.handleCreateMicrochipCase)
	s.mux.HandleFunc("POST /api/v1/microchip-federation/cases/{id}/trace-lot", s.handleTraceMicrochipBatchLot)
	s.mux.HandleFunc("POST /api/v1/microchip-federation/cases/{id}/resolve", s.handleResolveMicrochipCase)
	s.mux.HandleFunc("GET /api/v1/microchip-federation/stream", s.handleMicrochipFederationStream)
}

func defaultMicrochipScanCases(now time.Time) []domain.MicrochipScanCase {
	cases := make([]domain.MicrochipScanCase, 0, 3)

	if c1, err := microchip.EvaluateFederatedScan(microchip.FederatedScanInput{
		CaseID:          "chip-case-101",
		PetID:           "pet-2026-002",
		PetName:         "Luna",
		Species:         "CAT",
		RawChipInput:    "900-118000-449102",
		ScannerLocation: domain.GeoPoint{Latitude: 47.6615, Longitude: -122.4120},
		ScannedByAgency: "Seattle Animal Shelter Intake Scanner #02",
		SimulateMode:    "UNREGISTERED_DEAD_END",
	}, now.Add(-1*time.Minute)); err == nil && c1 != nil {
		cases = append(cases, *c1)
	}

	if c2, err := microchip.EvaluateFederatedScan(microchip.FederatedScanInput{
		CaseID:          "chip-case-102",
		PetID:           "pet-2026-001",
		PetName:         "Buster",
		Species:         "DOG",
		RawChipInput:    "041-298-771",
		ScannerLocation: domain.GeoPoint{Latitude: 47.6812, Longitude: -122.3405},
		ScannedByAgency: "SAR Field Unit K9-04 (Halo Wand)",
		SimulateMode:    "OUTDATED_PRIMARY_PHONE",
	}, now.Add(-2*time.Minute)); err == nil && c2 != nil {
		cases = append(cases, *c2)
	}

	if c3, err := microchip.EvaluateFederatedScan(microchip.FederatedScanInput{
		CaseID:          "chip-case-103",
		PetID:           "pet-2026-003",
		PetName:         "Cooper",
		Species:         "DOG",
		RawChipInput:    "985-141002-883920",
		ScannerLocation: domain.GeoPoint{Latitude: 47.6253, Longitude: -122.3222},
		ScannedByAgency: "Capitol Hill Emergency Vet Triage",
		SimulateMode:    "REGISTERED_PRIMARY",
	}, now.Add(-4*time.Minute)); err == nil && c3 != nil {
		_ = microchip.ResolveMicrochipOwnership(c3, microchip.OwnershipResolutionInput{
			VerificationMethod: "PRIMARY_REGISTRY_MATCH",
			Notes:              "Primary owner verified via HomeAgain 2FA callback; reunion transport dispatched.",
		}, now.Add(-3*time.Minute))
		c3.PromotedSightingID = "sighting-chip-seed-103"
		cases = append(cases, *c3)
	}

	return cases
}

func (s *Server) ensureDefaultMicrochipCases(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	microchipSeedMu.Lock()
	defer microchipSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionMicrochipScanCases)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultMicrochipScanCases(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionMicrochipScanCases, item.CaseID, data)
		}
	}
}

func (s *Server) loadMicrochipCases(ctx context.Context, petIDFilter, statusFilter string) []domain.MicrochipScanCase {
	cases := make([]domain.MicrochipScanCase, 0)
	if s.stateStore == nil {
		return cases
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionMicrochipScanCases)
	if err != nil {
		return cases
	}
	for _, b := range raw {
		var item domain.MicrochipScanCase
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.Status), statusFilter) {
				continue
			}
			cases = append(cases, item)
		}
	}
	sort.Slice(cases, func(i, j int) bool {
		if !cases[i].UpdatedAt.Equal(cases[j].UpdatedAt) {
			return cases[i].UpdatedAt.After(cases[j].UpdatedAt)
		}
		return cases[i].CaseID < cases[j].CaseID
	})
	return cases
}

func (s *Server) promoteMicrochipCaseToSighting(ctx context.Context, scanCase domain.MicrochipScanCase, now time.Time) (domain.Sighting, error) {
	sightingID := "sighting-chip-" + randomHex8()
	coords := &domain.LocationPoint{
		Latitude:  scanCase.ScannerLocation.Latitude,
		Longitude: scanCase.ScannerLocation.Longitude,
	}
	sighting := domain.Sighting{
		ID:                  sightingID,
		SightingID:          sightingID,
		PetID:               scanCase.PetID,
		LostPetID:           scanCase.PetID,
		ReportedAt:          now,
		SightedAt:           now,
		LocationDescription: fmt.Sprintf("Microchip Federation Scan (%s — %s)", scanCase.MaskedChipDisplay, scanCase.ScannedByAgency),
		Coordinates:         coords,
		Notes:               scanCase.ResolutionNotes,
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

func (s *Server) handleMicrochipFederationPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultMicrochipCases(r.Context())
	cases := s.loadMicrochipCases(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MicrochipFederationPageData{
		Cases:  cases,
		Locale: locale,
	}

	s.renderCockpitTemplate(w, r, "microchip_federation.html", "Microchip Federation", data)
}

func (s *Server) handleListMicrochipCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultMicrochipCases(r.Context())
	q := r.URL.Query()
	cases := s.loadMicrochipCases(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"cases": cases,
	})
}

func (s *Server) handleCreateMicrochipCase(w http.ResponseWriter, r *http.Request) {
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
	var req createMicrochipCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultMicrochipCases(r.Context())

	seqNano := nextMicrochipNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	caseID := strings.TrimSpace(req.CaseID)
	if caseID == "" {
		caseID = "chip-case-" + randomHex8()
	}

	scanCase, err := microchip.EvaluateFederatedScan(microchip.FederatedScanInput{
		CaseID:       caseID,
		PetID:        req.PetID,
		PetName:      req.PetName,
		Species:      req.Species,
		RawChipInput: req.RawChipInput,
		ScannerLocation: domain.GeoPoint{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
		},
		ScannedByAgency: req.ScannedByAgency,
		SimulateMode:    req.SimulateMode,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	caseBytes, err := json.Marshal(scanCase)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode microchip scan case")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionMicrochipScanCases, scanCase.CaseID, caseBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save microchip scan case")
		return
	}

	s.broadcastMicrochipFederationEvent("microchip_case_updated", map[string]any{
		"type":     "microchip_case_updated",
		"scanCase": scanCase,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, scanCase)
}

func (s *Server) handleTraceMicrochipBatchLot(w http.ResponseWriter, r *http.Request) {
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
	var req traceMicrochipBatchLotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultMicrochipCases(r.Context())

	seqNano := nextMicrochipNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedCase domain.MicrochipScanCase
		traceErr    error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionMicrochipScanCases, caseID, func(current []byte) ([]byte, error) {
		var existing domain.MicrochipScanCase
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt microchip scan case: %w", err)
		}

		if tErr := microchip.TraceManufacturerBatchLot(&existing, microchip.LotTraceInput{
			ImplantingClinicName:    req.ImplantingClinicName,
			ImplantingClinicPhone:   req.ImplantingClinicPhone,
			ImplantPatientChartID:   req.ImplantPatientChartID,
			RecoveredOwnerName:      req.RecoveredOwnerName,
			RecoveredOwnerPhoneMask: req.RecoveredOwnerPhoneMask,
		}, now); tErr != nil {
			traceErr = tErr
			return nil, tErr
		}
		updatedCase = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Microchip scan case not found")
			return
		}
		if traceErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, traceErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to trace microchip batch lot: %v", err))
		return
	}

	s.broadcastMicrochipFederationEvent("microchip_case_updated", map[string]any{
		"type":     "microchip_case_updated",
		"scanCase": updatedCase,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedCase)
}

func (s *Server) handleResolveMicrochipCase(w http.ResponseWriter, r *http.Request) {
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
	var req resolveMicrochipCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultMicrochipCases(r.Context())

	seqNano := nextMicrochipNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)

	var (
		updatedCase domain.MicrochipScanCase
		resolveErr  error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionMicrochipScanCases, caseID, func(current []byte) ([]byte, error) {
		var existing domain.MicrochipScanCase
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt microchip scan case: %w", err)
		}

		if rErr := microchip.ResolveMicrochipOwnership(&existing, microchip.OwnershipResolutionInput{
			VerifiedOwnerName:      req.VerifiedOwnerName,
			VerifiedOwnerPhoneMask: req.VerifiedOwnerPhoneMask,
			VerificationMethod:     req.VerificationMethod,
			Notes:                  req.Notes,
		}, now); rErr != nil {
			resolveErr = rErr
			return nil, rErr
		}
		updatedCase = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Microchip scan case not found")
			return
		}
		if resolveErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, resolveErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to resolve microchip ownership: %v", err))
		return
	}

	var promotedSighting *domain.Sighting
	if updatedCase.PromotedSightingID == "" {
		sighting, promErr := s.promoteMicrochipCaseToSighting(r.Context(), updatedCase, now)
		if promErr == nil {
			updatedCase.PromotedSightingID = sighting.SightingID
			promotedSighting = &sighting
			if updatedBytes, mErr := json.Marshal(updatedCase); mErr == nil {
				_ = s.stateStore.SaveState(r.Context(), store.CollectionMicrochipScanCases, updatedCase.CaseID, updatedBytes)
			}
		}
	}

	s.broadcastMicrochipFederationEvent("microchip_case_updated", map[string]any{
		"type":     "microchip_case_updated",
		"scanCase": updatedCase,
		"sighting": promotedSighting,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedCase)
}

func (s *Server) handleMicrochipFederationStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "microchip_federation")
}

func (s *Server) broadcastMicrochipFederationEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("microchip_federation", "microchip-federation-hub", eventType, payload, now)
}
