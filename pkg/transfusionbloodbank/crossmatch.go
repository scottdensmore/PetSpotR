package transfusionbloodbank

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
	StepABDEA11TypingSlideCrossmatch     = "STEP-AB-DEA11-TYPING-SLIDE-CROSSMATCH"
	Step170MicronFilter09NaClOnlyLine    = "STEP-170MICRON-FILTER-09NACL-ONLY-LINE"
	StepSlow025MlKg15MinBiologicTrial    = "STEP-SLOW-025MLKG-15MIN-BIOLOGIC-TRIAL"
	StepPostPCVTotalSolidsHemolysisCheck = "STEP-POST-PCV-TOTAL-SOLIDS-HEMOLYSIS-CHECK"
)

// EvaluateTransfusionTriage deterministically evaluates a Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction triage request.
func EvaluateTransfusionTriage(req domain.TransfusionEvaluationRequest) (*domain.TransfusionBloodbankOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg < 1.0 || req.WeightKg > 100.0 {
		return nil, fmt.Errorf("weight_kg must be between 1.0 and 100.0, got %.1f", req.WeightKg)
	}
	if !req.BloodGroupType.IsValid() {
		return nil, fmt.Errorf("invalid blood_group_type: %q", req.BloodGroupType)
	}
	if req.RecipientPCVPct < 4.0 || req.RecipientPCVPct > 45.0 {
		return nil, fmt.Errorf("recipient_pcv_pct must be between 4.0 and 45.0, got %.1f", req.RecipientPCVPct)
	}
	if req.DonorPCVPct < 30.0 || req.DonorPCVPct > 65.0 {
		return nil, fmt.Errorf("donor_pcv_pct must be between 30.0 and 65.0, got %.1f", req.DonorPCVPct)
	}
	if req.TargetPCVPct <= req.RecipientPCVPct || req.TargetPCVPct > 55.0 {
		return nil, fmt.Errorf("target_pcv_pct must be greater than recipient_pcv_pct (%.1f) and <= 55.0, got %.1f", req.RecipientPCVPct, req.TargetPCVPct)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		if strings.HasPrefix(string(req.BloodGroupType), "FELINE") || req.BloodGroupType == domain.TransfusionBloodGroupEmergencyCanineToFelineXeno {
			species = "Cat - Domestic Shorthair"
		} else {
			species = "Dog - Mixed Breed"
		}
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%.1f", petName, req.BloodGroupType, req.RecipientPCVPct, req.DonorPCVPct, req.TargetPCVPct)))
		opID = fmt.Sprintf("TB-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-TB-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.TransfusionStatusStagedCrossmatchedMicronFilter
	if req.RecipientPCVPct < 14.0 ||
		!req.MajorMinorCrossmatchCompatible ||
		!req.InLine170MicronFilterAttached ||
		!req.LactatedRingersCoInfusionAvoided ||
		req.BloodGroupType == domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk ||
		req.BloodGroupType == domain.TransfusionBloodGroupEmergencyCanineToFelineXeno {
		status = domain.TransfusionStatusCriticalHemolyticAnaphylaxis
	}

	regimen := computeRequiredTransfusionRegimen(req)
	warningNote := computeAlloantibodyCalciumWarningNote(req)
	steps := buildTransfusionProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.TransfusionBloodbankOperation{
		ID:                               opID,
		PetID:                            petID,
		PetName:                          petName,
		Species:                          species,
		WeightKg:                         req.WeightKg,
		BloodGroupType:                   req.BloodGroupType,
		Status:                           status,
		RecipientPCVPct:                  req.RecipientPCVPct,
		DonorPCVPct:                      req.DonorPCVPct,
		TargetPCVPct:                     req.TargetPCVPct,
		MajorMinorCrossmatchCompatible:   req.MajorMinorCrossmatchCompatible,
		InLine170MicronFilterAttached:    req.InLine170MicronFilterAttached,
		LactatedRingersCoInfusionAvoided: req.LactatedRingersCoInfusionAvoided,
		RequiredTransfusionRegimen:       regimen,
		AlloantibodyCalciumWarningNote:   warningNote,
		ProtocolSteps:                    steps,
		CreatedAt:                        nowStr,
		UpdatedAt:                        nowStr,
	}, nil
}

func computeTransfusionVolumeMl(req domain.TransfusionEvaluationRequest) float64 {
	// Standard whole blood transfusion formula:
	// Blood volume Factor: ~66 mL/kg for cats, ~88 mL/kg for dogs
	factor := 88.0
	lowerSpecies := strings.ToLower(req.Species)
	if strings.Contains(lowerSpecies, "cat") || strings.Contains(lowerSpecies, "feline") ||
		strings.HasPrefix(string(req.BloodGroupType), "FELINE") ||
		req.BloodGroupType == domain.TransfusionBloodGroupEmergencyCanineToFelineXeno {
		factor = 66.0
	}
	if req.DonorPCVPct <= 0 {
		return req.WeightKg * 15.0
	}
	vol := req.WeightKg * factor * ((req.TargetPCVPct - req.RecipientPCVPct) / req.DonorPCVPct)
	if vol < 5.0 {
		vol = 5.0
	}
	return vol
}

func computeRequiredTransfusionRegimen(req domain.TransfusionEvaluationRequest) string {
	volMl := computeTransfusionVolumeMl(req)
	trialRateMlHr := req.WeightKg * 0.25
	var parts []string

	switch req.BloodGroupType {
	case domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk:
		parts = append(parts, fmt.Sprintf("STRICT FELINE TYPE B DONOR pRBC / WHOLE BLOOD ONLY (Calculated Volume %.1f mL; Biologic Trial %.2f mL/hr for 15 min) + Major/Minor Gel/Slide Crossmatch Verification", volMl, trialRateMlHr))
	case domain.TransfusionBloodGroupFelineTypeACrossmatched:
		parts = append(parts, fmt.Sprintf("Type-Specific Feline Type A Crossmatched Transfusion (Calculated Volume %.1f mL; Initial Trial %.2f mL/hr for 15 min via 18–20 Micron Hemo-Nate or 170-Micron Syringe Filter)", volMl, trialRateMlHr))
	case domain.TransfusionBloodGroupCanineDEA11NegativeUniversal:
		parts = append(parts, fmt.Sprintf("Canine DEA 1.1 Negative Universal Donor pRBC / Whole Blood Infusion (Calculated Volume %.1f mL; Biologic Trial %.2f mL/hr for 15 min, Then Escalate to Complete Within 4 Hours)", volMl, trialRateMlHr))
	case domain.TransfusionBloodGroupCanineDEA11PositiveSensitized:
		parts = append(parts, fmt.Sprintf("Sensitized Canine DEA 1.1 Positive Recipient — Mandatory Major/Minor Crossmatch & Leukoreduced pRBC Infusion (Calculated Volume %.1f mL; Slow Trial %.2f mL/hr)", volMl, trialRateMlHr))
	case domain.TransfusionBloodGroupEmergencyCanineToFelineXeno:
		parts = append(parts, fmt.Sprintf("SINGLE LIFESAVING EMERGENCY CANINE-TO-FELINE XENOTRANSFUSION (Calculated Volume %.1f mL; Trial %.2f mL/hr): Expect Delayed Extravascular Hemolysis of Canine RBCs in 4–6 Days — NEVER Repeat Canine Xenotransfusion After Day 4 Due to Fatal Anti-Canine Alloantibody Anaphylaxis", volMl, trialRateMlHr))
	default:
		parts = append(parts, fmt.Sprintf("Type-Matched Crossmatched Blood Transfusion (%.1f mL via 170-Micron Filter)", volMl))
	}

	if !req.MajorMinorCrossmatchCompatible {
		parts = append(parts, "HALT INFUSION IMMEDIATELY: Incompatible Major/Minor Crossmatch Agglutination or Hemolysis Detected — Select Alternative Type-Matched Donor Unit")
	}
	if !req.InLine170MicronFilterAttached {
		parts = append(parts, "MANDATORY HARDWARE: Attach In-Line 170–260 Micron Clot Filter (or 18-Micron Neonatal Hemo-Nate Micro-Aggregate Filter) Prior to Venous Access")
	}
	if !req.LactatedRingersCoInfusionAvoided {
		parts = append(parts, "CONTRAINDICATION ALERT: Disconnect Lactated Ringer's Solution (LRS) Immediately and Flush Line with 0.9% NaCl Only")
	}
	return strings.Join(parts, " | ")
}

func computeAlloantibodyCalciumWarningNote(req domain.TransfusionEvaluationRequest) string {
	base := "CRITICAL IMMUNOHEMATOLOGY & COAGULATION ALERT: Cats possess potent naturally occurring anti-A and anti-B alloantibodies without prior sensitization — administering even 1 mL of Type A blood to a Type B cat (such as Devon Rex, British Shorthair, Abyssinian, or Persian breeds) triggers fatal acute intravascular hemolytic anaphylaxis, apnea, bradycardia, and hemoglobinuria within minutes; furthermore, NEVER co-infuse Calcium-containing Lactated Ringer's Solution (LRS) in the same IV line as citrated CPDA-1 blood products because exogenous calcium chelates the citrate anticoagulant and causes immediate in-line clot formation (use dedicated 0.9% NaCl only)."
	if req.BloodGroupType == domain.TransfusionBloodGroupEmergencyCanineToFelineXeno {
		return base + " XENOTRANSFUSION PROTOCOL: Emergency canine-to-feline xenotransfusion is strictly a once-in-a-lifetime bridge when feline Type B blood is unavailable; anti-canine antibodies form within 4–6 days and any second canine transfusion causes immediate fatal anaphylaxis."
	}
	if req.RecipientPCVPct < 14.0 || !req.MajorMinorCrossmatchCompatible || !req.LactatedRingersCoInfusionAvoided {
		return base + fmt.Sprintf(" Urgent stabilization required for recipient PCV %.1f%% (target %.1f%%).", req.RecipientPCVPct, req.TargetPCVPct)
	}
	return base + fmt.Sprintf(" Recipient PCV %.1f%% staged for crossmatched restoration to %.1f%% using donor unit PCV %.1f%%.", req.RecipientPCVPct, req.TargetPCVPct, req.DonorPCVPct)
}

func buildTransfusionProtocolSteps(req domain.TransfusionEvaluationRequest) []domain.TransfusionProtocolStep {
	volMl := computeTransfusionVolumeMl(req)
	return []domain.TransfusionProtocolStep{
		{
			ID:          StepABDEA11TypingSlideCrossmatch,
			Title:       fmt.Sprintf("Perform Feline AB / Canine DEA 1.1 Immunochromatographic Typing & Major/Minor Crossmatch (PCV %.1f%% → %.1f%%)", req.RecipientPCVPct, req.TargetPCVPct),
			Description: "Confirm recipient and donor blood group typing (DEA 1.1 or Feline A/B/AB) and verify zero macroscopic/microscopic agglutination or hemolysis on major and minor crossmatch.",
			Completed:   false,
		},
		{
			ID:          Step170MicronFilter09NaClOnlyLine,
			Title:       fmt.Sprintf("Prime Dedicated 170-Micron In-Line Filter Set with 0.9%% NaCl Only (Calculated Volume %.1f mL — Zero LRS)", volMl),
			Description: "Attach a dedicated 170–260 micron blood administration filter (or 18-micron Hemo-Nate syringe filter for cats) primed strictly with 0.9% NaCl; never co-infuse calcium-containing Lactated Ringer's Solution in citrated CPDA-1 lines.",
			Completed:   false,
		},
		{
			ID:          StepSlow025MlKg15MinBiologicTrial,
			Title:       fmt.Sprintf("Initiate Slow 0.25 mL/kg/hr (%.2f mL/hr) 15-Minute Biologic Anaphylaxis Trial & q15m Vital Checks", req.WeightKg*0.25),
			Description: "Infuse at 0.25 mL/kg/hr for the first 15 minutes while monitoring rectal temperature, heart rate, respiratory effort, urticaria, vomiting, or vocalization before advancing to the 4-hour completion rate.",
			Completed:   false,
		},
		{
			ID:          StepPostPCVTotalSolidsHemolysisCheck,
			Title:       "Complete Transfusion Within 4 Hours & Verify 1-Hour Post-Infusion Spun PCV / Total Solids & Clear Serum",
			Description: "Finish each blood bag unit within 4 hours of entry to prevent bacterial proliferation, then spin microhematocrit tubes to verify target PCV restoration and absence of icteric/hemolyzed plasma or hemoglobinuria.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a transfusion bloodbank operation.
func UpdateProtocolStep(op *domain.TransfusionBloodbankOperation, stepID string, completed bool) error {
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
		if op.Status == domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
			op.Status = domain.TransfusionStatusStagedCrossmatchedMicronFilter
		}
	} else {
		if op.Status == domain.TransfusionStatusSafePCVRestoredNormothermic {
			op.Status = domain.TransfusionStatusStagedCrossmatchedMicronFilter
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueTransfusionClearance validates that all 4 protocol steps are completed, transitions the operation
// to TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE, and generates a deterministic BLOODBANK-SAFE-<12HEX> clearance token.
func IssueTransfusionClearance(op *domain.TransfusionBloodbankOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue transfusion clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%.1f", op.ID, op.PetName, op.BloodGroupType, op.RecipientPCVPct, op.DonorPCVPct, op.TargetPCVPct)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("BLOODBANK-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.TransfusionStatusSafePCVRestoredNormothermic
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.TransfusionBloodbankOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (TB-OP-101, TB-OP-102, TB-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.TransfusionBloodbankOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateTransfusionTriage(domain.TransfusionEvaluationRequest{
		ID:                               "TB-OP-101",
		PetID:                            "PET-TB-101",
		PetName:                          "Saffron",
		Species:                          "Cat - Devon Rex",
		WeightKg:                         3.2,
		BloodGroupType:                   domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk,
		RecipientPCVPct:                  10.5,
		DonorPCVPct:                      38.0,
		TargetPCVPct:                     24.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	})
	if err == nil {
		op101.Status = domain.TransfusionStatusCriticalHemolyticAnaphylaxis
		op101.CreatedAt = "2026-10-19T01:10:00Z"
		op101.UpdatedAt = "2026-10-19T01:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateTransfusionTriage(domain.TransfusionEvaluationRequest{
		ID:                               "TB-OP-102",
		PetID:                            "PET-TB-102",
		PetName:                          "Ranger",
		Species:                          "Dog - Greyhound",
		WeightKg:                         31.0,
		BloodGroupType:                   domain.TransfusionBloodGroupCanineDEA11NegativeUniversal,
		RecipientPCVPct:                  17.5,
		DonorPCVPct:                      48.0,
		TargetPCVPct:                     30.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepABDEA11TypingSlideCrossmatch, true)
		_ = UpdateProtocolStep(op102, Step170MicronFilter09NaClOnlyLine, true)
		op102.Status = domain.TransfusionStatusStagedCrossmatchedMicronFilter
		op102.CreatedAt = "2026-10-19T01:20:00Z"
		op102.UpdatedAt = "2026-10-19T01:28:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateTransfusionTriage(domain.TransfusionEvaluationRequest{
		ID:                               "TB-OP-103",
		PetID:                            "PET-TB-103",
		PetName:                          "Cleo",
		Species:                          "Cat - Domestic Longhair",
		WeightKg:                         4.4,
		BloodGroupType:                   domain.TransfusionBloodGroupFelineTypeACrossmatched,
		RecipientPCVPct:                  16.0,
		DonorPCVPct:                      40.0,
		TargetPCVPct:                     28.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueTransfusionClearance(op103)
		op103.CreatedAt = "2026-10-19T00:30:00Z"
		op103.UpdatedAt = "2026-10-19T01:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.TransfusionBloodbankOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.TransfusionBloodbankOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.TransfusionBloodbankOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.TransfusionBloodbankOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.TransfusionEvaluationRequest) (domain.TransfusionBloodbankOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("TB-OP-%d", s.seq+1)
	}

	op, err := EvaluateTransfusionTriage(req)
	if err != nil {
		return domain.TransfusionBloodbankOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.TransfusionBloodbankOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.TransfusionBloodbankOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.TransfusionBloodbankOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a transfusion normothermic PCV restoration clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.TransfusionBloodbankOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.TransfusionBloodbankOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueTransfusionClearance(op); err != nil {
		return domain.TransfusionBloodbankOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.TransfusionBloodbankOperation) domain.TransfusionBloodbankOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.TransfusionProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
