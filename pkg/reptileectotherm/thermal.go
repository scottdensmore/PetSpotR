package reptileectotherm

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

// DefaultProtocolSteps constructs the 5 standard ectotherm crevice borescope sweep, ceramic IR basking lure,
// and gradual coelomic rewarming protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.EctothermProtocolStep {
	steps := []domain.EctothermProtocolStep{
		{
			ID:       "step-endoscope-crevice-sweep",
			Label:    "Deploy articulating 5.5mm fiber-optic borescope & FLIR micro-bolometer to locate cold-torpid ectotherm in burrow or wall void",
			Category: "ENDOSCOPE_CREVICE_SWEEP",
			Required: true,
		},
		{
			ID:       "step-burrow-shoring-probe",
			Label:    "Shore unconsolidated burrow roof or isolate appliance compressor power before crevice extraction",
			Category: "ENDOSCOPE_CREVICE_SWEEP",
			Required: true,
		},
		{
			ID:       "step-ceramic-ir-basking-lure",
			Label:    "Stage thermostat-regulated 100W ceramic infrared heat emitter (30–32°C basking spot) & UV-B/prey-scent hide box",
			Category: "CERAMIC_IR_BASKING_LURE",
			Required: true,
		},
		{
			ID:       "step-humidity-hydration-soak",
			Label:    "Establish 65–80% RH sphagnum humidity hide or lukewarm (28°C) shallow electrolyte cloacal hydration soak",
			Category: "CERAMIC_IR_BASKING_LURE",
			Required: true,
		},
		{
			ID:       "step-gradual-coelomic-rewarm",
			Label:    "Execute gradual coelomic rewarming (≤2°C/hr) across thermal gradient to prevent reperfusion shock & verify righting reflex",
			Category: "GRADUAL_COELOMIC_REWARMING",
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

// EvaluateEctothermRescue deterministically evaluates ectothermic cold-stun torpor risks, hypovolemic desiccation hazards,
// ceramic IR basking lures, containment gear, and thermal readiness for a reptile, chelonian, or amphibian recovery operation.
func EvaluateEctothermRescue(req domain.EctothermEvaluationRequest) (domain.EctothermRescueOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.EctothermRescueOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Reptile"
	}

	if !req.TaxonType.IsValid() {
		return domain.EctothermRescueOperation{}, fmt.Errorf("invalid taxonType: %q", req.TaxonType)
	}
	if req.WeightKg <= 0 || req.WeightKg > 250 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("weightKg must be in (0, 250], got %v", req.WeightKg)
	}
	if req.SubstrateTempCelsius < -20 || req.SubstrateTempCelsius > 55 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("substrateTempCelsius must be in [-20, 55], got %v", req.SubstrateTempCelsius)
	}
	if req.PreferredOptimumTempC < 10 || req.PreferredOptimumTempC > 48 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("preferredOptimumTempC must be in [10, 48], got %v", req.PreferredOptimumTempC)
	}
	if req.RelativeHumidityPct < 0 || req.RelativeHumidityPct > 100 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("relativeHumidityPct must be in [0, 100], got %v", req.RelativeHumidityPct)
	}
	if req.HoursDormant < 0 || req.HoursDormant > 2160 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("hoursDormant must be in [0, 2160], got %v", req.HoursDormant)
	}
	if req.BurrowDepthCm < 0 || req.BurrowDepthCm > 1000 {
		return domain.EctothermRescueOperation{}, fmt.Errorf("burrowDepthCm must be in [0, 1000], got %v", req.BurrowDepthCm)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f", petName, species, req.TaxonType, req.SubstrateTempCelsius, req.PreferredOptimumTempC)))
		opID = "RE-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	habitatGrid := strings.TrimSpace(req.MicroHabitatGrid)
	if habitatGrid == "" {
		habitatGrid = "SUBSTRATE-SECTOR-01A"
	}

	callsign := strings.TrimSpace(req.LeadHerpTechCallsign)
	if callsign == "" {
		callsign = "HERP-TECH-01"
	}

	tempDeficit := req.PreferredOptimumTempC - req.SubstrateTempCelsius
	criticalTorporAlert := tempDeficit >= 12.0 ||
		req.SubstrateTempCelsius <= 14.0 ||
		req.HoursDormant >= 36.0

	desiccationDysecdysisAlert := req.RelativeHumidityPct <= 35.0 ||
		req.TaxonType == domain.EctothermTaxonAmphibianSalamanderAxolotl

	riskFlags := make([]string, 0, 4)
	if criticalTorporAlert {
		riskFlags = append(riskFlags, "EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK")
	}
	if desiccationDysecdysisAlert {
		riskFlags = append(riskFlags, "CRITICAL_HYPOVOLEMIC_DESICCATION_HAZARD")
	}
	if req.BurrowDepthCm >= 60.0 {
		riskFlags = append(riskFlags, "BURROW_ROOF_COLLAPSE_ENTRAPMENT_RISK")
	}
	if req.TaxonType == domain.EctothermTaxonBoidPythonConstrictor || req.TaxonType == domain.EctothermTaxonColubridCornKingSnake {
		riskFlags = append(riskFlags, "APPLIANCE_COMPRESSOR_THERMAL_BURN_HAZARD")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "SUBOPTIMAL_BASKING_THERMAL_GRADIENT_ZONE")
	}

	baseHazard := 26
	switch req.TaxonType {
	case domain.EctothermTaxonChelonianTortoiseBurrow:
		baseHazard = 36
	case domain.EctothermTaxonBoidPythonConstrictor:
		baseHazard = 32
	case domain.EctothermTaxonAgamidIguanidArboreal:
		baseHazard = 28
	case domain.EctothermTaxonColubridCornKingSnake:
		baseHazard = 26
	case domain.EctothermTaxonAmphibianSalamanderAxolotl:
		baseHazard = 38
	}

	if criticalTorporAlert {
		baseHazard += 30
	}
	if desiccationDysecdysisAlert {
		baseHazard += 21
	}
	if req.BurrowDepthCm >= 60.0 {
		baseHazard += 8
	}

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	thermalLure := "100W Ceramic Infrared Heat Emitter (31°C Basking Spot) + Thermostat-Regulated Warm Hide Box"
	switch req.TaxonType {
	case domain.EctothermTaxonChelonianTortoiseBurrow:
		thermalLure = "150W Deep-Penetration Ceramic IR Burrow Portal Lamp (32°C) + Hibiscus/Dandelion Olfactory Trail"
	case domain.EctothermTaxonBoidPythonConstrictor, domain.EctothermTaxonColubridCornKingSnake:
		thermalLure = "Thermostat-Controlled 31°C Under-Tank Heating Pad + Enclosed Dark Crevice Hide + Warmed Rodent Scent Trail"
	case domain.EctothermTaxonAgamidIguanidArboreal:
		thermalLure = "Mercury-Vapor UV-B/IR Basking Spot Lamp (34°C) + Elevated Cork-Bark Perch + Acoustic Insect Feeder Lure"
	case domain.EctothermTaxonAmphibianSalamanderAxolotl:
		thermalLure = "Chilled Dechlorinated 18°C Saturated Sphagnum Moss Refuge + Zero-UV Damp Hide Tunnel"
	}

	containment := "Insulated Coelomic Thermal-Gradient Transport Tub (26–30°C) + Breathable Cotton Restraint Sack"
	switch req.TaxonType {
	case domain.EctothermTaxonChelonianTortoiseBurrow:
		containment = "Rigid High-Wall Chelonian Extraction Crate + 28°C Warm-Water Cloacal Hydration Soak Basin"
	case domain.EctothermTaxonAmphibianSalamanderAxolotl:
		containment = "Oxygenated Dechlorinated Amphibian Buffer Carrier + Nitrile Non-Latex Wet-Glove Handling Kit"
	}

	initialCompleted := 2
	status := domain.EctothermRescueStatusColdTorporBrumationCritical
	if !criticalTorporAlert && !desiccationDysecdysisAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.EctothermRescueStatusBaskingLureCreviceStaged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeReadinessScore(steps)

	op := domain.EctothermRescueOperation{
		OperationID:                opID,
		PetID:                      petID,
		PetName:                    petName,
		Species:                    species,
		WeightKg:                   req.WeightKg,
		TaxonType:                  req.TaxonType,
		Status:                     status,
		SubstrateTempCelsius:       req.SubstrateTempCelsius,
		PreferredOptimumTempC:      req.PreferredOptimumTempC,
		RelativeHumidityPct:        req.RelativeHumidityPct,
		HoursDormant:               req.HoursDormant,
		BurrowDepthCm:              req.BurrowDepthCm,
		HazardScore:                baseHazard,
		ThermalReadinessScore:      readiness,
		CriticalTorporAlert:        criticalTorporAlert,
		DesiccationDysecdysisAlert: desiccationDysecdysisAlert,
		RecommendedThermalLure:     thermalLure,
		RecommendedContainment:     containment,
		MicroHabitatGrid:           habitatGrid,
		RiskFlags:                  riskFlags,
		ProtocolSteps:              steps,
		LeadHerpTechCallsign:       callsign,
		Notes:                      strings.TrimSpace(req.Notes),
		UpdatedAt:                  nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.EctothermRescueOperation{}, err
	}
	return op, nil
}

