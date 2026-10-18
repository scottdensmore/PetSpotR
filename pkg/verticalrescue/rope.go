package verticalrescue

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func defaultProtocolSteps() []domain.VerticalProtocolStep {
	return []domain.VerticalProtocolStep{
		{
			ID:        "vr-step-dropzone-net",
			Label:     "Deploy 6m x 6m ballistic break-fall catch net & ground spotter perimeter",
			Category:  "DROP_ZONE_NETTING",
			Required:  true,
			Completed: false,
		},
		{
			ID:        "vr-step-twin-anchor",
			Label:     "Rig NFPA 2500 / ANSI Z133 Twin-Tension Rope System (TTRS) main & belay anchors",
			Category:  "TWIN_TENSION_ANCHOR",
			Required:  true,
			Completed: false,
		},
		{
			ID:        "vr-step-throwline-cambium",
			Label:     "Install friction-saver cambium/edge-roller protection & verify 10:1 safety factor",
			Category:  "TWIN_TENSION_ANCHOR",
			Required:  true,
			Completed: false,
		},
		{
			ID:        "vr-step-calming-bag",
			Label:     "Secure animal inside breathable dark-mesh calming capture bag / dorsal webbing harness",
			Category:  "CALMING_HARNESS_BAG",
			Required:  true,
			Completed: false,
		},
		{
			ID:        "vr-step-controlled-lower",
			Label:     "Execute synchronized MPD/descender controlled lower to ground triage team",
			Category:  "CALMING_HARNESS_BAG",
			Required:  true,
			Completed: false,
		},
	}
}

func recommendedEquipment(env domain.VerticalEnvironmentType, species string, weightKg float64) (ropeSystem, captureBag, dropZone string) {
	switch env {
	case domain.VerticalEnvTreeCanopyCrown:
		ropeSystem = "ANSI Z133 SRS/MRS Arborist 11.7mm Static Kernmantle Twin-Line + Cambium Friction Saver"
		dropZone = "DZ-CANOPY-RADIAL-8M (360° Ballistic Break-Fall Catch Net)"
	case domain.VerticalEnvCliffsideRockLedge:
		ropeSystem = "NFPA 2500 11mm Twin-Tension Rope System (TTRS) + Edge Vortex Artificial High Directional"
		dropZone = "DZ-TALUS-BASE-6M (High-Angle Scree Cushion & Belay Corridor)"
	case domain.VerticalEnvHighRiseBalconyParapet:
		ropeSystem = "High-Rise Parapet Davit Arm + Twin 11mm Low-Stretch Kernmantle Belay & Edge Pad"
		dropZone = "DZ-PLAZA-CORDON-10M (Inflatable Landing Cushion & Balcony Net)"
	case domain.VerticalEnvBridgeOverpassGirder:
		ropeSystem = "Structural Steel Girder Beam-Clamp Anchor + Twin-Tension MPD Lowering System"
		dropZone = "DZ-UNDERPASS-LANE-12M (Traffic Lane Closure & Catch Netting)"
	case domain.VerticalEnvUtilityTowerCrossarm:
		ropeSystem = "Dielectric Insulated Fiberglass Hot-Stick Tagline + Non-Conductive Aramid Twin Rig"
		dropZone = "DZ-SUBSTATION-ARC-15M (De-Energized Ground Exclusion Perimeter)"
	default:
		ropeSystem = "NFPA 2500 Twin-Tension 11mm Static Kernmantle System"
		dropZone = "DZ-STANDARD-6M"
	}

	upperSpecies := strings.ToUpper(strings.TrimSpace(species))
	if strings.Contains(upperSpecies, "CAT") || strings.Contains(upperSpecies, "FELINE") || weightKg < 8.0 {
		captureBag = "Breathable Ripstop Dark-Mesh Feline Calming Bag + Telescopic Snare Pole"
	} else {
		captureBag = "Full-Body Canine Dorsal-Lift Webbing Rescue Harness + Muzzle Calming Hood"
	}
	return ropeSystem, captureBag, dropZone
}

