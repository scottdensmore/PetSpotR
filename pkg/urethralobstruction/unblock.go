package urethralobstruction

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
	StepCalciumGluconateInsulinDextrose = "STEP-CALCIUM-GLUCONATE-INSULIN-DEXTROSE"
	StepSacrococcygealRetrohydro        = "STEP-SACROCOCCYGEAL-BLOCK-RETROHYDROPROPULSION"
	StepClosedCollectionDiuresis        = "STEP-CLOSED-COLLECTION-POST-OBSTRUCTIVE-DIURESIS"
	StepPrazosinNormoKClearance         = "STEP-PRAZOSIN-NORMO-K-CLEARANCE"
)

// EvaluateUrethralTriage deterministically evaluates a Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection triage request.
func EvaluateUrethralTriage(req domain.UrethralEvaluationRequest) (*domain.UrethralObstructionOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg < 1.5 || req.WeightKg > 65.0 {
		return nil, fmt.Errorf("weight_kg must be between 1.5 and 65.0, got %.1f", req.WeightKg)
	}
	if !req.ObstructionCause.IsValid() {
		return nil, fmt.Errorf("invalid obstruction_cause: %q", req.ObstructionCause)
	}
	if req.SerumPotassiumMmolL < 2.5 || req.SerumPotassiumMmolL > 12.5 {
		return nil, fmt.Errorf("serum_potassium_mmol_l must be between 2.5 and 12.5, got %.1f", req.SerumPotassiumMmolL)
	}
	if req.HeartRateBpm < 60 || req.HeartRateBpm > 260 {
		return nil, fmt.Errorf("heart_rate_bpm must be between 60 and 260, got %d", req.HeartRateBpm)
	}
	if req.BladderPalpationDiameterCm < 2.0 || req.BladderPalpationDiameterCm > 20.0 {
		return nil, fmt.Errorf("bladder_palpation_diameter_cm must be between 2.0 and 20.0, got %.1f", req.BladderPalpationDiameterCm)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Cat - Domestic Shorthair Neutered Male"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%d|%.1f", petName, req.ObstructionCause, req.SerumPotassiumMmolL, req.HeartRateBpm, req.BladderPalpationDiameterCm)))
		opID = fmt.Sprintf("UO-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-UO-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.UrethralStatusStagedCalciumRetrohydropropulsion
	if req.SerumPotassiumMmolL >= 7.0 ||
		req.HeartRateBpm < 140 ||
		req.AbsentPWaveTentedTWave ||
		!req.CalciumGluconateBeforeSedation ||
		!req.ManualBladderExpressionAvoided {
		status = domain.UrethralStatusCriticalHyperkalemicBradycardia
	}

	protocol := computeRequiredUnblockingProtocol(req)
	warningNote := computeCardioprotectionWarningNote(req)
	steps := buildUrethralProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.UrethralObstructionOperation{
		ID:                             opID,
		PetID:                          petID,
		PetName:                        petName,
		Species:                        species,
		WeightKg:                       req.WeightKg,
		ObstructionCause:               req.ObstructionCause,
		Status:                         status,
		SerumPotassiumMmolL:            req.SerumPotassiumMmolL,
		HeartRateBpm:                   req.HeartRateBpm,
		BladderPalpationDiameterCm:     req.BladderPalpationDiameterCm,
		AbsentPWaveTentedTWave:         req.AbsentPWaveTentedTWave,
		CalciumGluconateBeforeSedation: req.CalciumGluconateBeforeSedation,
		ManualBladderExpressionAvoided: req.ManualBladderExpressionAvoided,
		RequiredUnblockingProtocol:     protocol,
		CardioprotectionWarningNote:    warningNote,
		ProtocolSteps:                  steps,
		CreatedAt:                      nowStr,
		UpdatedAt:                      nowStr,
	}, nil
}