func computeReadinessScore(steps []domain.EctothermProtocolStep) int {
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

// UpdateProtocolStep updates the completion state of an ectotherm protocol checklist step and recomputes readiness and status.
func UpdateProtocolStep(op domain.EctothermRescueOperation, req domain.EctothermStepUpdateRequest) (domain.EctothermRescueOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.EctothermProtocolStep, len(op.ProtocolSteps))
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
	op.ThermalReadinessScore = computeReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.EctothermRescueStatusRecoveredThermalGradientSafe {
		if op.ThermalReadinessScore >= 60 {
			op.Status = domain.EctothermRescueStatusBaskingLureCreviceStaged
		} else {
			op.Status = domain.EctothermRescueStatusColdTorporBrumationCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueEctothermClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted ECTOTHERM-SAFE-<12HEX>, and transitions the operation to RECOVERED_THERMAL_GRADIENT_SAFE.
func IssueEctothermClearance(op domain.EctothermRescueOperation, leadHerpTechCallsign, notes string) (domain.EctothermRescueOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue ectotherm clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadHerpTechCallsign); trimmedCallsign != "" {
		op.LeadHerpTechCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%s",
		op.OperationID,
		op.PetID,
		op.TaxonType,
		op.SubstrateTempCelsius,
		op.PreferredOptimumTempC,
		op.LeadHerpTechCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "ECTOTHERM-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.EctothermRescueStatusRecoveredThermalGradientSafe
	op.HazardScore = 10
	op.ThermalReadinessScore = 100
	op.CriticalTorporAlert = false
	op.DesiccationDysecdysisAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded ectotherm thermal-dormancy rescue operations (RE-OP-101, RE-OP-102, RE-OP-103).
func SeedOperations() []domain.EctothermRescueOperation {
	baseTime := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)

	op101 := domain.EctothermRescueOperation{
		OperationID:                "RE-OP-101",
		PetID:                      "PET-RE-101",
		PetName:                    "Tank",
		Species:                    "Reptile",
		WeightKg:                   18.5,
		TaxonType:                  domain.EctothermTaxonChelonianTortoiseBurrow,
		Status:                     domain.EctothermRescueStatusColdTorporBrumationCritical,
		SubstrateTempCelsius:       11.2,
		PreferredOptimumTempC:      31.0,
		RelativeHumidityPct:        28.0,
		HoursDormant:               48.0,
		BurrowDepthCm:              110.0,
		HazardScore:                95,
		ThermalReadinessScore:      40,
		CriticalTorporAlert:        true,
		DesiccationDysecdysisAlert: true,
		RecommendedThermalLure:     "150W Deep-Penetration Ceramic IR Burrow Portal Lamp (32°C) + Hibiscus/Dandelion Olfactory Trail",
		RecommendedContainment:     "Rigid High-Wall Chelonian Extraction Crate + 28°C Warm-Water Cloacal Hydration Soak Basin",
		MicroHabitatGrid:           "BURROW-SOUTH-03A (Collapsed Foundation Crawlspace Embankment)",
		RiskFlags: []string{
			"EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK",
			"CRITICAL_HYPOVOLEMIC_DESICCATION_HAZARD",
			"BURROW_ROOF_COLLAPSE_ENTRAPMENT_RISK",
		},
		ProtocolSteps:        DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadHerpTechCallsign: "HERP-TECH-01 (Dr. Alvarez)",
		Notes:                "African Sulcata Tortoise cold-stunned at 11.2°C in 110cm sub-foundation soil burrow; borescope confirmed carapace position and shoring struts installed.",
		UpdatedAt:            baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.EctothermRescueOperation{
		OperationID:                "RE-OP-102",
		PetID:                      "PET-RE-102",
		PetName:                    "Saffron",
		Species:                    "Reptile",
		WeightKg:                   2.4,
		TaxonType:                  domain.EctothermTaxonBoidPythonConstrictor,
		Status:                     domain.EctothermRescueStatusBaskingLureCreviceStaged,
		SubstrateTempCelsius:       22.5,
		PreferredOptimumTempC:      30.5,
		RelativeHumidityPct:        62.0,
		HoursDormant:               14.0,
		BurrowDepthCm:              25.0,
		HazardScore:                45,
		ThermalReadinessScore:      100,
		CriticalTorporAlert:        false,
		DesiccationDysecdysisAlert: false,
		RecommendedThermalLure:     "Thermostat-Controlled 31°C Under-Tank Heating Pad + Enclosed Dark Crevice Hide + Warmed Rodent Scent Trail",
		RecommendedContainment:     "Insulated Coelomic Thermal-Gradient Transport Tub (26–30°C) + Breathable Cotton Restraint Sack",
		MicroHabitatGrid:           "VOID-KITCHEN-07B (Sub-Cabinet Hydronic Baseboard Conduit)",
		RiskFlags: []string{
			"APPLIANCE_COMPRESSOR_THERMAL_BURN_HAZARD",
		},
		ProtocolSteps:        DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadHerpTechCallsign: "HERP-TECH-02 (Takahashi)",
		Notes:                "Ball Python coiled inside hydronic cabinet void; appliance power isolated, 31°C ceramic IR hide lured snake out safely, and gradual coelomic rewarming completed.",
		UpdatedAt:            baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.EctothermRescueOperation{
		OperationID:                "RE-OP-103",
		PetID:                      "PET-RE-103",
		PetName:                    "Figment",
		Species:                    "Reptile",
		WeightKg:                   0.45,
		TaxonType:                  domain.EctothermTaxonAgamidIguanidArboreal,
		Status:                     domain.EctothermRescueStatusRecoveredThermalGradientSafe,
		SubstrateTempCelsius:       29.5,
		PreferredOptimumTempC:      32.0,
		RelativeHumidityPct:        55.0,
		HoursDormant:               3.0,
		BurrowDepthCm:              0.0,
		HazardScore:                10,
		ThermalReadinessScore:      100,
		CriticalTorporAlert:        false,
		DesiccationDysecdysisAlert: false,
		RecommendedThermalLure:     "Mercury-Vapor UV-B/IR Basking Spot Lamp (34°C) + Elevated Cork-Bark Perch + Acoustic Insect Feeder Lure",
		RecommendedContainment:     "Insulated Coelomic Thermal-Gradient Transport Tub (26–30°C) + Breathable Cotton Restraint Sack",
		MicroHabitatGrid:           "PERCH-SUNROOM-02C (South-Facing Conservatory Cedar Rafter)",
		RiskFlags: []string{
			"SUBOPTIMAL_BASKING_THERMAL_GRADIENT_ZONE",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "ECTOTHERM-SAFE-8A3F19C4E207",
		LeadHerpTechCallsign:     "HERP-TECH-03 (Kowalski)",
		Notes:                    "Bearded Dragon lured onto 32°C UV-B basking perch from conservatory rafter crevice; coelomic temperature stabilized within POTZ and righting reflex verified.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.EctothermRescueOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for ectotherm thermal-dormancy rescue operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.EctothermRescueOperation
	order   []string
	counter int
}

// NewStore creates and seeds an EctothermRescueOperation store with RE-OP-101, RE-OP-102, and RE-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.EctothermRescueOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.EctothermRescueOperation) domain.EctothermRescueOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.EctothermProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (RE-OP-101, RE-OP-102, RE-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.EctothermRescueOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.EctothermRescueOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.EctothermRescueOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.EctothermRescueOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new ectotherm rescue operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.EctothermEvaluationRequest) (domain.EctothermRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("RE-OP-%d", s.counter+1)
	}

	op, err := EvaluateEctothermRescue(req)
	if err != nil {
		return domain.EctothermRescueOperation{}, err
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
func (s *Store) UpdateStep(id string, req domain.EctothermStepUpdateRequest) (domain.EctothermRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.EctothermRescueOperation{}, fmt.Errorf("ectotherm rescue operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.EctothermRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues ectotherm thermal-gradient recovery clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadHerpTechCallsign, notes string) (domain.EctothermRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.EctothermRescueOperation{}, fmt.Errorf("ectotherm rescue operation %q not found", key)
	}

	updated, err := IssueEctothermClearance(existing, leadHerpTechCallsign, notes)
	if err != nil {
		return domain.EctothermRescueOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
