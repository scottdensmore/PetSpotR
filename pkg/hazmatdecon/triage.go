package hazmatdecon

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

// DefaultProtocolSteps constructs the 5 standard respiratory & dermal hazmat decontamination protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.HazmatProtocolStep {
	steps := []domain.HazmatProtocolStep{
		{
			ID:       "step-ocular-flush",
			Label:    "Irrigate corneas with buffered isotonic sterile saline & apply protective ophthalmic lubricant",
			Category: "OCULAR_DERMAL_FLUSH",
			Required: true,
		},
		{
			ID:       "step-elizabethan-collar",
			Label:    "Secure Elizabethan anti-grooming collar to block oral ingestion of coat particulates & hydrocarbons",
			Category: "TOXIN_BINDER_MONITORING",
			Required: true,
		},
		{
			ID:       "step-warm-surfactant-wash",
			Label:    "Perform warm-water surfactant dermal wash in warm-zone decon corridor with normothermic drying",
			Category: "OCULAR_DERMAL_FLUSH",
			Required: true,
		},
		{
			ID:       "step-oxygen-nebulizer",
			Label:    "Administer 100% humidified normobaric oxygen cage & isotonic saline bronchodilator nebulization",
			Category: "OXYGEN_NEBULIZATION",
			Required: true,
		},
		{
			ID:       "step-cohb-binder-panel",
			Label:    "Verify pulse CO-oximetry (COHb < 3%) & administer oral cholestyramine / activated charcoal binder",
			Category: "TOXIN_BINDER_MONITORING",
			Required: true,
		},
	}

	if completedAt == "" {
		completedAt = time.Now().UTC().Format(time.RFC3339)
	}
	for i := range steps {
		if i < completedCount {
			steps[i].Completed = true
			steps[i].CompletedAt = completedAt
		}
	}
	return steps
}

