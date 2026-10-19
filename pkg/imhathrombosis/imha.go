package imhathrombosis

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// IMHAMetrics summarizes operational KPI counts across IMHA & Thromboprophylaxis triage operations.
type IMHAMetrics struct {
	TotalOperations        int `json:"total_operations"`
	CriticalHemolyticCount int `json:"critical_hemolytic_count"`
	StagedRegimenCount     int `json:"staged_regimen_count"`
	ClearedStableCount     int `json:"cleared_stable_count"`
	PTEAlertCount          int `json:"pte_alert_count"`
}

// IMHAService manages Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP),
// Saline Agglutination, Spherocytosis & Thromboprophylaxis triage operations in a thread-safe in-memory store.
type IMHAService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.IMHAThrombosisOperation
	order []string
	seq   int
}

// NewIMHAService initializes an IMHAService with 3 seeded operations (IM-OP-101, IM-OP-102, IM-OP-103).
func NewIMHAService() *IMHAService {
	s := &IMHAService{
		ops:   make(map[string]*domain.IMHAThrombosisOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildImmunosuppressiveRegimen deterministically computes an IMHAImmunosuppressiveRegimen for a patient profile.
func BuildImmunosuppressiveRegimen(profile domain.IMHAPatientProfile) *domain.IMHAImmunosuppressiveRegimen {
	prednisoloneMg := round2(2.0 * profile.WeightKg)
	mycophenolateMg := round2(10.0 * profile.WeightKg)

	var clopidogrelMg float64
	var rivaroxabanMg float64
	var vincristineMg float64
	if profile.PlateletCountKUl >= 30.0 {
		clopidogrelMg = round2(2.0 * profile.WeightKg)
		rivaroxabanMg = round2(1.0 * profile.WeightKg)
		vincristineMg = 0.0
	} else {
		clopidogrelMg = 0.0
		rivaroxabanMg = 0.0
		vincristineMg = round2(0.02 * profile.WeightKg)
	}

	var prbcMl float64
	if profile.PackedCellVolumePercent < 18.0 {
		prbcMl = round2(15.0 * profile.WeightKg)
	}

	pitfallAlert := "CRITICAL IMHA & PULMONARY THROMBOEMBOLISM PITFALL ALERT: NEVER omit dual thromboprophylaxis (Clopidogrel 2 mg/kg/day + Rivaroxaban 1 mg/kg/day) in hypercoagulable IMHA when platelets >= 30 K/µL (pulmonary thromboembolism is the #1 cause of acute mortality in canine IMHA); NEVER perform jugular venipuncture or place central jugular catheters in hypercoagulable IMHA or severely thrombocytopenic Evans Syndrome (< 30 K/µL) patients; and NEVER mistake rouleaux for true autoagglutination without a 1:4 saline dilution wash."

	rationaleParts := []string{
		fmt.Sprintf("Prednisolone %.1f mg/day (2.0 mg/kg/day) + Mycophenolate mofetil %.1f mg BID (10 mg/kg BID second-line steroid-sparing immunosuppression) for %.1f kg %s.", prednisoloneMg, mycophenolateMg, profile.WeightKg, profile.Species),
	}
	if profile.PlateletCountKUl >= 30.0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Platelets %.0f K/µL (>= 30 K/µL): initiate Clopidogrel %.1f mg/day (2.0 mg/kg/day) + Rivaroxaban %.1f mg/day (1.0 mg/kg/day) to prevent fatal pulmonary thromboembolism (PTE).", profile.PlateletCountKUl, clopidogrelMg, rivaroxabanMg))
	} else {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Evans Syndrome / Severe IMTP with platelets %.0f K/µL (< 30 K/µL): hold anticoagulants during spontaneous hemorrhage risk and administer single-dose Vincristine %.2f mg IV (0.02 mg/kg) to accelerate megakaryocyte platelet release.", profile.PlateletCountKUl, vincristineMg))
	}
	if prbcMl > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Severe anemia (PCV %.1f%% < 18%%): transfuse DEA 1.1-negative / crossmatched packed RBCs %.1f mL (15 mL/kg over 4h).", profile.PackedCellVolumePercent, prbcMl))
	}
	if strings.EqualFold(profile.Species, domain.IMHASpeciesFelineMycoplasmaSecondaryIMHA) {
		rationaleParts = append(rationaleParts, "Feline secondary IMHA: add Doxycycline 5 mg/kg PO q12h to clear Mycoplasma haemofelis epicellular hemotropic parasitemia.")
	}
	rationaleParts = append(rationaleParts, "Strictly enforce zero jugular venipuncture (peripheral saphenous/cephalic sampling only).")

	return &domain.IMHAImmunosuppressiveRegimen{
		Prednisolone2MgKgDayMg:       prednisoloneMg,
		Mycophenolate10MgKgBIDMg:     mycophenolateMg,
		Clopidogrel2MgKgDayMg:        clopidogrelMg,
		Rivaroxaban1MgKgDayMg:        rivaroxabanMg,
		Vincristine002MgKgIMTPMg:     vincristineMg,
		PackedRBCTransfusionMl:       prbcMl,
		ZeroJugularVenipunctureGuard: true,
		IMHAPitfallAlert:             pitfallAlert,
		ClinicalRationale:            strings.Join(rationaleParts, " "),
	}
}

func needsPulmonaryThromboembolismAlert(p domain.IMHAPatientProfile) bool {
	return p.PackedCellVolumePercent < 25.0 ||
		p.TotalBilirubinMgDl >= 1.5 ||
		p.PlateletCountKUl < 30.0 ||
		p.SpherocytesPerHPF >= 5 ||
		p.SalineAgglutination4To1Positive ||
		p.IntravascularHemoglobinuria
}

func (s *IMHAService) seed() {
	t101 := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)
	p101 := domain.IMHAPatientProfile{
		Species:                         domain.IMHASpeciesCaninePrimaryIMHA,
		Breed:                           "Cocker Spaniel",
		WeightKg:                        11.5,
		PackedCellVolumePercent:         12.0,
		TotalBilirubinMgDl:              6.4,
		PlateletCountKUl:                85.0,
		SpherocytesPerHPF:               18,
		SalineAgglutination4To1Positive: true,
		IntravascularHemoglobinuria:     true,
	}
	op101 := &domain.IMHAThrombosisOperation{
		ID:                            "IM-OP-101",
		IncidentID:                    "INC-IM-9701",
		PetName:                       "Penny",
		Status:                        domain.IMHAStatusHemolyticCrisisCritical,
		PulmonaryThromboembolismAlert: true,
		PatientProfile:                p101,
		ImmunosuppressiveRegimen:      BuildImmunosuppressiveRegimen(p101),
		HemolysisChecks: []domain.IMHAHemolysisSerialCheck{
			{
				Timestamp:                     t101.Add(10 * time.Minute),
				PackedCellVolumePercent:       12.0,
				ReticulocyteCountKUl:          42.0,
				TotalBilirubinMgDl:            6.4,
				SalineAgglutinationPersistent: true,
				RestingRespiratoryRateBpm:     36,
				ClinicianCallsign:             "DR-HEME-11",
				Notes:                         "Fulminant canine primary IMHA crisis; PCV 12.0%, icteric plasma (TBili 6.4 mg/dL), 18 spherocytes/HPF, 1:4 saline macro-agglutination positive, port-wine hemoglobinuria; crossmatching pRBC 172.5 mL.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 2, 20, 0, 0, time.UTC)
	p102 := domain.IMHAPatientProfile{
		Species:                         domain.IMHASpeciesCanineEvansSyndromeIMTP,
		Breed:                           "English Springer Spaniel",
		WeightKg:                        18.2,
		PackedCellVolumePercent:         19.0,
		TotalBilirubinMgDl:              2.8,
		PlateletCountKUl:                14.0,
		SpherocytesPerHPF:               9,
		SalineAgglutination4To1Positive: true,
		IntravascularHemoglobinuria:     false,
	}
	op102 := &domain.IMHAThrombosisOperation{
		ID:                            "IM-OP-102",
		IncidentID:                    "INC-IM-9702",
		PetName:                       "Scout",
		Status:                        domain.IMHAStatusImmunosuppressantAnticoagStaged,
		PulmonaryThromboembolismAlert: true,
		PatientProfile:                p102,
		ImmunosuppressiveRegimen:      BuildImmunosuppressiveRegimen(p102),
		HemolysisChecks: []domain.IMHAHemolysisSerialCheck{
			{
				Timestamp:                     t102.Add(15 * time.Minute),
				PackedCellVolumePercent:       19.0,
				ReticulocyteCountKUl:          95.0,
				TotalBilirubinMgDl:            2.8,
				SalineAgglutinationPersistent: true,
				RestingRespiratoryRateBpm:     28,
				ClinicianCallsign:             "DR-ECC-24",
				Notes:                         "Concurrent IMHA + severe IMTP (Evans Syndrome, platelets 14 K/µL); Vincristine 0.36 mg IV bolus administered; Prednisolone + Mycophenolate staged; strict zero jugular venipuncture.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	p103 := domain.IMHAPatientProfile{
		Species:                         domain.IMHASpeciesCaninePrimaryIMHA,
		Breed:                           "Miniature Schnauzer",
		WeightKg:                        14.0,
		PackedCellVolumePercent:         31.0,
		TotalBilirubinMgDl:              0.5,
		PlateletCountKUl:                210.0,
		SpherocytesPerHPF:               1,
		SalineAgglutination4To1Positive: false,
		IntravascularHemoglobinuria:     false,
	}
	clearance103 := &domain.IMHADischargeClearance{
		ClearedAt:                         t103.Add(48 * time.Hour),
		FinalPCVStable48HrPercent:         31.0,
		FinalReticulocyteCountKUl:         148.0,
		SalineAgglutinationResolved:       true,
		NoPulmonaryThromboembolismDyspnea: true,
		ThromboprophylaxisDispensed:       true,
		VetLicense:                        "VET-IM-97103",
		Summary:                           "PCV stable at 31.0% for >48h without transfusion, robust regenerative reticulocytosis (148 K/µL), 1:4 saline agglutination negative, eupneic (no PTE), discharged on Prednisolone + Mycophenolate + Clopidogrel with 2-week CBC recheck before slow taper.",
	}
	op103 := &domain.IMHAThrombosisOperation{
		ID:                            "IM-OP-103",
		IncidentID:                    "INC-IM-9703",
		PetName:                       "Maple",
		Status:                        domain.IMHAStatusPCVStableReticulocytosisCleared,
		PulmonaryThromboembolismAlert: false,
		PatientProfile:                p103,
		ImmunosuppressiveRegimen:      BuildImmunosuppressiveRegimen(p103),
		HemolysisChecks: []domain.IMHAHemolysisSerialCheck{
			{
				Timestamp:                     t103.Add(46 * time.Hour),
				PackedCellVolumePercent:       31.0,
				ReticulocyteCountKUl:          148.0,
				TotalBilirubinMgDl:            0.5,
				SalineAgglutinationPersistent: false,
				RestingRespiratoryRateBpm:     20,
				ClinicianCallsign:             "DR-HEME-09",
				Notes:                         "PCV stable at 31% over 48h, serum clear (TBili 0.5 mg/dL), strong bone marrow regeneration (148 K/µL reticulocytes), 1:4 saline wash negative.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(48 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *IMHAService) ListOperations(statusFilter string) []domain.IMHAThrombosisOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.IMHAThrombosisOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneIMHAOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *IMHAService) GetOperation(id string) (*domain.IMHAThrombosisOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("imha thrombosis operation %q not found", id)
	}
	cp := cloneIMHAOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new IMHA & Thromboprophylaxis triage operation.
func (s *IMHAService) CreateOperation(incidentID, petName string, profile domain.IMHAPatientProfile) (*domain.IMHAThrombosisOperation, error) {
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
	opID := fmt.Sprintf("IM-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsPulmonaryThromboembolismAlert(profile)
	regimen := BuildImmunosuppressiveRegimen(profile)

	op := &domain.IMHAThrombosisOperation{
		ID:                            opID,
		IncidentID:                    trimmedIncident,
		PetName:                       trimmedPet,
		Status:                        domain.IMHAStatusHemolyticCrisisCritical,
		PulmonaryThromboembolismAlert: alert,
		PatientProfile:                profile,
		ImmunosuppressiveRegimen:      regimen,
		HemolysisChecks:               []domain.IMHAHemolysisSerialCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneIMHAOperation(op)
	return &cp, nil
}

// StageImmunosuppressiveRegimen computes the Prednisolone, Mycophenolate, Clopidogrel/Rivaroxaban or Vincristine,
// and pRBC transfusion regimen and transitions the operation to PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED.
func (s *IMHAService) StageImmunosuppressiveRegimen(id string, profile domain.IMHAPatientProfile) (*domain.IMHAThrombosisOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("imha thrombosis operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.ImmunosuppressiveRegimen = BuildImmunosuppressiveRegimen(profile)
	op.PulmonaryThromboembolismAlert = needsPulmonaryThromboembolismAlert(profile)
	op.Status = domain.IMHAStatusImmunosuppressantAnticoagStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneIMHAOperation(op)
	return &cp, nil
}

// RecordHemolysisCheck appends a serial q6h PCV/reticulocyte/bilirubin/agglutination/respiratory check to an existing operation.
func (s *IMHAService) RecordHemolysisCheck(id string, check domain.IMHAHemolysisSerialCheck) (*domain.IMHAThrombosisOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("imha thrombosis operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.HemolysisChecks = append(op.HemolysisChecks, check)
	op.PatientProfile.PackedCellVolumePercent = check.PackedCellVolumePercent
	op.PatientProfile.TotalBilirubinMgDl = check.TotalBilirubinMgDl
	op.PatientProfile.SalineAgglutination4To1Positive = check.SalineAgglutinationPersistent
	op.ImmunosuppressiveRegimen = BuildImmunosuppressiveRegimen(op.PatientProfile)

	if check.PackedCellVolumePercent < 18.0 || check.RestingRespiratoryRateBpm > 40 || check.SalineAgglutinationPersistent {
		op.PulmonaryThromboembolismAlert = true
		if check.PackedCellVolumePercent < 15.0 || check.RestingRespiratoryRateBpm >= 48 {
			op.Status = domain.IMHAStatusHemolyticCrisisCritical
			op.DischargeClearance = nil
		}
	} else if check.PackedCellVolumePercent >= 25.0 && check.ReticulocyteCountKUl >= 60.0 && check.TotalBilirubinMgDl < 1.5 && !check.SalineAgglutinationPersistent && check.RestingRespiratoryRateBpm <= 32 {
		op.PulmonaryThromboembolismAlert = false
	}

	op.UpdatedAt = now
	cp := cloneIMHAOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates 48h PCV stability (>= 25%), regenerative reticulocytosis (>= 60 K/µL),
// resolved saline autoagglutination, absence of PTE dyspnea, and dispensed thromboprophylaxis,
// transitioning the operation to PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE.
func (s *IMHAService) IssueDischargeClearance(id string, clearance domain.IMHADischargeClearance) (*domain.IMHAThrombosisOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("imha thrombosis operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("48h stable PCV %.1f%% (>= 25%%) with regenerative reticulocytosis %.0f K/µL (>= 60 K/µL); 1:4 saline agglutination resolved; no PTE dyspnea; outpatient thromboprophylaxis & slow taper protocol dispensed.", clearance.FinalPCVStable48HrPercent, clearance.FinalReticulocyteCountKUl)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.IMHAStatusPCVStableReticulocytosisCleared
	op.PulmonaryThromboembolismAlert = false
	op.PatientProfile.PackedCellVolumePercent = clearance.FinalPCVStable48HrPercent
	op.PatientProfile.SalineAgglutination4To1Positive = false
	op.PatientProfile.IntravascularHemoglobinuria = false
	op.UpdatedAt = now

	cp := cloneIMHAOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all IMHA & Thromboprophylaxis operations in the service.
func (s *IMHAService) ComputeMetrics() IMHAMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m IMHAMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.IMHAStatusHemolyticCrisisCritical:
			m.CriticalHemolyticCount++
		case domain.IMHAStatusImmunosuppressantAnticoagStaged:
			m.StagedRegimenCount++
		case domain.IMHAStatusPCVStableReticulocytosisCleared:
			m.ClearedStableCount++
		}
		if op.PulmonaryThromboembolismAlert {
			m.PTEAlertCount++
		}
	}
	return m
}

func cloneIMHAOperation(op *domain.IMHAThrombosisOperation) domain.IMHAThrombosisOperation {
	cp := *op
	if op.ImmunosuppressiveRegimen != nil {
		reg := *op.ImmunosuppressiveRegimen
		cp.ImmunosuppressiveRegimen = &reg
	}
	if op.HemolysisChecks != nil {
		cp.HemolysisChecks = make([]domain.IMHAHemolysisSerialCheck, len(op.HemolysisChecks))
		copy(cp.HemolysisChecks, op.HemolysisChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
