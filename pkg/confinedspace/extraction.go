package confinedspace

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

// DefaultProtocolSteps constructs the 5 standard subterranean & culvert rescue protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.ConfinedSpaceProtocolStep {
	steps := []domain.ConfinedSpaceProtocolStep{
		{
			ID:       "step-gas-purge",
			Label:    "Deploy 4-gas atmospheric monitor (O2/H2S/CO/LEL) & positive-pressure saddle-vent blower purge",
			Category: "GAS_VENTILATION",
			Required: true,
		},
		{
			ID:       "step-inflow-dam",
			Label:    "Install upstream pneumatic pipe-plug dam & downstream catch-grate containment barrier",
			Category: "ONE_WAY_PIPE_GATE",
			Required: true,
		},
		{
			ID:       "step-borescope-loc",
			Label:    "Advance articulating fiber-optic borescope & acoustic piezo transducer to confirm posture & respiration",
			Category: "BORESCOPE_OPTICS",
			Required: true,
		},
		{
			ID:       "step-oneway-gate",
			Label:    "Rig one-way acrylic butterfly pipe-gate & thermal lure chamber at junction access shaft",
			Category: "ONE_WAY_PIPE_GATE",
			Required: true,
		},
		{
			ID:       "step-haul-extract",
			Label:    "Execute low-stress padded sleeve extraction & immediate supplemental O2 normobaric check",
			Category: "BORESCOPE_OPTICS",
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

// EvaluateConfinedSpace deterministically evaluates atmospheric hazards, culvert inflow risks,
// borescope optics recommendations, and rigging readiness for a confined-space pet extraction.
func EvaluateConfinedSpace(req domain.ConfinedSpaceEvaluationRequest) (domain.ConfinedSpaceOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Canine"
	}

	if !req.SpaceType.IsValid() {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("invalid spaceType: %q", req.SpaceType)
	}
	if req.WeightKg <= 0 || req.WeightKg > 250 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("weightKg must be in (0, 250], got %v", req.WeightKg)
	}
	if req.PipeDiameterCm <= 0 || req.PipeDiameterCm > 500 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("pipeDiameterCm must be in (0, 500], got %v", req.PipeDiameterCm)
	}
	if req.DepthMeters < 0 || req.DepthMeters > 100 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("depthMeters must be in [0, 100], got %v", req.DepthMeters)
	}
	if req.OxygenPercent <= 0 || req.OxygenPercent > 25 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("oxygenPercent must be in (0, 25], got %v", req.OxygenPercent)
	}
	if req.H2SPpm < 0 || req.H2SPpm > 500 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("h2sPpm must be in [0, 500], got %v", req.H2SPpm)
	}
	if req.InflowRiskIndex < 0 || req.InflowRiskIndex > 100 {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("inflowRiskIndex must be in [0, 100], got %d", req.InflowRiskIndex)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f", petName, species, req.SpaceType, req.PipeDiameterCm, req.DepthMeters)))
		opID = "CS-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	sectorGrid := strings.TrimSpace(req.ManholeSectorGrid)
	if sectorGrid == "" {
		sectorGrid = "MH-SECTOR-01A"
	}

	callsign := strings.TrimSpace(req.LeadTechCallsign)
	if callsign == "" {
		callsign = "TECH-SUB-01"
	}

	toxicGasAlert := req.OxygenPercent < 19.5 ||
		req.H2SPpm >= 10.0 ||
		(req.DepthMeters >= 3.0 && req.SpaceType == domain.ConfinedSpaceSubterraneanUtilityPipe)

	flashInflowAlert := req.InflowRiskIndex >= 70 ||
		(req.SpaceType == domain.ConfinedSpaceStormDrainCulvert && req.PipeDiameterCm <= 45.0)

	riskFlags := make([]string, 0, 4)
	if toxicGasAlert {
		riskFlags = append(riskFlags, "EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK")
	}
	if flashInflowAlert {
		riskFlags = append(riskFlags, "CRITICAL_CULVERT_INFLOW_SURGE_RISK")
	}
	if req.PipeDiameterCm <= 40.0 {
		riskFlags = append(riskFlags, "NARROW_DIAMETER_WEDGING_HAZARD")
	}
	if req.DepthMeters >= 3.5 {
		riskFlags = append(riskFlags, "DEEP_SHAFT_VERTICAL_HAUL_REQUIRED")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "SUB_FOUNDATION_STRUCTURAL_PINCH_ZONE")
	}

	baseHazard := 28
	switch req.SpaceType {
	case domain.ConfinedSpaceStormDrainCulvert:
		baseHazard = 36
	case domain.ConfinedSpaceSubterraneanUtilityPipe:
		baseHazard = 38
	case domain.ConfinedSpaceChimneyFlueShaft:
		baseHazard = 34
	case domain.ConfinedSpaceCollapsedBurrowTrench:
		baseHazard = 40
	case domain.ConfinedSpaceFoundationCrawlspace:
		baseHazard = 26
	}

	if toxicGasAlert {
		baseHazard += 28
	}
	if flashInflowAlert {
		baseHazard += 20
	}
	if req.PipeDiameterCm <= 40.0 {
		baseHazard += 8
	}
	if req.DepthMeters >= 3.0 {
		baseHazard += 6
	}
	baseHazard += req.InflowRiskIndex / 10

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	opticsRig := "Tracked Crawler Inspection ROV + Wide-Angle Low-Lux IR Endoscope"
	if req.PipeDiameterCm <= 40.0 {
		opticsRig = "Articulating 5mm Dual-LED Thermal/Fiber-Optic Borescope + Acoustic Piezo Probe (15m Push-Rod)"
	} else if req.SpaceType == domain.ConfinedSpaceChimneyFlueShaft {
		opticsRig = "Vertical Drop 360° IR Pan-Tilt Flue Inspection Camera + Soft-Noose Snare Guide"
	}

	ventilation := "Low-Noise Positive-Pressure Crawlspace Air Mover + 4-Gas Multi-Sensor Monitor"
	if toxicGasAlert {
		ventilation = "Positive-Pressure Explosion-Proof Axial Blower + Saddle-Vent Ducting (4-Gas O2/H2S/CO/LEL Continuous Purge)"
	} else if flashInflowAlert {
		ventilation = "Upstream Pneumatic Pipe Plug Bladder + Inflatable Weir Bypass & High-CFM Snorkel"
	}

	initialCompleted := 2
	status := domain.ConfinedSpaceStatusAtmosphericHazardCritical
	if !toxicGasAlert && !flashInflowAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.ConfinedSpaceStatusBorescopePipeGateRigged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeReadinessScore(steps)

	op := domain.ConfinedSpaceOperation{
		OperationID:            opID,
		PetID:                  petID,
		PetName:                petName,
		Species:                species,
		WeightKg:               req.WeightKg,
		SpaceType:              req.SpaceType,
		Status:                 status,
		PipeDiameterCm:         req.PipeDiameterCm,
		DepthMeters:            req.DepthMeters,
		OxygenPercent:          req.OxygenPercent,
		H2SPpm:                 req.H2SPpm,
		InflowRiskIndex:        req.InflowRiskIndex,
		HazardScore:            baseHazard,
		RiggingReadinessScore:  readiness,
		ToxicGasAlert:          toxicGasAlert,
		FlashInflowAlert:       flashInflowAlert,
		RecommendedOpticsRig:   opticsRig,
		RecommendedVentilation: ventilation,
		ManholeSectorGrid:      sectorGrid,
		RiskFlags:              riskFlags,
		ProtocolSteps:          steps,
		LeadTechCallsign:       callsign,
		Notes:                  strings.TrimSpace(req.Notes),
		UpdatedAt:              nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.ConfinedSpaceOperation{}, err
	}
	return op, nil
}