// EvaluateVerticalRescue deterministically evaluates vertical rescue hazards, alerts, equipment, and protocol steps.
func EvaluateVerticalRescue(req domain.VerticalRescueEvaluationRequest) (domain.VerticalRescueOperation, error) {
	petID := strings.TrimSpace(req.PetID)
	petName := strings.TrimSpace(req.PetName)
	species := strings.TrimSpace(req.Species)
	if petID == "" {
		return domain.VerticalRescueOperation{}, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return domain.VerticalRescueOperation{}, fmt.Errorf("petName is required")
	}
	if species == "" {
		species = "Feline"
	}
	if req.WeightKg <= 0 {
		req.WeightKg = 4.5
	}
	if req.WeightKg > 250 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("weightKg must be <= 250, got %v", req.WeightKg)
	}
	env := req.EnvironmentType
	if env == "" {
		env = domain.VerticalEnvTreeCanopyCrown
	}
	if !env.IsValid() {
		return domain.VerticalRescueOperation{}, fmt.Errorf("invalid environmentType: %q", env)
	}
	if req.ElevationMeters < 1.0 || req.ElevationMeters > 500.0 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("elevationMeters must be in [1.0, 500.0], got %v", req.ElevationMeters)
	}
	if req.SlopeAngleDeg < 0.0 || req.SlopeAngleDeg > 90.0 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("slopeAngleDeg must be in [0.0, 90.0], got %v", req.SlopeAngleDeg)
	}
	if req.WindGustKnots < 0.0 || req.WindGustKnots > 120.0 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("windGustKnots must be in [0.0, 120.0], got %v", req.WindGustKnots)
	}
	if req.HoursStranded < 0.0 || req.HoursStranded > 720.0 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("hoursStranded must be in [0.0, 720.0], got %v", req.HoursStranded)
	}
	if req.BranchStabilityIndex < 0 || req.BranchStabilityIndex > 100 {
		return domain.VerticalRescueOperation{}, fmt.Errorf("branchStabilityIndex must be in [0, 100], got %d", req.BranchStabilityIndex)
	}

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f", petID, petName, env, req.ElevationMeters, req.WindGustKnots)))
		opID = "VR-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	callsign := strings.TrimSpace(req.LeadRiggerCallsign)
	if callsign == "" {
		callsign = "ARBOR-LEAD-1"
	}

	fallShockAlert := req.ElevationMeters >= 12.0 || req.SlopeAngleDeg >= 75.0 || req.HoursStranded >= 24.0
	canopyFailureAlert := req.BranchStabilityIndex <= 35 || req.WindGustKnots >= 25.0

	flags := make([]string, 0, 4)
	if fallShockAlert {
		flags = append(flags, "EXTREME_VERTICAL_FALL_TRAUMA_RISK")
	}
	if canopyFailureAlert {
		flags = append(flags, "CRITICAL_DEADWOOD_WIND_SHEAR_HAZARD")
	}
	upperSpecies := strings.ToUpper(species)
	if (strings.Contains(upperSpecies, "FELINE") || strings.Contains(upperSpecies, "CAT")) && (req.ElevationMeters >= 12.0 || req.HoursStranded >= 12.0) {
		flags = append(flags, "FELINE_PANIC_LEAP_SYNDROME")
	}
	if env == domain.VerticalEnvUtilityTowerCrossarm {
		flags = append(flags, "HIGH_VOLTAGE_ARC_EXCLUSION_ZONE")
	}
	if len(flags) == 0 {
		flags = append(flags, "STANDARD_VERTICAL_BELAY_PRECAUTION")
	}

	// Compute deterministic hazard score [10..99]
	elevComponent := math.Min(35.0, req.ElevationMeters*1.2)
	slopeComponent := math.Min(20.0, req.SlopeAngleDeg*0.22)
	windComponent := math.Min(20.0, req.WindGustKnots*0.65)
	hoursComponent := math.Min(15.0, req.HoursStranded*0.4)
	instabilityComponent := math.Max(0.0, float64(100-req.BranchStabilityIndex)*0.22)

	rawHazard := int(math.Round(elevComponent + slopeComponent + windComponent + hoursComponent + instabilityComponent))
	if rawHazard > 98 {
		rawHazard = 98
	}
	if rawHazard < 15 {
		rawHazard = 15
	}

	status := domain.VerticalStatusTwinTensionRigged
	if fallShockAlert || canopyFailureAlert || rawHazard >= 65 {
		status = domain.VerticalStatusFallShockCritical
	}

	ropeSys, bag, dz := recommendedEquipment(env, species, req.WeightKg)
	steps := defaultProtocolSteps()

	op := domain.VerticalRescueOperation{
		OperationID:            opID,
		PetID:                  petID,
		PetName:                petName,
		Species:                species,
		WeightKg:               req.WeightKg,
		EnvironmentType:        env,
		Status:                 status,
		ElevationMeters:        req.ElevationMeters,
		SlopeAngleDeg:          req.SlopeAngleDeg,
		WindGustKnots:          req.WindGustKnots,
		HoursStranded:          req.HoursStranded,
		BranchStabilityIndex:   req.BranchStabilityIndex,
		HazardScore:            rawHazard,
		RiggingReadinessScore:  0,
		HighRiseFallShockAlert: fallShockAlert,
		CanopyFailureAlert:     canopyFailureAlert,
		RecommendedRopeSystem:  ropeSys,
		RecommendedCaptureBag:  bag,
		DropZoneGrid:           dz,
		RiskFlags:              flags,
		ProtocolSteps:          steps,
		LeadRiggerCallsign:     callsign,
		Notes:                  strings.TrimSpace(req.Notes),
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339),
	}

	if err := op.Validate(); err != nil {
		return domain.VerticalRescueOperation{}, err
	}
	return op, nil
}

