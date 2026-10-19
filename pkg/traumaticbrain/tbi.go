package traumaticbrain

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TBIMetrics summarizes operational KPI counts across Traumatic Brain Injury (TBI) triage operations.
type TBIMetrics struct {
	TotalOperations           int `json:"total_operations"`
	CushingHerniationCount    int `json:"cushing_herniation_count"`
	HypertonicMannitolStaged  int `json:"hypertonic_mannitol_staged"`
	MGCSRecoveredClearedCount int `json:"mgcs_recovered_cleared_count"`
	CushingSteroidAlertCount  int `json:"cushing_steroid_alert_count"`
}

// TBIService manages Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS),
// Cushing's Reflex & Hypertonic Saline/Mannitol triage operations in a thread-safe in-memory store.
type TBIService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.TraumaticBrainOperation
	order []string
	seq   int
}

// NewTBIService initializes a TBIService with 3 seeded operations (TBI-OP-101, TBI-OP-102, TBI-OP-103).
func NewTBIService() *TBIService {
	s := &TBIService{
		ops:   make(map[string]*domain.TraumaticBrainOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildNeuroprotectivePlan deterministically computes a TBINeuroprotectivePlan for a patient profile.
func BuildNeuroprotectivePlan(profile domain.TBIPatientProfile) *domain.TBINeuroprotectivePlan {
	htsMlKg := 4.0
	if strings.EqualFold(profile.Species, domain.TBISpeciesFelineMaxillofacialCranial) {
		htsMlKg = 2.0
	}
	htsTotalMl := round2(htsMlKg * profile.WeightKg)

	mannitolGrams := 0.0
	if profile.MeanArterialPressureMmHg >= 80 {
		mannitolGrams = round2(0.5 * profile.WeightKg)
	}

	pitfallAlert := "CRITICAL TBI, CUSHING'S REFLEX & ICP HERNIATION PITFALL ALERT: NEVER administer corticosteroids (Dexamethasone/Prednisolone) in Traumatic Brain Injury (contraindicated per CRASH trial — increases mortality and hyperglycemia); NEVER compress external jugular veins or flex the neck (obstructs cranial venous outflow and spikes intracranial pressure); and NEVER give Mannitol to hypovolemic/hypotensive patients (MAP < 80 mmHg) before restoring intravascular volume with 7.5% Hypertonic Saline."

	rationaleParts := []string{
		fmt.Sprintf("7.5%% Hypertonic Saline %.1f mL/kg (%.1f mL IV over 15 min) for %s (%.1f kg) to rapidly expand intravascular volume, preserve Cerebral Perfusion Pressure (CPP = MAP - ICP, target MAP >= 80 mmHg), and osmotically reduce cerebral edema.", htsMlKg, htsTotalMl, profile.Species, profile.WeightKg),
	}
	if mannitolGrams > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Mannitol 0.5 g/kg (%.2f g IV over 20 min via in-line filter) staged after normotensive MAP %d mmHg (>= 80 mmHg) verified.", mannitolGrams, profile.MeanArterialPressureMmHg))
	} else {
		rationaleParts = append(rationaleParts, fmt.Sprintf("HOLD Mannitol while hypotensive (MAP %d mmHg < 80 mmHg); resuscitate with 7.5%% Hypertonic Saline first to prevent osmotic diuresis-induced hypotension.", profile.MeanArterialPressureMmHg))
	}
	rationaleParts = append(rationaleParts, "Elevate head/shoulders 30 degrees on a rigid slant board (zero jugular compression or neck flexion), maintain normocapnia (EtCO2 35-40 mmHg), and strictly avoid corticosteroids.")

	return &domain.TBINeuroprotectivePlan{
		HypertonicSaline75PctMlKg:                 htsMlKg,
		HypertonicSalineTotalDoseMl:               htsTotalMl,
		Mannitol05GKgSlow20MinGrams:               mannitolGrams,
		HeadElevationAngleDegrees:                 30,
		TargetMAPMinimumMmHg:                      80,
		TargetEtCO2RangeMmHg:                      "35-40 mmHg",
		ZeroJugularCompressionCorticosteroidGuard: true,
		TBIPitfallAlert:                           pitfallAlert,
		ClinicalRationale:                         strings.Join(rationaleParts, " "),
	}
}

func needsCushingHerniationSteroidAlert(p domain.TBIPatientProfile) bool {
	return p.CushingReflexPresent ||
		p.ModifiedGlasgowComaScore <= 14 ||
		p.MeanArterialPressureMmHg < 80 ||
		p.MeanArterialPressureMmHg >= 140 ||
		p.EndTidalCO2MmHg > 42.0 ||
		!strings.EqualFold(p.PupillaryStatus, domain.TBIPupilsResponsiveIsocoric)
}

func (s *TBIService) seed() {
	t101 := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)
	p101 := domain.TBIPatientProfile{
		Species:                  domain.TBISpeciesCanineBluntVehicular,
		Breed:                    "Rottweiler",
		WeightKg:                 42.0,
		ModifiedGlasgowComaScore: 8,
		MeanArterialPressureMmHg: 148,
		HeartRateBpm:             46,
		EndTidalCO2MmHg:          48.0,
		PupillaryStatus:          domain.TBIPupilsAnisocoriaUnilateralMydriasis,
		CushingReflexPresent:     true,
	}
	op101 := &domain.TraumaticBrainOperation{
		ID:                            "TBI-OP-101",
		IncidentID:                    "INC-TBI-9801",
		PetName:                       "Atlas",
		Status:                        domain.TBIStatusCushingHerniationCritical,
		CushingHerniationSteroidAlert: true,
		PatientProfile:                p101,
		NeuroprotectivePlan:           BuildNeuroprotectivePlan(p101),
		NeuroChecks: []domain.TBINeuroSerialCheck{
			{
				Timestamp:                t101.Add(10 * time.Minute),
				ModifiedGlasgowComaScore: 8,
				MeanArterialPressureMmHg: 148,
				HeartRateBpm:             46,
				EndTidalCO2MmHg:          48.0,
				PupillaryStatus:          domain.TBIPupilsAnisocoriaUnilateralMydriasis,
				ClinicianCallsign:        "DR-NEURO-11",
				Notes:                    "Vehicular blunt TBI; MGCS 8/18, Cushing's reflex (MAP 148 mmHg + reflex bradycardia 46 bpm), left mydriatic anisocoria; 30-deg slant board, 7.5% HTS 168 mL IV over 15m, zero steroids.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 2, 20, 0, 0, time.UTC)
	p102 := domain.TBIPatientProfile{
		Species:                  domain.TBISpeciesFelineMaxillofacialCranial,
		Breed:                    "Domestic Shorthair",
		WeightKg:                 4.6,
		ModifiedGlasgowComaScore: 13,
		MeanArterialPressureMmHg: 92,
		HeartRateBpm:             165,
		EndTidalCO2MmHg:          38.0,
		PupillaryStatus:          domain.TBIPupilsMioticPinpointBilateral,
		CushingReflexPresent:     false,
	}
	op102 := &domain.TraumaticBrainOperation{
		ID:                            "TBI-OP-102",
		IncidentID:                    "INC-TBI-9802",
		PetName:                       "Milo",
		Status:                        domain.TBIStatusHypertonicMannitolStaged,
		CushingHerniationSteroidAlert: true,
		PatientProfile:                p102,
		NeuroprotectivePlan:           BuildNeuroprotectivePlan(p102),
		NeuroChecks: []domain.TBINeuroSerialCheck{
			{
				Timestamp:                t102.Add(15 * time.Minute),
				ModifiedGlasgowComaScore: 13,
				MeanArterialPressureMmHg: 92,
				HeartRateBpm:             165,
				EndTidalCO2MmHg:          38.0,
				PupillaryStatus:          domain.TBIPupilsMioticPinpointBilateral,
				ClinicianCallsign:        "DR-ECC-08",
				Notes:                    "Feline cranial trauma; MGCS 13/18, bilateral miotic pupils, MAP 92 mmHg; 7.5% HTS 2.0 mL/kg (9.2 mL) + Mannitol 0.5 g/kg (2.30 g) staged with 30-deg head elevation.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 6, 0, 0, 0, time.UTC)
	p103 := domain.TBIPatientProfile{
		Species:                  domain.TBISpeciesCanineBluntVehicular,
		Breed:                    "Australian Shepherd",
		WeightKg:                 21.0,
		ModifiedGlasgowComaScore: 17,
		MeanArterialPressureMmHg: 96,
		HeartRateBpm:             94,
		EndTidalCO2MmHg:          37.0,
		PupillaryStatus:          domain.TBIPupilsResponsiveIsocoric,
		CushingReflexPresent:     false,
	}
	clearance103 := &domain.TBIDischargeClearance{
		ClearedAt:                     t103.Add(24 * time.Hour),
		FinalModifiedGlasgowScore:     17,
		FinalMAPMmHg:                  96,
		PupilsIsocoricResponsive:      true,
		GagSwallowReflexIntact:        true,
		AmbulatoryNormocapnicVerified: true,
		VetLicense:                    "VET-CA-98412",
		Summary:                       "MGCS recovered to 17/18, normotensive MAP 96 mmHg, isocoric responsive PLRs, intact gag/swallow reflex, ambulatory normocapnic (EtCO2 37 mmHg).",
	}
	op103 := &domain.TraumaticBrainOperation{
		ID:                            "TBI-OP-103",
		IncidentID:                    "INC-TBI-9803",
		PetName:                       "Sierra",
		Status:                        domain.TBIStatusMGCSRecoveredNormocapnicCleared,
		CushingHerniationSteroidAlert: false,
		PatientProfile:                p103,
		NeuroprotectivePlan:           BuildNeuroprotectivePlan(p103),
		NeuroChecks: []domain.TBINeuroSerialCheck{
			{
				Timestamp:                t103.Add(22 * time.Hour),
				ModifiedGlasgowComaScore: 17,
				MeanArterialPressureMmHg: 96,
				HeartRateBpm:             94,
				EndTidalCO2MmHg:          37.0,
				PupillaryStatus:          domain.TBIPupilsResponsiveIsocoric,
				ClinicianCallsign:        "DR-NEURO-04",
				Notes:                    "MGCS 17/18, pupils isocoric & briskly responsive, normotensive MAP 96 mmHg, eating softly with intact gag reflex.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(24 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *TBIService) ListOperations(statusFilter string) []domain.TraumaticBrainOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.TraumaticBrainOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneTBIOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *TBIService) GetOperation(id string) (*domain.TraumaticBrainOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("traumatic brain operation %q not found", id)
	}
	cp := cloneTBIOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Traumatic Brain Injury triage operation.
func (s *TBIService) CreateOperation(incidentID, petName string, profile domain.TBIPatientProfile) (*domain.TraumaticBrainOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.PupillaryStatus = strings.ToUpper(strings.TrimSpace(profile.PupillaryStatus))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("TBI-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsCushingHerniationSteroidAlert(profile)
	plan := BuildNeuroprotectivePlan(profile)

	op := &domain.TraumaticBrainOperation{
		ID:                            opID,
		IncidentID:                    trimmedIncident,
		PetName:                       trimmedPet,
		Status:                        domain.TBIStatusCushingHerniationCritical,
		CushingHerniationSteroidAlert: alert,
		PatientProfile:                profile,
		NeuroprotectivePlan:           plan,
		NeuroChecks:                   []domain.TBINeuroSerialCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneTBIOperation(op)
	return &cp, nil
}

// StageNeuroprotectivePlan computes the 7.5% Hypertonic Saline, Mannitol, 30-deg head elevation,
// MAP >= 80 mmHg, and EtCO2 35-40 mmHg plan and transitions the operation to HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED.
func (s *TBIService) StageNeuroprotectivePlan(id string, profile domain.TBIPatientProfile) (*domain.TraumaticBrainOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.PupillaryStatus = strings.ToUpper(strings.TrimSpace(profile.PupillaryStatus))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("traumatic brain operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.NeuroprotectivePlan = BuildNeuroprotectivePlan(profile)
	op.CushingHerniationSteroidAlert = needsCushingHerniationSteroidAlert(profile)
	op.Status = domain.TBIStatusHypertonicMannitolStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneTBIOperation(op)
	return &cp, nil
}

// RecordNeuroCheck appends a serial MGCS, MAP, HR, EtCO2 & pupillary check to an existing operation.
func (s *TBIService) RecordNeuroCheck(id string, check domain.TBINeuroSerialCheck) (*domain.TraumaticBrainOperation, error) {
	check.PupillaryStatus = strings.ToUpper(strings.TrimSpace(check.PupillaryStatus))
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("traumatic brain operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.NeuroChecks = append(op.NeuroChecks, check)
	op.PatientProfile.ModifiedGlasgowComaScore = check.ModifiedGlasgowComaScore
	op.PatientProfile.MeanArterialPressureMmHg = check.MeanArterialPressureMmHg
	op.PatientProfile.HeartRateBpm = check.HeartRateBpm
	op.PatientProfile.EndTidalCO2MmHg = check.EndTidalCO2MmHg
	op.PatientProfile.PupillaryStatus = check.PupillaryStatus
	op.NeuroprotectivePlan = BuildNeuroprotectivePlan(op.PatientProfile)

	if check.ModifiedGlasgowComaScore <= 9 || check.PupillaryStatus == domain.TBIPupilsAnisocoriaUnilateralMydriasis || (check.MeanArterialPressureMmHg >= 140 && check.HeartRateBpm <= 55) {
		op.PatientProfile.CushingReflexPresent = true
		op.CushingHerniationSteroidAlert = true
		op.Status = domain.TBIStatusCushingHerniationCritical
		op.DischargeClearance = nil
	} else if check.ModifiedGlasgowComaScore >= 15 && check.PupillaryStatus == domain.TBIPupilsResponsiveIsocoric && check.MeanArterialPressureMmHg >= 80 && check.MeanArterialPressureMmHg <= 125 && check.EndTidalCO2MmHg >= 35.0 && check.EndTidalCO2MmHg <= 40.0 {
		op.PatientProfile.CushingReflexPresent = false
		op.CushingHerniationSteroidAlert = false
	}

	op.UpdatedAt = now
	cp := cloneTBIOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates MGCS recovery (16-18), normotension (MAP 80-120 mmHg),
// responsive isocoric pupils, intact gag/swallow reflex, and ambulatory normocapnia,
// transitioning the operation to MGCS_RECOVERED_NORMOCAPNIC_CLEARED.
func (s *TBIService) IssueDischargeClearance(id string, clearance domain.TBIDischargeClearance) (*domain.TraumaticBrainOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("traumatic brain operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("MGCS recovered to %d/18, normotensive MAP %d mmHg, responsive isocoric pupils, intact gag/swallow reflex, ambulatory normocapnic.", clearance.FinalModifiedGlasgowScore, clearance.FinalMAPMmHg)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.TBIStatusMGCSRecoveredNormocapnicCleared
	op.CushingHerniationSteroidAlert = false
	op.PatientProfile.ModifiedGlasgowComaScore = clearance.FinalModifiedGlasgowScore
	op.PatientProfile.MeanArterialPressureMmHg = clearance.FinalMAPMmHg
	op.PatientProfile.PupillaryStatus = domain.TBIPupilsResponsiveIsocoric
	op.PatientProfile.CushingReflexPresent = false
	op.UpdatedAt = now

	cp := cloneTBIOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all TBI operations in the service.
func (s *TBIService) ComputeMetrics() TBIMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m TBIMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.TBIStatusCushingHerniationCritical:
			m.CushingHerniationCount++
		case domain.TBIStatusHypertonicMannitolStaged:
			m.HypertonicMannitolStaged++
		case domain.TBIStatusMGCSRecoveredNormocapnicCleared:
			m.MGCSRecoveredClearedCount++
		}
		if op.CushingHerniationSteroidAlert {
			m.CushingSteroidAlertCount++
		}
	}
	return m
}

func cloneTBIOperation(op *domain.TraumaticBrainOperation) domain.TraumaticBrainOperation {
	cp := *op
	if op.NeuroprotectivePlan != nil {
		plan := *op.NeuroprotectivePlan
		cp.NeuroprotectivePlan = &plan
	}
	if op.NeuroChecks != nil {
		cp.NeuroChecks = make([]domain.TBINeuroSerialCheck, len(op.NeuroChecks))
		copy(cp.NeuroChecks, op.NeuroChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