func computeReadinessScore(steps []domain.ConfinedSpaceProtocolStep) int {
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
func UpdateProtocolStep(op domain.ConfinedSpaceOperation, req domain.ConfinedSpaceStepUpdateRequest) (domain.ConfinedSpaceOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.ConfinedSpaceProtocolStep, len(op.ProtocolSteps))
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
	op.RiggingReadinessScore = computeReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.ConfinedSpaceStatusExtractedSubterraneanSafe {
		if op.RiggingReadinessScore >= 60 {
			op.Status = domain.ConfinedSpaceStatusBorescopePipeGateRigged
		} else {
			op.Status = domain.ConfinedSpaceStatusAtmosphericHazardCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueSubterraneanClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted CONFINED-SAFE-<12HEX>, and transitions the operation to EXTRACTED_SUBTERRANEAN_SAFE.
func IssueSubterraneanClearance(op domain.ConfinedSpaceOperation, leadTechCallsign, notes string) (domain.ConfinedSpaceOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue subterranean clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadTechCallsign); trimmedCallsign != "" {
		op.LeadTechCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%s",
		op.OperationID,
		op.PetID,
		op.SpaceType,
		op.PipeDiameterCm,
		op.DepthMeters,
		op.LeadTechCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "CONFINED-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.ConfinedSpaceStatusExtractedSubterraneanSafe
	op.HazardScore = 10
	op.RiggingReadinessScore = 100
	op.ToxicGasAlert = false
	op.FlashInflowAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded confined-space extraction operations (CS-OP-101, CS-OP-102, CS-OP-103).
func SeedOperations() []domain.ConfinedSpaceOperation {
	baseTime := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)

	op101 := domain.ConfinedSpaceOperation{
		OperationID:            "CS-OP-101",
		PetID:                  "PET-CS-101",
		PetName:                "Pip",
		Species:                "Feline",
		WeightKg:               4.2,
		SpaceType:              domain.ConfinedSpaceStormDrainCulvert,
		Status:                 domain.ConfinedSpaceStatusAtmosphericHazardCritical,
		PipeDiameterCm:         35.0,
		DepthMeters:            3.8,
		OxygenPercent:          18.2,
		H2SPpm:                 14.5,
		InflowRiskIndex:        82,
		HazardScore:            95,
		RiggingReadinessScore:  40,
		ToxicGasAlert:          true,
		FlashInflowAlert:       true,
		RecommendedOpticsRig:   "Articulating 5mm Dual-LED Thermal/Fiber-Optic Borescope + Acoustic Piezo Probe (15m Push-Rod)",
		RecommendedVentilation: "Positive-Pressure Explosion-Proof Axial Blower + Saddle-Vent Ducting (4-Gas O2/H2S/CO/LEL Continuous Purge)",
		ManholeSectorGrid:      "MH-NORTH-42B (Ravenna Creek Storm Culvert)",
		RiskFlags: []string{
			"EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK",
			"CRITICAL_CULVERT_INFLOW_SURGE_RISK",
			"NARROW_DIAMETER_WEDGING_HAZARD",
		},
		ProtocolSteps:    DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadTechCallsign: "TECH-SUB-01 (Vance)",
		Notes:            "Feline wedged 11m inside 35cm corrugated storm culvert; O2 depleted at 18.2% with 14.5 ppm H2S pocket. Continuous axial blower purge active.",
		UpdatedAt:        baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.ConfinedSpaceOperation{
		OperationID:            "CS-OP-102",
		PetID:                  "PET-CS-102",
		PetName:                "Buster",
		Species:                "Canine",
		WeightKg:               14.8,
		SpaceType:              domain.ConfinedSpaceFoundationCrawlspace,
		Status:                 domain.ConfinedSpaceStatusBorescopePipeGateRigged,
		PipeDiameterCm:         60.0,
		DepthMeters:            1.8,
		OxygenPercent:          20.6,
		H2SPpm:                 1.5,
		InflowRiskIndex:        30,
		HazardScore:            44,
		RiggingReadinessScore:  100,
		ToxicGasAlert:          false,
		FlashInflowAlert:       false,
		RecommendedOpticsRig:   "Tracked Crawler Inspection ROV + Wide-Angle Low-Lux IR Endoscope",
		RecommendedVentilation: "Low-Noise Positive-Pressure Crawlspace Air Mover + 4-Gas Multi-Sensor Monitor",
		ManholeSectorGrid:      "SECTOR-CRAWL-19A (Ballard Pier Sub-Foundation)",
		RiskFlags: []string{
			"SUB_FOUNDATION_STRUCTURAL_PINCH_ZONE",
		},
		ProtocolSteps:    DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadTechCallsign: "TECH-SUB-04 (Moreno)",
		Notes:            "Terrier mix localized behind load-bearing pier footing via IR borescope; one-way acrylic pipe gate rigged and ready for extraction clearance.",
		UpdatedAt:        baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.ConfinedSpaceOperation{
		OperationID:            "CS-OP-103",
		PetID:                  "PET-CS-103",
		PetName:                "Peanut",
		Species:                "Canine",
		WeightKg:               6.5,
		SpaceType:              domain.ConfinedSpaceChimneyFlueShaft,
		Status:                 domain.ConfinedSpaceStatusExtractedSubterraneanSafe,
		PipeDiameterCm:         30.0,
		DepthMeters:            4.5,
		OxygenPercent:          20.9,
		H2SPpm:                 0.0,
		InflowRiskIndex:        10,
		HazardScore:            10,
		RiggingReadinessScore:  100,
		ToxicGasAlert:          false,
		FlashInflowAlert:       false,
		RecommendedOpticsRig:   "Articulating 5mm Dual-LED Thermal/Fiber-Optic Borescope + Acoustic Piezo Probe (15m Push-Rod)",
		RecommendedVentilation: "Low-Noise Positive-Pressure Crawlspace Air Mover + 4-Gas Multi-Sensor Monitor",
		ManholeSectorGrid:      "FLUE-SHAFT-07C (Queen Anne Masonry Chimney)",
		RiskFlags: []string{
			"NARROW_DIAMETER_WEDGING_HAZARD",
			"DEEP_SHAFT_VERTICAL_HAUL_REQUIRED",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "CONFINED-SAFE-7C3E91A4B802",
		LeadTechCallsign:         "TECH-SUB-02 (Kowalski)",
		Notes:                    "Extracted safely via vertical padded harness sleeve through cleanout door; vitals stable and normobaric O2 verified.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.ConfinedSpaceOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for confined-space rescue operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.ConfinedSpaceOperation
	order   []string
	counter int
}

// NewStore creates and seeds a ConfinedSpaceOperation store with CS-OP-101, CS-OP-102, and CS-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.ConfinedSpaceOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.ConfinedSpaceOperation) domain.ConfinedSpaceOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.ConfinedSpaceProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (CS-OP-101, CS-OP-102, CS-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.ConfinedSpaceOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ConfinedSpaceOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.ConfinedSpaceOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.ConfinedSpaceOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new confined-space operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.ConfinedSpaceEvaluationRequest) (domain.ConfinedSpaceOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("CS-OP-%d", s.counter+1)
	}

	op, err := EvaluateConfinedSpace(req)
	if err != nil {
		return domain.ConfinedSpaceOperation{}, err
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
func (s *Store) UpdateStep(id string, req domain.ConfinedSpaceStepUpdateRequest) (domain.ConfinedSpaceOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("confined space operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.ConfinedSpaceOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues subterranean extraction clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadTechCallsign, notes string) (domain.ConfinedSpaceOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.ConfinedSpaceOperation{}, fmt.Errorf("confined space operation %q not found", key)
	}

	updated, err := IssueSubterraneanClearance(existing, leadTechCallsign, notes)
	if err != nil {
		return domain.ConfinedSpaceOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
