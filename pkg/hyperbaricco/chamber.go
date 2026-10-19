package hyperbaricco

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	StepCoOximetryFalseSpO2Bypass = "STEP-CO-OXIMETRY-FALSE-SPO2-BYPASS"
	StepHydroxocobalaminCyanide   = "STEP-HYDROXOCOBALAMIN-CYANIDE-BIND"
	StepHBOTChamber2AtaWashout    = "STEP-HBOT-CHAMBER-2ATA-WASHOUT"
	StepDelayedNeuroSequelaeCheck = "STEP-DELAYED-NEURO-SEQUELAE-CHECK"
)

// EvaluateHyperbaricCOTriage deterministically evaluates a hyperbaric oxygen (HBOT), carboxyhemoglobin & cyanide smoke triage request.
func EvaluateHyperbaricCOTriage(req domain.HyperbaricEvaluationRequest) (*domain.HyperbaricCOOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 || req.WeightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be between 0.1 and 120.0, got %.1f", req.WeightKg)
	}
	if !req.ExposureSource.IsValid() {
		return nil, fmt.Errorf("invalid exposure_source: %q", req.ExposureSource)
	}
	if req.CarboxyhemoglobinPct < 0.0 || req.CarboxyhemoglobinPct > 85.0 {
		return nil, fmt.Errorf("carboxyhemoglobin_pct must be between 0.0 and 85.0, got %.1f", req.CarboxyhemoglobinPct)
	}
	if req.BloodLactateMmolL < 0.5 || req.BloodLactateMmolL > 30.0 {
		return nil, fmt.Errorf("blood_lactate_mmol_l must be between 0.5 and 30.0, got %.1f", req.BloodLactateMmolL)
	}
	if req.ChamberPressureAta < 1.0 || req.ChamberPressureAta > 3.0 {
		return nil, fmt.Errorf("chamber_pressure_ata must be between 1.0 and 3.0, got %.1f", req.ChamberPressureAta)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%.1f", petName, req.ExposureSource, req.CarboxyhemoglobinPct, req.BloodLactateMmolL, req.ChamberPressureAta)))
		opID = fmt.Sprintf("HB-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-HB-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.HyperbaricStatusStagedHBOTHydroxocobalamin
	if req.CarboxyhemoglobinPct >= 20.0 ||
		req.BloodLactateMmolL >= 8.0 ||
		req.SingeVibrissaeSootNares ||
		!req.PulseOximetryFalseNormalNoted {
		status = domain.HyperbaricStatusCriticalCOHbCyanide
	}

	regimen := computeRequiredHBOTRegimen(req)
	warningNote := computeCoOximetryWarningNote(req)
	steps := buildHyperbaricProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.HyperbaricCOOperation{
		ID:                            opID,
		PetID:                         petID,
		PetName:                       petName,
		Species:                       species,
		WeightKg:                      req.WeightKg,
		ExposureSource:                req.ExposureSource,
		Status:                        status,
		CarboxyhemoglobinPct:          req.CarboxyhemoglobinPct,
		BloodLactateMmolL:             req.BloodLactateMmolL,
		ChamberPressureAta:            req.ChamberPressureAta,
		SingeVibrissaeSootNares:       req.SingeVibrissaeSootNares,
		PulseOximetryFalseNormalNoted: req.PulseOximetryFalseNormalNoted,
		HydroxocobalaminAdministered:  req.HydroxocobalaminAdministered,
		RequiredHBOTRegimen:           regimen,
		CoOximetryWarningNote:         warningNote,
		ProtocolSteps:                 steps,
		CreatedAt:                     nowStr,
		UpdatedAt:                     nowStr,
	}, nil
}

func computeRequiredHBOTRegimen(req domain.HyperbaricEvaluationRequest) string {
	var parts []string
	switch req.ExposureSource {
	case domain.COExposureStructuralHouseFireSynthetics:
		parts = append(parts, fmt.Sprintf("Monoplace Hyperbaric Oxygen Chamber at %.1f ATA (100%% O2, 60–90 min dive) + Hydroxocobalamin 150 mg/kg IV Cyanide Antidote Infusion", req.ChamberPressureAta))
	case domain.COExposureEnclosedGarageExhaustCO:
		parts = append(parts, fmt.Sprintf("Hyperbaric Chamber Washout at %.1f ATA (Accelerates COHb Half-Life from 320 min to ~23 min) + High-Flow Normobaric 100%% O2 Bridge", req.ChamberPressureAta))
	case domain.COExposurePortableGeneratorCabinLeak:
		parts = append(parts, fmt.Sprintf("Pressurized HBOT Dive at %.1f ATA + Multi-Wavelength Venous CO-Oximetry Serial Washout Verification", req.ChamberPressureAta))
	case domain.COExposureSmolderingBarnTorfSmoke:
		parts = append(parts, fmt.Sprintf("Hyperbaric Oxygenation at %.1f ATA + Nebulized N-Acetylcysteine / Bronchodilator Airway Soot Clearance", req.ChamberPressureAta))
	case domain.COExposurePropaneHeaterRVAsphyxia:
		parts = append(parts, fmt.Sprintf("Hyperbaric Chamber De-Carboxylation at %.1f ATA + Myocardial Troponin & Cerebral Edema Neuro-Protection", req.ChamberPressureAta))
	default:
		parts = append(parts, fmt.Sprintf("Hyperbaric Oxygen Chamber Washout at %.1f ATA with 100%% FiO2", req.ChamberPressureAta))
	}

	if req.SingeVibrissaeSootNares {
		parts = append(parts, "Supraglottic Thermal Edema & Carbonaceous Soot Airway Intubation / Bronchoscopy Standby")
	}
	if req.BloodLactateMmolL >= 8.0 || !req.HydroxocobalaminAdministered {
		parts = append(parts, fmt.Sprintf("Hydroxocobalamin (Vitamin B12a) %.0f mg IV Bolus over 15 min to Chelate Mitochondrial Cyanide into Non-Toxic Cyanocobalamin", req.WeightKg*150.0))
	}
	if !req.PulseOximetryFalseNormalNoted {
		parts = append(parts, "URGENT: Disregard Standard 2-Wavelength Pulse Oximetry SpO2 & Order Immediate Multi-Wavelength Blood Gas CO-Oximetry")
	}
	return strings.Join(parts, " | ")
}

func computeCoOximetryWarningNote(req domain.HyperbaricEvaluationRequest) string {
	base := "CRITICAL DIAGNOSTIC ALERT: Standard two-wavelength pulse oximetry SpO2 falsely reads 98-100% due to cherry-red carboxyhemoglobin absorbance at 660 nm and requires multi-wavelength CO-oximetry plus hydroxocobalamin for cyanide lactatemia."
	if req.CarboxyhemoglobinPct >= 20.0 || req.BloodLactateMmolL >= 8.0 {
		return base + fmt.Sprintf(" Severe COHb (%.1f%%) and blood lactate (%.1f mmol/L) indicate dual carbon monoxide cellular hypoxia and hydrogen cyanide cytochrome c oxidase inhibition: initiate %.1f ATA HBOT washout and IV hydroxocobalamin immediately.", req.CarboxyhemoglobinPct, req.BloodLactateMmolL, req.ChamberPressureAta)
	}
	return base + fmt.Sprintf(" Continue 100%% O2 / %.1f ATA chamber washout until venous COHb < 3.0%% and monitor 14–21 days for delayed neurological sequelae (DNS).", req.ChamberPressureAta)
}

func buildHyperbaricProtocolSteps(req domain.HyperbaricEvaluationRequest) []domain.HyperbaricProtocolStep {
	return []domain.HyperbaricProtocolStep{
		{
			ID:          StepCoOximetryFalseSpO2Bypass,
			Title:       "Bypass False-Normal 2-Wavelength SpO2 & Run Multi-Wavelength CO-Oximetry",
			Description: "Reject standard 660/940 nm pulse oximetry (which falsely reads 98-100% on cherry-red COHb) and measure direct arterial/venous carboxyhemoglobin percentage via 8-wavelength CO-oximeter.",
			Completed:   false,
		},
		{
			ID:          StepHydroxocobalaminCyanide,
			Title:       fmt.Sprintf("Hydroxocobalamin IV Cyanide Chelation (Lactate %.1f mmol/L, Dose %.0f mg)", req.BloodLactateMmolL, req.WeightKg*150.0),
			Description: "Administer hydroxocobalamin 150 mg/kg IV over 15 minutes to bind intracellular cyanide ions from synthetic polymer smoke into renally excreted cyanocobalamin.",
			Completed:   false,
		},
		{
			ID:          StepHBOTChamber2AtaWashout,
			Title:       fmt.Sprintf("Pressurize Monoplace HBOT Chamber to %.1f ATA for COHb Dissociation Washout", req.ChamberPressureAta),
			Description: "Conduct 100% oxygen hyperbaric dive with controlled ear/sinus barotrauma compression and cotton-only anti-static grounding until COHb < 3.0%.",
			Completed:   false,
		},
		{
			ID:          StepDelayedNeuroSequelaeCheck,
			Title:       "Supraglottic Soot Airway Evaluation & Delayed Neurological Sequelae (DNS) Clearance",
			Description: "Verify absence of laryngeal thermal stridor, confirm normalized blood lactate (< 2.0 mmol/L), and schedule 21-day white-matter demyelination follow-up.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a hyperbaric CO operation.
func UpdateProtocolStep(op *domain.HyperbaricCOOperation, stepID string, completed bool) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	trimmedID := strings.TrimSpace(stepID)
	if trimmedID == "" {
		return fmt.Errorf("step_id is required")
	}

	found := false
	nowStr := time.Now().UTC().Format(time.RFC3339)
	for i := range op.ProtocolSteps {
		if op.ProtocolSteps[i].ID == trimmedID {
			op.ProtocolSteps[i].Completed = completed
			if completed {
				op.ProtocolSteps[i].CompletedAt = nowStr
			} else {
				op.ProtocolSteps[i].CompletedAt = ""
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("protocol step %q not found", trimmedID)
	}

	allCompleted := true
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		if op.Status == domain.HyperbaricStatusCriticalCOHbCyanide {
			op.Status = domain.HyperbaricStatusStagedHBOTHydroxocobalamin
		}
	} else {
		if op.Status == domain.HyperbaricStatusClearedSafeNormoxic {
			op.Status = domain.HyperbaricStatusStagedHBOTHydroxocobalamin
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueHyperbaricClearance validates that all 4 protocol steps are completed, transitions the operation
// to NORMOXIC_COHB_CLEARED_SAFE, and generates a deterministic HBOT-SAFE-<12HEX> clearance token.
func IssueHyperbaricClearance(op *domain.HyperbaricCOOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue hyperbaric clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%.1f", op.ID, op.PetName, op.ExposureSource, op.CarboxyhemoglobinPct, op.BloodLactateMmolL, op.ChamberPressureAta)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("HBOT-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.HyperbaricStatusClearedSafeNormoxic
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory hyperbaric oxygen & CO/cyanide smoke triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.HyperbaricCOOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (HB-OP-101, HB-OP-102, HB-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.HyperbaricCOOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateHyperbaricCOTriage(domain.HyperbaricEvaluationRequest{
		ID:                            "HB-OP-101",
		PetID:                         "PET-HB-101",
		PetName:                       "Cinder",
		Species:                       "Cat - Persian",
		WeightKg:                      4.2,
		ExposureSource:                domain.COExposureStructuralHouseFireSynthetics,
		CarboxyhemoglobinPct:          34.5,
		BloodLactateMmolL:             11.2,
		ChamberPressureAta:            2.0,
		SingeVibrissaeSootNares:       true,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  true,
	})
	if err == nil {
		op101.Status = domain.HyperbaricStatusCriticalCOHbCyanide
		op101.CreatedAt = "2026-10-19T00:10:00Z"
		op101.UpdatedAt = "2026-10-19T00:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateHyperbaricCOTriage(domain.HyperbaricEvaluationRequest{
		ID:                            "HB-OP-102",
		PetID:                         "PET-HB-102",
		PetName:                       "Diesel",
		Species:                       "Dog - Boxer",
		WeightKg:                      29.5,
		ExposureSource:                domain.COExposureEnclosedGarageExhaustCO,
		CarboxyhemoglobinPct:          14.0,
		BloodLactateMmolL:             3.4,
		ChamberPressureAta:            1.8,
		SingeVibrissaeSootNares:       false,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  false,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepCoOximetryFalseSpO2Bypass, true)
		_ = UpdateProtocolStep(op102, StepHBOTChamber2AtaWashout, true)
		op102.Status = domain.HyperbaricStatusStagedHBOTHydroxocobalamin
		op102.CreatedAt = "2026-10-19T00:20:00Z"
		op102.UpdatedAt = "2026-10-19T00:28:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateHyperbaricCOTriage(domain.HyperbaricEvaluationRequest{
		ID:                            "HB-OP-103",
		PetID:                         "PET-HB-103",
		PetName:                       "Pebble",
		Species:                       "Dog - Beagle",
		WeightKg:                      11.0,
		ExposureSource:                domain.COExposurePortableGeneratorCabinLeak,
		CarboxyhemoglobinPct:          3.2,
		BloodLactateMmolL:             1.6,
		ChamberPressureAta:            1.5,
		SingeVibrissaeSootNares:       false,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueHyperbaricClearance(op103)
		op103.CreatedAt = "2026-10-18T23:30:00Z"
		op103.UpdatedAt = "2026-10-19T00:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.HyperbaricCOOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.HyperbaricCOOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.HyperbaricCOOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.HyperbaricCOOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.HyperbaricEvaluationRequest) (domain.HyperbaricCOOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("HB-OP-%d", s.seq+1)
	}

	op, err := EvaluateHyperbaricCOTriage(req)
	if err != nil {
		return domain.HyperbaricCOOperation{}, err
	}
	if autoID {
		s.seq++
	}

	if _, exists := s.ops[op.ID]; !exists {
		s.order = append(s.order, op.ID)
	}
	s.ops[op.ID] = op
	return cloneOperation(op), nil
}

// UpdateStep updates a protocol step on an existing operation in the store.
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.HyperbaricCOOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.HyperbaricCOOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.HyperbaricCOOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a normoxic hyperbaric COHb clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.HyperbaricCOOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.HyperbaricCOOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueHyperbaricClearance(op); err != nil {
		return domain.HyperbaricCOOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.HyperbaricCOOperation) domain.HyperbaricCOOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.HyperbaricProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
