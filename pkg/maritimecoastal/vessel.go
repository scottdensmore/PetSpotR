package maritimecoastal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const defaultTimestamp = "2026-10-18T23:00:00Z"

// EvaluateMaritimeRescue deterministically evaluates a maritime, coastal, or tidal mudflat rescue request.
func EvaluateMaritimeRescue(req domain.MaritimeEvaluationRequest) (*domain.MaritimeRescueOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 {
		return nil, fmt.Errorf("weight_kg must be greater than 0")
	}
	if !req.MaritimeZone.IsValid() {
		return nil, fmt.Errorf("invalid maritime_zone: %q", req.MaritimeZone)
	}
	if req.TideSurgeRateCmHr < 0 {
		return nil, fmt.Errorf("tide_surge_rate_cm_hr must be >= 0")
	}
	if req.SwellHeightMeters < 0 {
		return nil, fmt.Errorf("swell_height_meters must be >= 0")
	}
	if req.WaterTempCelsius < -2.0 || req.WaterTempCelsius > 40.0 {
		return nil, fmt.Errorf("water_temp_celsius must be between -2.0 and 40.0")
	}
	if req.MudSuctionDepthCm < 0 {
		return nil, fmt.Errorf("mud_suction_depth_cm must be >= 0")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f", petName, species, req.MaritimeZone, req.WeightKg)))
		opID = "MC-OP-" + strings.ToUpper(hex.EncodeToString(sum[:3]))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("PET|%s|%s", petName, opID)))
		petID = "PET-MC-" + strings.ToUpper(hex.EncodeToString(sum[:3]))
	}

	vessel := selectRequiredVessel(req.MaritimeZone, req.WeightKg, req.MudSuctionDepthCm)

	step1CompletedAt := ""
	if req.MudWalkersAirLanceDeployed {
		step1CompletedAt = defaultTimestamp
	}
	step2CompletedAt := ""
	if req.CaninePFDHarnessSecured {
		step2CompletedAt = defaultTimestamp
	}

	steps := []domain.MaritimeProtocolStep{
		{
			ID:          "STEP-TIDE-SUCTION-RELIEF",
			Title:       "Tidal Mudflat Vacuum Suction Relief & Mud-Walker Staging",
			Description: "Deploy mud-walker pontoons, rescue sled, and low-pressure air/water lance to break anaerobic mud suction around limbs before incoming flood tide.",
			Completed:   req.MudWalkersAirLanceDeployed,
			CompletedAt: step1CompletedAt,
		},
		{
			ID:          "STEP-VESSEL-PFD-RIGGING",
			Title:       "Buoyant Canine PFD Harness & Vessel Tow-Sled Rigging",
			Description: "Secure high-buoyancy dorsal-lift PFD harness with quick-release tether and stage horizontal boarding onto hovercraft, PWC sled, or RHIB.",
			Completed:   req.CaninePFDHarnessSecured,
			CompletedAt: step2CompletedAt,
		},
		{
			ID:          "STEP-SALTWATER-ASPIRATION-O2",
			Title:       "Saltwater Aspiration Auscultation & Humidified O2 Support",
			Description: "Clear upper airway of saline foam/silt, auscultate bilateral lung fields for rales, and administer 100% humidified flow-by oxygen.",
			Completed:   false,
		},
		{
			ID:          "STEP-HARBOR-DECK-DECON-WARM",
			Title:       "Harbor Deck Freshwater Rinse & Active Hypothermia Rewarming",
			Description: "Flush hypertonic salt and estuarine silt from coat/eyes and wrap in active forced-air thermal blanket on harbor deck prior to veterinary transport.",
			Completed:   false,
		},
	}

	isCritical := isCriticalMaritimeCondition(
		req.SaltwaterAspirationSuspected,
		req.TideSurgeRateCmHr,
		req.SwellHeightMeters,
		req.MudSuctionDepthCm,
		req.MudWalkersAirLanceDeployed,
		req.WaterTempCelsius,
	)

	status := domain.MaritimeStatusHovercraftMudSledStaged
	if isCritical {
		status = domain.MaritimeStatusTidalFloodAspirationCritical
	}

	advisory := buildTidalRiskAdvisory(
		status,
		req.MaritimeZone,
		req.TideSurgeRateCmHr,
		req.SwellHeightMeters,
		req.WaterTempCelsius,
		req.MudSuctionDepthCm,
		req.SaltwaterAspirationSuspected,
	)

	return &domain.MaritimeRescueOperation{
		ID:                           opID,
		PetID:                        petID,
		PetName:                      petName,
		Species:                      species,
		WeightKg:                     req.WeightKg,
		MaritimeZone:                 req.MaritimeZone,
		Status:                       status,
		TideSurgeRateCmHr:            req.TideSurgeRateCmHr,
		SwellHeightMeters:            req.SwellHeightMeters,
		WaterTempCelsius:             req.WaterTempCelsius,
		SaltwaterAspirationSuspected: req.SaltwaterAspirationSuspected,
		MudSuctionDepthCm:            req.MudSuctionDepthCm,
		MudWalkersAirLanceDeployed:   req.MudWalkersAirLanceDeployed,
		CaninePFDHarnessSecured:      req.CaninePFDHarnessSecured,
		RequiredVessel:               vessel,
		TidalRiskAdvisory:            advisory,
		ProtocolSteps:                steps,
		CreatedAt:                    defaultTimestamp,
		UpdatedAt:                    defaultTimestamp,
	}, nil
}

