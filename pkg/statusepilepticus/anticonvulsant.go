package statusepilepticus

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// EpilepticusMetrics summarizes operational KPI counts across status epilepticus triage operations.
type EpilepticusMetrics struct {
	TotalOperations              int `json:"total_operations"`
	ActiveIctalCriticalCount     int `json:"active_ictal_critical_count"`
	AnticonvulsantCRIStagedCount int `json:"anticonvulsant_cri_staged_count"`
	SeizureFreeClearedCount      int `json:"seizure_free_cleared_count"`
	HyperthermiaAlertCount       int `json:"hyperthermia_alert_count"`
}

// EpilepticusService manages Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor
// & Levetiracetam/Propofol CRI triage operations in a thread-safe in-memory store.
type EpilepticusService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.StatusEpilepticusOperation
	order []string
	seq   int
}

// NewEpilepticusService initializes an EpilepticusService with 3 seeded operations (SE-OP-101, SE-OP-102, SE-OP-103).
func NewEpilepticusService() *EpilepticusService {
	s := &EpilepticusService{
		ops:   make(map[string]*domain.StatusEpilepticusOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildTitrationRegimen deterministically computes an AnticonvulsantTitrationRegimen for a patient profile.
func BuildTitrationRegimen(profile domain.EpilepticusPatientProfile) *domain.AnticonvulsantTitrationRegimen {
	midazolamMg := round2(profile.WeightKg * 0.2)
	levetiracetamMg := round2(profile.WeightKg * 60.0)
	phenobarbitalMg := round2(profile.WeightKg * 16.0)

	var methocarbamolMg float64
	if profile.NeuroEtiology == domain.NeuroEtiologyMetaldehydeSnailBaitTremor ||
		profile.NeuroEtiology == domain.NeuroEtiologyPyrethroidFelinePermethrin {
		methocarbamolMg = round2(profile.WeightKg * 55.0)
	}

	var mannitolGrams float64
	if profile.NeuroEtiology == domain.NeuroEtiologyBromethalinCerebralEdema ||
		profile.ModifiedGlasgowComaScore <= 10 ||
		profile.ContinuousIctalMinutes >= 10 {
		mannitolGrams = round2(profile.WeightKg * 0.5)
	}

	propofolCRI := 0.1
	if profile.ContinuousIctalMinutes < 5 && profile.ClusterCount24Hr < 3 &&
		profile.NeuroEtiology == domain.NeuroEtiologyIdiopathicStatusEpilepticus {
		propofolCRI = 0.05
	}

	pitfallAlert := "CRITICAL NEUROPROTECTION PITFALL ALERT: NEVER place fingers, bite blocks, or objects into an ictal animal's mouth (severe bite trauma & laryngeal/vagal obstruction); enforce ZERO oral medication or water while gag reflex is obtunded (aspiration pneumonia); NEVER administer oral diazepam in cats (idiosyncratic fulminant hepatic necrosis); and STOP active external cooling at 103.0°F (39.4°C) to prevent hypothalamic overcooling and rebound hypothermia."

	rationaleParts := []string{
		fmt.Sprintf("First-line Midazolam 0.2 mg/kg IV/IN bolus (%.2f mg) + Levetiracetam (Keppra) 60 mg/kg IV loading dose (%.1f mg) + Phenobarbital 16 mg/kg IV loading regimen (%.1f mg divided in 4 mg/kg increments).", midazolamMg, levetiracetamMg, phenobarbitalMg),
	}
	if methocarbamolMg > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Methocarbamol 55 mg/kg IV slow bolus (%.1f mg) indicated for %s tremorgenic myoclonus.", methocarbamolMg, profile.NeuroEtiology))
	}
	if mannitolGrams > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Mannitol 0.5 g/kg IV over 20 min (%.2f g) for suspected intracranial hypertension / %s cerebral edema (MGCS %d).", mannitolGrams, profile.NeuroEtiology, profile.ModifiedGlasgowComaScore))
	}
	if profile.BloodGlucoseMgDl < 65.0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Hypoglycemic neuroglycopenia rescue (BG %.1f mg/dL): administer 0.5–1.0 mL/kg 50%% Dextrose diluted 1:4 IV over 5 min.", profile.BloodGlucoseMgDl))
	}
	if profile.RectalTempFahrenheit >= 103.5 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Neurogenic hyperthermia (%.1f°F): initiate evaporative cooling and cease active cooling strictly at 103.0°F.", profile.RectalTempFahrenheit))
	}
	rationaleParts = append(rationaleParts, fmt.Sprintf("Refractory status epilepticus backup: Propofol CRI at %.2f mg/kg/min with endotracheal intubation & capnography.", propofolCRI))

	return &domain.AnticonvulsantTitrationRegimen{
		MidazolamFirstLineBolusMg:       midazolamMg,
		LevetiracetamLoading60MgKgMg:    levetiracetamMg,
		PhenobarbitalLoadingDoseMg:      phenobarbitalMg,
		Methocarbamol55MgKgDoseMg:       methocarbamolMg,
		PropofolCRIRateMgKgMin:          propofolCRI,
		Mannitol05GKgCerebralEdemaGrams: mannitolGrams,
		StopCoolingThresholdTempF:       103.0,
		ZeroOralGagAspirationGuard:      true,
		NeuroprotectionPitfallAlert:     pitfallAlert,
		ClinicalRationale:               strings.Join(rationaleParts, " "),
	}
}