// EvaluateHazmatDecon deterministically evaluates wildfire smoke, ashfall, petrochemical coat spills,
// cyanotoxin blooms, and chemical aerosol plumes to compute hazard scores, alerts, and decon protocols.
func EvaluateHazmatDecon(req domain.HazmatEvaluationRequest) (domain.HazmatDeconOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.HazmatDeconOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Canine"
	}

	if !req.ExposureType.IsValid() {
		return domain.HazmatDeconOperation{}, fmt.Errorf("invalid exposureType: %q", req.ExposureType)
	}
	if req.WeightKg <= 0 || req.WeightKg > 250 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("weightKg must be in (0, 250], got %v", req.WeightKg)
	}
	if req.AQIIndex < 0 || req.AQIIndex > 1000 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("aqiIndex must be in [0, 1000], got %d", req.AQIIndex)
	}
	if req.PM25UgM3 < 0 || req.PM25UgM3 > 2000 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("pm25UgM3 must be in [0, 2000], got %v", req.PM25UgM3)
	}
	if req.CarboxyhemoglobinPct < 0 || req.CarboxyhemoglobinPct > 100 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("carboxyhemoglobinPct must be in [0, 100], got %v", req.CarboxyhemoglobinPct)
	}
	if req.CoatContaminationPct < 0 || req.CoatContaminationPct > 100 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("coatContaminationPct must be in [0, 100], got %v", req.CoatContaminationPct)
	}
	if req.RespiratoryRateBpm <= 0 || req.RespiratoryRateBpm > 250 {
		return domain.HazmatDeconOperation{}, fmt.Errorf("respiratoryRateBpm must be in (0, 250], got %d", req.RespiratoryRateBpm)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%.1f", petName, species, req.ExposureType, req.AQIIndex, req.CoatContaminationPct)))
		opID = "HZ-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	corridorZone := strings.TrimSpace(req.DeconCorridorZone)
	if corridorZone == "" {
		corridorZone = "DECON-CORRIDOR-WARM-01"
	}

	callsign := strings.TrimSpace(req.LeadToxTechCallsign)
	if callsign == "" {
		callsign = "TOX-VET-01"
	}

	smokeInhalationAlert := req.AQIIndex >= 300 ||
		req.PM25UgM3 >= 250.0 ||
		req.CarboxyhemoglobinPct >= 10.0 ||
		req.RespiratoryRateBpm >= 55

	dermalToxinIngestionAlert := req.CoatContaminationPct >= 35.0 ||
		req.ExposureType == domain.HazmatExposurePetrochemicalSpillCoat ||
		req.ExposureType == domain.HazmatExposureToxicAlgaeCyanotoxin

	riskFlags := make([]string, 0, 4)
	if smokeInhalationAlert {
		riskFlags = append(riskFlags, "EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK")
	}
	if dermalToxinIngestionAlert {
		riskFlags = append(riskFlags, "CRITICAL_COAT_GROOMING_TOXICOSIS_RISK")
	}
	if req.ExposureType == domain.HazmatExposureWildfireSmokeAshfall || req.PM25UgM3 >= 150.0 {
		riskFlags = append(riskFlags, "CORNEAL_ASH_ABRASION_HAZARD")
	}
	if req.ExposureType == domain.HazmatExposureToxicAlgaeCyanotoxin {
		riskFlags = append(riskFlags, "HEPATOTOXIC_NEUROTOXIC_BLOOM_HAZARD")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "BRONCHOSPASM_MUCOSAL_IRRITATION_RISK")
	}

	baseHazard := 28
	switch req.ExposureType {
	case domain.HazmatExposureWildfireSmokeAshfall:
		baseHazard = 38
	case domain.HazmatExposurePetrochemicalSpillCoat:
		baseHazard = 36
	case domain.HazmatExposureToxicAlgaeCyanotoxin:
		baseHazard = 40
	case domain.HazmatExposureTearGasAerosolPlume:
		baseHazard = 34
	case domain.HazmatExposureIndustrialSolventVapor:
		baseHazard = 35
	}

	if smokeInhalationAlert {
		baseHazard += 28
	}
	if dermalToxinIngestionAlert {
		baseHazard += 20
	}
	if req.CarboxyhemoglobinPct >= 12.0 {
		baseHazard += 6
	}
	if req.CoatContaminationPct >= 50.0 {
		baseHazard += 6
	}
	baseHazard += req.AQIIndex / 100

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	washAgent := "Warm Isotonic Saline Ocular Irrigation + Mild pH-Balanced Veterinary Dermal Flush"
	switch req.ExposureType {
	case domain.HazmatExposureWildfireSmokeAshfall:
		washAgent = "Sterile Buffered Isotonic Saline Corneal Flush + Warm Low-Alkaline Surfactant Ash Wash (E-Collar Mandatory)"
	case domain.HazmatExposurePetrochemicalSpillCoat:
		washAgent = "Warm Lipophilic Dish-Surfactant Emulsification Bath (Dawn 2% Solution) + Normothermic Forced-Air Drying"
	case domain.HazmatExposureToxicAlgaeCyanotoxin:
		washAgent = "High-Volume Warm Fresh-Water Dermal Rinse + Oral Cholestyramine / Activated Charcoal Toxin Binder"
	case domain.HazmatExposureTearGasAerosolPlume, domain.HazmatExposureIndustrialSolventVapor:
		washAgent = "Copious Ophthalmic Lactated Ringer's Irrigation + Neutral pH Mucosal Decontamination Rinse"
	}

	oxygenTherapy := "Ambient Filtered HEPA Recovery Kennel + Continuous Pulse Oximetry Monitoring"
	if smokeInhalationAlert {
		oxygenTherapy = "100% Humidified Normobaric Oxygen Cage (FiO2 0.60–0.90) + Isotonic Saline & N-Acetylcysteine Nebulizer + Hydroxocobalamin Standby"
	} else if req.RespiratoryRateBpm >= 40 || req.AQIIndex >= 150 {
		oxygenTherapy = "Flow-By Supplemental Humidified O2 (3 L/min) + Ultrasonic Saline Coupage Nebulization"
	}

	initialCompleted := 2
	status := domain.HazmatStatusRespiratoryDermalCritical
	if !smokeInhalationAlert && !dermalToxinIngestionAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.HazmatStatusDeconWashNebulizerStaged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeReadinessScore(steps)

	op := domain.HazmatDeconOperation{
		OperationID:               opID,
		PetID:                     petID,
		PetName:                   petName,
		Species:                   species,
		WeightKg:                  req.WeightKg,
		ExposureType:              req.ExposureType,
		Status:                    status,
		AQIIndex:                  req.AQIIndex,
		PM25UgM3:                  req.PM25UgM3,
		CarboxyhemoglobinPct:      req.CarboxyhemoglobinPct,
		CoatContaminationPct:      req.CoatContaminationPct,
		RespiratoryRateBpm:        req.RespiratoryRateBpm,
		HazardScore:               baseHazard,
		DeconReadinessScore:       readiness,
		SmokeInhalationAlert:      smokeInhalationAlert,
		DermalToxinIngestionAlert: dermalToxinIngestionAlert,
		RecommendedWashAgent:      washAgent,
		RecommendedOxygenTherapy:  oxygenTherapy,
		DeconCorridorZone:         corridorZone,
		RiskFlags:                 riskFlags,
		ProtocolSteps:             steps,
		LeadToxTechCallsign:       callsign,
		Notes:                     strings.TrimSpace(req.Notes),
		UpdatedAt:                 nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.HazmatDeconOperation{}, err
	}
	return op, nil
}