func isCriticalMaritimeCondition(
	saltwaterAspiration bool,
	tideSurgeCmHr float64,
	swellMeters float64,
	mudSuctionCm float64,
	airLanceDeployed bool,
	waterTempC float64,
) bool {
	if saltwaterAspiration {
		return true
	}
	if tideSurgeCmHr >= 45.0 {
		return true
	}
	if swellMeters >= 2.5 {
		return true
	}
	if mudSuctionCm >= 35.0 && !airLanceDeployed {
		return true
	}
	if waterTempC <= 10.0 {
		return true
	}
	return false
}

func selectRequiredVessel(zone domain.MaritimeZoneType, weightKg float64, mudSuctionCm float64) string {
	switch zone {
	case domain.MaritimeZoneTidalMudflatEstuary:
		if mudSuctionCm >= 35.0 || weightKg >= 35.0 {
			return "Amphibious Hovercraft & Pneumatic Air-Lance Mud Sled"
		}
		return "Inflatable Mud-Walker Pontoons & Low-Friction Extraction Sled"
	case domain.MaritimeZoneRipCurrentSurfBreak:
		return "Rescue Jet-Ski (PWC) & High-Buoyancy Surf Tow-In Sled"
	case domain.MaritimeZoneJettyTetrapodBreakwater:
		return "Shallow-Draft Rigid Hull Inflatable (RHIB) & Tetrapod A-Frame Hoist"
	case domain.MaritimeZoneOffshoreVesselOverboard:
		return "USCG 47-ft Motor Lifeboat & Thermal Recovery Jason's Cradle"
	case domain.MaritimeZoneSaltMarshCordgrassSlough:
		return "Flat-Bottom Marsh Airboat & Cordgrass Extraction Mat"
	default:
		return "Coastal SAR Rigid Hull Inflatable (RHIB)"
	}
}

