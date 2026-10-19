package envenomationtoxin

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
	StepDemarcationPerimeterMark   = "STEP-DEMARCATION-PERIMETER-MARK"
	StepOralForwardLavageAntivenom = "STEP-ORAL-FORWARD-LAVAGE-ANTIVENOM"
	StepSedatedQuillExtraction     = "STEP-SEDATED-QUILL-EXTRACTION"
	StepCoagEchinocyteRecheck      = "STEP-COAG-ECHINOCYTE-RECHECK"
)

// EvaluateEnvenomationTriage deterministically evaluates a pit-viper envenomation, Bufo toad bufotoxin, porcupine quill, or toxic flora triage request.
func EvaluateEnvenomationTriage(req domain.EnvenomationEvaluationRequest) (*domain.EnvenomationTriageOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 || req.WeightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %.1f", req.WeightKg)
	}
	if !req.ToxinType.IsValid() {
		return nil, fmt.Errorf("invalid toxin_type: %q", req.ToxinType)
	}
	if req.EdemaSpreadCmHr < 0 || req.EdemaSpreadCmHr > 50.0 {
		return nil, fmt.Errorf("edema_spread_cm_hr must be between 0.0 and 50.0, got %.1f", req.EdemaSpreadCmHr)
	}
	if req.EchinocytePct < 0 || req.EchinocytePct > 100.0 {
		return nil, fmt.Errorf("echinocyte_pct must be between 0.0 and 100.0, got %.1f", req.EchinocytePct)
	}
	if req.QuillCountMuzzleThorax < 0 || req.QuillCountMuzzleThorax > 500 {
		return nil, fmt.Errorf("quill_count_muzzle_thorax must be between 0 and 500, got %d", req.QuillCountMuzzleThorax)
	}
	if req.AntivenomVialsStaged < 0 || req.AntivenomVialsStaged > 20 {
		return nil, fmt.Errorf("antivenom_vials_staged must be between 0 and 20, got %d", req.AntivenomVialsStaged)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%d", petName, req.ToxinType, req.EdemaSpreadCmHr, req.EchinocytePct, req.QuillCountMuzzleThorax)))
		opID = fmt.Sprintf("ET-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-ET-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.EnvenomationStatusStaged
	if req.EdemaSpreadCmHr >= 4.0 ||
		req.EchinocytePct >= 25.0 ||
		req.QuillCountMuzzleThorax >= 20 ||
		!req.QuillsCutOrPulledAwakeAvoided ||
		(req.ToxinType == domain.ToxinBufoMarinusCardiotoxin && !req.OralMucosaFlushedForward) ||
		req.ToxinType == domain.ToxinBlueGreenAlgaeCyanotoxin {
		status = domain.EnvenomationStatusCritical
	}

	antidoteProtocol := computeRequiredAntidoteProtocol(req)
	warningNote := computeContraindicationWarningNote(req)
	steps := buildEnvenomationProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.EnvenomationTriageOperation{
		ID:                            opID,
		PetID:                         petID,
		PetName:                       petName,
		Species:                       species,
		WeightKg:                      req.WeightKg,
		ToxinType:                     req.ToxinType,
		Status:                        status,
		EdemaSpreadCmHr:               req.EdemaSpreadCmHr,
		EchinocytePct:                 req.EchinocytePct,
		QuillCountMuzzleThorax:        req.QuillCountMuzzleThorax,
		OralMucosaFlushedForward:      req.OralMucosaFlushedForward,
		QuillsCutOrPulledAwakeAvoided: req.QuillsCutOrPulledAwakeAvoided,
		AntivenomVialsStaged:          req.AntivenomVialsStaged,
		RequiredAntidoteProtocol:      antidoteProtocol,
		ContraindicationWarningNote:   warningNote,
		ProtocolSteps:                 steps,
		CreatedAt:                     nowStr,
		UpdatedAt:                     nowStr,
	}, nil
}

func computeRequiredAntidoteProtocol(req domain.EnvenomationEvaluationRequest) string {
	var parts []string
	switch req.ToxinType {
	case domain.ToxinPitViperCrotalidHEI:
		parts = append(parts, fmt.Sprintf("Polyvalent Crotalidae F(ab')2 / IgG Antivenom IV Infusion (%d Vials Staged) + Circumferential Edema Skin-Marker Tracking & Pure Mu-Opioid Analgesia (Avoid NSAIDs)", req.AntivenomVialsStaged))
	case domain.ToxinElapidCoralNeurotoxin:
		parts = append(parts, fmt.Sprintf("Micrurus Elapid Antivenom Standby (%d Vials Staged) + Early Endotracheal Intubation / Mechanical Ventilation Readiness for Delayed Ascending Bulbar & Diaphragmatic Paralysis", req.AntivenomVialsStaged))
	case domain.ToxinBufoMarinusCardiotoxin:
		parts = append(parts, "Immediate Rostral-Forward Gingival Wipe & Low-Pressure Forward Oral Lavage + Continuous ECG / Lidocaine IV for Ventricular Tachycardia & Diazepam/Levetiracetam Seizure Control")
	case domain.ToxinPorcupineRetrogradeQuill:
		parts = append(parts, fmt.Sprintf("Deep Dexmedetomidine/Butorphanol/Propofol Sedation + Intact Parallel Hemostat Traction of %d Retrograde Barbed Quills & Thoracic/Orbital Ultrasound Screening", req.QuillCountMuzzleThorax))
	case domain.ToxinBlueGreenAlgaeCyanotoxin:
		parts = append(parts, "Immediate Coat Bathing & Cholestyramine/Activated Charcoal Enterosorption + N-Acetylcysteine/SAMe Hepatoprotection & Atropine/Ventilatory Support for Anatoxin-a/Microcystin")
	default:
		parts = append(parts, "Targeted Antivenom / Decontamination Lavage & Coagulopathy Stabilization")
	}

	if req.EdemaSpreadCmHr >= 4.0 || req.EchinocytePct >= 25.0 {
		parts = append(parts, fmt.Sprintf("Rapid Hemotoxic Progression (Edema %.1f cm/hr, Echinocytes %.1f%%): Serial PT/aPTT, Fibrinogen & Platelet Monitoring", req.EdemaSpreadCmHr, req.EchinocytePct))
	}
	if !req.QuillsCutOrPulledAwakeAvoided {
		parts = append(parts, "URGENT SURGICAL EXPLORATION: Cut or fractured quills detected — perform high-frequency ultrasound localization for submerged retrograde quill shafts")
	}
	if req.ToxinType == domain.ToxinBufoMarinusCardiotoxin && !req.OralMucosaFlushedForward {
		parts = append(parts, "URGENT AIRWAY & GI GUARD: Posterior pharyngeal flushing risk — inspect for bufotoxin aspiration and initiate IV lipid emulsion / digoxin-specific Fab evaluation")
	}

	return strings.Join(parts, " | ")
}

func computeContraindicationWarningNote(req domain.EnvenomationEvaluationRequest) string {
	base := "CRITICAL CONTRAINDICATIONS: NEVER cut porcupine quills prior to extraction (cutting releases internal air pressure, splinters the shaft, and accelerates retrograde barb migration into thoracic/orbital cavities) and NEVER pull quills on an awake patient; NEVER point a direct hose down the throat during Bufo toad oral lavage (always wipe and flush forward out of the mouth with head angled downward to prevent fatal bufotoxin ingestion and aspiration pneumonia); NEVER apply tourniquets, ice packs, electric shock, or cruciate incisions to snakebite wounds, and avoid NSAIDs due to severe venom-induced thrombocytopenia and coagulopathy."
	if req.ToxinType == domain.ToxinBlueGreenAlgaeCyanotoxin {
		return base + " Blue-green algae (Microcystis / Dolichospermum) cyanotoxins cause fulminant hepatic necrosis and nicotinic respiratory arrest within minutes: wear gloves while bathing coat to prevent human dermal/inhalation exposure."
	}
	if req.EdemaSpreadCmHr >= 4.0 || req.EchinocytePct >= 25.0 {
		return base + " Severe crotalid venom-induced consumptive coagulopathy and echinocytosis detected: escalate antivenom titration immediately."
	}
	return base
}

func buildEnvenomationProtocolSteps(req domain.EnvenomationEvaluationRequest) []domain.EnvenomationProtocolStep {
	return []domain.EnvenomationProtocolStep{
		{
			ID:          StepDemarcationPerimeterMark,
			Title:       fmt.Sprintf("Mark Leading Edema Perimeter & Fang/Quill Anatomical Entry Sites (%.1f cm/hr Spread)", req.EdemaSpreadCmHr),
			Description: "Trace the proximal margin of hemorrhagic edema with an indelible surgical skin marker every 15 minutes, immobilize the affected limb at heart level without tourniquets or ice, and establish large-bore IV access.",
			Completed:   false,
		},
		{
			ID:          StepOralForwardLavageAntivenom,
			Title:       fmt.Sprintf("Execute Forward-Angled Oral Mucosal Lavage & Antivenom Titration (%d Vials Staged)", req.AntivenomVialsStaged),
			Description: "Wipe gingival sulci forward with damp gauze and flush rostrally out of the dependent mouth (never caudally down the pharynx), or infuse reconstituted polyvalent/elapid antivenom with diphenhydramine/epinephrine anaphylaxis standby.",
			Completed:   false,
		},
		{
			ID:          StepSedatedQuillExtraction,
			Title:       fmt.Sprintf("Perform Sedated Intact-Shaft Quill Extraction & Subcutaneous Migration Audit (%d Quills)", req.QuillCountMuzzleThorax),
			Description: "Under deep sedation and local analgesia, grasp each quill at the skin entry point with hemostats and extract along the exact entry vector without cutting or splintering the shaft; palpate oral cavity, hard palate, and periorbital tissues.",
			Completed:   false,
		},
		{
			ID:          StepCoagEchinocyteRecheck,
			Title:       fmt.Sprintf("Serial Peripheral Smear Echinocyte (%.1f%%), PT/aPTT Coagulation & ECG Neuro-Cardiotoxin Recheck", req.EchinocytePct),
			Description: "Verify resolution of echinocytosis (crenated/spiculated RBCs), normalize PT/aPTT/platelets, confirm sinus rhythm free of bufotoxin ventricular arrhythmias, and document intact gag/palpebral reflexes.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on an operation.
func UpdateProtocolStep(op *domain.EnvenomationTriageOperation, stepID string, completed bool) error {
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
		if op.Status == domain.EnvenomationStatusCritical {
			op.Status = domain.EnvenomationStatusStaged
		}
	} else {
		if op.Status == domain.EnvenomationStatusStabilized {
			op.Status = domain.EnvenomationStatusStaged
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueEnvenomationClearance validates that all 4 protocol steps are completed, transitions the operation
// to STABILIZED_NEUTRALIZED_CLEARED, and generates a deterministic TOXIN-SAFE-<12HEX> clearance token.
func IssueEnvenomationClearance(op *domain.EnvenomationTriageOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue envenomation clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%d", op.ID, op.PetName, op.ToxinType, op.EdemaSpreadCmHr, op.EchinocytePct, op.QuillCountMuzzleThorax)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("TOXIN-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.EnvenomationStatusStabilized
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory envenomation, bufotoxin, quill & toxic flora triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.EnvenomationTriageOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (ET-OP-101, ET-OP-102, ET-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.EnvenomationTriageOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateEnvenomationTriage(domain.EnvenomationEvaluationRequest{
		ID:                            "ET-OP-101",
		PetID:                         "PET-ET-101",
		PetName:                       "Copper",
		Species:                       "Dog - Catahoula Leopard Dog",
		WeightKg:                      26.5,
		ToxinType:                     domain.ToxinPitViperCrotalidHEI,
		EdemaSpreadCmHr:               6.5,
		EchinocytePct:                 38.0,
		QuillCountMuzzleThorax:        0,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          2,
	})
	if err == nil {
		op101.Status = domain.EnvenomationStatusCritical
		op101.CreatedAt = "2026-08-14T18:10:00Z"
		op101.UpdatedAt = "2026-08-14T18:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateEnvenomationTriage(domain.EnvenomationEvaluationRequest{
		ID:                            "ET-OP-102",
		PetID:                         "PET-ET-102",
		PetName:                       "Bramble",
		Species:                       "Dog - German Wirehaired Pointer",
		WeightKg:                      28.0,
		ToxinType:                     domain.ToxinPorcupineRetrogradeQuill,
		EdemaSpreadCmHr:               1.0,
		EchinocytePct:                 0.0,
		QuillCountMuzzleThorax:        14,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          0,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepDemarcationPerimeterMark, true)
		_ = UpdateProtocolStep(op102, StepSedatedQuillExtraction, true)
		op102.Status = domain.EnvenomationStatusStaged
		op102.CreatedAt = "2026-08-14T18:25:00Z"
		op102.UpdatedAt = "2026-08-14T18:35:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateEnvenomationTriage(domain.EnvenomationEvaluationRequest{
		ID:                            "ET-OP-103",
		PetID:                         "PET-ET-103",
		PetName:                       "Mango",
		Species:                       "Dog - Jack Russell Terrier",
		WeightKg:                      7.4,
		ToxinType:                     domain.ToxinBufoMarinusCardiotoxin,
		EdemaSpreadCmHr:               0.5,
		EchinocytePct:                 2.0,
		QuillCountMuzzleThorax:        0,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          0,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueEnvenomationClearance(op103)
		op103.CreatedAt = "2026-08-14T17:40:00Z"
		op103.UpdatedAt = "2026-08-14T18:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.EnvenomationTriageOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.EnvenomationTriageOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.EnvenomationTriageOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.EnvenomationTriageOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.EnvenomationEvaluationRequest) (domain.EnvenomationTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("ET-OP-%d", s.seq+1)
	}

	op, err := EvaluateEnvenomationTriage(req)
	if err != nil {
		return domain.EnvenomationTriageOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.EnvenomationTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.EnvenomationTriageOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.EnvenomationTriageOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a neutralized toxin clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.EnvenomationTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.EnvenomationTriageOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueEnvenomationClearance(op); err != nil {
		return domain.EnvenomationTriageOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.EnvenomationTriageOperation) domain.EnvenomationTriageOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.EnvenomationProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