func computeReadinessScore(steps []domain.HazmatProtocolStep) int {
	if len(steps) == 0 {
		return 0
	}
	done := 0
	for _, s := range steps {
		if s.Completed {
			done++
		}
	}
	return (done * 100) / len(steps)
}

// UpdateProtocolStep updates the completion state of a protocol checklist step and recomputes readiness and status.
func UpdateProtocolStep(op domain.HazmatDeconOperation, req domain.HazmatStepUpdateRequest) (domain.HazmatDeconOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.HazmatProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)

	for i := range steps {
		if steps[i].ID == stepID {
			steps[i].Completed = req.Completed
			if req.Completed {
				steps[i].CompletedAt = nowStr
			} else {
				steps[i].CompletedAt = ""
			}
			found = true
			break
		}
	}
	if !found {
		return op, fmt.Errorf("protocol step %q not found", stepID)
	}

	op.ProtocolSteps = steps
	op.DeconReadinessScore = computeReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.HazmatStatusDecontaminatedClearedSafe {
		if op.DeconReadinessScore >= 60 {
			op.Status = domain.HazmatStatusDeconWashNebulizerStaged
		} else {
			op.Status = domain.HazmatStatusRespiratoryDermalCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueDeconClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted HAZMAT-SAFE-<12HEX>, and transitions the operation to DECONTAMINATED_CLEARED_SAFE.
func IssueDeconClearance(op domain.HazmatDeconOperation, leadToxTechCallsign, notes string) (domain.HazmatDeconOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue hazmat decon clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadToxTechCallsign); trimmedCallsign != "" {
		op.LeadToxTechCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%d|%.1f|%s",
		op.OperationID,
		op.PetID,
		op.ExposureType,
		op.AQIIndex,
		op.CoatContaminationPct,
		op.LeadToxTechCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "HAZMAT-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.HazmatStatusDecontaminatedClearedSafe
	op.HazardScore = 10
	op.DeconReadinessScore = 100
	op.SmokeInhalationAlert = false
	op.DermalToxinIngestionAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded hazmat & respiratory decon operations (HZ-OP-101, HZ-OP-102, HZ-OP-103).
func SeedOperations() []domain.HazmatDeconOperation {
	baseTime := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)

	op101 := domain.HazmatDeconOperation{
		OperationID:               "HZ-OP-101",
		PetID:                     "PET-HZ-101",
		PetName:                   "Cinder",
		Species:                   "Feline",
		WeightKg:                  4.3,
		ExposureType:              domain.HazmatExposureWildfireSmokeAshfall,
		Status:                    domain.HazmatStatusRespiratoryDermalCritical,
		AQIIndex:                  420,
		PM25UgM3:                  380.0,
		CarboxyhemoglobinPct:      16.5,
		CoatContaminationPct:      65.0,
		RespiratoryRateBpm:        68,
		HazardScore:               96,
		DeconReadinessScore:       40,
		SmokeInhalationAlert:      true,
		DermalToxinIngestionAlert: true,
		RecommendedWashAgent:      "Sterile Buffered Isotonic Saline Corneal Flush + Warm Low-Alkaline Surfactant Ash Wash (E-Collar Mandatory)",
		RecommendedOxygenTherapy:  "100% Humidified Normobaric Oxygen Cage (FiO2 0.60–0.90) + Isotonic Saline & N-Acetylcysteine Nebulizer + Hydroxocobalamin Standby",
		DeconCorridorZone:         "DECON-CORRIDOR-HOT-01 (Cascade Wildfire Perimeter)",
		RiskFlags: []string{
			"EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK",
			"CRITICAL_COAT_GROOMING_TOXICOSIS_RISK",
			"CORNEAL_ASH_ABRASION_HAZARD",
		},
		ProtocolSteps:       DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadToxTechCallsign: "TOX-VET-01 (Dr. Aris Thorne)",
		Notes:               "Feline rescued from structural ember ashfall zone; tachypneic at 68 bpm with COHb 16.5% and heavy alkaline ash coat deposition. E-collar and corneal saline flush initiated.",
		UpdatedAt:           baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.HazmatDeconOperation{
		OperationID:               "HZ-OP-102",
		PetID:                     "PET-HZ-102",
		PetName:                   "Harbor",
		Species:                   "Canine",
		WeightKg:                  22.4,
		ExposureType:              domain.HazmatExposurePetrochemicalSpillCoat,
		Status:                    domain.HazmatStatusDeconWashNebulizerStaged,
		AQIIndex:                  110,
		PM25UgM3:                  42.0,
		CarboxyhemoglobinPct:      3.2,
		CoatContaminationPct:      48.0,
		RespiratoryRateBpm:        36,
		HazardScore:               46,
		DeconReadinessScore:       100,
		SmokeInhalationAlert:      false,
		DermalToxinIngestionAlert: false,
		RecommendedWashAgent:      "Warm Lipophilic Dish-Surfactant Emulsification Bath (Dawn 2% Solution) + Normothermic Forced-Air Drying",
		RecommendedOxygenTherapy:  "Ambient Filtered HEPA Recovery Kennel + Continuous Pulse Oximetry Monitoring",
		DeconCorridorZone:         "DECON-CORRIDOR-WARM-02 (Duwamish Waterway Spill Station)",
		RiskFlags: []string{
			"CRITICAL_COAT_GROOMING_TOXICOSIS_RISK",
		},
		ProtocolSteps:       DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadToxTechCallsign: "TOX-VET-04 (Chen)",
		Notes:               "Retriever exposed to diesel bilge sheen; two warm lipophilic surfactant wash cycles and normothermic drying completed. Ready for veterinary clearance certificate.",
		UpdatedAt:           baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.HazmatDeconOperation{
		OperationID:               "HZ-OP-103",
		PetID:                     "PET-HZ-103",
		PetName:                   "Willow",
		Species:                   "Canine",
		WeightKg:                  18.0,
		ExposureType:              domain.HazmatExposureToxicAlgaeCyanotoxin,
		Status:                    domain.HazmatStatusDecontaminatedClearedSafe,
		AQIIndex:                  45,
		PM25UgM3:                  12.0,
		CarboxyhemoglobinPct:      1.1,
		CoatContaminationPct:      0.0,
		RespiratoryRateBpm:        24,
		HazardScore:               10,
		DeconReadinessScore:       100,
		SmokeInhalationAlert:      false,
		DermalToxinIngestionAlert: false,
		RecommendedWashAgent:      "High-Volume Warm Fresh-Water Dermal Rinse + Oral Cholestyramine / Activated Charcoal Toxin Binder",
		RecommendedOxygenTherapy:  "Ambient Filtered HEPA Recovery Kennel + Continuous Pulse Oximetry Monitoring",
		DeconCorridorZone:         "DECON-CORRIDOR-COLD-03 (Green Lake Cyanotoxin Triage)",
		RiskFlags: []string{
			"CRITICAL_COAT_GROOMING_TOXICOSIS_RISK",
			"HEPATOTOXIC_NEUROTOXIC_BLOOM_HAZARD",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "HAZMAT-SAFE-9F4B21E8C305",
		LeadToxTechCallsign:      "TOX-VET-02 (Patel)",
		Notes:                    "Full dermal rinse and oral cholestyramine binder completed after shoreline microcystin exposure; hepatic panel normal and cleared safe.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.HazmatDeconOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for hazmat & respiratory decontamination operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.HazmatDeconOperation
	order   []string
	counter int
}

// NewStore creates and seeds a HazmatDeconOperation store with HZ-OP-101, HZ-OP-102, and HZ-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.HazmatDeconOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.HazmatDeconOperation) domain.HazmatDeconOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.HazmatProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (HZ-OP-101, HZ-OP-102, HZ-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.HazmatDeconOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.HazmatDeconOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.HazmatDeconOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.HazmatDeconOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new hazmat decontamination operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.HazmatEvaluationRequest) (domain.HazmatDeconOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("HZ-OP-%d", s.counter+1)
	}

	op, err := EvaluateHazmatDecon(req)
	if err != nil {
		return domain.HazmatDeconOperation{}, err
	}
	if autoID {
		s.counter++
	}

	if _, exists := s.ops[op.OperationID]; !exists {
		s.order = append(s.order, op.OperationID)
	}
	s.ops[op.OperationID] = cloneOperation(op)
	return cloneOperation(op), nil
}

// UpdateStep updates a protocol step on an existing operation in the store.
func (s *Store) UpdateStep(id string, req domain.HazmatStepUpdateRequest) (domain.HazmatDeconOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.HazmatDeconOperation{}, fmt.Errorf("hazmat decon operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.HazmatDeconOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues respiratory & dermal decontamination clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadToxTechCallsign, notes string) (domain.HazmatDeconOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.HazmatDeconOperation{}, fmt.Errorf("hazmat decon operation %q not found", key)
	}

	updated, err := IssueDeconClearance(existing, leadToxTechCallsign, notes)
	if err != nil {
		return domain.HazmatDeconOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
