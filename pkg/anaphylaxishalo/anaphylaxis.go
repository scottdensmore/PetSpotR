package anaphylaxishalo

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AnaphylaxisMetrics summarizes operational KPI counts across Acute Anaphylaxis & Hemoabdomen Halo Sign triage operations.
type AnaphylaxisMetrics struct {
	TotalOperations               int `json:"total_operations"`
	CriticalShockHaloCount        int `json:"critical_shock_halo_count"`
	EpinephrineFFPStagedCount     int `json:"epinephrine_ffp_staged_count"`
	Biphasic24HrClearedCount      int `json:"biphasic_24hr_cleared_count"`
	HepaticCoagulopathyAlertCount int `json:"hepatic_coagulopathy_alert_count"`
}

// AnaphylaxisService manages Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema),
// Epinephrine IM/CRI & Venous Pool Resuscitation triage operations in a thread-safe in-memory store.
type AnaphylaxisService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.AnaphylaxisHaloOperation
	order []string
	seq   int
}

// NewAnaphylaxisService initializes an AnaphylaxisService with 3 seeded operations (AH-OP-101, AH-OP-102, AH-OP-103).
func NewAnaphylaxisService() *AnaphylaxisService {
	s := &AnaphylaxisService{
		ops:   make(map[string]*domain.AnaphylaxisHaloOperation),
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

// BuildResuscitationPlan deterministically computes an AnaphylaxisResuscitationPlan for a patient profile.
func BuildResuscitationPlan(profile domain.AnaphylaxisPatientProfile) *domain.AnaphylaxisResuscitationPlan {
	epiIMMg := round2(0.01 * profile.WeightKg)
	if profile.WeightKg < 20.0 {
		if epiIMMg > 0.30 {
			epiIMMg = 0.30
		}
	} else {
		if epiIMMg > 0.50 {
			epiIMMg = 0.50
		}
	}

	var epiCRIMcgMin float64
	if profile.MeanArterialPressureMmHg < 60 {
		epiCRIMcgMin = round2(0.05 * profile.WeightKg)
	}

	crystalloidRateMlKg := 20.0
	if strings.EqualFold(profile.Species, domain.AnaphylaxisSpeciesFelineBronchospasmPulmonary) {
		crystalloidRateMlKg = 10.0
	}
	crystalloidBolusMl := round2(crystalloidRateMlKg * profile.WeightKg)

	var ffpMl float64
	if profile.ProthrombinTimeSeconds > 18.0 {
		ffpMl = round2(15.0 * profile.WeightKg)
	}

	diphenhydramineIMMg := round2(2.0 * profile.WeightKg)
	famotidineIVMg := round2(0.5 * profile.WeightKg)

	pitfallAlert := "CRITICAL ANAPHYLAXIS, GALLBLADDER HALO SIGN & HEMOABDOMEN PITFALL ALERT: NEVER mistake the ultrasound gallbladder wall striated 'Halo Sign' (> 3.0 mm edema) and anaphylactic non-clotting hemoabdomen (caused by acute canine hepatic venous sphincter constriction, severe portal hypertension, and mast-cell heparin/tryptase hyperfibrinolysis) for surgical gallbladder mucocele rupture — emergency cholecystectomy during anaphylactic shock is fatal; NEVER administer Diphenhydramine IV rapid bolus (triggers severe histamine-mediated hypotension — give 2.0 mg/kg IM only AFTER Epinephrine); NEVER administer undiluted 1:1000 Epinephrine rapid IV push; and NEVER discharge before >= 24h biphasic anaphylaxis observation."

	rationaleParts := []string{
		fmt.Sprintf("Epinephrine 0.01 mg/kg IM (%.2f mg IM into epaxial/quadriceps muscle, capped at 0.3 mg < 20 kg / 0.5 mg >= 20 kg) + Isotonic Crystalloid Shock Aliquot %.0f mL/kg (%.1f mL IV for %.1f kg %s).", epiIMMg, crystalloidRateMlKg, crystalloidBolusMl, profile.WeightKg, profile.Species),
	}
	if epiCRIMcgMin > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Refractory anaphylactic hypotension (MAP %d mmHg < 60 mmHg): initiate Epinephrine CRI at 0.05 mcg/kg/min (%.2f mcg/min IV titrated to MAP >= 75 mmHg).", profile.MeanArterialPressureMmHg, epiCRIMcgMin))
	}
	if ffpMl > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Anaphylactic coagulopathy / non-clotting hemoabdomen (PT %.1f s > 18.0 s, ALT %d U/L, GB wall %.1f mm): transfuse Fresh Frozen Plasma (FFP) 15 mL/kg (%.1f mL IV) to restore coagulation factors and antithrombin.", profile.ProthrombinTimeSeconds, profile.AlanineAminotransferaseUL, profile.GallbladderWallThicknessMm, ffpMl))
	}
	rationaleParts = append(rationaleParts, fmt.Sprintf("Adjunctive H1/H2 blockade after Epinephrine: Diphenhydramine %.1f mg IM (2.0 mg/kg IM — strictly avoid IV bolus) + Famotidine %.1f mg slow IV (0.5 mg/kg IV).", diphenhydramineIMMg, famotidineIVMg))

	return &domain.AnaphylaxisResuscitationPlan{
		EpinephrineIM001MgKgDoseMg:                     epiIMMg,
		EpinephrineCRI005McgKgMin:                      epiCRIMcgMin,
		IsotonicCrystalloidBolusMl:                     crystalloidBolusMl,
		FreshFrozenPlasma15MlKgMl:                      ffpMl,
		DiphenhydramineIM2MgKgMg:                       diphenhydramineIMMg,
		FamotidineIV05MgKgMg:                           famotidineIVMg,
		ZeroIVUndilutedEpiSurgicalCholecystectomyGuard: true,
		AnaphylaxisPitfallAlert:                        pitfallAlert,
		ClinicalRationale:                              strings.Join(rationaleParts, " "),
	}
}

