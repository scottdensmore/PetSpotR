package septicperitonitis

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// PeritonitisMetrics summarizes operational KPI counts across septic peritonitis triage operations.
type PeritonitisMetrics struct {
	TotalOperations               int `json:"total_operations"`
	SepticEffusionCriticalCount   int `json:"septic_effusion_critical_count"`
	AntibioticLavageJPStagedCount int `json:"antibiotic_lavage_jp_staged_count"`
	NonSepticClearedCount         int `json:"non_septic_cleared_count"`
	SepticGradientAlertCount      int `json:"septic_gradient_alert_count"`
}

// PeritonitisService manages Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient
// & Closed-Suction Jackson-Pratt Drain triage operations in a thread-safe in-memory store.
type PeritonitisService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.SepticPeritonitisOperation
	order []string
	seq   int
}

// NewPeritonitisService initializes a PeritonitisService with 3 seeded operations (SP-OP-101, SP-OP-102, SP-OP-103).
func NewPeritonitisService() *PeritonitisService {
	s := &PeritonitisService{
		ops:   make(map[string]*domain.SepticPeritonitisOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// IsSepticGradientAlert returns true when blood-fluid glucose gradient > 20 mg/dL,
// fluid-blood lactate gradient > 2.0 mmol/L, degenerate neutrophils >= 10000/uL, or intracellular bacteria are present.
func IsSepticGradientAlert(profile domain.PeritonitisPatientProfile) bool {
	glucoseGradient := round2(profile.BloodGlucoseMgDl - profile.EffusionGlucoseMgDl)
	lactateGradient := round2(profile.EffusionLactateMmolL - profile.BloodLactateMmolL)
	return glucoseGradient > 20.0 ||
		lactateGradient > 2.0 ||
		profile.IntracellularBacteriaSeen ||
		profile.DegenerateNeutrophilsCountUl >= 10000
}

// BuildSurgicalPlan deterministically computes a SepticPeritonitisSurgicalPlan for a patient profile.
func BuildSurgicalPlan(profile domain.PeritonitisPatientProfile) *domain.SepticPeritonitisSurgicalPlan {
	glucoseGradient := round2(profile.BloodGlucoseMgDl - profile.EffusionGlucoseMgDl)
	lactateGradient := round2(profile.EffusionLactateMmolL - profile.BloodLactateMmolL)
	ampSulbactamMg := round2(profile.WeightKg * 30.0)

	enroRateMgKg := 10.0
	if profile.Species == domain.PeritonitisSpeciesFelineBilePeritonitis {
		enroRateMgKg = 5.0
	}
	enrofloxacinMg := round2(profile.WeightKg * enroRateMgKg)

	lavageLiters := round2(profile.WeightKg * 0.3)

	jpDrains := 2
	if profile.Species == domain.PeritonitisSpeciesFelineBilePeritonitis || profile.WeightKg < 12.0 {
		jpDrains = 1
	}

	pitfallAlert := "CRITICAL SEPTIC PERITONITIS PITFALL ALERT: NEVER delay broad-spectrum IV antibiotics beyond 1 hour of septic abdominocentesis recognition (Surviving Sepsis Campaign — each hour of delay increases mortality); NEVER add chlorhexidine or povidone-iodine to abdominal lavage fluid (causes fatal chemical peritonitis & serosal necrosis — use ONLY warm sterile 0.9% NaCl at 200-300 mL/kg); and ALWAYS aspirate all residual lavage fluid completely before closing with Jackson-Pratt closed-suction drains (residual saline dilutes opsonins and impairs neutrophil phagocytosis)."

	rationaleParts := []string{
		fmt.Sprintf("Abdominocentesis Blood-to-Fluid Glucose Gradient %.1f mg/dL (diagnostic threshold > 20 mg/dL) and Fluid-to-Blood Lactate Gradient %.2f mmol/L (threshold > 2.0 mmol/L); degenerate neutrophils %d/µL (intracellular bacteria: %t).",
			glucoseGradient, lactateGradient, profile.DegenerateNeutrophilsCountUl, profile.IntracellularBacteriaSeen),
		fmt.Sprintf("Empiric 1-hour IV four-quadrant antimicrobial coverage: Ampicillin/Sulbactam 30 mg/kg IV (%.1f mg q8h) + Enrofloxacin %.0f mg/kg IV (%.1f mg q24h%s).",
			ampSulbactamMg, enroRateMgKg, enrofloxacinMg,
			func() string {
				if profile.Species == domain.PeritonitisSpeciesFelineBilePeritonitis {
					return " — capped at 5 mg/kg/day in cats to prevent acute retinal degeneration"
				}
				return ""
			}()),
		fmt.Sprintf("Exploratory laparotomy source control (%s) + copious warm sterile 0.9%% NaCl abdominal lavage at 300 mL/kg (%.2f L) with complete fluid aspiration and %d closed-suction Jackson-Pratt (JP) fenestrated drain(s).",
			profile.Species, lavageLiters, jpDrains),
	}

	return &domain.SepticPeritonitisSurgicalPlan{
		BloodFluidGlucoseGradientMgDl:     glucoseGradient,
		FluidBloodLactateGradientMmolL:    lactateGradient,
		AmpicillinSulbactam30MgKgMg:       ampSulbactamMg,
		Enrofloxacin10MgKgDoseMg:          enrofloxacinMg,
		WarmSterileSalineLavageLiters:     lavageLiters,
		JacksonPrattClosedSuctionDrains:   jpDrains,
		ZeroAntisepticLavageAdditionGuard: true,
		PeritonitisPitfallAlert:           pitfallAlert,
		ClinicalRationale:                 strings.Join(rationaleParts, " "),
	}
}

func (s *PeritonitisService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.PeritonitisPatientProfile{
		Species:                      domain.PeritonitisSpeciesCanineGIDehiscence,
		Breed:                        "Labrador Retriever",
		WeightKg:                     31.0,
		BloodGlucoseMgDl:             118.0,
		EffusionGlucoseMgDl:          42.0,
		BloodLactateMmolL:            2.4,
		EffusionLactateMmolL:         6.8,
		DegenerateNeutrophilsCountUl: 48000,
		IntracellularBacteriaSeen:    true,
	}
	op101 := &domain.SepticPeritonitisOperation{
		ID:                                 "SP-OP-101",
		IncidentID:                         "INC-SP-9501",
		PetName:                            "Baxter",
		Status:                             domain.PeritonitisStatusSepticEffusionCritical,
		SepticGradientAntibioticDelayAlert: true,
		PatientProfile:                     p101,
		SurgicalPlan:                       BuildSurgicalPlan(p101),
		DrainChecks: []domain.JacksonPrattDrainCheck{
			{
				Timestamp:                     t101.Add(15 * time.Minute),
				EffusionProductionMlKgDay:     8.4,
				CytologyIntracellularBacteria: true,
				DegenerateNeutrophilsPresent:  true,
				SerumAlbuminGdl:               1.8,
				MeanArterialPressureMmHg:      64,
				ClinicianCallsign:             "DR-SURG-14",
				Notes:                         "Post-enterotomy jejunal dehiscence; blood-fluid glucose gradient 76.0 mg/dL, fluid-blood lactate gradient +4.4 mmol/L; intracellular rods in degenerate neutrophils; prepping for immediate exploratory laparotomy & 9.3 L warm sterile saline lavage.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 20, 0, 0, time.UTC)
	p102 := domain.PeritonitisPatientProfile{
		Species:                      domain.PeritonitisSpeciesFelineBilePeritonitis,
		Breed:                        "Domestic Shorthair",
		WeightKg:                     4.5,
		BloodGlucoseMgDl:             134.0,
		EffusionGlucoseMgDl:          88.0,
		BloodLactateMmolL:            1.9,
		EffusionLactateMmolL:         4.6,
		DegenerateNeutrophilsCountUl: 22000,
		IntracellularBacteriaSeen:    true,
	}
	op102 := &domain.SepticPeritonitisOperation{
		ID:                                 "SP-OP-102",
		IncidentID:                         "INC-SP-9502",
		PetName:                            "Nala",
		Status:                             domain.PeritonitisStatusAntibioticLavageJPStaged,
		SepticGradientAntibioticDelayAlert: true,
		PatientProfile:                     p102,
		SurgicalPlan:                       BuildSurgicalPlan(p102),
		DrainChecks: []domain.JacksonPrattDrainCheck{
			{
				Timestamp:                     t102.Add(30 * time.Minute),
				EffusionProductionMlKgDay:     4.2,
				CytologyIntracellularBacteria: false,
				DegenerateNeutrophilsPresent:  true,
				SerumAlbuminGdl:               2.2,
				MeanArterialPressureMmHg:      76,
				ClinicianCallsign:             "DR-ECC-22",
				Notes:                         "Feline septic bile peritonitis cholecystectomy completed; 1.35 L warm 0.9% NaCl lavage aspirated; 1 closed-suction JP drain placed; Ampicillin/Sulbactam 135 mg + feline-safe Enrofloxacin 22.5 mg (5 mg/kg) staged.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(30 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)
	p103 := domain.PeritonitisPatientProfile{
		Species:                      domain.PeritonitisSpeciesCanineGIDehiscence,
		Breed:                        "Catahoula Leopard Dog",
		WeightKg:                     22.0,
		BloodGlucoseMgDl:             105.0,
		EffusionGlucoseMgDl:          98.0,
		BloodLactateMmolL:            1.4,
		EffusionLactateMmolL:         1.6,
		DegenerateNeutrophilsCountUl: 1800,
		IntracellularBacteriaSeen:    false,
	}
	clearance103 := &domain.PeritonitisDrainRemovalClearance{
		ClearedAt:                     t103.Add(48 * time.Hour),
		FinalDrainOutputMlKgDay:       1.4,
		FinalCytologyBacteriaNegative: true,
		FinalSerumAlbuminGdl:          2.6,
		EnteralNutritionTolerated:     true,
		JPDrainAsepticallyRemoved:     true,
		VetLicense:                    "VET-SURG-66109",
		Summary:                       "72h post-resection & anastomosis; JP drain output 1.4 mL/kg/day serosanguinous non-septic fluid; zero intracellular bacteria; serum albumin 2.6 g/dL; tolerating resting energy requirement enteral feeding; both JP drains aseptically pulled.",
	}
	op103 := &domain.SepticPeritonitisOperation{
		ID:                                 "SP-OP-103",
		IncidentID:                         "INC-SP-9503",
		PetName:                            "Gumbo",
		Status:                             domain.PeritonitisStatusNonSepticJPDrainRemoved,
		SepticGradientAntibioticDelayAlert: false,
		PatientProfile:                     p103,
		SurgicalPlan:                       BuildSurgicalPlan(p103),
		DrainChecks: []domain.JacksonPrattDrainCheck{
			{
				Timestamp:                     t103.Add(46 * time.Hour),
				EffusionProductionMlKgDay:     1.4,
				CytologyIntracellularBacteria: false,
				DegenerateNeutrophilsPresent:  false,
				SerumAlbuminGdl:               2.6,
				MeanArterialPressureMmHg:      88,
				ClinicianCallsign:             "DR-SURG-09",
				Notes:                         "Serial 48h JP bulb check: 1.4 mL/kg/day physiological output, non-degenerate cytology negative for bacteria, albumin 2.6 g/dL, MAP 88 mmHg.",
			},
		},
		DrainClearance: clearance103,
		CreatedAt:      t103,
		UpdatedAt:      t103.Add(48 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *PeritonitisService) ListOperations(statusFilter string) []domain.SepticPeritonitisOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.SepticPeritonitisOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, clonePeritonitisOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *PeritonitisService) GetOperation(id string) (*domain.SepticPeritonitisOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("septic peritonitis operation %q not found", id)
	}
	cp := clonePeritonitisOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new septic peritonitis triage operation.
func (s *PeritonitisService) CreateOperation(incidentID, petName string, profile domain.PeritonitisPatientProfile) (*domain.SepticPeritonitisOperation, error) {
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
	opID := fmt.Sprintf("SP-OP-%d", s.seq)
	now := time.Now().UTC()

	septicAlert := IsSepticGradientAlert(profile)
	plan := BuildSurgicalPlan(profile)

	op := &domain.SepticPeritonitisOperation{
		ID:                                 opID,
		IncidentID:                         trimmedIncident,
		PetName:                            trimmedPet,
		Status:                             domain.PeritonitisStatusSepticEffusionCritical,
		SepticGradientAntibioticDelayAlert: septicAlert,
		PatientProfile:                     profile,
		SurgicalPlan:                       plan,
		DrainChecks:                        []domain.JacksonPrattDrainCheck{},
		CreatedAt:                          now,
		UpdatedAt:                          now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := clonePeritonitisOperation(op)
	return &cp, nil
}

// StageSurgicalPlan computes the blood-fluid glucose/lactate gradients, empiric antibiotic doses,
// warm sterile saline lavage volume, and JP drain plan, transitioning the operation to EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED.
func (s *PeritonitisService) StageSurgicalPlan(id string, profile domain.PeritonitisPatientProfile) (*domain.SepticPeritonitisOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("septic peritonitis operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.SurgicalPlan = BuildSurgicalPlan(profile)
	op.SepticGradientAntibioticDelayAlert = IsSepticGradientAlert(profile)
	op.Status = domain.PeritonitisStatusAntibioticLavageJPStaged
	op.DrainClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := clonePeritonitisOperation(op)
	return &cp, nil
}

// RecordDrainCheck appends a serial post-operative Jackson-Pratt closed-suction drain check to an existing operation.
func (s *PeritonitisService) RecordDrainCheck(id string, check domain.JacksonPrattDrainCheck) (*domain.SepticPeritonitisOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("septic peritonitis operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.DrainChecks = append(op.DrainChecks, check)
	op.PatientProfile.IntracellularBacteriaSeen = check.CytologyIntracellularBacteria

	if check.CytologyIntracellularBacteria || check.EffusionProductionMlKgDay > 6.0 || check.MeanArterialPressureMmHg < 65 {
		op.SepticGradientAntibioticDelayAlert = true
		if check.CytologyIntracellularBacteria {
			op.Status = domain.PeritonitisStatusSepticEffusionCritical
			op.DrainClearance = nil
		}
	} else if !check.CytologyIntracellularBacteria && !check.DegenerateNeutrophilsPresent && check.EffusionProductionMlKgDay <= 2.5 {
		op.SepticGradientAntibioticDelayAlert = false
	}

	op.UpdatedAt = now
	cp := clonePeritonitisOperation(op)
	return &cp, nil
}

// IssueDrainClearance validates non-septic cytology, drain production <= 2.0 mL/kg/day, serum albumin >= 2.0 g/dL,
// enteral nutrition tolerance, and aseptic JP drain removal, transitioning to NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED.
func (s *PeritonitisService) IssueDrainClearance(id string, clearance domain.PeritonitisDrainRemovalClearance) (*domain.SepticPeritonitisOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("septic peritonitis operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Non-septic cytology verified; JP drain output %.1f mL/kg/day (<= 2.0 mL/kg/day); serum albumin %.1f g/dL; enteral nutrition tolerated; closed-suction JP drain aseptically removed.", clearance.FinalDrainOutputMlKgDay, clearance.FinalSerumAlbuminGdl)
	}

	op.DrainClearance = &clearance
	op.Status = domain.PeritonitisStatusNonSepticJPDrainRemoved
	op.SepticGradientAntibioticDelayAlert = false
	op.PatientProfile.IntracellularBacteriaSeen = false
	op.UpdatedAt = now

	cp := clonePeritonitisOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all operations in the service.
func (s *PeritonitisService) ComputeMetrics() PeritonitisMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m PeritonitisMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.PeritonitisStatusSepticEffusionCritical:
			m.SepticEffusionCriticalCount++
		case domain.PeritonitisStatusAntibioticLavageJPStaged:
			m.AntibioticLavageJPStagedCount++
		case domain.PeritonitisStatusNonSepticJPDrainRemoved:
			m.NonSepticClearedCount++
		}
		if op.SepticGradientAntibioticDelayAlert {
			m.SepticGradientAlertCount++
		}
	}
	return m
}

func clonePeritonitisOperation(op *domain.SepticPeritonitisOperation) domain.SepticPeritonitisOperation {
	cp := *op
	if op.SurgicalPlan != nil {
		plan := *op.SurgicalPlan
		cp.SurgicalPlan = &plan
	}
	if op.DrainChecks != nil {
		cp.DrainChecks = make([]domain.JacksonPrattDrainCheck, len(op.DrainChecks))
		copy(cp.DrainChecks, op.DrainChecks)
	}
	if op.DrainClearance != nil {
		clr := *op.DrainClearance
		cp.DrainClearance = &clr
	}
	return cp
}
