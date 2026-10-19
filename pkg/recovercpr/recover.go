package recovercpr

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// RecoverCPRMetrics summarizes operational KPI counts across RECOVER CPR triage operations.
type RecoverCPRMetrics struct {
	TotalOperations                    int `json:"total_operations"`
	ArrestInProgressCriticalCount      int `json:"arrest_in_progress_critical_count"`
	EpiDefibCycleStagedCount           int `json:"epi_defib_cycle_staged_count"`
	ROSCPostArrestClearedCount         int `json:"rosc_post_arrest_cleared_count"`
	HyperventilationHighDoseAlertCount int `json:"hyperventilation_high_dose_alert_count"`
}

// RecoverService manages Cardiopulmonary Resuscitation (RECOVER Guidelines),
// Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation triage operations in a thread-safe in-memory store.
type RecoverService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.RecoverCPROperation
	order []string
	seq   int
}

// NewRecoverService initializes a RecoverService with 3 seeded operations (CPR-OP-101, CPR-OP-102, CPR-OP-103).
func NewRecoverService() *RecoverService {
	s := &RecoverService{
		ops:   make(map[string]*domain.RecoverCPROperation),
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

// ComputeHandPlacementTechnique returns the RECOVER thoracic conformation-specific chest compression technique.
func ComputeHandPlacementTechnique(species string) string {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case domain.CPRSpeciesCanineRoundChestCardiac:
		return "Widest Part of Thorax Lateral Recumbency (Thoracic Pump)"
	case domain.CPRSpeciesCanineKeelChestThoracic:
		return "Directly Over Heart (4th-6th ICS) Lateral Recumbency"
	case domain.CPRSpeciesCanineBarrelChestSupine:
		return "Sternal Supine Compression Over Sternum"
	case domain.CPRSpeciesFelineCircumferentialCardiac:
		return "One-Hand Circumferential Cardiac Compression (4th-6th ICS)"
	default:
		return "Directly Over Heart (4th-6th ICS) Lateral Recumbency"
	}
}

// BuildALSProtocolPlan deterministically computes a RecoverALSProtocolPlan for a patient profile.
func BuildALSProtocolPlan(profile domain.CPRPatientProfile) *domain.RecoverALSProtocolPlan {
	handTechnique := ComputeHandPlacementTechnique(profile.Species)
	lowDoseEpiMg := round2(0.01 * profile.WeightKg)
	atropineMg := round2(0.04 * profile.WeightKg)

	var defibJoules float64
	var amiodaroneMg float64
	if strings.EqualFold(profile.ArrestRhythm, domain.CPRRhythmVFibPulselessVTShockable) {
		if profile.CompressionCyclesCompleted >= 2 {
			defibJoules = round1(4.0 * profile.WeightKg)
		} else {
			defibJoules = round1(2.0 * profile.WeightKg)
		}
		amiodaroneMg = round1(5.0 * profile.WeightKg)
	}

	pitfallAlert := "CRITICAL RECOVER 2012/2024 BLS/ALS PITFALL ALERT: NEVER hyperventilate arrest patients > 10 breaths/min (hyperventilation increases intrathoracic pressure, abolishes venous return/coronary perfusion, and causes severe hypocapnic cerebral vasoconstriction); NEVER administer Epinephrine every 2-minute cycle — administer Low-Dose Epinephrine (0.01 mg/kg IV/IO) strictly EVERY OTHER 2-minute cycle (q3–5 min); NEVER use routine High-Dose Epinephrine (0.1 mg/kg), which decreases survival to hospital discharge per RECOVER 2012/2024 guidelines; and NEVER interrupt chest compressions > 10 seconds during 2-minute rhythm checks."

	rationaleParts := []string{
		fmt.Sprintf("BLS Conformation-Targeted Compressions: %s at 110 compressions/min (1/3-1/2 thoracic width, full recoil, 2-min uninterrupted cycles with compressor rotation) + Endotracheal IPPV strictly 10 breaths/min (1s inspiratory time, 10 mL/kg) targeting EtCO2 > 15 mmHg (current EtCO2 %.1f mmHg).", handTechnique, profile.EndTidalCO2MmHg),
		fmt.Sprintf("ALS Vasopressor & Vagolytic Protocol: Low-Dose Epinephrine 0.01 mg/kg IV/IO (%.2f mg EVERY OTHER 2-min cycle q3–5 min) + Atropine 0.04 mg/kg IV/IO (%.2f mg).", lowDoseEpiMg, atropineMg),
	}
	if defibJoules > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Shockable Arrest Rhythm (%s, cycle %d): Biphasic Defibrillation %.1f J (%.0f J/kg) with immediate resumption of compressions + Amiodarone 5.0 mg/kg IV/IO (%.1f mg) for refractory VFib/pulseless VT.", profile.ArrestRhythm, profile.CompressionCyclesCompleted, defibJoules, defibJoules/profile.WeightKg, amiodaroneMg))
	}
	if profile.ReversibleOpioidAlpha2Present {
		rationaleParts = append(rationaleParts, "Reversible Anesthetic/Sedative Arrest: Administer immediate Naloxone (0.04 mg/kg IV), Atipamezole (100 mcg/kg IV), and/or Flumazenil (0.01 mg/kg IV).")
	}

	return &domain.RecoverALSProtocolPlan{
		HandPlacementTechnique:                      handTechnique,
		TargetCompressionRateBpm:                    110,
		TargetVentilationRateBpm:                    10,
		LowDoseEpinephrine001MgKgMg:                 lowDoseEpiMg,
		Atropine004MgKgDoseMg:                       atropineMg,
		BiphasicDefibJoules:                         defibJoules,
		Amiodarone5MgKgDoseMg:                       amiodaroneMg,
		NaloxoneAtipamezoleReversalIndicated:        profile.ReversibleOpioidAlpha2Present,
		ZeroHyperventilationEveryOtherCycleEpiGuard: true,
		RecoverPitfallAlert:                         pitfallAlert,
		ClinicalRationale:                           strings.Join(rationaleParts, " "),
	}
}