func computeRequiredUnblockingProtocol(req domain.UrethralEvaluationRequest) string {
	var parts []string
	caGluconateMl := req.WeightKg * 0.5
	switch req.ObstructionCause {
	case domain.ObstructionCauseStruwiteCrystallineMatrixPlug:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate 0.5 mL/kg Slow IV (%.1f mL over 10–15 min) + Regular Insulin 0.5 U + 50%% Dextrose 2 g/U IV + Sacrococcygeal (Co1–Co2) Epidural Block + Atraumatic Distal Penile Massage & Sterile Saline/Lube Retrohydropropulsion via 3.5 Fr Open-Ended Tomcat Catheter", caGluconateMl))
	case domain.ObstructionCauseCalciumOxalateUrethrolith:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate Cardioprotection (%.1f mL slow IV) + Decompressive 22G Oblique Cystocentesis + High-Volume Sterile Saline Retrohydropropulsion into Urinary Bladder for Cystotomy Removal", caGluconateMl))
	case domain.ObstructionCauseIdiopathicFICUrethralSpasm:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate Cardioprotection (%.1f mL IV) + Sacrococcygeal Epidural Coccygeal Block + Buprenorphine/Midazolam Opioid Analgesia + Prazosin / Acepromazine Urethral Smooth-Muscle Spasmolysis + Soft 3.5 Fr Slippery Sam Indwelling Catheterization", caGluconateMl))
	case domain.ObstructionCauseCanineOsPenisCalculusLodgment:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate IV Stabilization (%.1f mL) + Distal Urethral Occlusion & Pulsatile Saline Retrohydropropulsion Past Base of Os Penis into Urinary Bladder + Lateral Survey Radiograph", caGluconateMl))
	case domain.ObstructionCausePostCatheterStricturedReobstruction:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate Cardioprotection (%.1f mL IV) + Positive-Contrast Retrograde Urethrogram + Decompressive Cystocentesis & Perineal Urethrostomy (PU) Surgical Staging", caGluconateMl))
	default:
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate Slow IV (%.1f mL) + Sacrococcygeal Epidural + Atraumatic Retrohydropropulsion", caGluconateMl))
	}

	if req.SerumPotassiumMmolL >= 7.0 || req.AbsentPWaveTentedTWave || req.HeartRateBpm < 140 {
		parts = append(parts, fmt.Sprintf("HYPERKALEMIC ATRIAL STANDSTILL / BRADYCARDIA RESCUE (K+ %.1f mmol/L, HR %d bpm): Infuse 10%% Calcium Gluconate 0.5–1.0 mL/kg IV immediately to antagonize myocardial excitability, followed by Regular Insulin + Dextrose 2.5%% CRI and K+-free 0.9%% NaCl / Balanced Crystalloid IV", req.SerumPotassiumMmolL, req.HeartRateBpm))
	}
	if !req.CalciumGluconateBeforeSedation {
		parts = append(parts, "STOP SEDATION: Administer 10% Calcium Gluconate IV BEFORE any propofol, alfaxalone, or inhalant induction")
	}
	if !req.ManualBladderExpressionAvoided {
		parts = append(parts, "STOP MANUAL EXPRESSION: Perform oblique 22G decompressive cystocentesis instead of forceful manual compression to prevent devitalized bladder rupture")
	}
	return strings.Join(parts, " | ")
}

