package gdvbloat

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
	StepBilateralCephalicIVBolus   = "STEP-BILATERAL-CEPHALIC-IV-BOLUS"
	StepTrocarOrogastricDecompress = "STEP-TROCAR-OROGASTRIC-DECOMPRESS"
	StepRightLateralDoubleBubble   = "STEP-RIGHT-LATERAL-DOUBLE-BUBBLE-XRAY"
	StepLidocaineVPCGastropexyPrep = "STEP-LIDOCAINE-VPC-GASTROPEXY-PREP"
)

// EvaluateGDVTriage deterministically evaluates a Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression triage request.
func EvaluateGDVTriage(req domain.GDVEvaluationRequest) (*domain.GDVBloatOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg < 5.0 || req.WeightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be between 5.0 and 120.0, got %.1f", req.WeightKg)
	}
	if !req.PresentationType.IsValid() {
		return nil, fmt.Errorf("invalid presentation_type: %q", req.PresentationType)
	}
	if req.PlasmaLactateMmolL < 0.5 || req.PlasmaLactateMmolL > 25.0 {
		return nil, fmt.Errorf("plasma_lactate_mmol_l must be between 0.5 and 25.0, got %.1f", req.PlasmaLactateMmolL)
	}
	if req.HeartRateBpm < 40 || req.HeartRateBpm > 280 {
		return nil, fmt.Errorf("heart_rate_bpm must be between 40 and 280, got %d", req.HeartRateBpm)
	}
	if req.TympanicAbdominalGirthCm < 30.0 || req.TympanicAbdominalGirthCm > 180.0 {
		return nil, fmt.Errorf("tympanic_abdominal_girth_cm must be between 30.0 and 180.0, got %.1f", req.TympanicAbdominalGirthCm)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Deep-Chested Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%d|%.1f", petName, req.PresentationType, req.PlasmaLactateMmolL, req.HeartRateBpm, req.TympanicAbdominalGirthCm)))
		opID = fmt.Sprintf("GDV-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-GDV-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.GDVStatusStagedTrocarDualIV
	if req.PlasmaLactateMmolL >= 6.0 ||
		req.VentricularArrhythmiaVPCs ||
		!req.CranialCephalicDualIVAccess ||
		req.HeartRateBpm >= 180 ||
		req.PresentationType == domain.GDVPresentationAcuteGDV360PyloricTorsion {
		status = domain.GDVStatusCriticalNecrosisVPCShock
	}

	regimen := computeRequiredDecompressionRegimen(req)
	warningNote := computeCaudalVenaCavaWarningNote(req)
	steps := buildGDVProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.GDVBloatOperation{
		ID:                           opID,
		PetID:                        petID,
		PetName:                      petName,
		Species:                      species,
		WeightKg:                     req.WeightKg,
		PresentationType:             req.PresentationType,
		Status:                       status,
		PlasmaLactateMmolL:           req.PlasmaLactateMmolL,
		HeartRateBpm:                 req.HeartRateBpm,
		TympanicAbdominalGirthCm:     req.TympanicAbdominalGirthCm,
		NonProductiveRetchingPresent: req.NonProductiveRetchingPresent,
		CranialCephalicDualIVAccess:  req.CranialCephalicDualIVAccess,
		VentricularArrhythmiaVPCs:    req.VentricularArrhythmiaVPCs,
		RequiredDecompressionRegimen: regimen,
		CaudalVenaCavaWarningNote:    warningNote,
		ProtocolSteps:                steps,
		CreatedAt:                    nowStr,
		UpdatedAt:                    nowStr,
	}, nil
}

func computeRequiredDecompressionRegimen(req domain.GDVEvaluationRequest) string {
	var parts []string
	bolusMl := req.WeightKg * 20.0
	switch req.PresentationType {
	case domain.GDVPresentationAcuteGDV360PyloricTorsion:
		parts = append(parts, fmt.Sprintf("Bilateral 14G–16G Cranial Cephalic IV Isotonic Crystalloid Shock Bolus (%.0f mL rapid infusion) + Percutaneous 14G Trocarization at Maximal Tympany + Emergency Surgical Derotation & Incisional Right-Sided Gastropexy", bolusMl))
	case domain.GDVPresentationPartialGDV180Volvulus:
		parts = append(parts, fmt.Sprintf("Dual Cranial Cephalic IV Resuscitation (%.0f mL bolus) + Percutaneous Trocar / Pre-Measured Orogastric Tube Lavage + Right-Lateral Double-Bubble Radiograph & Prompt Gastropexy", bolusMl))
	case domain.GDVPresentationSimpleGastricDilatationFoodBloat:
		parts = append(parts, fmt.Sprintf("Cranial Cephalic IV Fluid Support (%.0f mL) + Sedated Orogastric Intubation & Warm-Water Gastric Lavage + Prophylactic Gastropexy Evaluation", bolusMl))
	case domain.GDVPresentationSplenicTorsionHemoabdomen:
		parts = append(parts, fmt.Sprintf("Bilateral Cranial Cephalic IV Resuscitation (%.0f mL) + En-Bloc Total Splenectomy WITHOUT Derotating Twisted Splenic Pedicle (Prevents Thromboembolic / Reperfusion Cytokine Release) + Gastropexy", bolusMl))
	case domain.GDVPresentationPostExerciseDeepChestAerophagia:
		parts = append(parts, fmt.Sprintf("Cranial Cephalic IV Crystalloid Bolus (%.0f mL) + Right-Lateral Radiograph to Rule Out Pyloric Dorsal Displacement + Nasogastric/Orogastric Gas Decompression", bolusMl))
	default:
		parts = append(parts, fmt.Sprintf("Bilateral Cranial Cephalic IV Shock Resuscitation (%.0f mL) + Percutaneous 14G Trocarization", bolusMl))
	}

	if req.VentricularArrhythmiaVPCs {
		parts = append(parts, fmt.Sprintf("Lidocaine 2 mg/kg IV Bolus (%.0f mg) Followed by 50–80 mcg/kg/min CRI for Reperfusion Ventricular Premature Contractions (VPCs) / R-on-T Tachycardia", req.WeightKg*2.0))
	}
	if req.PlasmaLactateMmolL >= 6.0 {
		parts = append(parts, fmt.Sprintf("HIGH GASTRIC SEROMUSCULAR NECROSIS RISK (Plasma Lactate %.1f mmol/L >= 6.0 mmol/L): Prepare for Partial Gastrectomy / Greater Curvature Invagination & Serial Lactate Clearance Check (< 50%% drop required within 60 min)", req.PlasmaLactateMmolL))
	}
	if !req.CranialCephalicDualIVAccess {
		parts = append(parts, "URGENT: Establish Bilateral Cranial Cephalic or External Jugular Large-Bore IV Access Immediately — Abandon Saphenous/Hindlimb Veins")
	}
	return strings.Join(parts, " | ")
}

func computeCaudalVenaCavaWarningNote(req domain.GDVEvaluationRequest) string {
	base := "CRITICAL VASCULAR & RADIOGRAPHIC ALERT: NEVER rely on saphenous or caudal IV catheters in GDV because the gas-distended rotated stomach mechanically occludes the caudal vena cava and portal vein, trapping hindlimb fluids out of central circulation; always place bilateral cranial cephalic or external jugular large-bore catheters and confirm pyloric malposition via a right-lateral abdominal radiograph ('double-bubble' / Popeye arm Compartmentalization sign) ONLY after initial trocar decompression and fluid stabilization."
	if req.PlasmaLactateMmolL >= 6.0 || req.VentricularArrhythmiaVPCs {
		return base + fmt.Sprintf(" Elevated plasma lactate (%.1f mmol/L) and HR (%d bpm) warn of gastric fundic necrosis and myocardial ischemia/reperfusion VPC arrhythmias — initiate lidocaine 2 mg/kg IV bolus and surgical derotation immediately.", req.PlasmaLactateMmolL, req.HeartRateBpm)
	}
	return base + fmt.Sprintf(" Monitor serial plasma lactate (currently %.1f mmol/L) and continuous 3-lead ECG for 48–72 hours post-decompression to catch delayed reperfusion VPCs.", req.PlasmaLactateMmolL)
}

func buildGDVProtocolSteps(req domain.GDVEvaluationRequest) []domain.GDVProtocolStep {
	return []domain.GDVProtocolStep{
		{
			ID:          StepBilateralCephalicIVBolus,
			Title:       fmt.Sprintf("Establish Bilateral Cranial Cephalic 14G–16G IV Access & Shock Bolus (%.0f mL)", req.WeightKg*20.0),
			Description: "Place two large-bore catheters in the cranial cephalic or jugular veins (never lateral saphenous due to caudal vena cava occlusion) and infuse isotonic crystalloid shock boluses while tracking serial plasma lactate.",
			Completed:   false,
		},
		{
			ID:          StepTrocarOrogastricDecompress,
			Title:       fmt.Sprintf("Percutaneous 14G Trocarization & Pre-Measured Orogastric Decompression (Girth %.1f cm)", req.TympanicAbdominalGirthCm),
			Description: "clipped/aseptically prepped flank trocarization over maximal tympany (avoiding spleen via percussion/AFAST) or lubricated chin-to-last-rib pre-measured orogastric tube decompression to restore venous return.",
			Completed:   false,
		},
		{
			ID:          StepRightLateralDoubleBubble,
			Title:       "Right-Lateral Abdominal Radiograph ('Double-Bubble' Pyloric Compartmentalization Check)",
			Description: "Obtain a single right-lateral abdominal radiograph ONLY after hemodynamic stabilization and trocar decompression to differentiate 180°–360° dorsal/craniodorsal pyloric volvulus from simple gastric dilatation.",
			Completed:   false,
		},
		{
			ID:          StepLidocaineVPCGastropexyPrep,
			Title:       fmt.Sprintf("Continuous ECG Lidocaine VPC Anti-Arrhythmic CRI & Surgical Derotation / Right Incisional Gastropexy (HR %d bpm)", req.HeartRateBpm),
			Description: "Treat ventricular premature contractions (VPCs / R-on-T) with lidocaine 2 mg/kg IV bolus + 50 mcg/kg/min CRI, resect necrotic greater curvature or torsioned spleen en-bloc, and anchor right pyloric antrum to body wall.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a GDV bloat operation.
func UpdateProtocolStep(op *domain.GDVBloatOperation, stepID string, completed bool) error {
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
		if op.Status == domain.GDVStatusCriticalNecrosisVPCShock {
			op.Status = domain.GDVStatusStagedTrocarDualIV
		}
	} else {
		if op.Status == domain.GDVStatusClearedGastropexySinus {
			op.Status = domain.GDVStatusStagedTrocarDualIV
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueGDVClearance validates that all 4 protocol steps are completed, transitions the operation
// to GASTROPEXY_SINUS_RHYTHM_CLEARED, and generates a deterministic GDV-SAFE-<12HEX> clearance token.
func IssueGDVClearance(op *domain.GDVBloatOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue GDV clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%d|%.1f", op.ID, op.PetName, op.PresentationType, op.PlasmaLactateMmolL, op.HeartRateBpm, op.TympanicAbdominalGirthCm)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("GDV-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.GDVStatusClearedGastropexySinus
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory GDV / Bloat, Splenic Torsion & Trocar Decompression triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.GDVBloatOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (GDV-OP-101, GDV-OP-102, GDV-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.GDVBloatOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateGDVTriage(domain.GDVEvaluationRequest{
		ID:                           "GDV-OP-101",
		PetID:                        "PET-GDV-101",
		PetName:                      "Titan",
		Species:                      "Dog - Great Dane",
		WeightKg:                     64.0,
		PresentationType:             domain.GDVPresentationAcuteGDV360PyloricTorsion,
		PlasmaLactateMmolL:           7.8,
		HeartRateBpm:                 196,
		TympanicAbdominalGirthCm:     112.0,
		NonProductiveRetchingPresent: true,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    true,
	})
	if err == nil {
		op101.Status = domain.GDVStatusCriticalNecrosisVPCShock
		op101.CreatedAt = "2026-10-19T00:10:00Z"
		op101.UpdatedAt = "2026-10-19T00:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateGDVTriage(domain.GDVEvaluationRequest{
		ID:                           "GDV-OP-102",
		PetID:                        "PET-GDV-102",
		PetName:                      "Baron",
		Species:                      "Dog - Weimaraner",
		WeightKg:                     36.5,
		PresentationType:             domain.GDVPresentationPartialGDV180Volvulus,
		PlasmaLactateMmolL:           3.4,
		HeartRateBpm:                 148,
		TympanicAbdominalGirthCm:     86.0,
		NonProductiveRetchingPresent: true,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    false,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepBilateralCephalicIVBolus, true)
		_ = UpdateProtocolStep(op102, StepTrocarOrogastricDecompress, true)
		op102.Status = domain.GDVStatusStagedTrocarDualIV
		op102.CreatedAt = "2026-10-19T00:20:00Z"
		op102.UpdatedAt = "2026-10-19T00:28:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateGDVTriage(domain.GDVEvaluationRequest{
		ID:                           "GDV-OP-103",
		PetID:                        "PET-GDV-103",
		PetName:                      "Duchess",
		Species:                      "Dog - Standard Poodle",
		WeightKg:                     24.0,
		PresentationType:             domain.GDVPresentationSimpleGastricDilatationFoodBloat,
		PlasmaLactateMmolL:           1.8,
		HeartRateBpm:                 108,
		TympanicAbdominalGirthCm:     68.0,
		NonProductiveRetchingPresent: false,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    false,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueGDVClearance(op103)
		op103.CreatedAt = "2026-10-18T23:30:00Z"
		op103.UpdatedAt = "2026-10-19T00:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.GDVBloatOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.GDVBloatOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.GDVBloatOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.GDVBloatOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.GDVEvaluationRequest) (domain.GDVBloatOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("GDV-OP-%d", s.seq+1)
	}

	op, err := EvaluateGDVTriage(req)
	if err != nil {
		return domain.GDVBloatOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.GDVBloatOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.GDVBloatOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.GDVBloatOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a gastropexy & sinus rhythm clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.GDVBloatOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.GDVBloatOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueGDVClearance(op); err != nil {
		return domain.GDVBloatOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.GDVBloatOperation) domain.GDVBloatOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.GDVProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