func needsHepaticCongestionCoagulopathyAlert(p domain.AnaphylaxisPatientProfile) bool {
	return p.GallbladderHaloSignPresent ||
		p.GallbladderWallThicknessMm > 3.0 ||
		p.MeanArterialPressureMmHg < 70 ||
		p.AlanineAminotransferaseUL > 500 ||
		p.PackedCellVolumePercent >= 60.0 ||
		p.ProthrombinTimeSeconds > 15.0
}

func (s *AnaphylaxisService) seed() {
	t101 := time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC)
	p101 := domain.AnaphylaxisPatientProfile{
		Species:                    domain.AnaphylaxisSpeciesCanineHymenopteraHemoabdomen,
		Breed:                      "Dachshund",
		WeightKg:                   6.8,
		GallbladderWallThicknessMm: 6.4,
		GallbladderHaloSignPresent: true,
		MeanArterialPressureMmHg:   46,
		AlanineAminotransferaseUL:  2450,
		PackedCellVolumePercent:    68.0,
		ProthrombinTimeSeconds:     28.5,
	}
	op101 := &domain.AnaphylaxisHaloOperation{
		ID:                                 "AH-OP-101",
		IncidentID:                         "INC-AH-9901",
		PetName:                            "Waffle",
		Status:                             domain.AnaphylaxisStatusShockHaloCritical,
		HepaticCongestionCoagulopathyAlert: true,
		PatientProfile:                     p101,
		ResuscitationPlan:                  BuildResuscitationPlan(p101),
		BiphasicChecks: []domain.AnaphylaxisBiphasicCheck{
			{
				Timestamp:                  t101.Add(10 * time.Minute),
				MeanArterialPressureMmHg:   46,
				GallbladderWallThicknessMm: 6.4,
				PackedCellVolumePercent:    68.0,
				AlanineAminotransferaseUL:  2450,
				BiphasicRelapseSignsNoted:  true,
				ClinicianCallsign:          "DR-ECC-41",
				Notes:                      "Acute Hymenoptera sting anaphylaxis in Dachshund; AFAST striated double-layered gallbladder wall Halo Sign (6.4 mm), hemoconcentration PCV 68%, ALT 2450 U/L, PT 28.5 s medical anaphylactic hemoabdomen; surgical cholecystectomy blocked.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 3, 20, 0, 0, time.UTC)
	p102 := domain.AnaphylaxisPatientProfile{
		Species:                    domain.AnaphylaxisSpeciesFelineBronchospasmPulmonary,
		Breed:                      "Domestic Shorthair",
		WeightKg:                   4.4,
		GallbladderWallThicknessMm: 2.1,
		GallbladderHaloSignPresent: false,
		MeanArterialPressureMmHg:   68,
		AlanineAminotransferaseUL:  180,
		PackedCellVolumePercent:    44.0,
		ProthrombinTimeSeconds:     11.2,
	}
	op102 := &domain.AnaphylaxisHaloOperation{
		ID:                                 "AH-OP-102",
		IncidentID:                         "INC-AH-9902",
		PetName:                            "Bramble",
		Status:                             domain.AnaphylaxisStatusEpinephrineFFPStaged,
		HepaticCongestionCoagulopathyAlert: true,
		PatientProfile:                     p102,
		ResuscitationPlan:                  BuildResuscitationPlan(p102),
		BiphasicChecks: []domain.AnaphylaxisBiphasicCheck{
			{
				Timestamp:                  t102.Add(15 * time.Minute),
				MeanArterialPressureMmHg:   68,
				GallbladderWallThicknessMm: 2.1,
				PackedCellVolumePercent:    44.0,
				AlanineAminotransferaseUL:  180,
				BiphasicRelapseSignsNoted:  false,
				ClinicianCallsign:          "DR-FELINE-12",
				Notes:                      "Feline pulmonary/bronchospasm anaphylaxis; Epinephrine 0.04 mg IM + conservative 10 mL/kg (44.0 mL) isotonic crystalloid bolus + Diphenhydramine 8.8 mg IM staged.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	p103 := domain.AnaphylaxisPatientProfile{
		Species:                    domain.AnaphylaxisSpeciesCanineHepaticSplanchnic,
		Breed:                      "Beagle",
		WeightKg:                   15.0,
		GallbladderWallThicknessMm: 1.8,
		GallbladderHaloSignPresent: false,
		MeanArterialPressureMmHg:   92,
		AlanineAminotransferaseUL:  210,
		PackedCellVolumePercent:    45.0,
		ProthrombinTimeSeconds:     9.8,
	}
	clearance103 := &domain.AnaphylaxisDischargeClearance{
		ClearedAt:                  t103.Add(26 * time.Hour),
		BiphasicObservationHours:   26,
		FinalMAPMmHg:               92,
		GallbladderHaloResolved:    true,
		CoagulopathyNormalized:     true,
		EpiPenAutoInjectorEducated: true,
		VetLicense:                 "VET-ECC-99103",
		Summary:                    "26h biphasic anaphylaxis observation completed without relapse; AFAST gallbladder wall Halo Sign resolved (1.8 mm); normotensive MAP 92 mmHg, PCV 45%, PT 9.8 s; owner trained on pediatric EpiPen Jr (0.15 mg IM).",
	}
	op103 := &domain.AnaphylaxisHaloOperation{
		ID:                                 "AH-OP-103",
		IncidentID:                         "INC-AH-9903",
		PetName:                            "Clover",
		Status:                             domain.AnaphylaxisStatusBiphasic24HrCleared,
		HepaticCongestionCoagulopathyAlert: false,
		PatientProfile:                     p103,
		ResuscitationPlan:                  BuildResuscitationPlan(p103),
		BiphasicChecks: []domain.AnaphylaxisBiphasicCheck{
			{
				Timestamp:                  t103.Add(24 * time.Hour),
				MeanArterialPressureMmHg:   92,
				GallbladderWallThicknessMm: 1.8,
				PackedCellVolumePercent:    45.0,
				AlanineAminotransferaseUL:  210,
				BiphasicRelapseSignsNoted:  false,
				ClinicianCallsign:          "DR-ECC-08",
				Notes:                      "24h post-resuscitation AFAST confirms complete resolution of gallbladder wall edema (1.8 mm); hepatic venous congestion and PT normalized.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(26 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *AnaphylaxisService) ListOperations(statusFilter string) []domain.AnaphylaxisHaloOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.AnaphylaxisHaloOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneAnaphylaxisOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *AnaphylaxisService) GetOperation(id string) (*domain.AnaphylaxisHaloOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("anaphylaxis halo operation %q not found", id)
	}
	cp := cloneAnaphylaxisOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Acute Anaphylaxis & Hemoabdomen Halo Sign triage operation.
func (s *AnaphylaxisService) CreateOperation(incidentID, petName string, profile domain.AnaphylaxisPatientProfile) (*domain.AnaphylaxisHaloOperation, error) {
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
	opID := fmt.Sprintf("AH-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsHepaticCongestionCoagulopathyAlert(profile)
	plan := BuildResuscitationPlan(profile)

	op := &domain.AnaphylaxisHaloOperation{
		ID:                                 opID,
		IncidentID:                         trimmedIncident,
		PetName:                            trimmedPet,
		Status:                             domain.AnaphylaxisStatusShockHaloCritical,
		HepaticCongestionCoagulopathyAlert: alert,
		PatientProfile:                     profile,
		ResuscitationPlan:                  plan,
		BiphasicChecks:                     []domain.AnaphylaxisBiphasicCheck{},
		CreatedAt:                          now,
		UpdatedAt:                          now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneAnaphylaxisOperation(op)
	return &cp, nil
}

// StageResuscitationPlan computes Epinephrine 0.01 mg/kg IM (capped at 0.3 mg < 20 kg / 0.5 mg >= 20 kg),
// Epinephrine CRI 0.05 mcg/kg/min when MAP < 60 mmHg, isotonic crystalloid bolus (20 mL/kg canine vs 10 mL/kg feline),
// Fresh Frozen Plasma 15 mL/kg when PT > 18.0 s, Diphenhydramine 2.0 mg/kg IM, and Famotidine 0.5 mg/kg IV,
// transitioning the operation to EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED.
func (s *AnaphylaxisService) StageResuscitationPlan(id string, profile domain.AnaphylaxisPatientProfile) (*domain.AnaphylaxisHaloOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("anaphylaxis halo operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.ResuscitationPlan = BuildResuscitationPlan(profile)
	op.HepaticCongestionCoagulopathyAlert = needsHepaticCongestionCoagulopathyAlert(profile)
	op.Status = domain.AnaphylaxisStatusEpinephrineFFPStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneAnaphylaxisOperation(op)
	return &cp, nil
}

// RecordBiphasicCheck appends a serial 24h biphasic anaphylaxis observation, MAP, gallbladder wall thickness, PCV, and ALT check.
func (s *AnaphylaxisService) RecordBiphasicCheck(id string, check domain.AnaphylaxisBiphasicCheck) (*domain.AnaphylaxisHaloOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("anaphylaxis halo operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.BiphasicChecks = append(op.BiphasicChecks, check)
	op.PatientProfile.MeanArterialPressureMmHg = check.MeanArterialPressureMmHg
	op.PatientProfile.GallbladderWallThicknessMm = round1(check.GallbladderWallThicknessMm)
	op.PatientProfile.GallbladderHaloSignPresent = check.GallbladderWallThicknessMm > 3.0
	op.PatientProfile.PackedCellVolumePercent = round1(check.PackedCellVolumePercent)
	op.PatientProfile.AlanineAminotransferaseUL = check.AlanineAminotransferaseUL
	op.ResuscitationPlan = BuildResuscitationPlan(op.PatientProfile)

	if check.BiphasicRelapseSignsNoted || check.MeanArterialPressureMmHg < 65 || check.GallbladderWallThicknessMm > 4.5 {
		op.HepaticCongestionCoagulopathyAlert = true
		if check.BiphasicRelapseSignsNoted || check.MeanArterialPressureMmHg < 60 {
			op.Status = domain.AnaphylaxisStatusShockHaloCritical
			op.DischargeClearance = nil
		}
	} else if check.MeanArterialPressureMmHg >= 75 && check.GallbladderWallThicknessMm <= 3.0 && check.PackedCellVolumePercent <= 55.0 {
		op.HepaticCongestionCoagulopathyAlert = false
	}

	op.UpdatedAt = now
	cp := cloneAnaphylaxisOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates >= 24h biphasic observation, normotensive MAP 75-120 mmHg,
// gallbladder Halo Sign resolution, coagulopathy normalization, and owner EpiPen education,
// transitioning the operation to BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED.
func (s *AnaphylaxisService) IssueDischargeClearance(id string, clearance domain.AnaphylaxisDischargeClearance) (*domain.AnaphylaxisHaloOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("anaphylaxis halo operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Anaphylactic shock & gallbladder Halo Sign resolved after %dh biphasic observation (final MAP %d mmHg); coagulopathy normalized and owner educated on EpiPen auto-injector.", clearance.BiphasicObservationHours, clearance.FinalMAPMmHg)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.AnaphylaxisStatusBiphasic24HrCleared
	op.HepaticCongestionCoagulopathyAlert = false
	op.PatientProfile.MeanArterialPressureMmHg = clearance.FinalMAPMmHg
	op.PatientProfile.GallbladderHaloSignPresent = false
	if op.PatientProfile.GallbladderWallThicknessMm > 2.5 {
		op.PatientProfile.GallbladderWallThicknessMm = 2.0
	}
	op.ResuscitationPlan = BuildResuscitationPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneAnaphylaxisOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all Anaphylaxis Halo operations in the service.
func (s *AnaphylaxisService) ComputeMetrics() AnaphylaxisMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m AnaphylaxisMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.AnaphylaxisStatusShockHaloCritical:
			m.CriticalShockHaloCount++
		case domain.AnaphylaxisStatusEpinephrineFFPStaged:
			m.EpinephrineFFPStagedCount++
		case domain.AnaphylaxisStatusBiphasic24HrCleared:
			m.Biphasic24HrClearedCount++
		}
		if op.HepaticCongestionCoagulopathyAlert {
			m.HepaticCoagulopathyAlertCount++
		}
	}
	return m
}

func cloneAnaphylaxisOperation(op *domain.AnaphylaxisHaloOperation) domain.AnaphylaxisHaloOperation {
	cp := *op
	if op.ResuscitationPlan != nil {
		plan := *op.ResuscitationPlan
		cp.ResuscitationPlan = &plan
	}
	if op.BiphasicChecks != nil {
		cp.BiphasicChecks = make([]domain.AnaphylaxisBiphasicCheck, len(op.BiphasicChecks))
		copy(cp.BiphasicChecks, op.BiphasicChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