func computeCardioprotectionWarningNote(req domain.UrethralEvaluationRequest) string {
	base := "CRITICAL CARDIOPROTECTION & UROABDOMEN ALERT: NEVER induce general anesthesia or heavy sedation before stabilizing hyperkalemic bradycardia and atrial standstill with 10% Calcium Gluconate slow IV (0.5–1.0 mL/kg over 10–15 min with continuous Lead II ECG) plus Regular Insulin + Dextrose IV, and NEVER attempt forceful manual expression of a turgid obstructed feline bladder because ischemic detrusor necrosis causes catastrophic bladder rupture and lethal hyperkalemic uroabdomen."
	if req.SerumPotassiumMmolL >= 7.0 || req.AbsentPWaveTentedTWave || req.HeartRateBpm < 140 {
		return base + fmt.Sprintf(" Serum K+ (%.1f mmol/L) and heart rate (%d bpm) indicate life-threatening myocardial conduction block (absent P waves / tented T waves / wide QRS) — administer 10%% Calcium Gluconate IV immediately.", req.SerumPotassiumMmolL, req.HeartRateBpm)
	}
	return base + fmt.Sprintf(" Serum K+ is currently %.1f mmol/L (HR %d bpm); maintain closed urine collection bag and track ins-and-outs every 2 hours for massive post-obstructive diuresis and rebound hypokalemia.", req.SerumPotassiumMmolL, req.HeartRateBpm)
}

