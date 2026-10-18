package usarcollapse

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

// DefaultProtocolSteps constructs the 5 standard FEMA/INSARAG USAR structural collapse & void-space extraction protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.USARProtocolStep {
	steps := []domain.USARProtocolStep{
		{
			ID:       "step-delsar-triangulation",
			Label:    "Deploy 6-sensor Delsar seismic/acoustic geophone array & 360° articulating SearchCam snake into rubble void",
			Category: "DELSAR_SEISMIC_PINPOINT",
			Required: true,
		},
		{
			ID:       "step-utility-lockout-monitor",
			Label:    "Execute natural-gas LEL atmospheric sweep, lock out ruptured utilities & station total-station laser aftershock spotter",
			Category: "DELSAR_SEISMIC_PINPOINT",
			Required: true,
		},
		{
			ID:       "step-box-cribbing-shoring",
			Label:    "Erect 4x4/6x6 timber box-cribbing tower & pneumatic Paratech raker shore before breaching load-bearing slab",
			Category: "BOX_CRIBBING_SHORING",
			Required: true,
		},
		{
			ID:       "step-rotary-core-breach",
			Label:    "Core-drill low-vibration diamond relief aperture & supply positive-pressure HEPA filtered air into void pocket",
			Category: "BOX_CRIBBING_SHORING",
			Required: true,
		},
		{
			ID:       "step-crush-rhabdo-decon",
			Label:    "Administer pre-release IV fluid bolus for crush-syndrome hyperkalemia & execute sterile saline silica eye/coat decon",
			Category: "CRUSH_SYNDROME_EXTRACTION",
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

// EvaluateUSARCollapse deterministically evaluates seismic rubble overburden, secondary collapse risks,
// crush-syndrome rhabdomyolysis / silica inhalation alerts, Delsar acoustic arrays, and shoring readiness.
func EvaluateUSARCollapse(req domain.USAREvaluationRequest) (domain.USARRescueOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.USARRescueOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Canine"
	}

	if !req.VoidType.IsValid() {
		return domain.USARRescueOperation{}, fmt.Errorf("invalid voidType: %q", req.VoidType)
	}
	if req.WeightKg <= 0 || req.WeightKg > 250 {
		return domain.USARRescueOperation{}, fmt.Errorf("weightKg must be in (0, 250], got %v", req.WeightKg)
	}
	if req.RubbleOverburdenMeters < 0 || req.RubbleOverburdenMeters > 30 {
		return domain.USARRescueOperation{}, fmt.Errorf("rubbleOverburdenMeters must be in [0, 30], got %v", req.RubbleOverburdenMeters)
	}
	if req.AftershockPGA < 0 || req.AftershockPGA > 3.0 {
		return domain.USARRescueOperation{}, fmt.Errorf("aftershockPGA must be in [0, 3.0], got %v", req.AftershockPGA)
	}
	if req.SilicaDustMgM3 < 0 || req.SilicaDustMgM3 > 100 {
		return domain.USARRescueOperation{}, fmt.Errorf("silicaDustMgM3 must be in [0, 100], got %v", req.SilicaDustMgM3)
	}
	if req.HoursEntrapped < 0 || req.HoursEntrapped > 720 {
		return domain.USARRescueOperation{}, fmt.Errorf("hoursEntrapped must be in [0, 720], got %v", req.HoursEntrapped)
	}
	if req.StructuralStabilityIdx < 0 || req.StructuralStabilityIdx > 100 {
		return domain.USARRescueOperation{}, fmt.Errorf("structuralStabilityIdx must be in [0, 100], got %d", req.StructuralStabilityIdx)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.2f", petName, species, req.VoidType, req.RubbleOverburdenMeters, req.AftershockPGA)))
		opID = "USAR-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	structureGrid := strings.TrimSpace(req.CollapseStructureGrid)
	if structureGrid == "" {
		structureGrid = "SECTOR-BRAVO-04 (Collapsed Parking Deck B2)"
	}

	callsign := strings.TrimSpace(req.LeadUSARSpecCallsign)
	if callsign == "" {
		callsign = "USAR-TF1-SPEC-01"
	}

	secondaryCollapseAlert := req.StructuralStabilityIdx <= 35 ||
		req.AftershockPGA >= 0.25 ||
		req.VoidType == domain.USARVoidCantileverBeamOverhang

	crushRhabdoAlert := req.HoursEntrapped >= 12.0 ||
		req.RubbleOverburdenMeters >= 2.0 ||
		req.SilicaDustMgM3 >= 5.0

	riskFlags := make([]string, 0, 4)
	if secondaryCollapseAlert {
		riskFlags = append(riskFlags, "EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK")
	}
	if crushRhabdoAlert {
		riskFlags = append(riskFlags, "CRITICAL_CRUSH_SYNDROME_HYPERKALEMIA_RISK")
	}
	if req.AftershockPGA >= 0.30 || req.VoidType == domain.USARVoidSubBasementUtilityVault {
		riskFlags = append(riskFlags, "UTILITY_GAS_RUPTURE_SPARK_HAZARD")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "SILICA_DUST_OCULAR_RESPIRATORY_EXPOSURE")
	}

	baseHazard := 28
	switch req.VoidType {
	case domain.USARVoidPancakeFloorSlabVoid:
		baseHazard = 40
	case domain.USARVoidCantileverBeamOverhang:
		baseHazard = 42
	case domain.USARVoidSubBasementUtilityVault:
		baseHazard = 36
	case domain.USARVoidLeanToTimberTrussCavity:
		baseHazard = 32
	case domain.USARVoidVShapeMasonryRubble:
		baseHazard = 26
	}

	if secondaryCollapseAlert {
		baseHazard += 26
	}
	if crushRhabdoAlert {
		baseHazard += 22
	}
	if req.AftershockPGA >= 0.30 {
		baseHazard += 9
	}

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	acousticArray := "Delsar LD3 6-Geophone Seismic Acoustic Array + SearchCam 3000 Articulating Borehole Snake"
	switch req.VoidType {
	case domain.USARVoidCantileverBeamOverhang:
		acousticArray = "Laser Total-Station Tilt Theodolite + Wireless Piezo Geophone Array + Thermal FLIR Probe"
	case domain.USARVoidSubBasementUtilityVault:
		acousticArray = "Intrinsically Safe 4-Gas LEL Monitor + Sub-Surface Acoustic Hydrophone + Fiber-Optic Borescope"
	}

	shoringRig := "6x6 Douglas Fir 2-Tier Box-Cribbing Tower + Paratech Gold Strut Pneumatic Raker Shore"
	if secondaryCollapseAlert {
		shoringRig = "Heavy Flying-Raker Pneumatic Strut Array + 6x6 Cross-Tied Box Cribbing + Low-Vibration Diamond Core Drill"
	} else if req.VoidType == domain.USARVoidLeanToTimberTrussCavity {
		shoringRig = "4x4 T-Spot Shore + Double-Wedge Timber Box-Cribbing + Positive-Pressure HEPA Air Line"
	}

	initialCompleted := 2
	status := domain.USARRescueStatusSecondaryCollapseCrushCritical
	if !secondaryCollapseAlert && !crushRhabdoAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.USARRescueStatusShoringCribbingDelsarStaged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeReadinessScore(steps)

	op := domain.USARRescueOperation{
		OperationID:              opID,
		PetID:                    petID,
		PetName:                  petName,
		Species:                  species,
		WeightKg:                 req.WeightKg,
		VoidType:                 req.VoidType,
		Status:                   status,
		RubbleOverburdenMeters:   req.RubbleOverburdenMeters,
		AftershockPGA:            req.AftershockPGA,
		SilicaDustMgM3:           req.SilicaDustMgM3,
		HoursEntrapped:           req.HoursEntrapped,
		StructuralStabilityIdx:   req.StructuralStabilityIdx,
		HazardScore:              baseHazard,
		ShoringReadinessScore:    readiness,
		SecondaryCollapseAlert:   secondaryCollapseAlert,
		CrushRhabdoAlert:         crushRhabdoAlert,
		RecommendedAcousticArray: acousticArray,
		RecommendedShoringRig:    shoringRig,
		CollapseStructureGrid:    structureGrid,
		RiskFlags:                riskFlags,
		ProtocolSteps:            steps,
		LeadUSARSpecCallsign:     callsign,
		Notes:                    strings.TrimSpace(req.Notes),
		UpdatedAt:                nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.USARRescueOperation{}, err
	}
	return op, nil
}

func computeReadinessScore(steps []domain.USARProtocolStep) int {
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

// UpdateProtocolStep updates the completion state of a USAR protocol checklist step and recomputes readiness and status.
func UpdateProtocolStep(op domain.USARRescueOperation, req domain.USARStepUpdateRequest) (domain.USARRescueOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.USARProtocolStep, len(op.ProtocolSteps))
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
	op.ShoringReadinessScore = computeReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.USARRescueStatusExtractedUSARDeconSafe {
		if op.ShoringReadinessScore >= 60 {
			op.Status = domain.USARRescueStatusShoringCribbingDelsarStaged
		} else {
			op.Status = domain.USARRescueStatusSecondaryCollapseCrushCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueUSARClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted USAR-SAFE-<12HEX>, and transitions the operation to EXTRACTED_USAR_DECON_SAFE.
func IssueUSARClearance(op domain.USARRescueOperation, leadUSARSpecCallsign, notes string) (domain.USARRescueOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue USAR clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadUSARSpecCallsign); trimmedCallsign != "" {
		op.LeadUSARSpecCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%.1f|%.2f|%s",
		op.OperationID,
		op.PetID,
		op.VoidType,
		op.RubbleOverburdenMeters,
		op.AftershockPGA,
		op.LeadUSARSpecCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "USAR-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.USARRescueStatusExtractedUSARDeconSafe
	op.HazardScore = 10
	op.ShoringReadinessScore = 100
	op.SecondaryCollapseAlert = false
	op.CrushRhabdoAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded USAR structural collapse operations (USAR-OP-101, USAR-OP-102, USAR-OP-103).
func SeedOperations() []domain.USARRescueOperation {
	baseTime := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)

	op101 := domain.USARRescueOperation{
		OperationID:              "USAR-OP-101",
		PetID:                    "PET-USAR-101",
		PetName:                  "Diesel",
		Species:                  "Canine",
		WeightKg:                 31.0,
		VoidType:                 domain.USARVoidPancakeFloorSlabVoid,
		Status:                   domain.USARRescueStatusSecondaryCollapseCrushCritical,
		RubbleOverburdenMeters:   2.8,
		AftershockPGA:            0.34,
		SilicaDustMgM3:           8.2,
		HoursEntrapped:           18.5,
		StructuralStabilityIdx:   22,
		HazardScore:              97,
		ShoringReadinessScore:    40,
		SecondaryCollapseAlert:   true,
		CrushRhabdoAlert:         true,
		RecommendedAcousticArray: "Delsar LD3 6-Geophone Seismic Acoustic Array + SearchCam 3000 Articulating Borehole Snake",
		RecommendedShoringRig:    "Heavy Flying-Raker Pneumatic Strut Array + 6x6 Cross-Tied Box Cribbing + Low-Vibration Diamond Core Drill",
		CollapseStructureGrid:    "SECTOR-BRAVO-04 (Collapsed Reinforced Concrete Parking Deck B2)",
		RiskFlags: []string{
			"EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK",
			"CRITICAL_CRUSH_SYNDROME_HYPERKALEMIA_RISK",
			"UTILITY_GAS_RUPTURE_SPARK_HAZARD",
		},
		ProtocolSteps:        DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadUSARSpecCallsign: "USAR-TF1-SPEC-01 (Castellano)",
		Notes:                "German Shepherd entrapped 2.8m beneath pancake reinforced-concrete slab void; Delsar geophone array locked on rhythmic scratch/whine in Quadrant B2. Erecting 6x6 box-cribbing tower before diamond core breach.",
		UpdatedAt:            baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.USARRescueOperation{
		OperationID:              "USAR-OP-102",
		PetID:                    "PET-USAR-102",
		PetName:                  "Milo",
		Species:                  "Feline",
		WeightKg:                 4.6,
		VoidType:                 domain.USARVoidLeanToTimberTrussCavity,
		Status:                   domain.USARRescueStatusShoringCribbingDelsarStaged,
		RubbleOverburdenMeters:   1.2,
		AftershockPGA:            0.12,
		SilicaDustMgM3:           2.1,
		HoursEntrapped:           6.0,
		StructuralStabilityIdx:   72,
		HazardScore:              46,
		ShoringReadinessScore:    100,
		SecondaryCollapseAlert:   false,
		CrushRhabdoAlert:         false,
		RecommendedAcousticArray: "Delsar LD3 6-Geophone Seismic Acoustic Array + SearchCam 3000 Articulating Borehole Snake",
		RecommendedShoringRig:    "4x4 T-Spot Shore + Double-Wedge Timber Box-Cribbing + Positive-Pressure HEPA Air Line",
		CollapseStructureGrid:    "SECTOR-ALPHA-09 (Lean-To Timber Truss Residential Void)",
		RiskFlags: []string{
			"SILICA_DUST_OCULAR_RESPIRATORY_EXPOSURE",
		},
		ProtocolSteps:        DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadUSARSpecCallsign: "USAR-TF1-SPEC-03 (Nakamura)",
		Notes:                "Domestic Shorthair pinpointed inside lean-to timber roof truss cavity via articulating SearchCam; 4x4 T-shore wedged and core relief aperture cut for safe extraction.",
		UpdatedAt:            baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.USARRescueOperation{
		OperationID:              "USAR-OP-103",
		PetID:                    "PET-USAR-103",
		PetName:                  "Rosie",
		Species:                  "Canine",
		WeightKg:                 19.5,
		VoidType:                 domain.USARVoidVShapeMasonryRubble,
		Status:                   domain.USARRescueStatusExtractedUSARDeconSafe,
		RubbleOverburdenMeters:   0.5,
		AftershockPGA:            0.05,
		SilicaDustMgM3:           0.4,
		HoursEntrapped:           2.0,
		StructuralStabilityIdx:   92,
		HazardScore:              10,
		ShoringReadinessScore:    100,
		SecondaryCollapseAlert:   false,
		CrushRhabdoAlert:         false,
		RecommendedAcousticArray: "Delsar LD3 6-Geophone Seismic Acoustic Array + SearchCam 3000 Articulating Borehole Snake",
		RecommendedShoringRig:    "6x6 Douglas Fir 2-Tier Box-Cribbing Tower + Paratech Gold Strut Pneumatic Raker Shore",
		CollapseStructureGrid:    "SECTOR-CHARLIE-02 (Unreinforced Masonry V-Shape Pocket)",
		RiskFlags: []string{
			"SILICA_DUST_OCULAR_RESPIRATORY_EXPOSURE",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "USAR-SAFE-5C9D41B8E702",
		LeadUSARSpecCallsign:     "USAR-TF1-SPEC-02 (Kowalski)",
		Notes:                    "Australian Shepherd extracted from V-shape masonry rubble void; sterile saline ocular flush and silica particulate decontamination completed with normal serum potassium.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.USARRescueOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for USAR structural collapse rescue operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.USARRescueOperation
	order   []string
	counter int
}

// NewStore creates and seeds a USARRescueOperation store with USAR-OP-101, USAR-OP-102, and USAR-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.USARRescueOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.USARRescueOperation) domain.USARRescueOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.USARProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (USAR-OP-101, USAR-OP-102, USAR-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.USARRescueOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.USARRescueOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.USARRescueOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.USARRescueOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new USAR structural collapse operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.USAREvaluationRequest) (domain.USARRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("USAR-OP-%d", s.counter+1)
	}

	op, err := EvaluateUSARCollapse(req)
	if err != nil {
		return domain.USARRescueOperation{}, err
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
func (s *Store) UpdateStep(id string, req domain.USARStepUpdateRequest) (domain.USARRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.USARRescueOperation{}, fmt.Errorf("USAR rescue operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.USARRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues USAR void extraction & silica decontamination clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadUSARSpecCallsign, notes string) (domain.USARRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.USARRescueOperation{}, fmt.Errorf("USAR rescue operation %q not found", key)
	}

	updated, err := IssueUSARClearance(existing, leadUSARSpecCallsign, notes)
	if err != nil {
		return domain.USARRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