func needsHyperventilationHighDoseEpiAlert(p domain.CPRPatientProfile) bool {
	return p.EndTidalCO2MmHg < 25.0 ||
		strings.EqualFold(p.ArrestRhythm, domain.CPRRhythmVFibPulselessVTShockable) ||
		strings.EqualFold(p.ArrestRhythm, domain.CPRRhythmAsystolePEANonShockable) ||
		p.CompressionCyclesCompleted >= 1
}

func (s *RecoverService) seed() {
	t101 := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	p101 := domain.CPRPatientProfile{
		Species:                       domain.CPRSpeciesCanineKeelChestThoracic,
		Breed:                         "Greyhound",
		WeightKg:                      30.0,
		ArrestRhythm:                  domain.CPRRhythmVFibPulselessVTShockable,
		EndTidalCO2MmHg:               11.0,
		CompressionCyclesCompleted:    1,
		ReversibleOpioidAlpha2Present: true,
	}
	op101 := &domain.RecoverCPROperation{
		ID:                               "CPR-OP-101",
		IncidentID:                       "INC-CPR-9941",
		PetName:                          "Duke",
		Status:                           domain.CPRStatusArrestInProgressCritical,
		HyperventilationHighDoseEpiAlert: true,
		PatientProfile:                   p101,
		ALSProtocolPlan:                  BuildALSProtocolPlan(p101),
		CycleChecks: []domain.CPRTwoMinuteCycleCheck{
			{
				Timestamp:            t101.Add(2 * time.Minute),
				CycleNumber:          1,
				EndTidalCO2MmHg:      11.0,
				RhythmAt2MinPause:    domain.CPRRhythmVFibPulselessVTShockable,
				CompressorRotated:    true,
				PauseDurationSeconds: 6,
				ClinicianCallsign:    "DR-RECOVER-01",
				Notes:                "Cycle 1 keel-chest compressions over 4th-6th ICS; EtCO2 11.0 mmHg (< 15 mmHg indicates deeper compressions needed); coarse VFib on 6s rhythm check; Naloxone 1.2 mg IV administered.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(2 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 4, 15, 0, 0, time.UTC)
	p102 := domain.CPRPatientProfile{
		Species:                       domain.CPRSpeciesCanineBarrelChestSupine,
		Breed:                         "English Bulldog",
		WeightKg:                      22.0,
		ArrestRhythm:                  domain.CPRRhythmAsystolePEANonShockable,
		EndTidalCO2MmHg:               18.0,
		CompressionCyclesCompleted:    2,
		ReversibleOpioidAlpha2Present: false,
	}
	op102 := &domain.RecoverCPROperation{
		ID:                               "CPR-OP-102",
		IncidentID:                       "INC-CPR-9942",
		PetName:                          "Gizmo",
		Status:                           domain.CPRStatusEpiDefibCycleStaged,
		HyperventilationHighDoseEpiAlert: true,
		PatientProfile:                   p102,
		ALSProtocolPlan:                  BuildALSProtocolPlan(p102),
		CycleChecks: []domain.CPRTwoMinuteCycleCheck{
			{
				Timestamp:            t102.Add(4 * time.Minute),
				CycleNumber:          2,
				EndTidalCO2MmHg:      18.0,
				RhythmAt2MinPause:    domain.CPRRhythmAsystolePEANonShockable,
				CompressorRotated:    true,
				PauseDurationSeconds: 5,
				ClinicianCallsign:    "DR-RECOVER-14",
				Notes:                "Supine sternal compressions in English Bulldog achieving EtCO2 18.0 mmHg (> 15 mmHg target); low-dose Epinephrine 0.22 mg IV + Atropine 0.88 mg IV staged every other cycle; IPPV locked at 10 breaths/min.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(4 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 19, 2, 30, 0, 0, time.UTC)
	p103 := domain.CPRPatientProfile{
		Species:                       domain.CPRSpeciesFelineCircumferentialCardiac,
		Breed:                         "Domestic Shorthair",
		WeightKg:                      4.2,
		ArrestRhythm:                  domain.CPRRhythmVagalBradycardicArrest,
		EndTidalCO2MmHg:               38.0,
		CompressionCyclesCompleted:    2,
		ReversibleOpioidAlpha2Present: true,
	}
	clearance103 := &domain.ROSCDischargeClearance{
		ClearedAt:                     t103.Add(45 * time.Minute),
		SustainedROSCMinutes:          35,
		FinalEtCO2MmHg:                38.0,
		FinalMAPMmHg:                  94,
		SpO2Titrated94To98NoHyperoxia: true,
		PermissiveHypothermiaOrNormothermiaMaintained: true,
		VetLicense: "VET-RECOVER-99403",
		Summary:    "Abrupt EtCO2 jump from 19.0 to 38.0 mmHg after Cycle 2 one-hand circumferential feline compressions + Atipamezole/Atropine; sustained ROSC 35 min, MAP 94 mmHg, SpO2 96% without hyperoxia.",
	}
	op103 := &domain.RecoverCPROperation{
		ID:                               "CPR-OP-103",
		IncidentID:                       "INC-CPR-9943",
		PetName:                          "Pixel",
		Status:                           domain.CPRStatusROSCPostArrestCleared,
		HyperventilationHighDoseEpiAlert: false,
		PatientProfile:                   p103,
		ALSProtocolPlan:                  BuildALSProtocolPlan(p103),
		CycleChecks: []domain.CPRTwoMinuteCycleCheck{
			{
				Timestamp:            t103.Add(4 * time.Minute),
				CycleNumber:          2,
				EndTidalCO2MmHg:      38.0,
				RhythmAt2MinPause:    "ROSC_SINUS_RHYTHM_STRONG_FEMORAL_PULSE",
				CompressorRotated:    true,
				PauseDurationSeconds: 5,
				ClinicianCallsign:    "DR-RECOVER-09",
				Notes:                "Abrupt EtCO2 surge to 38.0 mmHg confirming Return of Spontaneous Circulation (ROSC) after Atipamezole reversal and one-hand circumferential cardiac compressions.",
			},
		},
		ROSCClearance: clearance103,
		CreatedAt:     t103,
		UpdatedAt:     t103.Add(45 * time.Minute),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *RecoverService) ListOperations(statusFilter string) []domain.RecoverCPROperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.RecoverCPROperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneRecoverOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *RecoverService) GetOperation(id string) (*domain.RecoverCPROperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("recover cpr operation %q not found", id)
	}
	cp := cloneRecoverOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new RECOVER CPR triage operation.
func (s *RecoverService) CreateOperation(incidentID, petName string, profile domain.CPRPatientProfile) (*domain.RecoverCPROperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.ArrestRhythm = strings.ToUpper(strings.TrimSpace(profile.ArrestRhythm))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("CPR-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsHyperventilationHighDoseEpiAlert(profile)
	plan := BuildALSProtocolPlan(profile)

	op := &domain.RecoverCPROperation{
		ID:                               opID,
		IncidentID:                       trimmedIncident,
		PetName:                          trimmedPet,
		Status:                           domain.CPRStatusArrestInProgressCritical,
		HyperventilationHighDoseEpiAlert: alert,
		PatientProfile:                   profile,
		ALSProtocolPlan:                  plan,
		CycleChecks:                      []domain.CPRTwoMinuteCycleCheck{},
		CreatedAt:                        now,
		UpdatedAt:                        now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneRecoverOperation(op)
	return &cp, nil
}

// StageALSProtocolPlan computes conformation-specific hand placement, 110 bpm compression rate,
// strict 10 breaths/min ventilation rate, low-dose Epinephrine (0.01 mg/kg), Atropine (0.04 mg/kg),
// biphasic defibrillation energy (2.0 J/kg or 4.0 J/kg if cycles >= 2), and Amiodarone (5.0 mg/kg),
// transitioning the operation to RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED.
func (s *RecoverService) StageALSProtocolPlan(id string, profile domain.CPRPatientProfile) (*domain.RecoverCPROperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.ArrestRhythm = strings.ToUpper(strings.TrimSpace(profile.ArrestRhythm))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("recover cpr operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.ALSProtocolPlan = BuildALSProtocolPlan(profile)
	op.HyperventilationHighDoseEpiAlert = needsHyperventilationHighDoseEpiAlert(profile)
	op.Status = domain.CPRStatusEpiDefibCycleStaged
	op.ROSCClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneRecoverOperation(op)
	return &cp, nil
}

// RecordCycleCheck appends a 2-minute BLS/ALS compression cycle check (enforcing pause_duration_seconds <= 10 and compressor rotation)
// and updates patient capnography EtCO2 and completed cycles.
func (s *RecoverService) RecordCycleCheck(id string, check domain.CPRTwoMinuteCycleCheck) (*domain.RecoverCPROperation, error) {
	check.RhythmAt2MinPause = strings.TrimSpace(check.RhythmAt2MinPause)
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("recover cpr operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.CycleChecks = append(op.CycleChecks, check)
	op.PatientProfile.EndTidalCO2MmHg = round1(check.EndTidalCO2MmHg)
	if check.CycleNumber > op.PatientProfile.CompressionCyclesCompleted {
		op.PatientProfile.CompressionCyclesCompleted = check.CycleNumber
	}
	if domain.IsValidCPRArrestRhythm(check.RhythmAt2MinPause) {
		op.PatientProfile.ArrestRhythm = strings.ToUpper(strings.TrimSpace(check.RhythmAt2MinPause))
	}
	op.ALSProtocolPlan = BuildALSProtocolPlan(op.PatientProfile)

	if check.EndTidalCO2MmHg < 15.0 {
		op.HyperventilationHighDoseEpiAlert = true
		op.Status = domain.CPRStatusArrestInProgressCritical
		op.ROSCClearance = nil
	} else if check.EndTidalCO2MmHg >= 35.0 {
		op.HyperventilationHighDoseEpiAlert = false
	}

	op.UpdatedAt = now
	cp := cloneRecoverOperation(op)
	return &cp, nil
}

// IssueROSCClearance validates sustained ROSC >= 20 min, normocapnia (EtCO2 35-45 mmHg),
// normotensive MAP (80-120 mmHg), SpO2 94-98% without hyperoxia, and post-arrest temperature management,
// transitioning the operation to ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE.
func (s *RecoverService) IssueROSCClearance(id string, clearance domain.ROSCDischargeClearance) (*domain.RecoverCPROperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("recover cpr operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Sustained ROSC (%d min) achieved with normocapnic EtCO2 %.1f mmHg, MAP %d mmHg, SpO2 titrated 94–98%% without hyperoxia, and PCA neuroprotective temperature control.", clearance.SustainedROSCMinutes, clearance.FinalEtCO2MmHg, clearance.FinalMAPMmHg)
	}

	op.ROSCClearance = &clearance
	op.Status = domain.CPRStatusROSCPostArrestCleared
	op.HyperventilationHighDoseEpiAlert = false
	op.PatientProfile.EndTidalCO2MmHg = round1(clearance.FinalEtCO2MmHg)
	op.ALSProtocolPlan = BuildALSProtocolPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneRecoverOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all RECOVER CPR operations in the service.
func (s *RecoverService) ComputeMetrics() RecoverCPRMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m RecoverCPRMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.CPRStatusArrestInProgressCritical:
			m.ArrestInProgressCriticalCount++
		case domain.CPRStatusEpiDefibCycleStaged:
			m.EpiDefibCycleStagedCount++
		case domain.CPRStatusROSCPostArrestCleared:
			m.ROSCPostArrestClearedCount++
		}
		if op.HyperventilationHighDoseEpiAlert {
			m.HyperventilationHighDoseAlertCount++
		}
	}
	return m
}

func cloneRecoverOperation(op *domain.RecoverCPROperation) domain.RecoverCPROperation {
	cp := *op
	if op.ALSProtocolPlan != nil {
		plan := *op.ALSProtocolPlan
		cp.ALSProtocolPlan = &plan
	}
	if op.CycleChecks != nil {
		cp.CycleChecks = make([]domain.CPRTwoMinuteCycleCheck, len(op.CycleChecks))
		copy(cp.CycleChecks, op.CycleChecks)
	}
	if op.ROSCClearance != nil {
		clr := *op.ROSCClearance
		cp.ROSCClearance = &clr
	}
	return cp
}