func buildTidalRiskAdvisory(
	status domain.MaritimeRescueStatus,
	zone domain.MaritimeZoneType,
	tideSurgeCmHr float64,
	swellMeters float64,
	waterTempC float64,
	mudSuctionCm float64,
	saltwaterAspiration bool,
) string {
	if status == domain.MaritimeStatusExtractedHarborDeckSafe {
		return fmt.Sprintf(
			"HARBOR DECK SAFE: Extracted from %s; saline rinsed, normothermic rewarming verified, and cleared for veterinary handoff.",
			zone,
		)
	}

	var hazards []string
	if saltwaterAspiration {
		hazards = append(hazards, "suspected hypertonic saltwater aspiration & non-cardiogenic pulmonary edema risk")
	}
	if tideSurgeCmHr >= 45.0 {
		hazards = append(hazards, fmt.Sprintf("rapid incoming flood tide surge (%.1f cm/hr)", tideSurgeCmHr))
	}
	if swellMeters >= 2.5 {
		hazards = append(hazards, fmt.Sprintf("heavy breaking surf swell (%.1f m)", swellMeters))
	}
	if mudSuctionCm >= 35.0 {
		hazards = append(hazards, fmt.Sprintf("deep anaerobic mud vacuum entrapment (%.1f cm)", mudSuctionCm))
	}
	if waterTempC <= 10.0 {
		hazards = append(hazards, fmt.Sprintf("cold-water immersion hypothermia (%.1f °C)", waterTempC))
	}

	if status == domain.MaritimeStatusTidalFloodAspirationCritical {
		detail := "high tidal/immersion exposure"
		if len(hazards) > 0 {
			detail = strings.Join(hazards, "; ")
		}
		return fmt.Sprintf(
			"CRITICAL TIDAL / ASPIRATION EMERGENCY (%s): %s — deploy air-lance suction relief, dorsal PFD harness, and humidified O2 immediately.",
			zone,
			detail,
		)
	}

	return fmt.Sprintf(
		"STAGED MARITIME EXTRACTION (%s): Tide surge %.1f cm/hr, swell %.1f m, water %.1f °C — complete protocol checklist for harbor deck clearance.",
		zone,
		tideSurgeCmHr,
		swellMeters,
		waterTempC,
	)
}

