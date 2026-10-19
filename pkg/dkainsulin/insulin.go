package dkainsulin

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// DKAMetrics summarizes operational KPI counts across DKA & Regular Insulin CRI triage operations.
type DKAMetrics struct {
	TotalOperations            int `json:"total_operations"`
	CriticalAcidosisCount      int `json:"critical_acidosis_count"`
	InsulinCRIStagedCount      int `json:"insulin_cri_staged_count"`
	KetoneClearedCount         int `json:"ketone_cleared_count"`
	HypokalemiaEdemaAlertCount int `json:"hypokalemia_edema_alert_count"`
}

// DKAService manages Diabetic Ketoacidosis (DKA), Regular Insulin CRI,
// Hypokalemia/Hypophosphatemia & Cerebral Edema triage operations in a thread-safe in-memory store.
type DKAService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.DKAInsulinOperation
	order []string
	seq   int
}

// NewDKAService initializes a DKAService with 3 seeded operations (DKA-OP-101, DKA-OP-102, DKA-OP-103).
func NewDKAService() *DKAService {
	s := &DKAService{
		ops:   make(map[string]*domain.DKAInsulinOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildInsulinRegimen deterministically computes a DKAElectrolyteInsulinRegimen for a patient profile.
func BuildInsulinRegimen(profile domain.DKAPatientProfile) *domain.DKAElectrolyteInsulinRegimen {
	rehydrationDelayHours := 4
	if profile.DehydrationPercent >= 7.0 || profile.SerumPotassiumMeqL < 3.3 {
		rehydrationDelayHours = 6
	}

	var kclMeqPerLiter int
	switch {
	case profile.SerumPotassiumMeqL < 2.0:
		kclMeqPerLiter = 80
	case profile.SerumPotassiumMeqL < 2.5:
		kclMeqPerLiter = 60
	case profile.SerumPotassiumMeqL < 3.0:
		kclMeqPerLiter = 40
	case profile.SerumPotassiumMeqL < 3.5:
		kclMeqPerLiter = 30
	default:
		kclMeqPerLiter = 20
	}

	var kPhosMmolKgHr float64
	switch {
	case profile.SerumPhosphorusMgDl < 1.5:
		kPhosMmolKgHr = 0.12
	case profile.SerumPhosphorusMgDl < 2.0:
		kPhosMmolKgHr = 0.06
	case profile.SerumPhosphorusMgDl < 2.5:
		kPhosMmolKgHr = 0.03
	default:
		kPhosMmolKgHr = 0.0
	}

	insulinCRIUkgDay := 2.2
	if strings.EqualFold(profile.Species, domain.DKASpeciesFelineHepaticLipidosis) {
		insulinCRIUkgDay = 1.1
	}

	var dextrosePercent float64
	switch {
	case profile.BloodGlucoseMgDl < 150.0:
		dextrosePercent = 5.0
	case profile.BloodGlucoseMgDl < 250.0:
		dextrosePercent = 2.5
	default:
		dextrosePercent = 0.0
	}

	pitfallAlert := "CRITICAL DKA & CEREBRAL EDEMA PITFALL ALERT: NEVER initiate Regular Insulin CRI before 4–6h of isotonic fluid rehydration and potassium correction (K+ >= 3.3 mEq/L); NEVER drop blood glucose > 50–75 mg/dL/hr (rapid osmotic shift triggers fatal cerebral edema); supplement 2.5%–5.0% Dextrose when BG < 250 mg/dL so insulin continues clearing BHB ketones without hypoglycemia; and treat hypophosphatemia (< 1.5 mg/dL) with Potassium Phosphate CRI to prevent Heinz body hemolytic anemia."

	rationaleParts := []string{
		fmt.Sprintf("Stage 1: %dh isotonic buffered crystalloid rehydration before starting insulin.", rehydrationDelayHours),
		fmt.Sprintf("Sliding-Scale KCl %d mEq/L for serum K+ %.1f mEq/L (strictly hold insulin while K+ < 3.3 mEq/L).", kclMeqPerLiter, profile.SerumPotassiumMeqL),
	}
	if kPhosMmolKgHr > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Potassium Phosphate CRI %.2f mmol/kg/hr for serum phosphorus %.1f mg/dL (prevents erythrocyte ATP depletion & Heinz body hemolysis).", kPhosMmolKgHr, profile.SerumPhosphorusMgDl))
	}
	rationaleParts = append(rationaleParts, fmt.Sprintf("Regular Crystalline Insulin CRI at %.1f U/kg/day (%.2f U/day for %.1f kg %s) with max BG drop <= 50 mg/dL/hr.", insulinCRIUkgDay, round2(insulinCRIUkgDay*profile.WeightKg), profile.WeightKg, profile.Species))
	if dextrosePercent > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Add %.1f%% Dextrose supplementation for BG %.1f mg/dL (< 250 mg/dL) to maintain insulin-driven ketogenesis suppression without hypoglycemia.", dextrosePercent, profile.BloodGlucoseMgDl))
	}

	return &domain.DKAElectrolyteInsulinRegimen{
		FluidRehydrationDelayHours:         rehydrationDelayHours,
		PotassiumChlorideMeqPerLiter:       kclMeqPerLiter,
		PotassiumPhosphateMmolKgHr:         kPhosMmolKgHr,
		RegularInsulinCRIUkgDay:            insulinCRIUkgDay,
		DextroseSupplementationPercent:     dextrosePercent,
		MaxGlucoseDropRateMgDlHr:           50.0,
		HoldInsulinIfPotassiumBelow33Guard: true,
		DKAPitfallAlert:                    pitfallAlert,
		ClinicalRationale:                  strings.Join(rationaleParts, " "),
	}
}

func needsHypokalemiaCerebralEdemaAlert(p domain.DKAPatientProfile) bool {
	return p.SerumPotassiumMeqL < 3.5 ||
		p.SerumPhosphorusMgDl < 2.5 ||
		p.VenousPH < 7.25 ||
		p.EffectiveOsmolalityMosmKg >= 315.0 ||
		p.BetaHydroxybutyrateMmolL >= 3.0
}

func (s *DKAService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesFelineHepaticLipidosis,
		Breed:                     "Domestic Longhair",
		WeightKg:                  5.4,
		BloodGlucoseMgDl:          485.0,
		BetaHydroxybutyrateMmolL:  5.8,
		VenousPH:                  7.12,
		SerumPotassiumMeqL:        2.8,
		SerumPhosphorusMgDl:       1.6,
		EffectiveOsmolalityMosmKg: 328.0,
		DehydrationPercent:        8.0,
	}
	op101 := &domain.DKAInsulinOperation{
		ID:                            "DKA-OP-101",
		IncidentID:                    "INC-DKA-9301",
		PetName:                       "Winston",
		Status:                        domain.DKAStatusAcidosisDehydratedCritical,
		HypokalemiaCerebralEdemaAlert: true,
		PatientProfile:                p101,
		InsulinRegimen:                BuildInsulinRegimen(p101),
		SerialChecks: []domain.DKAGlucoseElectrolyteCheck{
			{
				Timestamp:                      t101.Add(10 * time.Minute),
				BloodGlucoseMgDl:               485.0,
				BetaHydroxybutyrateMmolL:       5.8,
				SerumPotassiumMeqL:             2.8,
				SerumPhosphorusMgDl:            1.6,
				DextrosePercentActive:          0.0,
				MentationNormalNoCerebralEdema: true,
				ClinicianCallsign:              "DR-ENDO-14",
				Notes:                          "Severe feline DKA with hepatic lipidosis; venous pH 7.12, BHB 5.8 mmol/L, K+ 2.8 mEq/L, Phos 1.6 mg/dL; holding insulin during 6h isotonic rehydration + KCl 40 mEq/L + KPhos 0.06 mmol/kg/hr.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 20, 0, 0, time.UTC)
	p102 := domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesCaninePancreatitis,
		Breed:                     "Miniature Schnauzer",
		WeightKg:                  8.2,
		BloodGlucoseMgDl:          220.0,
		BetaHydroxybutyrateMmolL:  3.1,
		VenousPH:                  7.26,
		SerumPotassiumMeqL:        3.8,
		SerumPhosphorusMgDl:       2.9,
		EffectiveOsmolalityMosmKg: 304.0,
		DehydrationPercent:        5.0,
	}
	op102 := &domain.DKAInsulinOperation{
		ID:                            "DKA-OP-102",
		IncidentID:                    "INC-DKA-9302",
		PetName:                       "Daisy",
		Status:                        domain.DKAStatusKClPhosInsulinCRIStaged,
		HypokalemiaCerebralEdemaAlert: true,
		PatientProfile:                p102,
		InsulinRegimen:                BuildInsulinRegimen(p102),
		SerialChecks: []domain.DKAGlucoseElectrolyteCheck{
			{
				Timestamp:                      t102.Add(15 * time.Minute),
				BloodGlucoseMgDl:               220.0,
				BetaHydroxybutyrateMmolL:       3.1,
				SerumPotassiumMeqL:             3.8,
				SerumPhosphorusMgDl:            2.9,
				DextrosePercentActive:          2.5,
				MentationNormalNoCerebralEdema: true,
				ClinicianCallsign:              "DR-ECC-22",
				Notes:                          "Canine pancreatitis-associated DKA; K+ corrected to 3.8 mEq/L; Regular Insulin CRI 2.2 U/kg/day running with 2.5% Dextrose added for BG 220 mg/dL (< 250 mg/dL).",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	p103 := domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesFelineHepaticLipidosis,
		Breed:                     "Orange Tabby",
		WeightKg:                  4.8,
		BloodGlucoseMgDl:          165.0,
		BetaHydroxybutyrateMmolL:  0.4,
		VenousPH:                  7.39,
		SerumPotassiumMeqL:        4.2,
		SerumPhosphorusMgDl:       3.8,
		EffectiveOsmolalityMosmKg: 292.0,
		DehydrationPercent:        0.0,
	}
	clearance103 := &domain.DKADischargeClearance{
		ClearedAt:                      t103.Add(30 * time.Hour),
		FinalBetaHydroxybutyrateMmolL:  0.4,
		FinalVenousPH:                  7.39,
		FinalSerumPotassiumMeqL:        4.2,
		FinalSerumPhosphorusMgDl:       3.8,
		VoluntaryEatingNoVomiting:      true,
		SubQInsulinTransitionCompleted: true,
		VetLicense:                     "VET-CA-93104",
		Summary:                        "BHB ketones cleared to 0.4 mmol/L, venous pH 7.39, K+ 4.2 mEq/L, Phos 3.8 mg/dL; eating voluntarily without vomiting; transitioned from Regular Insulin CRI to SubQ Glargine 1.5 U q12h.",
	}
	op103 := &domain.DKAInsulinOperation{
		ID:                            "DKA-OP-103",
		IncidentID:                    "INC-DKA-9303",
		PetName:                       "Chester",
		Status:                        domain.DKAStatusKetoneResolvedGlargineCleared,
		HypokalemiaCerebralEdemaAlert: false,
		PatientProfile:                p103,
		InsulinRegimen:                BuildInsulinRegimen(p103),
		SerialChecks: []domain.DKAGlucoseElectrolyteCheck{
			{
				Timestamp:                      t103.Add(28 * time.Hour),
				BloodGlucoseMgDl:               165.0,
				BetaHydroxybutyrateMmolL:       0.4,
				SerumPotassiumMeqL:             4.2,
				SerumPhosphorusMgDl:            3.8,
				DextrosePercentActive:          0.0,
				MentationNormalNoCerebralEdema: true,
				ClinicianCallsign:              "DR-ENDO-09",
				Notes:                          "Ketones resolved (BHB 0.4 mmol/L), normokalemic, normophosphatemic, alert mentation; eating recovery diet voluntarily.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(30 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *DKAService) ListOperations(statusFilter string) []domain.DKAInsulinOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.DKAInsulinOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneDKAOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *DKAService) GetOperation(id string) (*domain.DKAInsulinOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("dka insulin operation %q not found", id)
	}
	cp := cloneDKAOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new DKA & Regular Insulin CRI triage operation.
func (s *DKAService) CreateOperation(incidentID, petName string, profile domain.DKAPatientProfile) (*domain.DKAInsulinOperation, error) {
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
	opID := fmt.Sprintf("DKA-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsHypokalemiaCerebralEdemaAlert(profile)
	regimen := BuildInsulinRegimen(profile)

	op := &domain.DKAInsulinOperation{
		ID:                            opID,
		IncidentID:                    trimmedIncident,
		PetName:                       trimmedPet,
		Status:                        domain.DKAStatusAcidosisDehydratedCritical,
		HypokalemiaCerebralEdemaAlert: alert,
		PatientProfile:                profile,
		InsulinRegimen:                regimen,
		SerialChecks:                  []domain.DKAGlucoseElectrolyteCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneDKAOperation(op)
	return &cp, nil
}

// StageInsulinRegimen computes the sliding-scale KCl, KPhos CRI, Regular Insulin CRI, and Dextrose regimen
// and transitions the operation to KCL_PHOS_REGULAR_INSULIN_CRI_STAGED.
func (s *DKAService) StageInsulinRegimen(id string, profile domain.DKAPatientProfile) (*domain.DKAInsulinOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("dka insulin operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.InsulinRegimen = BuildInsulinRegimen(profile)
	op.HypokalemiaCerebralEdemaAlert = needsHypokalemiaCerebralEdemaAlert(profile)
	op.Status = domain.DKAStatusKClPhosInsulinCRIStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneDKAOperation(op)
	return &cp, nil
}

// RecordSerialCheck appends a q2h glucose/electrolyte/ketone check to an existing operation.
func (s *DKAService) RecordSerialCheck(id string, check domain.DKAGlucoseElectrolyteCheck) (*domain.DKAInsulinOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("dka insulin operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.SerialChecks = append(op.SerialChecks, check)
	op.PatientProfile.BloodGlucoseMgDl = check.BloodGlucoseMgDl
	op.PatientProfile.BetaHydroxybutyrateMmolL = check.BetaHydroxybutyrateMmolL
	op.PatientProfile.SerumPotassiumMeqL = check.SerumPotassiumMeqL
	op.PatientProfile.SerumPhosphorusMgDl = check.SerumPhosphorusMgDl
	op.InsulinRegimen = BuildInsulinRegimen(op.PatientProfile)

	if !check.MentationNormalNoCerebralEdema || check.SerumPotassiumMeqL < 3.3 || check.SerumPhosphorusMgDl < 1.5 {
		op.HypokalemiaCerebralEdemaAlert = true
		if !check.MentationNormalNoCerebralEdema || check.SerumPotassiumMeqL < 3.0 {
			op.Status = domain.DKAStatusAcidosisDehydratedCritical
			op.DischargeClearance = nil
		}
	} else if check.SerumPotassiumMeqL >= 3.5 && check.SerumPhosphorusMgDl >= 2.5 && check.BetaHydroxybutyrateMmolL < 1.5 {
		op.HypokalemiaCerebralEdemaAlert = false
	}

	op.UpdatedAt = now
	cp := cloneDKAOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates ketone resolution, normal pH/electrolytes, voluntary appetite, and SubQ insulin transition,
// transitioning the operation to KETONES_CLEARED_SUBQ_GLARGINE_READY.
func (s *DKAService) IssueDischargeClearance(id string, clearance domain.DKADischargeClearance) (*domain.DKAInsulinOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("dka insulin operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Ketones cleared (BHB %.2f mmol/L, pH %.2f, K+ %.1f mEq/L, Phos %.1f mg/dL); voluntary eating without vomiting; SubQ long-acting insulin transition completed.", clearance.FinalBetaHydroxybutyrateMmolL, clearance.FinalVenousPH, clearance.FinalSerumPotassiumMeqL, clearance.FinalSerumPhosphorusMgDl)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.DKAStatusKetoneResolvedGlargineCleared
	op.HypokalemiaCerebralEdemaAlert = false
	op.PatientProfile.BetaHydroxybutyrateMmolL = clearance.FinalBetaHydroxybutyrateMmolL
	op.PatientProfile.VenousPH = clearance.FinalVenousPH
	op.PatientProfile.SerumPotassiumMeqL = clearance.FinalSerumPotassiumMeqL
	op.PatientProfile.SerumPhosphorusMgDl = clearance.FinalSerumPhosphorusMgDl
	op.PatientProfile.DehydrationPercent = 0.0
	op.UpdatedAt = now

	cp := cloneDKAOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all DKA operations in the service.
func (s *DKAService) ComputeMetrics() DKAMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m DKAMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.DKAStatusAcidosisDehydratedCritical:
			m.CriticalAcidosisCount++
		case domain.DKAStatusKClPhosInsulinCRIStaged:
			m.InsulinCRIStagedCount++
		case domain.DKAStatusKetoneResolvedGlargineCleared:
			m.KetoneClearedCount++
		}
		if op.HypokalemiaCerebralEdemaAlert {
			m.HypokalemiaEdemaAlertCount++
		}
	}
	return m
}

func cloneDKAOperation(op *domain.DKAInsulinOperation) domain.DKAInsulinOperation {
	cp := *op
	if op.InsulinRegimen != nil {
		reg := *op.InsulinRegimen
		cp.InsulinRegimen = &reg
	}
	if op.SerialChecks != nil {
		cp.SerialChecks = make([]domain.DKAGlucoseElectrolyteCheck, len(op.SerialChecks))
		copy(cp.SerialChecks, op.SerialChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
