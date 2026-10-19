package neonatalincubator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	StepIDIncubatorWarmingGate = "STEP-INCUBATOR-WARMING-GATE"
	StepIDSublingualDextrose50 = "STEP-SUBLINGUAL-DEXTROSE-50"
	StepIDOrogastricTubeGavage = "STEP-OROGASTRIC-TUBE-GAVAGE"
	StepIDAnogenitalStimWeigh  = "STEP-ANOGENITAL-STIM-WEIGH"
)

// DefaultProtocolSteps constructs the 4 standard neonatal incubator warming, sublingual dextrose,
// orogastric gavage, and anogenital stimulation protocol steps.
func DefaultProtocolSteps(warmedGateDone, dextroseDone, gavageDone, stimWeighDone bool, completedAt string) []domain.NeonatalProtocolStep {
	if completedAt == "" {
		completedAt = time.Now().UTC().Format(time.RFC3339)
	}
	steps := []domain.NeonatalProtocolStep{
		{
			ID:          StepIDIncubatorWarmingGate,
			Title:       "Gradual Incubator Core Rewarming Gate (≥ 36.1°C / 97.0°F)",
			Description: "Warm neonate slowly over 1–2 hours in 32–34°C / 55–65% RH oxygenated incubator until rectal temp reaches ≥ 36.1°C (97.0°F) before any enteral formula.",
			Completed:   warmedGateDone,
		},
		{
			ID:          StepIDSublingualDextrose50,
			Title:       "Sublingual 50% Dextrose Mucosal Resuscitation & Warmed Lactated Ringer's",
			Description: "Apply 1 drop/100g of 50% dextrose diluted 1:1 onto gingival/sublingual mucosa for hypoglycemia (< 60 mg/dL) and administer warmed SC crystalloid bolus.",
			Completed:   dextroseDone,
		},
		{
			ID:          StepIDOrogastricTubeGavage,
			Title:       "Measured 5Fr/8Fr Orogastric Tube Gavage or Miracle Nipple Feeding (38°C Formula)",
			Description: "Measure nose-to-last-rib gavage tube length, verify negative laryngeal resistance, and deliver warmed species-specific colostrum/milk replacer at 4–5 mL/100g.",
			Completed:   gavageDone,
		},
		{
			ID:          StepIDAnogenitalStimWeigh,
			Title:       "Post-Prandial Anogenital Elimination Stimulation & Gram-Scale Weight Log",
			Description: "Stimulate perineal/preputial reflex with warm moist cotton after feeding to induce micturition/defecation and log 0.1g digital gram-scale daily weight gain.",
			Completed:   stimWeighDone,
		},
	}

	for i := range steps {
		if steps[i].Completed {
			steps[i].CompletedAt = completedAt
		}
	}
	return steps
}

func computeRequiredIncubatorProtocol(req domain.NeonatalEvaluationRequest) string {
	switch req.LitterType {
	case domain.NeonatalLitterUnweanedKitten0To2Wk:
		return "ICU Brooder 32.0–34.0°C (89.6–93.2°F) @ 60–65% RH • Q2H 5Fr Orogastric Gavage / Miracle Nipple (KMR 38.0°C, 4.5 mL/100g)"
	case domain.NeonatalLitterTransitionalKitten2To4Wk:
		return "Step-Down Brooder 27.0–29.5°C (80.6–85.1°F) @ 50–60% RH • Q4H Bottle/Slurry Gruel Transition & Litter Pan Imprinting"
	case domain.NeonatalLitterPuppyCleft:
		return "Cleft-Palate Aspiration-Safe Brooder 31.0–33.0°C @ 58–65% RH • Strict 8Fr Orogastric Tube Gavage Only (Zero Bottle/Nipple Feeding)"
	case domain.NeonatalLitterSingletonOrphanColostrumDeprived:
		return "Colostrum-Deprived Isolette 32.0–33.5°C @ 55–65% RH • Hyperimmune Plasma/IgG Passive Transfer + Q2H Warmed Esbilac/KMR + Co-Sibling Thermal Plush"
	case domain.NeonatalLitterMaternalAgalactiaRejected:
		return "Maternal-Rejection Brooder 31.5–33.5°C @ 55–62% RH • Q2–3H Syringe/Nipple Supplemental Nursing + Oxytocin/Domperidone Maternal Assessment"
	default:
		return "ICU Neonatal Brooder 32.0°C @ 60% RH • Q2H Warmed Formula Protocol"
	}
}

