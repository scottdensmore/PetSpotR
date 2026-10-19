package cardiogenicedema

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// CardiogenicMetrics summarizes real-time KPI counts across all cardiogenic pulmonary edema operations.
type CardiogenicMetrics struct {
	TotalOperations      int `json:"total_operations"`
	FulminantCritical    int `json:"fulminant_critical"`
	FurosemideO2Staged   int `json:"furosemide_o2_staged"`
	EupneicCleared       int `json:"eupneic_cleared"`
	ActiveFluidGuardLock int `json:"active_fluid_guard_lock"`
}

// CardiogenicService manages Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture,
// Furosemide/Pimobendan & Oxygen Cage operations in a thread-safe store.
type CardiogenicService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.CardiogenicEdemaOperation
	order []string
	seq   int
}

// NewCardiogenicService creates a new CardiogenicService seeded with CE-OP-101, CE-OP-102, and CE-OP-103.
func NewCardiogenicService() *CardiogenicService {
	svc := &CardiogenicService{
		ops:   make(map[string]*domain.CardiogenicEdemaOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	svc.seed()
	return svc
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildDiureticInotropeRegimen computes the species-specific loop diuretic, pimobendan inodilator,
// butorphanol anxiolysis, oxygen cage, and zero-IV-crystalloid fluid guard protocol.
func BuildDiureticInotropeRegimen(profile domain.CardiogenicPatientProfile) *domain.DiureticInotropeRegimen {
	isFeline := strings.TrimSpace(profile.Species) == domain.CardiogenicSpeciesFelineHCM

	// Canine ACVIM Stage C/D acute CHF: Furosemide 2.0-4.0 mg/kg IV initial bolus (3.0 mg/kg standard; 4.0 mg/kg if chordae rupture / RR >= 80)
	// Feline HCM acute CHF: Conservative Furosemide 1.0-2.0 mg/kg IV initial bolus (1.5 mg/kg standard; 2.0 mg/kg if severe) to prevent acute kidney injury & dehydration
	var bolusMgKg float64
	if isFeline {
		bolusMgKg = 1.5
		if profile.RespiratoryRateBpm >= 70 || profile.TfastBLinesPerSite >= 6 {
			bolusMgKg = 2.0
		}
	} else {
		bolusMgKg = 3.0
		if profile.MurmurGrade == domain.MurmurGradeVIChordaeRupture || profile.RespiratoryRateBpm >= 80 {
			bolusMgKg = 4.0
		}
	}

	furosemideBolusMg := round2(profile.WeightKg * bolusMgKg)
	furosemideCRIMgHr := round2(profile.WeightKg * 0.66)
	pimobendanDoseMg := round2(profile.WeightKg * 0.25)
	butorphanolDoseMg := round2(profile.WeightKg * 0.20)

	fio2 := 50
	if profile.RespiratoryRateBpm >= 80 || profile.TfastBLinesPerSite >= 7 {
		fio2 = 60
	}

	pitfallAlert := "CRITICAL CARDIOGENIC EDEMA PITFALL ALERT: STRICT ZERO IV CRYSTALLOID FLUID BOLUS CONTRAINDICATION — Administering isotonic IV crystalloid fluid boluses (LRS / 0.9% NaCl) in acute left-sided congestive heart failure or mitral chordae tendineae rupture precipitates fatal fulminant alveolar flooding. Furthermore, NEVER force dyspneic patients into dorsal or lateral recumbency for thoracic radiographs prior to oxygen cage (FiO2 40–60%), IV furosemide diuresis, and butorphanol anxiolytic stabilization; use minimal-restraint standing/sternal TFAST ultrasound (B-lines / lung rockets & La:Ao ratio) first."

	var rationaleParts []string
	if isFeline {
		rationaleParts = append(rationaleParts, fmt.Sprintf(
			"Feline HCM Cardiogenic Edema Regimen (%.1f kg): Conservative Furosemide %.1f mg/kg IV bolus (%.2f mg) + %.2f mg/hr CRI (0.66 mg/kg/hr) to clear pulmonary/pleural edema while preventing feline prerenal azotemia & acute kidney injury",
			profile.WeightKg, bolusMgKg, furosemideBolusMg, furosemideCRIMgHr,
		))
	} else {
		rationaleParts = append(rationaleParts, fmt.Sprintf(
			"Canine MMVD/DCM Acute CHF Regimen (%.1f kg, %s): Furosemide %.1f mg/kg IV bolus (%.2f mg) + %.2f mg/hr Constant Rate Infusion (0.66 mg/kg/hr for superior continuous natriuresis)",
			profile.WeightKg, profile.MurmurGrade, bolusMgKg, furosemideBolusMg, furosemideCRIMgHr,
		))
	}
	rationaleParts = append(rationaleParts, fmt.Sprintf(
		"Pimobendan 0.25 mg/kg PO/IV inodilator (%.2f mg) for calcium sensitization & afterload reduction (La:Ao %.2f) + Butorphanol 0.2 mg/kg IV/IM (%.2f mg) anxiolytic respiratory sedation + Temperature-Controlled Oxygen Cage (%d%% FiO2)",
		pimobendanDoseMg, profile.LaAoRatio, butorphanolDoseMg, fio2,
	))
	if profile.PleuralEffusionPresent {
		rationaleParts = append(rationaleParts, "Immediate ultrasound-guided therapeutic thoracocentesis indicated for concurrent pleural effusion")
	}

	return &domain.DiureticInotropeRegimen{
		FurosemideInitialBolusMg:    furosemideBolusMg,
		FurosemideCRI066MgKgHrMgHr:  furosemideCRIMgHr,
		Pimobendan025MgKgDoseMg:     pimobendanDoseMg,
		ButorphanolAnxiolyticDoseMg: butorphanolDoseMg,
		OxygenCageFiO2Percent:       fio2,
		ThoracocentesisIndicated:    profile.PleuralEffusionPresent,
		ZeroIVCrystalloidFluidGuard: true,
		CardiogenicPitfallAlert:     pitfallAlert,
		ClinicalRationale:           strings.Join(rationaleParts, " | "),
	}
}

func (s *CardiogenicService) seed() {
	baseTime := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)

	profile101 := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesCanineMMVD,
		WeightKg:               9.2,
		RespiratoryRateBpm:     88,
		HeartRateBpm:           190,
		MurmurGrade:            domain.MurmurGradeVIChordaeRupture,
		TfastBLinesPerSite:     8,
		LaAoRatio:              2.45,
		PleuralEffusionPresent: false,
	}
	op101 := &domain.CardiogenicEdemaOperation{
		ID:                                 "CE-OP-101",
		IncidentID:                         "INC-CE-9101",
		PetName:                            "Oliver",
		Status:                             domain.CardiogenicStatusFulminantEdemaCritical,
		FluidOverloadContraindicationAlert: true,
		PatientProfile:                     profile101,
		DiureticRegimen:                    BuildDiureticInotropeRegimen(profile101),
		DiuresisChecks: []domain.DiuresisTelemetryCheck{
			{
				Timestamp:                 baseTime.Add(10 * time.Minute),
				RestingRespiratoryRateBpm: 84,
				UrineOutputMlKgHr:         1.8,
				TfastBLinesRemaining:      7,
				SystolicBloodPressureMmHg: 105,
				CreatinineMgDl:            1.2,
				PotassiumMeqL:             4.1,
				ClinicianCallsign:         "DR-CARDIO-11",
				Notes:                     "Cavalier King Charles Spaniel in fulminant alveolar pulmonary edema secondary to mitral valve chordae tendineae rupture; placed in 60% FiO2 oxygen cage, IV crystalloids locked out.",
			},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	profile102 := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesFelineHCM,
		WeightKg:               5.6,
		RespiratoryRateBpm:     64,
		HeartRateBpm:           210,
		MurmurGrade:            domain.MurmurGradeIVGallopRhythm,
		TfastBLinesPerSite:     5,
		LaAoRatio:              2.10,
		PleuralEffusionPresent: true,
	}
	op102 := &domain.CardiogenicEdemaOperation{
		ID:                                 "CE-OP-102",
		IncidentID:                         "INC-CE-9102",
		PetName:                            "Cleo",
		Status:                             domain.CardiogenicStatusFurosemideO2CageStaged,
		FluidOverloadContraindicationAlert: true,
		PatientProfile:                     profile102,
		DiureticRegimen:                    BuildDiureticInotropeRegimen(profile102),
		DiuresisChecks: []domain.DiuresisTelemetryCheck{
			{
				Timestamp:                 baseTime.Add(25 * time.Minute),
				RestingRespiratoryRateBpm: 46,
				UrineOutputMlKgHr:         3.2,
				TfastBLinesRemaining:      3,
				SystolicBloodPressureMmHg: 118,
				CreatinineMgDl:            1.5,
				PotassiumMeqL:             3.8,
				ClinicianCallsign:         "DR-CARDIO-14",
				Notes:                     "Maine Coon with HCM gallop rhythm and concurrent pleural effusion; conservative feline furosemide 1.5 mg/kg IV + thoracocentesis + 50% O2 cage staged.",
			},
		},
		CreatedAt: baseTime.Add(15 * time.Minute),
		UpdatedAt: baseTime.Add(25 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	profile103 := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesCanineMMVD,
		WeightKg:               7.8,
		RespiratoryRateBpm:     24,
		HeartRateBpm:           120,
		MurmurGrade:            domain.MurmurGradeVSystolicApical,
		TfastBLinesPerSite:     0,
		LaAoRatio:              1.75,
		PleuralEffusionPresent: false,
	}
	op103 := &domain.CardiogenicEdemaOperation{
		ID:                                 "CE-OP-103",
		IncidentID:                         "INC-CE-9103",
		PetName:                            "Teddy",
		Status:                             domain.CardiogenicStatusEupneicBLineResolved,
		FluidOverloadContraindicationAlert: false,
		PatientProfile:                     profile103,
		DiureticRegimen:                    BuildDiureticInotropeRegimen(profile103),
		DiuresisChecks: []domain.DiuresisTelemetryCheck{
			{
				Timestamp:                 baseTime.Add(40 * time.Minute),
				RestingRespiratoryRateBpm: 24,
				UrineOutputMlKgHr:         3.9,
				TfastBLinesRemaining:      0,
				SystolicBloodPressureMmHg: 124,
				CreatinineMgDl:            1.3,
				PotassiumMeqL:             4.0,
				ClinicianCallsign:         "DR-CARDIO-09",
				Notes:                     "Sustained diuresis achieved; TFAST B-lines resolved across all 4 quadrants and patient eupneic on room air.",
			},
		},
		DischargeClearance: &domain.CardiogenicDischargeClearance{
			ClearedAt:                 baseTime.Add(55 * time.Minute),
			FinalSleepingRespRateBpm:  22,
			FinalTfastBLinesCount:     0,
			RoomAirSpO2Percent:        98.0,
			RenalAzotemiaStable:       true,
			HomeSRRMonitoringEducated: true,
			VetLicense:                "VET-CA-88412",
			Summary:                   "Eupneic on room air (SRR 22 bpm, SpO2 98%), TFAST B-lines cleared (0/site), renal panel stable on oral furosemide + pimobendan + benazepril + spironolactone; owner educated on daily Sleeping Respiratory Rate (<30 bpm) log.",
		},
		CreatedAt: baseTime.Add(30 * time.Minute),
		UpdatedAt: baseTime.Add(55 * time.Minute),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by status.
func (s *CardiogenicService) ListOperations(statusFilter string) []domain.CardiogenicEdemaOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filter := strings.TrimSpace(statusFilter)
	out := make([]domain.CardiogenicEdemaOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if filter != "" && !strings.EqualFold(string(op.Status), filter) {
			continue
		}
		out = append(out, cloneOp(op))
	}
	return out
}

// GetOperation returns a deep copy of a single operation by ID.
func (s *CardiogenicService) GetOperation(id string) (*domain.CardiogenicEdemaOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("cardiogenic edema operation %q not found", trimmed)
	}
	cp := cloneOp(op)
	return &cp, nil
}

// CreateOperation validates and creates a new cardiogenic pulmonary edema operation in FULMINANT_PULMONARY_EDEMA_CRITICAL state.
func (s *CardiogenicService) CreateOperation(incidentID, petName string, profile domain.CardiogenicPatientProfile) (*domain.CardiogenicEdemaOperation, error) {
	inc := strings.TrimSpace(incidentID)
	if inc == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	name := strings.TrimSpace(petName)
	if name == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	id := fmt.Sprintf("CE-OP-%d", s.seq)
	now := time.Now().UTC()

	op := &domain.CardiogenicEdemaOperation{
		ID:                                 id,
		IncidentID:                         inc,
		PetName:                            name,
		Status:                             domain.CardiogenicStatusFulminantEdemaCritical,
		FluidOverloadContraindicationAlert: true,
		PatientProfile:                     profile,
		DiureticRegimen:                    BuildDiureticInotropeRegimen(profile),
		DiuresisChecks:                     []domain.DiuresisTelemetryCheck{},
		CreatedAt:                          now,
		UpdatedAt:                          now,
	}

	s.ops[id] = op
	s.order = append(s.order, id)
	cp := cloneOp(op)
	return &cp, nil
}

// StageDiureticRegimen computes the species-specific Furosemide bolus/CRI, Pimobendan, Butorphanol, and Oxygen Cage protocol
// and transitions the operation to FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED.
func (s *CardiogenicService) StageDiureticRegimen(id string, profile domain.CardiogenicPatientProfile) (*domain.CardiogenicEdemaOperation, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("cardiogenic edema operation %q not found", trimmed)
	}

	op.PatientProfile = profile
	op.DiureticRegimen = BuildDiureticInotropeRegimen(profile)
	op.FluidOverloadContraindicationAlert = true
	op.Status = domain.CardiogenicStatusFurosemideO2CageStaged
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOp(op)
	return &cp, nil
}

// RecordDiuresisCheck validates and appends a serial DiuresisTelemetryCheck to an operation.
func (s *CardiogenicService) RecordDiuresisCheck(id string, check domain.DiuresisTelemetryCheck) (*domain.CardiogenicEdemaOperation, error) {
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("cardiogenic edema operation %q not found", trimmed)
	}

	if check.Timestamp.IsZero() {
		check.Timestamp = time.Now().UTC()
	}
	op.DiuresisChecks = append(op.DiuresisChecks, check)
	op.PatientProfile.RespiratoryRateBpm = check.RestingRespiratoryRateBpm
	op.PatientProfile.TfastBLinesPerSite = check.TfastBLinesRemaining
	if op.Status == domain.CardiogenicStatusFulminantEdemaCritical {
		op.Status = domain.CardiogenicStatusFurosemideO2CageStaged
	}
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOp(op)
	return &cp, nil
}

// IssueDischargeClearance validates eupneic & B-line resolution discharge criteria and transitions
// the operation to EUPNEIC_BLINES_RESOLVED_CLEARED with FluidOverloadContraindicationAlert = false.
func (s *CardiogenicService) IssueDischargeClearance(id string, clearance domain.CardiogenicDischargeClearance) (*domain.CardiogenicEdemaOperation, error) {
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("cardiogenic edema operation %q not found", trimmed)
	}

	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = time.Now().UTC()
	}
	op.DischargeClearance = &clearance
	op.PatientProfile.RespiratoryRateBpm = clearance.FinalSleepingRespRateBpm
	op.PatientProfile.TfastBLinesPerSite = clearance.FinalTfastBLinesCount
	op.FluidOverloadContraindicationAlert = false
	op.Status = domain.CardiogenicStatusEupneicBLineResolved
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOp(op)
	return &cp, nil
}

// ComputeMetrics calculates real-time summary counts across all operations.
func (s *CardiogenicService) ComputeMetrics() CardiogenicMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m CardiogenicMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.CardiogenicStatusFulminantEdemaCritical:
			m.FulminantCritical++
		case domain.CardiogenicStatusFurosemideO2CageStaged:
			m.FurosemideO2Staged++
		case domain.CardiogenicStatusEupneicBLineResolved:
			m.EupneicCleared++
		}
		if op.FluidOverloadContraindicationAlert {
			m.ActiveFluidGuardLock++
		}
	}
	return m
}

func cloneOp(op *domain.CardiogenicEdemaOperation) domain.CardiogenicEdemaOperation {
	cp := *op
	if op.DiureticRegimen != nil {
		reg := *op.DiureticRegimen
		cp.DiureticRegimen = &reg
	}
	if op.DiuresisChecks != nil {
		cp.DiuresisChecks = make([]domain.DiuresisTelemetryCheck, len(op.DiuresisChecks))
		copy(cp.DiuresisChecks, op.DiuresisChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