// UpdateProtocolStep updates a specific checklist step on a MaritimeRescueOperation and recalculates status.
func UpdateProtocolStep(op *domain.MaritimeRescueOperation, stepID string, completed bool) error {
	if op == nil {
		return fmt.Errorf("operation cannot be nil")
	}
	targetID := strings.TrimSpace(stepID)
	if targetID == "" {
		return fmt.Errorf("step_id is required")
	}

	found := false
	nowStr := time.Now().UTC().Format(time.RFC3339)
	for i := range op.ProtocolSteps {
		if op.ProtocolSteps[i].ID == targetID {
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
		return fmt.Errorf("unknown protocol step_id: %q", targetID)
	}

	switch targetID {
	case "STEP-TIDE-SUCTION-RELIEF":
		op.MudWalkersAirLanceDeployed = completed
	case "STEP-VESSEL-PFD-RIGGING":
		op.CaninePFDHarnessSecured = completed
	}

	allCompleted := len(op.ProtocolSteps) > 0
	completedCount := 0
	for _, s := range op.ProtocolSteps {
		if s.Completed {
			completedCount++
		} else {
			allCompleted = false
		}
	}

	if !allCompleted {
		op.ClearanceToken = ""
		if completedCount >= 2 {
			op.Status = domain.MaritimeStatusHovercraftMudSledStaged
		} else if isCriticalMaritimeCondition(
			op.SaltwaterAspirationSuspected,
			op.TideSurgeRateCmHr,
			op.SwellHeightMeters,
			op.MudSuctionDepthCm,
			op.MudWalkersAirLanceDeployed,
			op.WaterTempCelsius,
		) {
			op.Status = domain.MaritimeStatusTidalFloodAspirationCritical
		} else {
			op.Status = domain.MaritimeStatusHovercraftMudSledStaged
		}
	} else {
		if op.ClearanceToken != "" {
			op.Status = domain.MaritimeStatusExtractedHarborDeckSafe
		} else {
			op.Status = domain.MaritimeStatusHovercraftMudSledStaged
		}
	}

	op.TidalRiskAdvisory = buildTidalRiskAdvisory(
		op.Status,
		op.MaritimeZone,
		op.TideSurgeRateCmHr,
		op.SwellHeightMeters,
		op.WaterTempCelsius,
		op.MudSuctionDepthCm,
		op.SaltwaterAspirationSuspected,
	)
	op.UpdatedAt = nowStr
	return nil
}

// IssueMaritimeClearance verifies all 4 protocol steps are completed, transitions the operation
// to EXTRACTED_HARBOR_DECK_SAFE, and generates a deterministic MARITIME-SAFE-<12HEX> token.
func IssueMaritimeClearance(op *domain.MaritimeRescueOperation) error {
	if op == nil {
		return fmt.Errorf("operation cannot be nil")
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("protocol_steps cannot be empty")
	}
	for _, step := range op.ProtocolSteps {
		if !step.Completed {
			return fmt.Errorf("cannot issue maritime clearance: step %q (%s) is not completed", step.ID, step.Title)
		}
	}

	payload := fmt.Sprintf("MARITIME-CLEARANCE|%s|%s|%s|%s", op.ID, op.PetID, op.PetName, op.MaritimeZone)
	sum := sha256.Sum256([]byte(payload))
	token := "MARITIME-SAFE-" + strings.ToUpper(hex.EncodeToString(sum[:6]))

	op.Status = domain.MaritimeStatusExtractedHarborDeckSafe
	op.ClearanceToken = token
	op.TidalRiskAdvisory = buildTidalRiskAdvisory(
		op.Status,
		op.MaritimeZone,
		op.TideSurgeRateCmHr,
		op.SwellHeightMeters,
		op.WaterTempCelsius,
		op.MudSuctionDepthCm,
		op.SaltwaterAspirationSuspected,
	)
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains thread-safe in-memory state for maritime and coastal rescue operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.MaritimeRescueOperation
	order   []string
	counter int
}

// NewStore initializes a Store pre-populated with 3 deterministic maritime operations (MC-OP-101, MC-OP-102, MC-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.MaritimeRescueOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	// MC-OP-101: "Anchor" (Dog - Newfoundland, 54.0 kg, TIDAL_MUDFLAT_ESTUARY, TIDAL_FLOOD_ASPIRATION_CRITICAL)
	op101, err := EvaluateMaritimeRescue(domain.MaritimeEvaluationRequest{
		ID:                           "MC-OP-101",
		PetID:                        "PET-MC-101",
		PetName:                      "Anchor",
		Species:                      "Dog - Newfoundland",
		WeightKg:                     54.0,
		MaritimeZone:                 domain.MaritimeZoneTidalMudflatEstuary,
		TideSurgeRateCmHr:            62.0,
		SwellHeightMeters:            0.8,
		WaterTempCelsius:             8.5,
		SaltwaterAspirationSuspected: true,
		MudSuctionDepthCm:            48.0,
		MudWalkersAirLanceDeployed:   false,
		CaninePFDHarnessSecured:      false,
	})
	if err == nil {
		s.ops[op101.ID] = *op101
		s.order = append(s.order, op101.ID)
	}

	// MC-OP-102: "Coral" (Dog - Portuguese Water Dog, 22.5 kg, RIP_CURRENT_SURF_BREAK, HOVERCRAFT_MUD_SLED_STAGED, 2 steps completed)
	op102, err := EvaluateMaritimeRescue(domain.MaritimeEvaluationRequest{
		ID:                           "MC-OP-102",
		PetID:                        "PET-MC-102",
		PetName:                      "Coral",
		Species:                      "Dog - Portuguese Water Dog",
		WeightKg:                     22.5,
		MaritimeZone:                 domain.MaritimeZoneRipCurrentSurfBreak,
		TideSurgeRateCmHr:            25.0,
		SwellHeightMeters:            1.8,
		WaterTempCelsius:             14.5,
		SaltwaterAspirationSuspected: false,
		MudSuctionDepthCm:            0.0,
		MudWalkersAirLanceDeployed:   true,
		CaninePFDHarnessSecured:      true,
	})
	if err == nil {
		s.ops[op102.ID] = *op102
		s.order = append(s.order, op102.ID)
	}

	// MC-OP-103: "Skipper" (Cat - Maine Coon, 6.8 kg, JETTY_TETRAPOD_BREAKWATER, EXTRACTED_HARBOR_DECK_SAFE, all 4 steps completed)
	op103, err := EvaluateMaritimeRescue(domain.MaritimeEvaluationRequest{
		ID:                           "MC-OP-103",
		PetID:                        "PET-MC-103",
		PetName:                      "Skipper",
		Species:                      "Cat - Maine Coon",
		WeightKg:                     6.8,
		MaritimeZone:                 domain.MaritimeZoneJettyTetrapodBreakwater,
		TideSurgeRateCmHr:            15.0,
		SwellHeightMeters:            0.6,
		WaterTempCelsius:             16.0,
		SaltwaterAspirationSuspected: false,
		MudSuctionDepthCm:            0.0,
		MudWalkersAirLanceDeployed:   true,
		CaninePFDHarnessSecured:      true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op103, "STEP-SALTWATER-ASPIRATION-O2", true)
		_ = UpdateProtocolStep(op103, "STEP-HARBOR-DECK-DECON-WARM", true)
		_ = IssueMaritimeClearance(op103)
		for i := range op103.ProtocolSteps {
			op103.ProtocolSteps[i].CompletedAt = defaultTimestamp
		}
		op103.UpdatedAt = defaultTimestamp
		s.ops[op103.ID] = *op103
		s.order = append(s.order, op103.ID)
	}
}

