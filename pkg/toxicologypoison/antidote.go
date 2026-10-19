package toxicologypoison

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
	StepApomorphineRopiniroleEmesisGate = "STEP-APOMORPHINE-ROPIFINIROLE-EMESIS-GATE"
	StepPhytonadioneK1Or20PctILELipid   = "STEP-PHYTONADIONE-K1-OR-20PCT-ILE-LIPID"
	StepDextroseNACSAMe48HrIVDiuresis   = "STEP-DEXTROSE-NAC-SAME-48HR-IV-DIURESIS"
	Step48HrPTRecheckRenalHepaticPanel  = "STEP-48HR-PT-RECHECK-RENAL-HEPATIC-PANEL"
)

// EvaluateToxicologyTriage deterministically evaluates an Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control triage request.
func EvaluateToxicologyTriage(req domain.ToxicologyEvaluationRequest) (*domain.ToxicologyPoisonOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg < 0.8 || req.WeightKg > 100.0 {
		return nil, fmt.Errorf("weight_kg must be between 0.8 and 100.0, got %.1f", req.WeightKg)
	}
	if !req.AgentType.IsValid() {
		return nil, fmt.Errorf("invalid agent_type: %q", req.AgentType)
	}
	if req.EstimatedDoseMgKg <= 0 || req.EstimatedDoseMgKg > 5000.0 {
		return nil, fmt.Errorf("estimated_dose_mg_kg must be between 0.1 and 5000.0, got %.1f", req.EstimatedDoseMgKg)
	}
	if req.ProthrombinTimeRatio < 0.8 || req.ProthrombinTimeRatio > 10.0 {
		return nil, fmt.Errorf("prothrombin_time_ratio must be between 0.8 and 10.0, got %.1f", req.ProthrombinTimeRatio)
	}
	if req.BloodGlucoseMgDl < 15.0 || req.BloodGlucoseMgDl > 400.0 {
		return nil, fmt.Errorf("blood_glucose_mg_dl must be between 15.0 and 400.0, got %.1f", req.BloodGlucoseMgDl)
	}
	if req.TimeSinceIngestionMins < 1 || req.TimeSinceIngestionMins > 10080 {
		return nil, fmt.Errorf("time_since_ingestion_mins must be between 1 and 10080, got %d", req.TimeSinceIngestionMins)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%.1f|%d", petName, req.AgentType, req.EstimatedDoseMgKg, req.ProthrombinTimeRatio, req.BloodGlucoseMgDl, req.TimeSinceIngestionMins)))
		opID = fmt.Sprintf("TP-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-TP-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon
	if req.ProthrombinTimeRatio >= 1.5 ||
		req.BloodGlucoseMgDl < 60.0 ||
		!req.CharcoalAvoidedForXylitol ||
		!req.VitaminK3MenadioneAvoided ||
		req.AgentType == domain.ToxicologyAgentLiliumSppFelineNephrotoxin ||
		req.AgentType == domain.ToxicologyAgentBromethalinNeurotoxicCerebralEdema {
		status = domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro
	}

	regimen := computeRequiredAntidoteDeconRegimen(req)
	warningNote := computeToxicologyPitfallWarningNote(req)
	steps := buildToxicologyProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.ToxicologyPoisonOperation{
		ID:                           opID,
		PetID:                        petID,
		PetName:                      petName,
		Species:                      species,
		WeightKg:                     req.WeightKg,
		AgentType:                    req.AgentType,
		Status:                       status,
		EstimatedDoseMgKg:            req.EstimatedDoseMgKg,
		ProthrombinTimeRatio:         req.ProthrombinTimeRatio,
		BloodGlucoseMgDl:             req.BloodGlucoseMgDl,
		TimeSinceIngestionMins:       req.TimeSinceIngestionMins,
		CharcoalAvoidedForXylitol:    req.CharcoalAvoidedForXylitol,
		VitaminK3MenadioneAvoided:    req.VitaminK3MenadioneAvoided,
		RequiredAntidoteDeconRegimen: regimen,
		ToxicologyPitfallWarningNote: warningNote,
		ProtocolSteps:                steps,
		CreatedAt:                    nowStr,
		UpdatedAt:                    nowStr,
	}, nil
}

func computeRequiredAntidoteDeconRegimen(req domain.ToxicologyEvaluationRequest) string {
	var parts []string
	switch req.AgentType {
	case domain.ToxicologyAgentSecondGenAnticoagulantBrodifacoum:
		k1Mg := req.WeightKg * 2.5
		parts = append(parts, fmt.Sprintf("Phytonadione (Vitamin K1) 2.5–5.0 mg/kg PO/SC (%.1f mg initial dose) with Fatty Meal for 28–30 Days (Second-Generation Long-Acting Brodifacoum) + Fresh Frozen Plasma (FFP 10–15 mL/kg) if Active Hemorrhage or PT Ratio %.1f >= 1.5", k1Mg, req.ProthrombinTimeRatio))
	case domain.ToxicologyAgentXylitolInsulinSurgeHepatotoxin:
		dexBolusMl := req.WeightKg * 1.0
		nacLoadMg := req.WeightKg * 140.0
		parts = append(parts, fmt.Sprintf("50%% Dextrose Diluted 1:4 IV Bolus (%.1f mL) + 2.5%%–5.0%% Dextrose CRI + N-Acetylcysteine (NAC %.0f mg IV load then 70 mg/kg q6h) & SAMe/Silybin Hepatoprotection (SKIP Activated Charcoal — Does Not Bind Xylitol)", dexBolusMl, nacLoadMg))
	case domain.ToxicologyAgentBromethalinNeurotoxicCerebralEdema:
		ileBolusMl := req.WeightKg * 1.5
		parts = append(parts, fmt.Sprintf("Multi-Dose Activated Charcoal (MDAC 1 g/kg q8h for Enterohepatic Recirculation) + 20%% Intravenous Lipid Emulsion (ILE %.1f mL IV bolus + 0.25 mL/kg/min CRI) + Mannitol 0.5–1.0 g/kg IV for Uncoupled Oxidative Phosphorylation Cerebral Edema", ileBolusMl))
	case domain.ToxicologyAgentLiliumSppFelineNephrotoxin:
		fluidRateMlHr := req.WeightKg * 6.0
		parts = append(parts, fmt.Sprintf("Immediate Emesis (Dexmedetomidine/Xylazine in Cats) + Activated Charcoal + Aggressive Isotonic Crystalloid IV Diuresis (2–3x Maintenance ~%.1f mL/hr) for 48–72 Hours to Prevent Anuric Proximal Renal Tubular Necrosis", fluidRateMlHr))
	case domain.ToxicologyAgentIvermectinPermethrinLipophilic:
		ileBolusMl := req.WeightKg * 1.5
		parts = append(parts, fmt.Sprintf("20%% Intravenous Lipid Emulsion (ILE 'Lipid Sink' %.1f mL IV bolus over 1 min + 0.25 mL/kg/min CRI for 30–60 min) + Methocarbamol 55–220 mg/kg IV for Permethrin Tremors + Dermal Bathing with Mild Dish Soap", ileBolusMl))
	default:
		parts = append(parts, "Targeted Antidote & Decontamination Protocol")
	}

	if req.BloodGlucoseMgDl < 60.0 {
		parts = append(parts, fmt.Sprintf("URGENT HYPOGLYCEMIA RESCUE (BG %.1f mg/dL < 60 mg/dL): Administer 0.5–1.0 mL/kg 50%% Dextrose diluted 1:4 IV over 5 min and check serial blood glucose q1–2h", req.BloodGlucoseMgDl))
	}
	if req.ProthrombinTimeRatio >= 1.5 {
		parts = append(parts, fmt.Sprintf("ACTIVE COAGULOPATHY ALERT (PT Ratio %.1f >= 1.5x baseline): Transfuse Fresh Frozen Plasma (10–20 mL/kg IV) + Phytonadione Vitamin K1", req.ProthrombinTimeRatio))
	}
	if !req.CharcoalAvoidedForXylitol {
		parts = append(parts, "CONTRAINDICATION: Abort Activated Charcoal for Xylitol — Poorly Adsorbed & Causes Aspiration/Vomiting")
	}
	if !req.VitaminK3MenadioneAvoided {
		parts = append(parts, "CONTRAINDICATION: Never Administer Vitamin K3 (Menadione) — Ineffective & Causes Heinz-Body Hemolytic Anemia / Nephrotoxicity")
	}

	return strings.Join(parts, " | ")
}

func computeToxicologyPitfallWarningNote(req domain.ToxicologyEvaluationRequest) string {
	base := "CRITICAL TOXICOLOGY & ANTIDOTE PITFALL ALERT: Activated charcoal does NOT bind xylitol and delays life-saving IV dextrose, N-acetylcysteine (NAC), and SAMe hepatoprotective therapy while increasing aspiration pneumonia risk; furthermore, Vitamin K3 (menadione) is completely ineffective and nephrotoxic/hemolytic for brodifacoum or second-generation anticoagulant rodenticide poisoning — always administer Phytonadione (Vitamin K1) for a full 28–30 days and recheck Prothrombin Time (PT) 48 hours after the final dose."
	if req.BloodGlucoseMgDl < 60.0 || req.ProthrombinTimeRatio >= 1.5 {
		return base + fmt.Sprintf(" Current patient exhibits critical derangement (PT Ratio %.1f, Blood Glucose %.1f mg/dL at %d mins post-ingestion) requiring immediate IV antidote/resuscitation.", req.ProthrombinTimeRatio, req.BloodGlucoseMgDl, req.TimeSinceIngestionMins)
	}
	return base + fmt.Sprintf(" Monitor serial Prothrombin Time (PT Ratio %.1f), Blood Glucose (%.1f mg/dL), ALT/bilirubin, and creatinine at 48–72 hours post-exposure (%d mins elapsed).", req.ProthrombinTimeRatio, req.BloodGlucoseMgDl, req.TimeSinceIngestionMins)
}

func buildToxicologyProtocolSteps(req domain.ToxicologyEvaluationRequest) []domain.ToxicologyProtocolStep {
	return []domain.ToxicologyProtocolStep{
		{
			ID:          StepApomorphineRopiniroleEmesisGate,
			Title:       fmt.Sprintf("Targeted Emesis (Apomorphine / Ropinirole / Feline Dexmedetomidine) & Charcoal Gate (%d mins Post-Ingestion)", req.TimeSinceIngestionMins),
			Description: "Induce emesis only if asymptomatic with intact gag reflex (<2–4h window); administer activated charcoal + sorbitol ONLY for adsorbable toxins (SKIP charcoal for xylitol).",
			Completed:   false,
		},
		{
			ID:          StepPhytonadioneK1Or20PctILELipid,
			Title:       fmt.Sprintf("Phytonadione Vitamin K1 (28–30d Regimen) or 20%% IV Lipid Emulsion (ILE 'Lipid Sink' %.1f mL Bolus)", req.WeightKg*1.5),
			Description: "Administer Phytonadione (Vitamin K1, NEVER Vitamin K3 menadione) with fatty meal + FFP for anticoagulant rodenticides, or 20% Intravenous Lipid Emulsion (1.5 mL/kg bolus + 0.25 mL/kg/min CRI) for lipophilic ivermectin/permethrin/bromethalin neurotoxins.",
			Completed:   false,
		},
		{
			ID:          StepDextroseNACSAMe48HrIVDiuresis,
			Title:       fmt.Sprintf("IV Dextrose Bolus/CRI, NAC/SAMe Hepatoprotection & 48–72h Feline Renal Diuresis (BG %.1f mg/dL)", req.BloodGlucoseMgDl),
			Description: "Infuse 2.5%–5% dextrose CRI + N-acetylcysteine (140 mg/kg load, 70 mg/kg q6h) + SAMe for xylitol insulin surge/hepatic necrosis, or 2–3x maintenance isotonic IV crystalloids for 48–72h in feline Lilium nephrotoxicity.",
			Completed:   false,
		},
		{
			ID:          Step48HrPTRecheckRenalHepaticPanel,
			Title:       fmt.Sprintf("48-Hour Post-Treatment Prothrombin Time (PT Ratio %.1f), ALT/Bilirubin & Renal Creatinine Verification", req.ProthrombinTimeRatio),
			Description: "Verify normal Prothrombin Time (PT) 48 hours after discontinuing Vitamin K1, normoglycemia off dextrose supplementation, and stable hepatic transaminases (ALT/AST) and renal azotemia markers prior to discharge.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a toxicology poison operation.
func UpdateProtocolStep(op *domain.ToxicologyPoisonOperation, stepID string, completed bool) error {
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
		if op.Status == domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro {
			op.Status = domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon
		}
	} else {
		if op.Status == domain.ToxicologyStatusClearedDecontaminatedPTGlucose {
			op.Status = domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueToxicologyClearance validates that all 4 protocol steps are completed, transitions the operation
// to DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED, and generates a deterministic POISON-SAFE-<12HEX> clearance token.
func IssueToxicologyClearance(op *domain.ToxicologyPoisonOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue toxicology clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%.1f|%d", op.ID, op.PetName, op.AgentType, op.EstimatedDoseMgKg, op.ProthrombinTimeRatio, op.BloodGlucoseMgDl, op.TimeSinceIngestionMins)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("POISON-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.ToxicologyStatusClearedDecontaminatedPTGlucose
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.ToxicologyPoisonOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (TP-OP-101, TP-OP-102, TP-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.ToxicologyPoisonOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateToxicologyTriage(domain.ToxicologyEvaluationRequest{
		ID:                        "TP-OP-101",
		PetID:                     "PET-TP-101",
		PetName:                   "Waffle",
		Species:                   "Dog - Beagle",
		WeightKg:                  13.4,
		AgentType:                 domain.ToxicologyAgentXylitolInsulinSurgeHepatotoxin,
		EstimatedDoseMgKg:         620.0,
		ProthrombinTimeRatio:      1.8,
		BloodGlucoseMgDl:          42.0,
		TimeSinceIngestionMins:    45,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	})
	if err == nil {
		op101.Status = domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro
		op101.CreatedAt = "2026-10-19T01:00:00Z"
		op101.UpdatedAt = "2026-10-19T01:05:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateToxicologyTriage(domain.ToxicologyEvaluationRequest{
		ID:                        "TP-OP-102",
		PetID:                     "PET-TP-102",
		PetName:                   "Bandit",
		Species:                   "Dog - Parson Russell Terrier",
		WeightKg:                  8.1,
		AgentType:                 domain.ToxicologyAgentSecondGenAnticoagulantBrodifacoum,
		EstimatedDoseMgKg:         1.2,
		ProthrombinTimeRatio:      1.1,
		BloodGlucoseMgDl:          96.0,
		TimeSinceIngestionMins:    60,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepApomorphineRopiniroleEmesisGate, true)
		_ = UpdateProtocolStep(op102, StepPhytonadioneK1Or20PctILELipid, true)
		op102.Status = domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon
		op102.CreatedAt = "2026-10-19T01:10:00Z"
		op102.UpdatedAt = "2026-10-19T01:18:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateToxicologyTriage(domain.ToxicologyEvaluationRequest{
		ID:                        "TP-OP-103",
		PetID:                     "PET-TP-103",
		PetName:                   "Tigger",
		Species:                   "Cat - Maine Coon",
		WeightKg:                  6.4,
		AgentType:                 domain.ToxicologyAgentIvermectinPermethrinLipophilic,
		EstimatedDoseMgKg:         18.0,
		ProthrombinTimeRatio:      1.0,
		BloodGlucoseMgDl:          104.0,
		TimeSinceIngestionMins:    180,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueToxicologyClearance(op103)
		op103.CreatedAt = "2026-10-18T22:45:00Z"
		op103.UpdatedAt = "2026-10-19T00:50:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.ToxicologyPoisonOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.ToxicologyPoisonOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.ToxicologyPoisonOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.ToxicologyPoisonOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.ToxicologyEvaluationRequest) (domain.ToxicologyPoisonOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("TP-OP-%d", s.seq+1)
	}

	op, err := EvaluateToxicologyTriage(req)
	if err != nil {
		return domain.ToxicologyPoisonOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.ToxicologyPoisonOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.ToxicologyPoisonOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.ToxicologyPoisonOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a decontaminated PT & glucose normal clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.ToxicologyPoisonOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.ToxicologyPoisonOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueToxicologyClearance(op); err != nil {
		return domain.ToxicologyPoisonOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.ToxicologyPoisonOperation) domain.ToxicologyPoisonOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.ToxicologyProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
