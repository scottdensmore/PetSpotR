package addisoniancrisis

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AddisonianMetrics summarizes operational KPI counts across Hypoadrenocorticism (Addisonian Crisis) operations.
type AddisonianMetrics struct {
	TotalOperations             int `json:"total_operations"`
	CriticalShockCount          int `json:"critical_shock_count"`
	DexSPACTHStagedCount        int `json:"dex_sp_acth_staged_count"`
	DOCPEukalemicClearedCount   int `json:"docp_eukalemic_cleared_count"`
	MyelinolysisAssayAlertCount int `json:"myelinolysis_assay_alert_count"`
}

// AddisonianService manages Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27,
// Dexamethasone-SP & DOCP triage operations in a thread-safe in-memory store.
type AddisonianService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.AddisonianCrisisOperation
	order []string
	seq   int
}

// NewAddisonianService initializes an AddisonianService with 3 seeded operations (AC-OP-101, AC-OP-102, AC-OP-103).
func NewAddisonianService() *AddisonianService {
	s := &AddisonianService{
		ops:   make(map[string]*domain.AddisonianCrisisOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildResuscitationRegimen deterministically computes an AddisonianResuscitationRegimen for a patient profile.
func BuildResuscitationRegimen(profile domain.AddisonianPatientProfile) *domain.AddisonianResuscitationRegimen {
	naKRatio := round1(profile.SerumSodiumMeqL / profile.SerumPotassiumMeqL)
	naclBolusMl := round2(20.0 * profile.WeightKg)
	dexSPMg := round2(0.25 * profile.WeightKg)
	cosyntropinMcg := round2(5.0 * profile.WeightKg)
	if cosyntropinMcg > 250.0 {
		cosyntropinMcg = 250.0
	}

	var caGluconateMl float64
	if profile.SerumPotassiumMeqL >= 7.0 || profile.HeartRateBpm < 60 {
		caGluconateMl = round2(0.5 * profile.WeightKg)
	}

	pitfallAlert := "CRITICAL ADDISONIAN CRISIS & OSMOTIC DEMYELINATION PITFALL ALERT: NEVER administer Prednisone, Prednisolone, or Hydrocortisone before completing the ACTH stimulation test (they cross-react with the cortisol immunoassay and invalidate diagnosis — administer Dexamethasone-SP 0.25 mg/kg IV instead); and NEVER raise serum Sodium > 10–12 mEq/L/24h in chronic hyponatremia (< 125 mEq/L), which causes fatal osmotic demyelination syndrome / central pontine myelinolysis."

	rationaleParts := []string{
		fmt.Sprintf("Na:K Ratio %.1f (Na+ %.1f mEq/L, K+ %.1f mEq/L).", naKRatio, profile.SerumSodiumMeqL, profile.SerumPotassiumMeqL),
		fmt.Sprintf("0.9%% Isotonic NaCl Shock Aliquot: %.1f mL (20 mL/kg for %.1f kg %s) with strict max Na+ rise <= 10.0 mEq/L/day (<= 12.0 mEq/L/24h hard ceiling).", naclBolusMl, profile.WeightKg, profile.Species),
		fmt.Sprintf("Dexamethasone-SP: %.2f mg IV (0.25 mg/kg — zero cortisol assay cross-reactivity) + Cosyntropin ACTH Stim: %.1f mcg IV (5 mcg/kg, max 250 mcg).", dexSPMg, cosyntropinMcg),
	}
	if caGluconateMl > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("10%% Calcium Gluconate Cardioprotection: %.2f mL slow IV over 15 min (0.5 mL/kg for K+ %.1f mEq/L / HR %d bpm).", caGluconateMl, profile.SerumPotassiumMeqL, profile.HeartRateBpm))
	}

	return &domain.AddisonianResuscitationRegimen{
		SodiumPotassiumRatio:         naKRatio,
		IsotonicNaCl09BolusMl:        naclBolusMl,
		DexamethasoneSP025MgKgMg:     dexSPMg,
		CosyntropinACTHStimDoseMcg:   cosyntropinMcg,
		CalciumGluconate10PctMl:      caGluconateMl,
		MaxSodiumRiseRateMeqLDay:     10.0,
		ZeroPrednisolonePreACTHGuard: true,
		AddisonianPitfallAlert:       pitfallAlert,
		ClinicalRationale:            strings.Join(rationaleParts, " "),
	}
}

func needsOsmoticMyelinolysisAssayAlert(p domain.AddisonianPatientProfile) bool {
	naKRatio := p.SerumSodiumMeqL / p.SerumPotassiumMeqL
	return naKRatio < 27.0 ||
		p.SerumSodiumMeqL < 135.0 ||
		p.SerumPotassiumMeqL > 5.5 ||
		p.HeartRateBpm < 65 ||
		p.SystolicBloodPressureMmHg < 90 ||
		p.RestingCortisolMcgDl < 2.0
}

func (s *AddisonianService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.AddisonianPatientProfile{
		Species:                   domain.AddisonianSpeciesCanineTypical,
		Breed:                     "Standard Poodle",
		WeightKg:                  23.5,
		SerumSodiumMeqL:           124.0,
		SerumPotassiumMeqL:        7.8,
		BloodGlucoseMgDl:          54.0,
		HeartRateBpm:              52,
		SystolicBloodPressureMmHg: 68,
		RestingCortisolMcgDl:      0.2,
	}
	op101 := &domain.AddisonianCrisisOperation{
		ID:                            "AC-OP-101",
		IncidentID:                    "INC-AC-9601",
		PetName:                       "Curly",
		Status:                        domain.AddisonianStatusShockHyperkalemicCritical,
		OsmoticMyelinolysisAssayAlert: true,
		PatientProfile:                p101,
		ResuscitationRegimen:          BuildResuscitationRegimen(p101),
		ElectrolyteChecks: []domain.AddisonianElectrolyteCheck{
			{
				Timestamp:             t101.Add(10 * time.Minute),
				SerumSodiumMeqL:       124.0,
				SerumPotassiumMeqL:    7.8,
				SodiumRise24HrMeqL:    0.0,
				PostACTHCortisolMcgDl: 0.2,
				HeartRateBpm:          52,
				ClinicianCallsign:     "DR-ENDO-19",
				Notes:                 "Acute Addisonian crisis in Standard Poodle; Na:K ratio 15.9, hyperkalemic bradycardia (HR 52 bpm, K+ 7.8 mEq/L), hypoglycemia 54 mg/dL; 10% Calcium Gluconate 11.75 mL slow IV + 0.9% NaCl 470 mL bolus + Dexamethasone-SP 5.88 mg IV staged.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 20, 0, 0, time.UTC)
	p102 := domain.AddisonianPatientProfile{
		Species:                   domain.AddisonianSpeciesCanineTypical,
		Breed:                     "Portuguese Water Dog",
		WeightKg:                  19.0,
		SerumSodiumMeqL:           131.0,
		SerumPotassiumMeqL:        6.1,
		BloodGlucoseMgDl:          76.0,
		HeartRateBpm:              88,
		SystolicBloodPressureMmHg: 94,
		RestingCortisolMcgDl:      0.4,
	}
	op102 := &domain.AddisonianCrisisOperation{
		ID:                            "AC-OP-102",
		IncidentID:                    "INC-AC-9602",
		PetName:                       "Pippa",
		Status:                        domain.AddisonianStatusDexSPNaClACTHStaged,
		OsmoticMyelinolysisAssayAlert: true,
		PatientProfile:                p102,
		ResuscitationRegimen:          BuildResuscitationRegimen(p102),
		ElectrolyteChecks: []domain.AddisonianElectrolyteCheck{
			{
				Timestamp:             t102.Add(15 * time.Minute),
				SerumSodiumMeqL:       131.0,
				SerumPotassiumMeqL:    6.1,
				SodiumRise24HrMeqL:    4.5,
				PostACTHCortisolMcgDl: 0.5,
				HeartRateBpm:          88,
				ClinicianCallsign:     "DR-ECC-31",
				Notes:                 "Portuguese Water Dog; Dexamethasone-SP 4.75 mg IV and Cosyntropin 95 mcg IV administered; 24h Na+ rise 4.5 mEq/L (<= 10.0 mEq/L/day target).",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	p103 := domain.AddisonianPatientProfile{
		Species:                   domain.AddisonianSpeciesCanineTypical,
		Breed:                     "Great Dane",
		WeightKg:                  48.0,
		SerumSodiumMeqL:           144.0,
		SerumPotassiumMeqL:        4.5,
		BloodGlucoseMgDl:          98.0,
		HeartRateBpm:              92,
		SystolicBloodPressureMmHg: 122,
		RestingCortisolMcgDl:      0.3,
	}
	clearance103 := &domain.AddisonianDischargeClearance{
		ClearedAt:                        t103.Add(28 * time.Hour),
		FinalSodiumPotassiumRatio:        32.0,
		FinalSerumPotassiumMeqL:          4.5,
		PostACTHCortisolConfirmedLow:     true,
		DOCP22MgKgSubQAdministered:       true,
		OralPrednisonePhysiologicStarted: true,
		VetLicense:                       "VET-CA-96108",
		Summary:                          "Post-ACTH cortisol confirmed < 2.0 mcg/dL (0.3 mcg/dL); eukalemic K+ 4.5 mEq/L, Na:K ratio 32.0; DOCP 2.2 mg/kg SubQ (105.6 mg) administered and oral physiologic Prednisone 0.2 mg/kg/day initiated.",
	}
	op103 := &domain.AddisonianCrisisOperation{
		ID:                            "AC-OP-103",
		IncidentID:                    "INC-AC-9603",
		PetName:                       "Archie",
		Status:                        domain.AddisonianStatusDOCPEukalemicCleared,
		OsmoticMyelinolysisAssayAlert: false,
		PatientProfile:                p103,
		ResuscitationRegimen:          BuildResuscitationRegimen(p103),
		ElectrolyteChecks: []domain.AddisonianElectrolyteCheck{
			{
				Timestamp:             t103.Add(24 * time.Hour),
				SerumSodiumMeqL:       144.0,
				SerumPotassiumMeqL:    4.5,
				SodiumRise24HrMeqL:    7.5,
				PostACTHCortisolMcgDl: 0.3,
				HeartRateBpm:          92,
				ClinicianCallsign:     "DR-ENDO-04",
				Notes:                 "Post-ACTH cortisol 0.3 mcg/dL confirms primary hypoadrenocorticism; Na+ rose 7.5 mEq/L over 24h without neurologic signs; eukalemic.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(28 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *AddisonianService) ListOperations(statusFilter string) []domain.AddisonianCrisisOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.AddisonianCrisisOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneAddisonianOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *AddisonianService) GetOperation(id string) (*domain.AddisonianCrisisOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("addisonian crisis operation %q not found", id)
	}
	cp := cloneAddisonianOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Hypoadrenocorticism (Addisonian Crisis) triage operation.
func (s *AddisonianService) CreateOperation(incidentID, petName string, profile domain.AddisonianPatientProfile) (*domain.AddisonianCrisisOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("AC-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsOsmoticMyelinolysisAssayAlert(profile)
	regimen := BuildResuscitationRegimen(profile)

	op := &domain.AddisonianCrisisOperation{
		ID:                            opID,
		IncidentID:                    trimmedIncident,
		PetName:                       trimmedPet,
		Status:                        domain.AddisonianStatusShockHyperkalemicCritical,
		OsmoticMyelinolysisAssayAlert: alert,
		PatientProfile:                profile,
		ResuscitationRegimen:          regimen,
		ElectrolyteChecks:             []domain.AddisonianElectrolyteCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneAddisonianOperation(op)
	return &cp, nil
}

// StageResuscitationRegimen computes the Na:K ratio, 0.9% NaCl bolus aliquot, Dexamethasone-SP,
// Cosyntropin ACTH stimulation dose, and 10% Calcium Gluconate cardioprotection,
// transitioning the operation to DEX_SP_NACL_ACTH_STIM_STAGED.
func (s *AddisonianService) StageResuscitationRegimen(id string, profile domain.AddisonianPatientProfile) (*domain.AddisonianCrisisOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("addisonian crisis operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.ResuscitationRegimen = BuildResuscitationRegimen(profile)
	op.OsmoticMyelinolysisAssayAlert = needsOsmoticMyelinolysisAssayAlert(profile)
	op.Status = domain.AddisonianStatusDexSPNaClACTHStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneAddisonianOperation(op)
	return &cp, nil
}

// RecordElectrolyteCheck appends a serial electrolyte, 24h Na+ rise rate, and post-ACTH cortisol check.
func (s *AddisonianService) RecordElectrolyteCheck(id string, check domain.AddisonianElectrolyteCheck) (*domain.AddisonianCrisisOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("addisonian crisis operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.ElectrolyteChecks = append(op.ElectrolyteChecks, check)
	op.PatientProfile.SerumSodiumMeqL = check.SerumSodiumMeqL
	op.PatientProfile.SerumPotassiumMeqL = check.SerumPotassiumMeqL
	op.PatientProfile.HeartRateBpm = check.HeartRateBpm
	op.ResuscitationRegimen = BuildResuscitationRegimen(op.PatientProfile)

	naKRatio := check.SerumSodiumMeqL / check.SerumPotassiumMeqL
	if check.SerumPotassiumMeqL >= 7.0 || check.HeartRateBpm < 60 || check.SodiumRise24HrMeqL > 10.0 {
		op.OsmoticMyelinolysisAssayAlert = true
		if check.SerumPotassiumMeqL >= 7.0 || check.HeartRateBpm < 55 {
			op.Status = domain.AddisonianStatusShockHyperkalemicCritical
			op.DischargeClearance = nil
		}
	} else if naKRatio >= 27.0 && check.SerumPotassiumMeqL <= 5.5 && check.SerumSodiumMeqL >= 135.0 {
		op.OsmoticMyelinolysisAssayAlert = false
	}

	op.UpdatedAt = now
	cp := cloneAddisonianOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates Na:K ratio >= 27.0, eukalemia, confirmed low post-ACTH cortisol,
// DOCP 2.2 mg/kg SubQ administration, and oral Prednisone start, transitioning the operation to DOCP_PREDNISONE_EUKALEMIC_CLEARED.
func (s *AddisonianService) IssueDischargeClearance(id string, clearance domain.AddisonianDischargeClearance) (*domain.AddisonianCrisisOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("addisonian crisis operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Addisonian crisis resolved (Na:K ratio %.1f, K+ %.1f mEq/L); post-ACTH cortisol confirmed < 2.0 mcg/dL; DOCP 2.2 mg/kg SubQ administered and oral physiologic Prednisone initiated.", clearance.FinalSodiumPotassiumRatio, clearance.FinalSerumPotassiumMeqL)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.AddisonianStatusDOCPEukalemicCleared
	op.OsmoticMyelinolysisAssayAlert = false
	op.PatientProfile.SerumPotassiumMeqL = clearance.FinalSerumPotassiumMeqL
	op.PatientProfile.SerumSodiumMeqL = round1(clearance.FinalSodiumPotassiumRatio * clearance.FinalSerumPotassiumMeqL)
	op.ResuscitationRegimen = BuildResuscitationRegimen(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneAddisonianOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all Addisonian crisis operations in the service.
func (s *AddisonianService) ComputeMetrics() AddisonianMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m AddisonianMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.AddisonianStatusShockHyperkalemicCritical:
			m.CriticalShockCount++
		case domain.AddisonianStatusDexSPNaClACTHStaged:
			m.DexSPACTHStagedCount++
		case domain.AddisonianStatusDOCPEukalemicCleared:
			m.DOCPEukalemicClearedCount++
		}
		if op.OsmoticMyelinolysisAssayAlert {
			m.MyelinolysisAssayAlertCount++
		}
	}
	return m
}

func cloneAddisonianOperation(op *domain.AddisonianCrisisOperation) domain.AddisonianCrisisOperation {
	cp := *op
	if op.ResuscitationRegimen != nil {
		reg := *op.ResuscitationRegimen
		cp.ResuscitationRegimen = &reg
	}
	if op.ElectrolyteChecks != nil {
		cp.ElectrolyteChecks = make([]domain.AddisonianElectrolyteCheck, len(op.ElectrolyteChecks))
		copy(cp.ElectrolyteChecks, op.ElectrolyteChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
