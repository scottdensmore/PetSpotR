package avalanchewinter

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

// DefaultProtocolSteps constructs the 5 standard cold-weather avalanche & hypothermia rewarming SAR protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.WinterProtocolStep {
	steps := []domain.WinterProtocolStep{
		{
			ID:       "step-beacon-pinpoint",
			Label:    "Execute 457 kHz transceiver fine-grid bracket & RECCO harmonic radar sweep to pinpoint burial coordinates",
			Category: "BEACON_PROBE_PINPOINT",
			Required: true,
		},
		{
			ID:       "step-probe-spiral",
			Label:    "Deploy 320cm carbon avalanche probe in 25cm concentric spiral & leave probe in place upon strike",
			Category: "BEACON_PROBE_PINPOINT",
			Required: true,
		},
		{
			ID:       "step-v-conveyor-trench",
			Label:    "Excavate downhill V-conveyor snow trench & clear ice-plug airway pocket without collapsing roof",
			Category: "AIRWAY_SNOW_EXCAVATION",
			Required: true,
		},
		{
			ID:       "step-horizontal-afterdrop-lift",
			Label:    "Extract horizontally in insulated litter to prevent cold peripheral blood afterdrop cardiac arrhythmia",
			Category: "AIRWAY_SNOW_EXCAVATION",
			Required: true,
		},
		{
			ID:       "step-active-core-rewarm",
			Label:    "Stage active thoracic chemical heat-pack bivy, warm humidified O2 & paw frostbite sterile wrap",
			Category: "ACTIVE_CORE_REWARMING",
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

// EvaluateWinterRescue deterministically evaluates snowpack burial asphyxia risks, hypothermia afterdrop alerts,
// locator gear recommendations, and rewarming readiness for a cold-weather SAR pet rescue.
func EvaluateWinterRescue(req domain.WinterEvaluationRequest) (domain.WinterRescueOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.WinterRescueOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Canine"
	}

	if !req.TerrainType.IsValid() {
		return domain.WinterRescueOperation{}, fmt.Errorf("invalid terrainType: %q", req.TerrainType)
	}
	if req.WeightKg <= 0 || req.WeightKg > 250 {
		return domain.WinterRescueOperation{}, fmt.Errorf("weightKg must be in (0, 250], got %v", req.WeightKg)
	}
	if req.BurialDepthCm < 0 || req.BurialDepthCm > 1000 {
		return domain.WinterRescueOperation{}, fmt.Errorf("burialDepthCm must be in [0, 1000], got %v", req.BurialDepthCm)
	}
	if req.WindChillCelsius < -70 || req.WindChillCelsius > 20 {
		return domain.WinterRescueOperation{}, fmt.Errorf("windChillCelsius must be in [-70, 20], got %v", req.WindChillCelsius)
	}
	if req.CoreTempCelsius < 20 || req.CoreTempCelsius > 43 {
		return domain.WinterRescueOperation{}, fmt.Errorf("coreTempCelsius must be in [20, 43], got %v", req.CoreTempCelsius)
	}
	if req.MinutesBuried < 0 || req.MinutesBuried > 1440 {
		return domain.WinterRescueOperation{}, fmt.Errorf("minutesBuried must be in [0, 1440], got %d", req.MinutesBuried)
	}
	if req.AvalancheDangerScale < 1 || req.AvalancheDangerScale > 5 {
		return domain.WinterRescueOperation{}, fmt.Errorf("avalancheDangerScale must be in [1, 5], got %d", req.AvalancheDangerScale)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f", petName, species, req.TerrainType, req.BurialDepthCm, req.CoreTempCelsius)))
		opID = "AW-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	sectorGrid := strings.TrimSpace(req.SnowpackSectorGrid)
	if sectorGrid == "" {
		sectorGrid = "ALPINE-BOWL-01A"
	}

	callsign := strings.TrimSpace(req.LeadSkiPatrolCallsign)
	if callsign == "" {
		callsign = "PATROL-ALPINE-01"
	}

	asphyxiaAirPocketAlert := req.BurialDepthCm >= 50.0 ||
		req.MinutesBuried >= 15 ||
		req.TerrainType == domain.WinterTerrainTreeWellPowderPit

	severeFrostbiteAfterdropAlert := req.CoreTempCelsius <= 35.0 ||
		req.WindChillCelsius <= -15.0

	riskFlags := make([]string, 0, 4)
	if asphyxiaAirPocketAlert {
		riskFlags = append(riskFlags, "EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK")
	}
	if severeFrostbiteAfterdropAlert {
		riskFlags = append(riskFlags, "CRITICAL_HYPOTHERMIC_AFTERDROP_ARRHYTHMIA_RISK")
	}
	if req.AvalancheDangerScale >= 4 {
		riskFlags = append(riskFlags, "SECONDARY_SLAB_TRIGGER_HAZARD")
	}
	if req.TerrainType == domain.WinterTerrainGlacialCrevasseLedge {
		riskFlags = append(riskFlags, "CREVASSE_SNOW_BRIDGE_COLLAPSE_HAZARD")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "SUBZERO_PAW_FROSTBITE_EXPOSURE_ZONE")
	}

	baseHazard := 26
	switch req.TerrainType {
	case domain.WinterTerrainAvalancheDebrisChute:
		baseHazard = 38
	case domain.WinterTerrainTreeWellPowderPit:
		baseHazard = 36
	case domain.WinterTerrainGlacialCrevasseLedge:
		baseHazard = 40
	case domain.WinterTerrainBlizzardSnowdriftTrench:
		baseHazard = 30
	case domain.WinterTerrainSubzeroFrozenTrail:
		baseHazard = 24
	}

	if asphyxiaAirPocketAlert {
		baseHazard += 26
	}
	if severeFrostbiteAfterdropAlert {
		baseHazard += 22
	}
	if req.AvalancheDangerScale >= 4 {
		baseHazard += 8
	}
	if req.BurialDepthCm >= 75.0 {
		baseHazard += 6
	}

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	locatorGear := "457 kHz Digital 3-Antenna Avalanche Transceiver + 320cm Carbon Strike Probe + RECCO Harmonic Detector"
	switch req.TerrainType {
	case domain.WinterTerrainTreeWellPowderPit:
		locatorGear = "320cm Sectional Carbon Probe + Downhill V-Trench Collapsible Aluminum Shovel + Thermal Drone FLIR"
	case domain.WinterTerrainGlacialCrevasseLedge:
		locatorGear = "Crevasse Belay Tripod + Articulating Borehole Thermal Camera + 457 kHz Collar Transceiver"
	}

	rewarmingBivy := "Active Thoracic Chemical Heat-Pack Vapor-Barrier Bivy + Warm Humidified O2 Mask (40°C)"
	if severeFrostbiteAfterdropAlert {
		rewarmingBivy = "Horizontal Anti-Afterdrop Vacuum Litter + Forced-Air Thoracic Rewarming Cocoon + Warmed IV/O2 Pack"
	} else if req.TerrainType == domain.WinterTerrainSubzeroFrozenTrail {
		rewarmingBivy = "Insulated Reflective Fleece-Lined Recovery Wrap + Sterile Warm-Water Paw Frostbite Booties"
	}

	initialCompleted := 2
	status := domain.WinterRescueStatusBurialHypothermiaCritical
	if !asphyxiaAirPocketAlert && !severeFrostbiteAfterdropAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.WinterRescueStatusProbeTrenchRewarmingStaged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeReadinessScore(steps)

	op := domain.WinterRescueOperation{
		OperationID:                   opID,
		PetID:                         petID,
		PetName:                       petName,
		Species:                       species,
		WeightKg:                      req.WeightKg,
		TerrainType:                   req.TerrainType,
		Status:                        status,
		BurialDepthCm:                 req.BurialDepthCm,
		WindChillCelsius:              req.WindChillCelsius,
		CoreTempCelsius:               req.CoreTempCelsius,
		MinutesBuried:                 req.MinutesBuried,
		AvalancheDangerScale:          req.AvalancheDangerScale,
		HazardScore:                   baseHazard,
		RewarmingReadinessScore:       readiness,
		AsphyxiaAirPocketAlert:        asphyxiaAirPocketAlert,
		SevereFrostbiteAfterdropAlert: severeFrostbiteAfterdropAlert,
		RecommendedLocatorGear:        locatorGear,
		RecommendedRewarmingBivy:      rewarmingBivy,
		SnowpackSectorGrid:            sectorGrid,
		RiskFlags:                     riskFlags,
		ProtocolSteps:                 steps,
		LeadSkiPatrolCallsign:         callsign,
		Notes:                         strings.TrimSpace(req.Notes),
		UpdatedAt:                     nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.WinterRescueOperation{}, err
	}
	return op, nil
}

func computeReadinessScore(steps []domain.WinterProtocolStep) int {
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

// UpdateProtocolStep updates the completion state of a winter protocol checklist step and recomputes readiness and status.
func UpdateProtocolStep(op domain.WinterRescueOperation, req domain.WinterStepUpdateRequest) (domain.WinterRescueOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.WinterProtocolStep, len(op.ProtocolSteps))
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
	op.RewarmingReadinessScore = computeReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.WinterRescueStatusExtractedNormothermicSafe {
		if op.RewarmingReadinessScore >= 60 {
			op.Status = domain.WinterRescueStatusProbeTrenchRewarmingStaged
		} else {
			op.Status = domain.WinterRescueStatusBurialHypothermiaCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueWinterClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted WINTER-SAFE-<12HEX>, and transitions the operation to EXTRACTED_NORMOTHERMIC_SAFE.
func IssueWinterClearance(op domain.WinterRescueOperation, leadSkiPatrolCallsign, notes string) (domain.WinterRescueOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue winter clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadSkiPatrolCallsign); trimmedCallsign != "" {
		op.LeadSkiPatrolCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%s",
		op.OperationID,
		op.PetID,
		op.TerrainType,
		op.BurialDepthCm,
		op.CoreTempCelsius,
		op.LeadSkiPatrolCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "WINTER-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.WinterRescueStatusExtractedNormothermicSafe
	op.HazardScore = 10
	op.RewarmingReadinessScore = 100
	op.AsphyxiaAirPocketAlert = false
	op.SevereFrostbiteAfterdropAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded cold-weather & avalanche rescue operations (AW-OP-101, AW-OP-102, AW-OP-103).
func SeedOperations() []domain.WinterRescueOperation {
	baseTime := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)

	op101 := domain.WinterRescueOperation{
		OperationID:                   "AW-OP-101",
		PetID:                         "PET-AW-101",
		PetName:                       "Yukon",
		Species:                       "Canine",
		WeightKg:                      28.5,
		TerrainType:                   domain.WinterTerrainAvalancheDebrisChute,
		Status:                        domain.WinterRescueStatusBurialHypothermiaCritical,
		BurialDepthCm:                 85.0,
		WindChillCelsius:              -22.0,
		CoreTempCelsius:               33.8,
		MinutesBuried:                 24,
		AvalancheDangerScale:          4,
		HazardScore:                   97,
		RewarmingReadinessScore:       40,
		AsphyxiaAirPocketAlert:        true,
		SevereFrostbiteAfterdropAlert: true,
		RecommendedLocatorGear:        "457 kHz Digital 3-Antenna Avalanche Transceiver + 320cm Carbon Strike Probe + RECCO Harmonic Detector",
		RecommendedRewarmingBivy:      "Horizontal Anti-Afterdrop Vacuum Litter + Forced-Air Thoracic Rewarming Cocoon + Warmed IV/O2 Pack",
		SnowpackSectorGrid:            "CHUTE-NORTH-04A (Alpental Backcountry Bowl)",
		RiskFlags: []string{
			"EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK",
			"CRITICAL_HYPOTHERMIC_AFTERDROP_ARRHYTHMIA_RISK",
			"SECONDARY_SLAB_TRIGGER_HAZARD",
		},
		ProtocolSteps:         DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadSkiPatrolCallsign: "PATROL-ALPINE-01 (Sorensen)",
		Notes:                 "Malamute buried 85cm in D2.5 wind-slab debris toe; 457 kHz collar beacon locked and carbon probe strike confirmed. Excavating V-conveyor airway trench.",
		UpdatedAt:             baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.WinterRescueOperation{
		OperationID:                   "AW-OP-102",
		PetID:                         "PET-AW-102",
		PetName:                       "Aspen",
		Species:                       "Canine",
		WeightKg:                      21.0,
		TerrainType:                   domain.WinterTerrainTreeWellPowderPit,
		Status:                        domain.WinterRescueStatusProbeTrenchRewarmingStaged,
		BurialDepthCm:                 40.0,
		WindChillCelsius:              -9.5,
		CoreTempCelsius:               36.4,
		MinutesBuried:                 10,
		AvalancheDangerScale:          3,
		HazardScore:                   48,
		RewarmingReadinessScore:       100,
		AsphyxiaAirPocketAlert:        false,
		SevereFrostbiteAfterdropAlert: false,
		RecommendedLocatorGear:        "320cm Sectional Carbon Probe + Downhill V-Trench Collapsible Aluminum Shovel + Thermal Drone FLIR",
		RecommendedRewarmingBivy:      "Active Thoracic Chemical Heat-Pack Vapor-Barrier Bivy + Warm Humidified O2 Mask (40°C)",
		SnowpackSectorGrid:            "GLADE-WEST-11B (Subalpine Hemlock Tree-Well Zone)",
		RiskFlags: []string{
			"EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK",
		},
		ProtocolSteps:         DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadSkiPatrolCallsign: "PATROL-ALPINE-03 (Lindqvist)",
		Notes:                 "Border Collie inverted in unconsolidated hemlock tree well; side-trench airway cleared and active thoracic heat-pack bivy staged for normothermic clearance.",
		UpdatedAt:             baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.WinterRescueOperation{
		OperationID:                   "AW-OP-103",
		PetID:                         "PET-AW-103",
		PetName:                       "Freya",
		Species:                       "Feline",
		WeightKg:                      4.8,
		TerrainType:                   domain.WinterTerrainBlizzardSnowdriftTrench,
		Status:                        domain.WinterRescueStatusExtractedNormothermicSafe,
		BurialDepthCm:                 0.0,
		WindChillCelsius:              -4.0,
		CoreTempCelsius:               38.2,
		MinutesBuried:                 0,
		AvalancheDangerScale:          1,
		HazardScore:                   10,
		RewarmingReadinessScore:       100,
		AsphyxiaAirPocketAlert:        false,
		SevereFrostbiteAfterdropAlert: false,
		RecommendedLocatorGear:        "457 kHz Digital 3-Antenna Avalanche Transceiver + 320cm Carbon Strike Probe + RECCO Harmonic Detector",
		RecommendedRewarmingBivy:      "Active Thoracic Chemical Heat-Pack Vapor-Barrier Bivy + Warm Humidified O2 Mask (40°C)",
		SnowpackSectorGrid:            "DRIFT-CABIN-02C (Snoqualmie Pass Timberline)",
		RiskFlags: []string{
			"SUBZERO_PAW_FROSTBITE_EXPOSURE_ZONE",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "WINTER-SAFE-6B2C84E1A904",
		LeadSkiPatrolCallsign:    "PATROL-ALPINE-02 (Mercer)",
		Notes:                    "Norwegian Forest Cat extracted from drifted snowbank shelter; core temperature restored to 38.2°C normothermia with zero paw frostbite.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.WinterRescueOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for cold-weather & avalanche rescue operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.WinterRescueOperation
	order   []string
	counter int
}

// NewStore creates and seeds a WinterRescueOperation store with AW-OP-101, AW-OP-102, and AW-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.WinterRescueOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.WinterRescueOperation) domain.WinterRescueOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.WinterProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (AW-OP-101, AW-OP-102, AW-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.WinterRescueOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.WinterRescueOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.WinterRescueOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.WinterRescueOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new winter rescue operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.WinterEvaluationRequest) (domain.WinterRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("AW-OP-%d", s.counter+1)
	}

	op, err := EvaluateWinterRescue(req)
	if err != nil {
		return domain.WinterRescueOperation{}, err
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
func (s *Store) UpdateStep(id string, req domain.WinterStepUpdateRequest) (domain.WinterRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.WinterRescueOperation{}, fmt.Errorf("winter rescue operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.WinterRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues winter normothermic extraction clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadSkiPatrolCallsign, notes string) (domain.WinterRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.WinterRescueOperation{}, fmt.Errorf("winter rescue operation %q not found", key)
	}

	updated, err := IssueWinterClearance(existing, leadSkiPatrolCallsign, notes)
	if err != nil {
		return domain.WinterRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