// UpdateProtocolStep updates a protocol step on a VerticalRescueOperation and recomputes readiness and status.
func UpdateProtocolStep(op domain.VerticalRescueOperation, req domain.VerticalStepUpdateRequest) (domain.VerticalRescueOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return domain.VerticalRescueOperation{}, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.VerticalProtocolStep, len(op.ProtocolSteps))
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
		return domain.VerticalRescueOperation{}, fmt.Errorf("protocol step %q not found", stepID)
	}

	op.ProtocolSteps = steps
	completedCount := 0
	allRequiredDone := true
	for _, s := range op.ProtocolSteps {
		if s.Completed {
			completedCount++
		} else if s.Required {
			allRequiredDone = false
		}
	}

	if len(op.ProtocolSteps) > 0 {
		op.RiggingReadinessScore = (completedCount * 100) / len(op.ProtocolSteps)
	}

	if op.Status != domain.VerticalStatusLoweredGroundSafe {
		if allRequiredDone || completedCount >= 3 {
			op.Status = domain.VerticalStatusTwinTensionRigged
			if allRequiredDone {
				op.HighRiseFallShockAlert = false
				op.CanopyFailureAlert = false
				if op.HazardScore > 48 {
					op.HazardScore = 48
				}
			}
		} else {
			op.HighRiseFallShockAlert = op.ElevationMeters >= 12.0 || op.SlopeAngleDeg >= 75.0 || op.HoursStranded >= 24.0
			op.CanopyFailureAlert = op.BranchStabilityIndex <= 35 || op.WindGustKnots >= 25.0
			if op.HighRiseFallShockAlert || op.CanopyFailureAlert {
				op.Status = domain.VerticalStatusFallShockCritical
			}
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueVerticalClearance verifies required protocol steps and issues a deterministic SHA-256 clearance hash.
func IssueVerticalClearance(op domain.VerticalRescueOperation) (domain.VerticalRescueOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return domain.VerticalRescueOperation{}, fmt.Errorf("operationId is required")
	}
	for _, s := range op.ProtocolSteps {
		if s.Required && !s.Completed {
			return domain.VerticalRescueOperation{}, fmt.Errorf("cannot issue clearance: required step %q (%s) is incomplete", s.ID, s.Label)
		}
	}

	payload := fmt.Sprintf("VERTICAL-SAFE|%s|%s|%s|%.1f", op.OperationID, op.PetID, op.EnvironmentType, op.ElevationMeters)
	digest := sha256.Sum256([]byte(payload))
	hashHex := strings.ToUpper(hex.EncodeToString(digest[:])[:12])

	op.Status = domain.VerticalStatusLoweredGroundSafe
	op.HazardScore = 10
	op.RiggingReadinessScore = 100
	op.HighRiseFallShockAlert = false
	op.CanopyFailureAlert = false
	op.ClearanceCertificateHash = "VERTICAL-SAFE-" + hashHex
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return op, nil
}

// DefaultSeedOperations returns the 3 realistic seeded vertical rescue operations.
func DefaultSeedOperations() []domain.VerticalRescueOperation {
	steps101 := defaultProtocolSteps()
	steps101[0].Completed = true
	steps101[0].CompletedAt = "2026-10-18T18:10:00Z"
	steps101[1].Completed = true
	steps101[1].CompletedAt = "2026-10-18T18:18:00Z"

	steps102 := defaultProtocolSteps()
	for i := range steps102 {
		steps102[i].Completed = true
		steps102[i].CompletedAt = "2026-10-18T19:05:00Z"
	}

	steps103 := defaultProtocolSteps()
	for i := range steps103 {
		steps103[i].Completed = true
		steps103[i].CompletedAt = "2026-10-18T17:40:00Z"
	}

	return []domain.VerticalRescueOperation{
		{
			OperationID:              "VR-OP-101",
			PetID:                    "PET-VR-901",
			PetName:                  "Cleo",
			Species:                  "Feline",
			WeightKg:                 4.1,
			EnvironmentType:          domain.VerticalEnvTreeCanopyCrown,
			Status:                   domain.VerticalStatusFallShockCritical,
			ElevationMeters:          22.5,
			SlopeAngleDeg:            85.0,
			WindGustKnots:            28.0,
			HoursStranded:            36.0,
			BranchStabilityIndex:     25,
			HazardScore:              96,
			RiggingReadinessScore:    40,
			HighRiseFallShockAlert:   true,
			CanopyFailureAlert:       true,
			RecommendedRopeSystem:    "ANSI Z133 SRS/MRS Arborist 11.7mm Static Kernmantle Twin-Line + Cambium Friction Saver",
			RecommendedCaptureBag:    "Breathable Ripstop Dark-Mesh Feline Calming Bag + Telescopic Snare Pole",
			DropZoneGrid:             "DZ-CANOPY-RADIAL-8M (360° Ballistic Break-Fall Catch Net)",
			RiskFlags:                []string{"EXTREME_VERTICAL_FALL_TRAUMA_RISK", "CRITICAL_DEADWOOD_WIND_SHEAR_HAZARD", "FELINE_PANIC_LEAP_SYNDROME"},
			ProtocolSteps:            steps101,
			ClearanceCertificateHash: "",
			LeadRiggerCallsign:       "ARBOR-7 (ISA Certified Arborist)",
			Notes:                    "Stranded 22.5m high in brittle deadwood Sitka spruce crown under 28kt gusts; deploying ballistic catch net.",
			UpdatedAt:                "2026-10-18T19:30:00Z",
		},
		{
			OperationID:              "VR-OP-102",
			PetID:                    "PET-VR-902",
			PetName:                  "Koda",
			Species:                  "Canine",
			WeightKg:                 26.5,
			EnvironmentType:          domain.VerticalEnvCliffsideRockLedge,
			Status:                   domain.VerticalStatusTwinTensionRigged,
			ElevationMeters:          14.0,
			SlopeAngleDeg:            78.0,
			WindGustKnots:            12.0,
			HoursStranded:            6.0,
			BranchStabilityIndex:     80,
			HazardScore:              48,
			RiggingReadinessScore:    100,
			HighRiseFallShockAlert:   false,
			CanopyFailureAlert:       false,
			RecommendedRopeSystem:    "NFPA 2500 11mm Twin-Tension Rope System (TTRS) + Edge Vortex Artificial High Directional",
			RecommendedCaptureBag:    "Full-Body Canine Dorsal-Lift Webbing Rescue Harness + Muzzle Calming Hood",
			DropZoneGrid:             "DZ-TALUS-BASE-6M (High-Angle Scree Cushion & Belay Corridor)",
			RiskFlags:                []string{"EXTREME_VERTICAL_FALL_TRAUMA_RISK"},
			ProtocolSteps:            steps102,
			ClearanceCertificateHash: "",
			LeadRiggerCallsign:       "ROPE-TECH-4 (NFPA 1006 Lead)",
			Notes:                    "Basalt gorge ledge 14m above talus slope; TTRS main/belay rigged and dorsal harness secured, ready for final ground clearance.",
			UpdatedAt:                "2026-10-18T19:15:00Z",
		},
		{
			OperationID:              "VR-OP-103",
			PetID:                    "PET-VR-903",
			PetName:                  "Mochi",
			Species:                  "Feline",
			WeightKg:                 3.8,
			EnvironmentType:          domain.VerticalEnvHighRiseBalconyParapet,
			Status:                   domain.VerticalStatusLoweredGroundSafe,
			ElevationMeters:          18.0,
			SlopeAngleDeg:            90.0,
			WindGustKnots:            8.0,
			HoursStranded:            2.0,
			BranchStabilityIndex:     95,
			HazardScore:              10,
			RiggingReadinessScore:    100,
			HighRiseFallShockAlert:   false,
			CanopyFailureAlert:       false,
			RecommendedRopeSystem:    "High-Rise Parapet Davit Arm + Twin 11mm Low-Stretch Kernmantle Belay & Edge Pad",
			RecommendedCaptureBag:    "Breathable Ripstop Dark-Mesh Feline Calming Bag + Telescopic Snare Pole",
			DropZoneGrid:             "DZ-PLAZA-CORDON-10M (Inflatable Landing Cushion & Balcony Net)",
			RiskFlags:                []string{"EXTREME_VERTICAL_FALL_TRAUMA_RISK", "FELINE_PANIC_LEAP_SYNDROME"},
			ProtocolSteps:            steps103,
			ClearanceCertificateHash: "VERTICAL-SAFE-5E8A29C1D407",
			LeadRiggerCallsign:       "HIGH-ANGLE-2 (Tower Rigging Specialist)",
			Notes:                    "6th-floor exterior parapet ledge extraction completed via dark-mesh calming bag; cat lowered safely to veterinary ground team.",
			UpdatedAt:                "2026-10-18T18:00:00Z",
		},
	}
}

// Store manages active VerticalRescueOperation records in memory with thread safety.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.VerticalRescueOperation
	order   []string
	counter int
}