func computeIleusWarmingGateNote(req domain.NeonatalEvaluationRequest) string {
	if req.NeonateRectalTempCelsius < 36.1 || !req.WarmedBeforeFeedingGate {
		return fmt.Sprintf(
			"CRITICAL PARALYTIC ILEUS GATE: Never formula-feed a hypothermic neonate below 36.1°C / 97.0°F (current %.1f°C) due to paralytic ileus, gastric fermentation, bloating & fatal aspiration pneumonia risk. Complete gradual incubator warming first.",
			req.NeonateRectalTempCelsius,
		)
	}
	return fmt.Sprintf(
		"WARMING GATE CLEARED (%.1f°C ≥ 36.1°C / 97.0°F): Normothermic gut motility confirmed; uphold strict rule of never formula-feeding a hypothermic neonate below 36.1°C / 97.0°F due to paralytic ileus & aspiration risk.",
		req.NeonateRectalTempCelsius,
	)
}

func isCriticalTriad(rectalTempC, glucoseMgDl float64, warmedGate, sucklingReflex bool, litterType domain.NeonatalLitterType) bool {
	return rectalTempC < 35.5 ||
		glucoseMgDl < 60.0 ||
		!warmedGate ||
		(!sucklingReflex && litterType == domain.NeonatalLitterPuppyCleft)
}

// EvaluateNeonatalTriage deterministically validates and evaluates a neonatal orphan litter and incubator critical care triage operation.
func EvaluateNeonatalTriage(req domain.NeonatalEvaluationRequest) (*domain.NeonatalTriageOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Cat - Domestic Shorthair Neonate"
	}

	if req.WeightGrams < 30.0 || req.WeightGrams > 5000.0 {
		return nil, fmt.Errorf("weight_grams must be between 30.0 and 5000.0, got %v", req.WeightGrams)
	}
	if req.LitterCount < 1 || req.LitterCount > 16 {
		return nil, fmt.Errorf("litter_count must be between 1 and 16, got %d", req.LitterCount)
	}
	if req.AgeDays < 0 || req.AgeDays > 42 {
		return nil, fmt.Errorf("age_days must be between 0 and 42, got %d", req.AgeDays)
	}
	if !req.LitterType.IsValid() {
		return nil, fmt.Errorf("invalid litter_type: %q", req.LitterType)
	}
	if req.NeonateRectalTempCelsius < 28.0 || req.NeonateRectalTempCelsius > 40.5 {
		return nil, fmt.Errorf("neonate_rectal_temp_celsius must be between 28.0 and 40.5, got %v", req.NeonateRectalTempCelsius)
	}
	if req.BloodGlucoseMgDl < 15.0 || req.BloodGlucoseMgDl > 300.0 {
		return nil, fmt.Errorf("blood_glucose_mg_dl must be between 15.0 and 300.0, got %v", req.BloodGlucoseMgDl)
	}
	if req.IncubatorHumidityPct < 20.0 || req.IncubatorHumidityPct > 95.0 {
		return nil, fmt.Errorf("incubator_humidity_pct must be between 20.0 and 95.0, got %v", req.IncubatorHumidityPct)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%.1f",
			petName, species, req.LitterType, req.WeightGrams, req.NeonateRectalTempCelsius, req.BloodGlucoseMgDl)))
		opID = "NI-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	steps := DefaultProtocolSteps(req.WarmedBeforeFeedingGate, req.SublingualDextroseApplied, false, false, nowStr)

	status := domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged
	if isCriticalTriad(req.NeonateRectalTempCelsius, req.BloodGlucoseMgDl, req.WarmedBeforeFeedingGate, req.SucklingReflexPresent, req.LitterType) {
		status = domain.NeonatalTriageStatusFadingTriadHypothermicCritical
	}

	allCompleted := true
	for _, s := range steps {
		if !s.Completed {
			allCompleted = false
			break
		}
	}
	if allCompleted && status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical {
		status = domain.NeonatalTriageStatusStabilizedNursingGainingSafe
	}

	op := &domain.NeonatalTriageOperation{
		ID:                        opID,
		PetID:                     petID,
		PetName:                   petName,
		Species:                   species,
		WeightGrams:               req.WeightGrams,
		LitterCount:               req.LitterCount,
		AgeDays:                   req.AgeDays,
		LitterType:                req.LitterType,
		Status:                    status,
		NeonateRectalTempCelsius:  req.NeonateRectalTempCelsius,
		BloodGlucoseMgDl:          req.BloodGlucoseMgDl,
		IncubatorHumidityPct:      req.IncubatorHumidityPct,
		SucklingReflexPresent:     req.SucklingReflexPresent,
		WarmedBeforeFeedingGate:   req.WarmedBeforeFeedingGate,
		SublingualDextroseApplied: req.SublingualDextroseApplied,
		RequiredIncubatorProtocol: computeRequiredIncubatorProtocol(req),
		IleusWarmingGateNote:      computeIleusWarmingGateNote(req),
		ProtocolSteps:             steps,
		CreatedAt:                 nowStr,
		UpdatedAt:                 nowStr,
	}

	if err := op.Validate(); err != nil {
		return nil, err
	}
	return op, nil
}