func buildUrethralProtocolSteps(req domain.UrethralEvaluationRequest) []domain.UrethralProtocolStep {
	return []domain.UrethralProtocolStep{
		{
			ID:          StepCalciumGluconateInsulinDextrose,
			Title:       fmt.Sprintf("10%% Calcium Gluconate Slow IV (%.1f mL) & Regular Insulin + Dextrose Cardioprotection (K+ %.1f mmol/L)", req.WeightKg*0.5, req.SerumPotassiumMmolL),
			Description: "Infuse 10% Calcium Gluconate (0.5–1.0 mL/kg IV over 10–15 min under Lead II ECG) to restore myocardial resting potential vs threshold potential, then shift K+ intracellularly with Regular Insulin (0.5 U/cat IV) + Dextrose (2 g/U IV + 2.5% CRI) before sedation.",
			Completed:   false,
		},
		{
			ID:          StepSacrococcygealRetrohydro,
			Title:       fmt.Sprintf("Sacrococcygeal (Co1–Co2) Epidural Block & Atraumatic Retrohydropropulsion (Bladder %.1f cm)", req.BladderPalpationDiameterCm),
			Description: "Perform a sacrococcygeal epidural with preservative-free 2% lidocaine (0.1–0.2 mL/kg) to relax the external urethral sphincter, massage the distal penis to dislodge struvite matrix plugs, and flush sterile saline/lubricant via an open-ended 3.5 Fr catheter (or perform 22G oblique decompressive cystocentesis — never forceful manual expression).",
			Completed:   false,
		},
		{
			ID:          StepClosedCollectionDiuresis,
			Title:       "Indwelling Soft Red-Rubber / Slippery Sam Catheter & Closed Collection Post-Obstructive Diuresis",
			Description: "Suture a soft atraumatic indwelling urethral catheter connected to a sterile closed collection system, measure urine output every 2–4 hours (often > 2–10 mL/kg/hr post-obstructive diuresis), and match IV fluid ins-and-outs plus potassium supplementation once K+ falls below 4.0 mmol/L.",
			Completed:   false,
		},
		{
			ID:          StepPrazosinNormoKClearance,
			Title:       fmt.Sprintf("Urethral Spasmolytic Therapy, Sinus Rhythm (HR %d bpm) & Normokalemic Spontaneous Voiding Clearance", req.HeartRateBpm),
			Description: "Confirm resolution of azotemia and normokalemia (3.5–5.0 mmol/L) with normal P-QRS-T sinus complexes, transition to clear non-hemorrhagic urine output, remove indwelling catheter, and verify strong spontaneous voiding stream prior to discharge.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a urethral obstruction operation.
func UpdateProtocolStep(op *domain.UrethralObstructionOperation, stepID string, completed bool) error {
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
		if op.Status == domain.UrethralStatusCriticalHyperkalemicBradycardia {
			op.Status = domain.UrethralStatusStagedCalciumRetrohydropropulsion
		}
	} else {
		if op.Status == domain.UrethralStatusClearedIndwellingNormokalemic {
			op.Status = domain.UrethralStatusStagedCalciumRetrohydropropulsion
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueUrethralClearance validates that all 4 protocol steps are completed, transitions the operation
// to INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED, and generates a deterministic URETHRAL-SAFE-<12HEX> clearance token.
func IssueUrethralClearance(op *domain.UrethralObstructionOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue urethral clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%d|%.1f", op.ID, op.PetName, op.ObstructionCause, op.SerumPotassiumMmolL, op.HeartRateBpm, op.BladderPalpationDiameterCm)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("URETHRAL-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.UrethralStatusClearedIndwellingNormokalemic
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory Feline Urethral Obstruction, Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.UrethralObstructionOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (UO-OP-101, UO-OP-102, UO-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.UrethralObstructionOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateUrethralTriage(domain.UrethralEvaluationRequest{
		ID:                             "UO-OP-101",
		PetID:                          "PET-UO-101",
		PetName:                        "Sylvester",
		Species:                        "Cat - Domestic Shorthair Neutered Male",
		WeightKg:                       5.8,
		ObstructionCause:               domain.ObstructionCauseStruwiteCrystallineMatrixPlug,
		SerumPotassiumMmolL:            8.9,
		HeartRateBpm:                   118,
		BladderPalpationDiameterCm:     9.5,
		AbsentPWaveTentedTWave:         true,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	})
	if err == nil {
		op101.Status = domain.UrethralStatusCriticalHyperkalemicBradycardia
		op101.CreatedAt = "2026-10-19T00:10:00Z"
		op101.UpdatedAt = "2026-10-19T00:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateUrethralTriage(domain.UrethralEvaluationRequest{
		ID:                             "UO-OP-102",
		PetID:                          "PET-UO-102",
		PetName:                        "Figaro",
		Species:                        "Cat - Siamese Neutered Male",
		WeightKg:                       4.7,
		ObstructionCause:               domain.ObstructionCauseIdiopathicFICUrethralSpasm,
		SerumPotassiumMmolL:            5.6,
		HeartRateBpm:                   172,
		BladderPalpationDiameterCm:     6.8,
		AbsentPWaveTentedTWave:         false,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepCalciumGluconateInsulinDextrose, true)
		_ = UpdateProtocolStep(op102, StepSacrococcygealRetrohydro, true)
		op102.Status = domain.UrethralStatusStagedCalciumRetrohydropropulsion
		op102.CreatedAt = "2026-10-19T00:20:00Z"
		op102.UpdatedAt = "2026-10-19T00:28:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateUrethralTriage(domain.UrethralEvaluationRequest{
		ID:                             "UO-OP-103",
		PetID:                          "PET-UO-103",
		PetName:                        "Chester",
		Species:                        "Cat - Orange Tabby Neutered Male",
		WeightKg:                       6.1,
		ObstructionCause:               domain.ObstructionCauseCalciumOxalateUrethrolith,
		SerumPotassiumMmolL:            4.2,
		HeartRateBpm:                   184,
		BladderPalpationDiameterCm:     4.0,
		AbsentPWaveTentedTWave:         false,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueUrethralClearance(op103)
		op103.CreatedAt = "2026-10-18T23:30:00Z"
		op103.UpdatedAt = "2026-10-19T00:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.UrethralObstructionOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.UrethralObstructionOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.UrethralObstructionOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.UrethralObstructionOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.UrethralEvaluationRequest) (domain.UrethralObstructionOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("UO-OP-%d", s.seq+1)
	}

	op, err := EvaluateUrethralTriage(req)
	if err != nil {
		return domain.UrethralObstructionOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.UrethralObstructionOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.UrethralObstructionOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.UrethralObstructionOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues an indwelling closed diuresis & normokalemic clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.UrethralObstructionOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.UrethralObstructionOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueUrethralClearance(op); err != nil {
		return domain.UrethralObstructionOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.UrethralObstructionOperation) domain.UrethralObstructionOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.UrethralProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