func (s *EpilepticusService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesCanine,
		Breed:                    "Australian Shepherd",
		WeightKg:                 24.0,
		NeuroEtiology:            domain.NeuroEtiologyIdiopathicStatusEpilepticus,
		ContinuousIctalMinutes:   14,
		ClusterCount24Hr:         5,
		RectalTempFahrenheit:     106.4,
		BloodGlucoseMgDl:         62.0,
		ModifiedGlasgowComaScore: 9,
	}
	op101 := &domain.StatusEpilepticusOperation{
		ID:                          "SE-OP-101",
		IncidentID:                  "INC-SE-9201",
		PetName:                     "Koda",
		Status:                      domain.EpilepticusStatusActiveIctalCritical,
		NeurogenicHyperthermiaAlert: true,
		PatientProfile:              p101,
		TitrationRegimen:            BuildTitrationRegimen(p101),
		NeuroChecks: []domain.NeuroIcuSerialCheck{
			{
				Timestamp:                t101.Add(5 * time.Minute),
				SeizureActivityObserved:  true,
				RectalTempFahrenheit:     106.4,
				ModifiedGlasgowComaScore: 9,
				PupillaryLightReflex:     domain.PupillaryReflexAnisocoriaCushingReflex,
				BloodGlucoseMgDl:         62.0,
				ClinicianCallsign:        "DR-NEURO-41",
				Notes:                    "Continuous generalized tonic-clonic status epilepticus (14 mins); neurogenic hyperthermia 106.4°F; intranasal/IV midazolam + levetiracetam 60 mg/kg loading initiated.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(5 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 15, 0, 0, time.UTC)
	p102 := domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesFeline,
		Breed:                    "Domestic Shorthair",
		WeightKg:                 4.2,
		NeuroEtiology:            domain.NeuroEtiologyPyrethroidFelinePermethrin,
		ContinuousIctalMinutes:   8,
		ClusterCount24Hr:         3,
		RectalTempFahrenheit:     104.6,
		BloodGlucoseMgDl:         98.0,
		ModifiedGlasgowComaScore: 13,
	}
	op102 := &domain.StatusEpilepticusOperation{
		ID:                          "SE-OP-102",
		IncidentID:                  "INC-SE-9202",
		PetName:                     "Mochi",
		Status:                      domain.EpilepticusStatusAnticonvulsantCRIStaged,
		NeurogenicHyperthermiaAlert: true,
		PatientProfile:              p102,
		TitrationRegimen:            BuildTitrationRegimen(p102),
		NeuroChecks: []domain.NeuroIcuSerialCheck{
			{
				Timestamp:                t102.Add(10 * time.Minute),
				SeizureActivityObserved:  false,
				RectalTempFahrenheit:     104.6,
				ModifiedGlasgowComaScore: 13,
				PupillaryLightReflex:     domain.PupillaryReflexMioticPinpoint,
				BloodGlucoseMgDl:         98.0,
				ClinicianCallsign:        "DR-ECC-19",
				Notes:                    "Canine spot-on permethrin toxicosis in cat; Methocarbamol 55 mg/kg IV + Midazolam + Levetiracetam staged; dermal decontamination completed.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(10 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC)
	p103 := domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesCanine,
		Breed:                    "Border Collie",
		WeightKg:                 18.5,
		NeuroEtiology:            domain.NeuroEtiologyMetaldehydeSnailBaitTremor,
		ContinuousIctalMinutes:   0,
		ClusterCount24Hr:         1,
		RectalTempFahrenheit:     101.2,
		BloodGlucoseMgDl:         104.0,
		ModifiedGlasgowComaScore: 18,
	}
	clearance103 := &domain.EpilepticusDischargeClearance{
		ClearedAt:                          t103.Add(25 * time.Hour),
		SeizureFreeHours:                   26,
		FinalModifiedGlasgowScore:          18,
		FinalRectalTempFahrenheit:          101.2,
		AmbulatoryMenaceIntact:             true,
		RescueIntranasalMidazolamDispensed: true,
		VetLicense:                         "VET-CA-88412",
		Summary:                            "26h seizure-free and tremor-free following metaldehyde decontamination, methocarbamol, and levetiracetam titration; normothermic 101.2°F; MGCS 18; home intranasal midazolam atomizer dispensed.",
	}
	op103 := &domain.StatusEpilepticusOperation{
		ID:                          "SE-OP-103",
		IncidentID:                  "INC-SE-9203",
		PetName:                     "Bowie",
		Status:                      domain.EpilepticusStatusSeizureFreeNormothermic,
		NeurogenicHyperthermiaAlert: false,
		PatientProfile:              p103,
		TitrationRegimen:            BuildTitrationRegimen(p103),
		NeuroChecks: []domain.NeuroIcuSerialCheck{
			{
				Timestamp:                t103.Add(24 * time.Hour),
				SeizureActivityObserved:  false,
				RectalTempFahrenheit:     101.2,
				ModifiedGlasgowComaScore: 18,
				PupillaryLightReflex:     domain.PupillaryReflexNormalBilateral,
				BloodGlucoseMgDl:         104.0,
				ClinicianCallsign:        "DR-NEURO-07",
				Notes:                    "24h serial ICU check normal; bilateral PLR and menace intact; normothermic.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(25 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *EpilepticusService) ListOperations(statusFilter string) []domain.StatusEpilepticusOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.StatusEpilepticusOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneEpilepticusOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *EpilepticusService) GetOperation(id string) (*domain.StatusEpilepticusOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("status epilepticus operation %q not found", id)
	}
	cp := cloneEpilepticusOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new status epilepticus triage operation.
func (s *EpilepticusService) CreateOperation(incidentID, petName string, profile domain.EpilepticusPatientProfile) (*domain.StatusEpilepticusOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.NeuroEtiology = strings.ToUpper(strings.TrimSpace(profile.NeuroEtiology))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("SE-OP-%d", s.seq)
	now := time.Now().UTC()

	hyperthermiaAlert := profile.RectalTempFahrenheit >= 103.5 || profile.ContinuousIctalMinutes >= 5 || profile.ModifiedGlasgowComaScore <= 12
	regimen := BuildTitrationRegimen(profile)

	op := &domain.StatusEpilepticusOperation{
		ID:                          opID,
		IncidentID:                  trimmedIncident,
		PetName:                     trimmedPet,
		Status:                      domain.EpilepticusStatusActiveIctalCritical,
		NeurogenicHyperthermiaAlert: hyperthermiaAlert,
		PatientProfile:              profile,
		TitrationRegimen:            regimen,
		NeuroChecks:                 []domain.NeuroIcuSerialCheck{},
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneEpilepticusOperation(op)
	return &cp, nil
}

// StageTitrationRegimen computes the anticonvulsant titration regimen and transitions the operation to MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED.
func (s *EpilepticusService) StageTitrationRegimen(id string, profile domain.EpilepticusPatientProfile) (*domain.StatusEpilepticusOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.NeuroEtiology = strings.ToUpper(strings.TrimSpace(profile.NeuroEtiology))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("status epilepticus operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.TitrationRegimen = BuildTitrationRegimen(profile)
	op.NeurogenicHyperthermiaAlert = profile.RectalTempFahrenheit >= 103.5 || profile.ContinuousIctalMinutes >= 5
	op.Status = domain.EpilepticusStatusAnticonvulsantCRIStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneEpilepticusOperation(op)
	return &cp, nil
}

// RecordNeuroCheck appends a serial neuro ICU check to an existing operation.
func (s *EpilepticusService) RecordNeuroCheck(id string, check domain.NeuroIcuSerialCheck) (*domain.StatusEpilepticusOperation, error) {
	check.PupillaryLightReflex = strings.ToUpper(strings.TrimSpace(check.PupillaryLightReflex))
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("status epilepticus operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.NeuroChecks = append(op.NeuroChecks, check)
	op.PatientProfile.RectalTempFahrenheit = check.RectalTempFahrenheit
	op.PatientProfile.ModifiedGlasgowComaScore = check.ModifiedGlasgowComaScore
	op.PatientProfile.BloodGlucoseMgDl = check.BloodGlucoseMgDl

	if check.SeizureActivityObserved || check.RectalTempFahrenheit >= 104.0 || check.PupillaryLightReflex == domain.PupillaryReflexAnisocoriaCushingReflex {
		op.NeurogenicHyperthermiaAlert = true
		if check.SeizureActivityObserved {
			op.Status = domain.EpilepticusStatusActiveIctalCritical
			op.DischargeClearance = nil
		}
	} else if check.RectalTempFahrenheit < 103.0 && !check.SeizureActivityObserved {
		op.NeurogenicHyperthermiaAlert = false
	}

	op.UpdatedAt = now
	cp := cloneEpilepticusOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates 24-hour seizure-free normothermic recovery and transitions the operation to SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED.
func (s *EpilepticusService) IssueDischargeClearance(id string, clearance domain.EpilepticusDischargeClearance) (*domain.StatusEpilepticusOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("status epilepticus operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("%dh seizure-free normothermic recovery (%.1f°F, MGCS %d); ambulatory & menace intact; home rescue intranasal midazolam dispensed.", clearance.SeizureFreeHours, clearance.FinalRectalTempFahrenheit, clearance.FinalModifiedGlasgowScore)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.EpilepticusStatusSeizureFreeNormothermic
	op.NeurogenicHyperthermiaAlert = false
	op.PatientProfile.ContinuousIctalMinutes = 0
	op.PatientProfile.RectalTempFahrenheit = clearance.FinalRectalTempFahrenheit
	op.PatientProfile.ModifiedGlasgowComaScore = clearance.FinalModifiedGlasgowScore
	op.UpdatedAt = now

	cp := cloneEpilepticusOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all operations in the service.
func (s *EpilepticusService) ComputeMetrics() EpilepticusMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m EpilepticusMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.EpilepticusStatusActiveIctalCritical:
			m.ActiveIctalCriticalCount++
		case domain.EpilepticusStatusAnticonvulsantCRIStaged:
			m.AnticonvulsantCRIStagedCount++
		case domain.EpilepticusStatusSeizureFreeNormothermic:
			m.SeizureFreeClearedCount++
		}
		if op.NeurogenicHyperthermiaAlert {
			m.HyperthermiaAlertCount++
		}
	}
	return m
}

func cloneEpilepticusOperation(op *domain.StatusEpilepticusOperation) domain.StatusEpilepticusOperation {
	cp := *op
	if op.TitrationRegimen != nil {
		reg := *op.TitrationRegimen
		cp.TitrationRegimen = &reg
	}
	if op.NeuroChecks != nil {
		cp.NeuroChecks = make([]domain.NeuroIcuSerialCheck, len(op.NeuroChecks))
		copy(cp.NeuroChecks, op.NeuroChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