// NewStore creates a new vertical rescue store seeded with the 3 default operations.
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]domain.VerticalRescueOperation),
		order: make([]string, 0, 8),
	}
	for _, op := range DefaultSeedOperations() {
		s.ops[op.OperationID] = op
		s.order = append(s.order, op.OperationID)
	}
	s.counter = 103
	return s
}

func (s *Store) List() []domain.VerticalRescueOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.VerticalRescueOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			result = append(result, op)
		}
	}
	return result
}

func (s *Store) Get(id string) (domain.VerticalRescueOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	return op, ok
}

func (s *Store) Evaluate(req domain.VerticalRescueEvaluationRequest) (domain.VerticalRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := false
	if strings.TrimSpace(req.OperationID) == "" {
		autoID = true
		req.OperationID = fmt.Sprintf("VR-OP-%d", s.counter+1)
	}

	op, err := EvaluateVerticalRescue(req)
	if err != nil {
		return domain.VerticalRescueOperation{}, err
	}
	if autoID {
		s.counter++
	}

	if _, exists := s.ops[op.OperationID]; !exists {
		s.order = append([]string{op.OperationID}, s.order...)
	}
	s.ops[op.OperationID] = op
	return op, nil
}

func (s *Store) UpdateStep(id string, req domain.VerticalStepUpdateRequest) (domain.VerticalRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.VerticalRescueOperation{}, fmt.Errorf("vertical rescue operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.VerticalRescueOperation{}, err
	}
	s.ops[key] = updated
	return updated, nil
}

func (s *Store) IssueClearance(id string) (domain.VerticalRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.VerticalRescueOperation{}, fmt.Errorf("vertical rescue operation %q not found", key)
	}

	updated, err := IssueVerticalClearance(existing)
	if err != nil {
		return domain.VerticalRescueOperation{}, err
	}
	s.ops[key] = updated
	return updated, nil
}