func cloneOperation(op domain.MaritimeRescueOperation) domain.MaritimeRescueOperation {
	steps := make([]domain.MaritimeProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.ProtocolSteps = steps
	return op
}

// List returns all maritime rescue operations in deterministic order.
func (s *Store) List() []domain.MaritimeRescueOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.MaritimeRescueOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get retrieves a maritime rescue operation by ID.
func (s *Store) Get(id string) (domain.MaritimeRescueOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.MaritimeRescueOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new or updated maritime rescue operation and stores it.
func (s *Store) EvaluateAndSave(req domain.MaritimeEvaluationRequest) (domain.MaritimeRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, err := EvaluateMaritimeRescue(req)
	if err != nil {
		return domain.MaritimeRescueOperation{}, err
	}

	if strings.TrimSpace(req.ID) == "" {
		s.counter++
		op.ID = fmt.Sprintf("MC-OP-%d", s.counter)
	}
	if strings.TrimSpace(req.PetID) == "" {
		op.PetID = fmt.Sprintf("PET-MC-%d", s.counter)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	op.CreatedAt = nowStr
	op.UpdatedAt = nowStr

	if _, exists := s.ops[op.ID]; !exists {
		s.order = append(s.order, op.ID)
	}
	s.ops[op.ID] = cloneOperation(*op)
	return cloneOperation(*op), nil
}

// UpdateStep updates a protocol step on an existing operation in the store.
func (s *Store) UpdateStep(id string, req domain.MaritimeStepUpdateRequest) (domain.MaritimeRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	opID := strings.TrimSpace(id)
	op, exists := s.ops[opID]
	if !exists {
		return domain.MaritimeRescueOperation{}, fmt.Errorf("maritime rescue operation %q not found", opID)
	}

	cloned := cloneOperation(op)
	if err := UpdateProtocolStep(&cloned, req.StepID, req.Completed); err != nil {
		return domain.MaritimeRescueOperation{}, err
	}

	s.ops[opID] = cloned
	return cloneOperation(cloned), nil
}

// IssueClearance issues harbor deck extraction clearance on an existing operation in the store.
func (s *Store) IssueClearance(id string) (domain.MaritimeRescueOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	opID := strings.TrimSpace(id)
	op, exists := s.ops[opID]
	if !exists {
		return domain.MaritimeRescueOperation{}, fmt.Errorf("maritime rescue operation %q not found", opID)
	}

	cloned := cloneOperation(op)
	if err := IssueMaritimeClearance(&cloned); err != nil {
		return domain.MaritimeRescueOperation{}, err
	}

	s.ops[opID] = cloned
	return cloneOperation(cloned), nil
}