// UpdateProtocolStep updates a specific protocol step on a NeonatalTriageOperation and recalculates status.
func UpdateProtocolStep(op *domain.NeonatalTriageOperation, stepID string, completed bool) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	trimmedID := strings.TrimSpace(stepID)
	if trimmedID == "" {
		return fmt.Errorf("step_id is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
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

	if trimmedID == StepIDIncubatorWarmingGate {
		op.WarmedBeforeFeedingGate = completed
	}
	if trimmedID == StepIDSublingualDextrose50 {
		op.SublingualDextroseApplied = completed
	}

	doneCount := 0
	warmingDone := false
	dextroseDone := false
	for _, step := range op.ProtocolSteps {
		if step.Completed {
			doneCount++
			if step.ID == StepIDIncubatorWarmingGate {
				warmingDone = true
			}
			if step.ID == StepIDSublingualDextrose50 {
				dextroseDone = true
			}
		}
	}

	if doneCount < len(op.ProtocolSteps) {
		op.ClearanceToken = ""
	}

	if op.ClearanceToken != "" && doneCount == len(op.ProtocolSteps) {
		op.Status = domain.NeonatalTriageStatusStabilizedNursingGainingSafe
	} else if (warmingDone && dextroseDone) || doneCount >= 2 {
		op.Status = domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged
	} else if isCriticalTriad(op.NeonateRectalTempCelsius, op.BloodGlucoseMgDl, op.WarmedBeforeFeedingGate, op.SucklingReflexPresent, op.LitterType) {
		op.Status = domain.NeonatalTriageStatusFadingTriadHypothermicCritical
	} else {
		op.Status = domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueNeonatalClearance verifies that all 4 protocol steps are completed, sets status to
// STABILIZED_NURSING_GAINING_SAFE, and generates a deterministic SHA-256 token NEONATAL-SAFE-<12HEX>.
func IssueNeonatalClearance(op *domain.NeonatalTriageOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("operation id is required")
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	for _, step := range op.ProtocolSteps {
		if !step.Completed {
			return fmt.Errorf("cannot issue neonatal clearance: protocol step %q is not completed", step.ID)
		}
	}

	payload := fmt.Sprintf("%s|%s|%s|%s|%.1f|%.1f|%.1f",
		op.ID,
		op.PetID,
		op.PetName,
		op.LitterType,
		op.WeightGrams,
		op.NeonateRectalTempCelsius,
		op.BloodGlucoseMgDl,
	)
	sum := sha256.Sum256([]byte(payload))
	op.ClearanceToken = "NEONATAL-SAFE-" + strings.ToUpper(hex.EncodeToString(sum[:])[:12])
	op.Status = domain.NeonatalTriageStatusStabilizedNursingGainingSafe
	op.WarmedBeforeFeedingGate = true
	op.SublingualDextroseApplied = true
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// SeedOperations returns the 3 deterministic neonatal triage operations (NI-OP-101, NI-OP-102, NI-OP-103).
func SeedOperations() []domain.NeonatalTriageOperation {
	baseTime := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	createdStr := baseTime.Add(-45 * time.Minute).Format(time.RFC3339)
	updatedStr := baseTime.Add(-10 * time.Minute).Format(time.RFC3339)

	req101 := domain.NeonatalEvaluationRequest{
		ID:                        "NI-OP-101",
		PetID:                     "PET-NI-101",
		PetName:                   "Clover Litter",
		Species:                   "Cat - Domestic Shorthair Neonates",
		WeightGrams:               92.0,
		LitterCount:               4,
		AgeDays:                   5,
		LitterType:                domain.NeonatalLitterUnweanedKitten0To2Wk,
		NeonateRectalTempCelsius:  33.8,
		BloodGlucoseMgDl:          44.0,
		IncubatorHumidityPct:      62.0,
		SucklingReflexPresent:     false,
		WarmedBeforeFeedingGate:   false,
		SublingualDextroseApplied: true,
	}
	op101 := domain.NeonatalTriageOperation{
		ID:                        req101.ID,
		PetID:                     req101.PetID,
		PetName:                   req101.PetName,
		Species:                   req101.Species,
		WeightGrams:               req101.WeightGrams,
		LitterCount:               req101.LitterCount,
		AgeDays:                   req101.AgeDays,
		LitterType:                req101.LitterType,
		Status:                    domain.NeonatalTriageStatusFadingTriadHypothermicCritical,
		NeonateRectalTempCelsius:  req101.NeonateRectalTempCelsius,
		BloodGlucoseMgDl:          req101.BloodGlucoseMgDl,
		IncubatorHumidityPct:      req101.IncubatorHumidityPct,
		SucklingReflexPresent:     req101.SucklingReflexPresent,
		WarmedBeforeFeedingGate:   req101.WarmedBeforeFeedingGate,
		SublingualDextroseApplied: req101.SublingualDextroseApplied,
		RequiredIncubatorProtocol: computeRequiredIncubatorProtocol(req101),
		IleusWarmingGateNote:      computeIleusWarmingGateNote(req101),
		ProtocolSteps:             DefaultProtocolSteps(false, true, false, false, updatedStr),
		CreatedAt:                 createdStr,
		UpdatedAt:                 updatedStr,
	}

	req102 := domain.NeonatalEvaluationRequest{
		ID:                        "NI-OP-102",
		PetID:                     "PET-NI-102",
		PetName:                   "Pippin",
		Species:                   "Dog - French Bulldog Neonate",
		WeightGrams:               185.0,
		LitterCount:               1,
		AgeDays:                   8,
		LitterType:                domain.NeonatalLitterSingletonOrphanColostrumDeprived,
		NeonateRectalTempCelsius:  36.8,
		BloodGlucoseMgDl:          88.0,
		IncubatorHumidityPct:      58.0,
		SucklingReflexPresent:     true,
		WarmedBeforeFeedingGate:   true,
		SublingualDextroseApplied: true,
	}
	op102 := domain.NeonatalTriageOperation{
		ID:                        req102.ID,
		PetID:                     req102.PetID,
		PetName:                   req102.PetName,
		Species:                   req102.Species,
		WeightGrams:               req102.WeightGrams,
		LitterCount:               req102.LitterCount,
		AgeDays:                   req102.AgeDays,
		LitterType:                req102.LitterType,
		Status:                    domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged,
		NeonateRectalTempCelsius:  req102.NeonateRectalTempCelsius,
		BloodGlucoseMgDl:          req102.BloodGlucoseMgDl,
		IncubatorHumidityPct:      req102.IncubatorHumidityPct,
		SucklingReflexPresent:     req102.SucklingReflexPresent,
		WarmedBeforeFeedingGate:   req102.WarmedBeforeFeedingGate,
		SublingualDextroseApplied: req102.SublingualDextroseApplied,
		RequiredIncubatorProtocol: computeRequiredIncubatorProtocol(req102),
		IleusWarmingGateNote:      computeIleusWarmingGateNote(req102),
		ProtocolSteps:             DefaultProtocolSteps(true, true, false, false, updatedStr),
		CreatedAt:                 createdStr,
		UpdatedAt:                 updatedStr,
	}

	req103 := domain.NeonatalEvaluationRequest{
		ID:                        "NI-OP-103",
		PetID:                     "PET-NI-103",
		PetName:                   "Buttercup Litter",
		Species:                   "Cat - Tabby Neonates",
		WeightGrams:               265.0,
		LitterCount:               3,
		AgeDays:                   18,
		LitterType:                domain.NeonatalLitterTransitionalKitten2To4Wk,
		NeonateRectalTempCelsius:  37.6,
		BloodGlucoseMgDl:          105.0,
		IncubatorHumidityPct:      55.0,
		SucklingReflexPresent:     true,
		WarmedBeforeFeedingGate:   true,
		SublingualDextroseApplied: true,
	}
	op103 := domain.NeonatalTriageOperation{
		ID:                        req103.ID,
		PetID:                     req103.PetID,
		PetName:                   req103.PetName,
		Species:                   req103.Species,
		WeightGrams:               req103.WeightGrams,
		LitterCount:               req103.LitterCount,
		AgeDays:                   req103.AgeDays,
		LitterType:                req103.LitterType,
		Status:                    domain.NeonatalTriageStatusStabilizedNursingGainingSafe,
		NeonateRectalTempCelsius:  req103.NeonateRectalTempCelsius,
		BloodGlucoseMgDl:          req103.BloodGlucoseMgDl,
		IncubatorHumidityPct:      req103.IncubatorHumidityPct,
		SucklingReflexPresent:     req103.SucklingReflexPresent,
		WarmedBeforeFeedingGate:   req103.WarmedBeforeFeedingGate,
		SublingualDextroseApplied: req103.SublingualDextroseApplied,
		RequiredIncubatorProtocol: computeRequiredIncubatorProtocol(req103),
		IleusWarmingGateNote:      computeIleusWarmingGateNote(req103),
		ProtocolSteps:             DefaultProtocolSteps(true, true, true, true, updatedStr),
		CreatedAt:                 createdStr,
		UpdatedAt:                 updatedStr,
	}
	_ = IssueNeonatalClearance(&op103)
	op103.UpdatedAt = updatedStr

	return []domain.NeonatalTriageOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory storage for neonatal incubator triage operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.NeonatalTriageOperation
	order   []string
	counter int
}

// NewStore creates and seeds a NeonatalTriageOperation store with NI-OP-101, NI-OP-102, and NI-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.NeonatalTriageOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.ID] = cloneOperation(op)
		s.order = append(s.order, op.ID)
	}
	return s
}

func cloneOperation(op domain.NeonatalTriageOperation) domain.NeonatalTriageOperation {
	steps := make([]domain.NeonatalProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (NI-OP-101, NI-OP-102, NI-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.NeonatalTriageOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.NeonatalTriageOperation, 0, len(s.ops))
	seen := make(map[string]bool, len(s.ops))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			result = append(result, cloneOperation(op))
			seen[id] = true
		}
	}
	if len(result) < len(s.ops) {
		extra := make([]string, 0)
		for id := range s.ops {
			if !seen[id] {
				extra = append(extra, id)
			}
		}
		sort.Strings(extra)
		for _, id := range extra {
			result = append(result, cloneOperation(s.ops[id]))
		}
	}
	return result
}

// Get retrieves a single operation by ID.
func (s *Store) Get(id string) (domain.NeonatalTriageOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.NeonatalTriageOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new neonatal triage operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.NeonatalEvaluationRequest) (domain.NeonatalTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("NI-OP-%d", s.counter+1)
	}

	op, err := EvaluateNeonatalTriage(req)
	if err != nil {
		return domain.NeonatalTriageOperation{}, err
	}
	if autoID {
		s.counter++
	}

	if _, exists := s.ops[op.ID]; !exists {
		s.order = append(s.order, op.ID)
	}
	s.ops[op.ID] = cloneOperation(*op)
	return cloneOperation(*op), nil
}

// UpdateStep updates a protocol step on an existing operation in the store.
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.NeonatalTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.NeonatalTriageOperation{}, fmt.Errorf("neonatal triage operation %q not found", key)
	}

	copied := cloneOperation(existing)
	if err := UpdateProtocolStep(&copied, stepID, completed); err != nil {
		return domain.NeonatalTriageOperation{}, err
	}
	s.ops[key] = cloneOperation(copied)
	return cloneOperation(copied), nil
}

// IssueClearance issues neonatal incubator nursing clearance for an existing operation in the store.
func (s *Store) IssueClearance(id string) (domain.NeonatalTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.NeonatalTriageOperation{}, fmt.Errorf("neonatal triage operation %q not found", key)
	}

	copied := cloneOperation(existing)
	if err := IssueNeonatalClearance(&copied); err != nil {
		return domain.NeonatalTriageOperation{}, err
	}
	s.ops[key] = cloneOperation(copied)
	return cloneOperation(copied), nil
}
