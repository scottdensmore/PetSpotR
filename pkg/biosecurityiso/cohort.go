package biosecurityiso

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
	StepAnteroomPPEFootbathLock     = "STEP-ANTEROOM-PPE-FOOTBATH-LOCK"
	StepAHPVirkon10MinDecon         = "STEP-AHP-VIRKON-10MIN-DECON"
	StepNasojejunalMaropitantSepsis = "STEP-NASOJEJUNAL-MAROPITANT-SEPSIS"
	StepPCRSheddingClearanceBath    = "STEP-PCR-SHEDDING-CLEARANCE-BATH"
)

// EvaluateBiosecurityTriage deterministically evaluates a canine parvovirus, panleukopenia, distemper & biosecurity isolation cohort triage request.
func EvaluateBiosecurityTriage(req domain.BiosecurityEvaluationRequest) (*domain.BiosecurityIsolationOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 || req.WeightKg > 100.0 {
		return nil, fmt.Errorf("weight_kg must be between 0.1 and 100.0, got %.1f", req.WeightKg)
	}
	if !req.PathogenType.IsValid() {
		return nil, fmt.Errorf("invalid pathogen_type: %q", req.PathogenType)
	}
	if req.NeutrophilCountCellsUl < 50 || req.NeutrophilCountCellsUl > 40000 {
		return nil, fmt.Errorf("neutrophil_count_cells_ul must be between 50 and 40000, got %d", req.NeutrophilCountCellsUl)
	}
	if req.RoomPressureDifferentialPa < -50.0 || req.RoomPressureDifferentialPa > 20.0 {
		return nil, fmt.Errorf("room_pressure_differential_pa must be between -50.0 and 20.0, got %.1f", req.RoomPressureDifferentialPa)
	}
	if req.DisinfectantContactMins < 1 || req.DisinfectantContactMins > 60 {
		return nil, fmt.Errorf("disinfectant_contact_mins must be between 1 and 60, got %d", req.DisinfectantContactMins)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%.1f|%d", petName, req.PathogenType, req.NeutrophilCountCellsUl, req.RoomPressureDifferentialPa, req.DisinfectantContactMins)))
		opID = fmt.Sprintf("BI-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-BI-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.BiosecurityStatusStagedNegativePressureIsoCohort
	if req.NeutrophilCountCellsUl < 1500 ||
		req.RoomPressureDifferentialPa > -2.5 ||
		req.DisinfectantContactMins < 10 ||
		!req.PotassiumPeroxymonosulfateFootbath ||
		!req.DedicatedPPEGownGlovesEnforced {
		status = domain.BiosecurityStatusCriticalNeutropenicSepsisBreach
	}

	regimen := computeRequiredIsolationRegimen(req)
	failureNote := computeQuatDisinfectantFailureNote(req)
	steps := buildBiosecurityProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.BiosecurityIsolationOperation{
		ID:                                 opID,
		PetID:                              petID,
		PetName:                            petName,
		Species:                            species,
		WeightKg:                           req.WeightKg,
		PathogenType:                       req.PathogenType,
		Status:                             status,
		NeutrophilCountCellsUl:             req.NeutrophilCountCellsUl,
		RoomPressureDifferentialPa:         req.RoomPressureDifferentialPa,
		DisinfectantContactMins:            req.DisinfectantContactMins,
		HemorrhagicDiarrheaPresent:         req.HemorrhagicDiarrheaPresent,
		PotassiumPeroxymonosulfateFootbath: req.PotassiumPeroxymonosulfateFootbath,
		DedicatedPPEGownGlovesEnforced:     req.DedicatedPPEGownGlovesEnforced,
		RequiredIsolationRegimen:           regimen,
		QuatDisinfectantFailureNote:        failureNote,
		ProtocolSteps:                      steps,
		CreatedAt:                          nowStr,
		UpdatedAt:                          nowStr,
	}, nil
}

func computeRequiredIsolationRegimen(req domain.BiosecurityEvaluationRequest) string {
	var parts []string
	switch req.PathogenType {
	case domain.BiosecurityPathogenCanineParvovirusCPV2Enteritis:
		parts = append(parts, fmt.Sprintf("BSL-2 Negative-Pressure Parvovirus Cohort Ward (%.1f Pa) + IV Crystalloid/Colloid Resuscitation, Maropitant 1 mg/kg IV, Early Nasojejunal Trickle Nutrition & Broad-Spectrum Ampicillin/Enrofloxacin Sepsis Shield", req.RoomPressureDifferentialPa))
	case domain.BiosecurityPathogenFelinePanleukopeniaFPV:
		parts = append(parts, fmt.Sprintf("Strict Feline Panleukopenia (FPV) Negative-Pressure Isolation Suite (%.1f Pa) + IV Cefovecin/Ampicillin, Filgrastim (G-CSF) Granulocyte Support & Vitamin B12/Potassium Supplementation", req.RoomPressureDifferentialPa))
	case domain.BiosecurityPathogenCanineDistemperCDVNeuro:
		parts = append(parts, fmt.Sprintf("Airborne Droplet & Fomite Negative-Pressure Distemper Cohort (%.1f Pa, 12 ACH HEPA Exhaust) + Anticonvulsant Levetiracetam Myoclonus Control & Nebulized Airway Humidification", req.RoomPressureDifferentialPa))
	case domain.BiosecurityPathogenLeptospirosisZoonoticRenal:
		parts = append(parts, fmt.Sprintf("Zoonotic Spirochete Urine Containment Isolation (%.1f Pa) + IV Ampicillin 20 mg/kg q6h / Doxycycline 5 mg/kg q12h, Closed Indwelling Urinary Fomite Collection & Face-Shield Splash PPE", req.RoomPressureDifferentialPa))
	case domain.BiosecurityPathogenStrepZooepidemicusHemorrhagic:
		parts = append(parts, fmt.Sprintf("High-Consequence Hemorrhagic Pneumonia Biosecurity Ward (%.1f Pa) + Immediate High-Dose IV Beta-Lactam Antimicrobial Therapy, Oxygen Cage & Strict Multi-Dog Kennel Quarantine", req.RoomPressureDifferentialPa))
	default:
		parts = append(parts, fmt.Sprintf("Negative-Pressure Biosecurity Isolation Suite (%.1f Pa) + Virkon / Accelerated Hydrogen Peroxide 10-Min Wet Contact Decontamination", req.RoomPressureDifferentialPa))
	}

	if req.NeutrophilCountCellsUl < 1500 {
		parts = append(parts, fmt.Sprintf("CRITICAL NEUTROPENIC SEPSIS (%d cells/uL < 1500): Initiate Dual-Agent IV Bactericidal Gram-Negative/Anaerobic Sepsis Coverage Immediately", req.NeutrophilCountCellsUl))
	}
	if req.HemorrhagicDiarrheaPresent {
		parts = append(parts, "Crypt-Epithelial Sloughing & Endotoxin Translocation: Fresh Frozen Plasma / Oncotic Albumin Support + Nasogastric Decompression")
	}
	if req.RoomPressureDifferentialPa > -2.5 {
		parts = append(parts, fmt.Sprintf("URGENT HVAC CONTAINMENT BREACH (%.1f Pa > -2.5 Pa): Restore Negative-Pressure Exhaust Differential >= -5.0 Pa", req.RoomPressureDifferentialPa))
	}
	if req.DisinfectantContactMins < 10 || !req.PotassiumPeroxymonosulfateFootbath || !req.DedicatedPPEGownGlovesEnforced {
		parts = append(parts, "ENFORCE BIOSECURITY LOCK: Mandatory 10-Min AHP/Virkon Wet Contact Time, Potassium Peroxymonosulfate Anteroom Footbath & Dedicated Impervious Gown/Bootie PPE")
	}
	return strings.Join(parts, " | ")
}

func computeQuatDisinfectantFailureNote(req domain.BiosecurityEvaluationRequest) string {
	base := "BIOSECURITY DISINFECTANT FAILURE ALERT: Non-enveloped parvoviruses CPV-2 and FPV are completely resistant to standard quaternary ammonium disinfectants and alcohol, requiring Accelerated Hydrogen Peroxide or Potassium Peroxymonosulfate / Virkon with a full 10-minute wet contact time."
	if req.NeutrophilCountCellsUl < 1500 || req.RoomPressureDifferentialPa > -2.5 || req.DisinfectantContactMins < 10 {
		return base + fmt.Sprintf(" Current neutrophil count (%d cells/uL), room pressure differential (%.1f Pa), and disinfectant contact time (%d min) indicate acute neutropenic sepsis or biosecurity containment breach risk.", req.NeutrophilCountCellsUl, req.RoomPressureDifferentialPa, req.DisinfectantContactMins)
	}
	return base + fmt.Sprintf(" Negative-pressure anteroom (%.1f Pa) and %d-minute Virkon/AHP virucidal contact protocol verified.", req.RoomPressureDifferentialPa, req.DisinfectantContactMins)
}

func buildBiosecurityProtocolSteps(req domain.BiosecurityEvaluationRequest) []domain.BiosecurityProtocolStep {
	return []domain.BiosecurityProtocolStep{
		{
			ID:          StepAnteroomPPEFootbathLock,
			Title:       fmt.Sprintf("Enforce Negative-Pressure Anteroom (%.1f Pa), Dedicated PPE & Virkon Footbath Lock", req.RoomPressureDifferentialPa),
			Description: "Verify negative-pressure airflow differential (<= -2.5 Pa), don dedicated impervious gown, nitrile gloves, and boot covers, and step through 1% Potassium Peroxymonosulfate (Virkon-S) anteroom footbath.",
			Completed:   false,
		},
		{
			ID:          StepAHPVirkon10MinDecon,
			Title:       fmt.Sprintf("Replace Quaternary Ammonium with 10-Min AHP / Virkon Wet Contact Decon (%d min)", req.DisinfectantContactMins),
			Description: "Prohibit ineffective quaternary ammonium and alcohol sprays; apply Accelerated Hydrogen Peroxide (Rescue) or Potassium Peroxymonosulfate (Virkon) with >= 10 minutes continuous wet contact on all surfaces and fomites.",
			Completed:   false,
		},
		{
			ID:          StepNasojejunalMaropitantSepsis,
			Title:       fmt.Sprintf("Neutropenic Sepsis IV Resuscitation, Maropitant & Enteral Support (Neutrophils %d/uL)", req.NeutrophilCountCellsUl),
			Description: "Administer IV balanced crystalloids/colloids, maropitant 1 mg/kg IV antiemetic, broad-spectrum IV bactericidal antibiotics for leukopenia, and early nasoesophageal/nasojejunal enterocyte micro-nutrition.",
			Completed:   false,
		},
		{
			ID:          StepPCRSheddingClearanceBath,
			Title:       "Serial Quantitative Fecal/Urine PCR Negative Shedding & Virucidal Exit Bath",
			Description: "Confirm negative fecal/respiratory/urine real-time PCR shedding status, perform full warm chlorhexidine/virucidal coat decontamination bath, and transfer to clean recovery ward.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a biosecurity isolation operation.
func UpdateProtocolStep(op *domain.BiosecurityIsolationOperation, stepID string, completed bool) error {
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
		if op.Status == domain.BiosecurityStatusCriticalNeutropenicSepsisBreach {
			op.Status = domain.BiosecurityStatusStagedNegativePressureIsoCohort
		}
	} else {
		if op.Status == domain.BiosecurityStatusClearedSafePCRNegativeDecon {
			op.Status = domain.BiosecurityStatusStagedNegativePressureIsoCohort
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueBiosecurityClearance validates that all 4 protocol steps are completed, transitions the operation
// to PCR_NEGATIVE_DECON_CLEARED, and generates a deterministic BIOSEC-SAFE-<12HEX> clearance token.
func IssueBiosecurityClearance(op *domain.BiosecurityIsolationOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue biosecurity clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%d|%.1f|%d", op.ID, op.PetName, op.PathogenType, op.NeutrophilCountCellsUl, op.RoomPressureDifferentialPa, op.DisinfectantContactMins)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("BIOSEC-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.BiosecurityStatusClearedSafePCRNegativeDecon
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory biosecurity isolation cohort triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.BiosecurityIsolationOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (BI-OP-101, BI-OP-102, BI-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.BiosecurityIsolationOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateBiosecurityTriage(domain.BiosecurityEvaluationRequest{
		ID:                                 "BI-OP-101",
		PetID:                              "PET-BI-101",
		PetName:                            "Baxter",
		Species:                            "Dog - Rottweiler Juvenile",
		WeightKg:                           14.2,
		PathogenType:                       domain.BiosecurityPathogenCanineParvovirusCPV2Enteritis,
		NeutrophilCountCellsUl:             620,
		RoomPressureDifferentialPa:         -5.0,
		DisinfectantContactMins:            10,
		HemorrhagicDiarrheaPresent:         true,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	})
	if err == nil {
		op101.Status = domain.BiosecurityStatusCriticalNeutropenicSepsisBreach
		op101.CreatedAt = "2026-10-19T00:10:00Z"
		op101.UpdatedAt = "2026-10-19T00:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateBiosecurityTriage(domain.BiosecurityEvaluationRequest{
		ID:                                 "BI-OP-102",
		PetID:                              "PET-BI-102",
		PetName:                            "Mittens",
		Species:                            "Cat - Domestic Shorthair",
		WeightKg:                           3.1,
		PathogenType:                       domain.BiosecurityPathogenFelinePanleukopeniaFPV,
		NeutrophilCountCellsUl:             2450,
		RoomPressureDifferentialPa:         -6.5,
		DisinfectantContactMins:            12,
		HemorrhagicDiarrheaPresent:         false,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepAnteroomPPEFootbathLock, true)
		_ = UpdateProtocolStep(op102, StepAHPVirkon10MinDecon, true)
		op102.Status = domain.BiosecurityStatusStagedNegativePressureIsoCohort
		op102.CreatedAt = "2026-10-19T00:20:00Z"
		op102.UpdatedAt = "2026-10-19T00:28:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateBiosecurityTriage(domain.BiosecurityEvaluationRequest{
		ID:                                 "BI-OP-103",
		PetID:                              "PET-BI-103",
		PetName:                            "Otis",
		Species:                            "Dog - Labrador Retriever",
		WeightKg:                           27.8,
		PathogenType:                       domain.BiosecurityPathogenLeptospirosisZoonoticRenal,
		NeutrophilCountCellsUl:             5800,
		RoomPressureDifferentialPa:         -8.0,
		DisinfectantContactMins:            15,
		HemorrhagicDiarrheaPresent:         false,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueBiosecurityClearance(op103)
		op103.CreatedAt = "2026-10-18T23:30:00Z"
		op103.UpdatedAt = "2026-10-19T00:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.BiosecurityIsolationOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.BiosecurityIsolationOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.BiosecurityIsolationOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.BiosecurityIsolationOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.BiosecurityEvaluationRequest) (domain.BiosecurityIsolationOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("BI-OP-%d", s.seq+1)
	}

	op, err := EvaluateBiosecurityTriage(req)
	if err != nil {
		return domain.BiosecurityIsolationOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.BiosecurityIsolationOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.BiosecurityIsolationOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.BiosecurityIsolationOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a PCR-negative biosecurity clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.BiosecurityIsolationOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.BiosecurityIsolationOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueBiosecurityClearance(op); err != nil {
		return domain.BiosecurityIsolationOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.BiosecurityIsolationOperation) domain.BiosecurityIsolationOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.BiosecurityProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
