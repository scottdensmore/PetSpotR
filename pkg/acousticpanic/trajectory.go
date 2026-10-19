package acousticpanic

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
	StepSilentNoChasePerimeter         = "STEP-SILENT-NO-CHASE-PERIMETER"
	StepScentPheromoneRefugeLure       = "STEP-SCENT-PHEROMONE-REFUGE-LURE"
	StepBarrierLacerationClawCheck     = "STEP-BARRIER-LACERATION-CLAW-CHECK"
	StepThundershirtAcousticDecompress = "STEP-THUNDERSHIRT-ACOUSTIC-DECOMPRESS"
)

// EvaluateAcousticPanic deterministically evaluates a fireworks/thunderstorm acoustic panic, bolt-trajectory vectoring & calming pheromone SAR request.
func EvaluateAcousticPanic(req domain.AcousticEvaluationRequest) (*domain.AcousticPanicOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 || req.WeightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be between 0 (exclusive) and 120.0, got %.1f", req.WeightKg)
	}
	if !req.TriggerType.IsValid() {
		return nil, fmt.Errorf("invalid trigger_type: %q", req.TriggerType)
	}
	if req.PeakDecibelsDb < 60.0 || req.PeakDecibelsDb > 180.0 {
		return nil, fmt.Errorf("peak_decibels_db must be between 60.0 and 180.0, got %.1f", req.PeakDecibelsDb)
	}
	if req.BarometricDropHpa < 0.0 || req.BarometricDropHpa > 60.0 {
		return nil, fmt.Errorf("barometric_drop_hpa must be between 0.0 and 60.0, got %.1f", req.BarometricDropHpa)
	}
	if req.BoltHeadingDegrees < 0 || req.BoltHeadingDegrees > 359 {
		return nil, fmt.Errorf("bolt_heading_degrees must be between 0 and 359, got %d", req.BoltHeadingDegrees)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%d", petName, req.TriggerType, req.PeakDecibelsDb, req.BarometricDropHpa, req.BoltHeadingDegrees)))
		opID = fmt.Sprintf("AP-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-AP-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.AcousticStatusStagedSniff
	if req.PeakDecibelsDb >= 115.0 ||
		req.BarrierBreachInjurySuspected ||
		!req.NoChasePassiveLureEnforced ||
		req.BarometricDropHpa >= 12.0 {
		status = domain.AcousticStatusCriticalBolt
	}

	corridor := computePredictedRefugeCorridor(req)
	advisory := computeAdrenalNoChaseAdvisory(req)
	steps := buildAcousticProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.AcousticPanicOperation{
		ID:                           opID,
		PetID:                        petID,
		PetName:                      petName,
		Species:                      species,
		WeightKg:                     req.WeightKg,
		TriggerType:                  req.TriggerType,
		Status:                       status,
		PeakDecibelsDb:               req.PeakDecibelsDb,
		BarometricDropHpa:            req.BarometricDropHpa,
		BoltHeadingDegrees:           req.BoltHeadingDegrees,
		BarrierBreachInjurySuspected: req.BarrierBreachInjurySuspected,
		NoChasePassiveLureEnforced:   req.NoChasePassiveLureEnforced,
		AdaptilFeliwayStationStaged:  req.AdaptilFeliwayStationStaged,
		PredictedRefugeCorridor:      corridor,
		AdrenalNoChaseAdvisory:       advisory,
		ProtocolSteps:                steps,
		CreatedAt:                    nowStr,
		UpdatedAt:                    nowStr,
	}, nil
}

func compassSector(deg int) string {
	sectors := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	idx := ((deg + 22) % 360) / 45
	if idx < 0 || idx >= len(sectors) {
		return "N"
	}
	return sectors[idx]
}

func computePredictedRefugeCorridor(req domain.AcousticEvaluationRequest) string {
	dir := compassSector(req.BoltHeadingDegrees)
	var parts []string
	parts = append(parts, fmt.Sprintf("Vector %03d° (%s) Acoustic-Shadow Bolt Trajectory", req.BoltHeadingDegrees, dir))

	switch req.TriggerType {
	case domain.TriggerFireworksBarrage:
		parts = append(parts, "Subterranean Culvert / Under-Deck Crawlspace & Dense Hedgerow Acoustic Dampening Corridor (1.2–3.5 km Straight-Line Adrenal Flight)")
	case domain.TriggerThunderstormBarometricDrop:
		parts = append(parts, fmt.Sprintf("Grounded Concrete Basement Window-Well / Storm-Drain Leeward Electrostatic Shelter (Barometric Drop %.1f hPa)", req.BarometricDropHpa))
	case domain.TriggerGunshotConstructionBlasting:
		parts = append(parts, "Opposite-Azimuth Wooded Ravine & Thick Brush Concealment Corridor Away from Concussion Impulse Source")
	case domain.TriggerHighVoltageTransformerArc:
		parts = append(parts, "Low-EMI Masonry Carport / Enclosed Shed Refuge Away from Corona Discharge & Ozone Plume")
	case domain.TriggerSirenSonicBoomStartle:
		parts = append(parts, "Immediate Structural Foundation Alcove / Recessed Porch & Heavy Shrubbery Tight-Wedge Hideout")
	default:
		parts = append(parts, "Acoustic-Shadow Subterranean Crawlspace & Dense Vegetative Refuge")
	}

	if req.AdaptilFeliwayStationStaged {
		parts = append(parts, "Adaptil (DAP) / Feliway Pheromone & Unwashed Owner Scent-Article Lure Triangulation Active")
	} else {
		parts = append(parts, "STAGE IMMEDIATELY: Deploy Adaptil/Feliway Calming Pheromone Diffuser Wipes & Warmed Scented Bedding at Last-Seen Point")
	}

	if req.BarrierBreachInjurySuspected {
		parts = append(parts, "Inspect Perimeter Chain-Link / Glass / Wooden Fence Breach for Blood Droplets, Tuft Hair & Torn Digital Pads")
	}

	return strings.Join(parts, " | ")
}

func computeAdrenalNoChaseAdvisory(req domain.AcousticEvaluationRequest) string {
	base := "CRITICAL ADRENAL FLIGHT PROTOCOL: NEVER call out loudly or chase an adrenalized acoustic-panic pet in flight mode — vocal pursuit or foot chase triggers sympathetic survival panic and drives them directly into high-speed roadway traffic. Enforce a strictly silent, low-posture scent-article & calming pheromone (Adaptil/Feliway) perimeter."
	if !req.NoChasePassiveLureEnforced {
		return base + " URGENT VIOLATION ALERT: Active search-party calling/chasing detected — halt all mobile foot pursuit and whistle/megaphone broadcasting immediately and transition to passive camera-monitored scent lure stations."
	}
	if req.PeakDecibelsDb >= 115.0 || req.BarrierBreachInjurySuspected {
		return base + fmt.Sprintf(" High-intensity impulse (%.1f dB) with suspected barrier-breach trauma: stage silent humane box/missy trap lined with owner pillowcase and prepare immediate exam for torn claws, oral/dental avulsions, and lacerations.", req.PeakDecibelsDb)
	}
	return base + " Maintain quiet 50m observation standoff with night-vision thermal optics until sympathetic cortisol surge subsides."
}

func buildAcousticProtocolSteps(req domain.AcousticEvaluationRequest) []domain.AcousticProtocolStep {
	return []domain.AcousticProtocolStep{
		{
			ID:          StepSilentNoChasePerimeter,
			Title:       "Enforce Silent No-Chase Perimeter & Halt Vocal/Foot Pursuit",
			Description: "Immediately stand down all loud calling, whistling, and mobile foot chases along the bolt trajectory to prevent driving the adrenalized pet into roadway traffic.",
			Completed:   false,
		},
		{
			ID:          StepScentPheromoneRefugeLure,
			Title:       fmt.Sprintf("Deploy Owner Scent-Article & Adaptil/Feliway Pheromone Station Along %03d° Vector", req.BoltHeadingDegrees),
			Description: "Stage unwashed owner clothing, warmed high-value aromatic food, and synthetic Dog Appeasing Pheromone (Adaptil) or feline facial pheromone (Feliway) wipes at acoustic-shadow refuge nodes.",
			Completed:   false,
		},
		{
			ID:          StepBarrierLacerationClawCheck,
			Title:       "Inspect Window/Fence Barrier Breach & Triage Claw/Dental/Pad Lacerations",
			Description: "Examine the escape breach point (broken glass, chain-link, or splintered wood) and perform gentle hemostasis and bandage care for avulsed dewclaws, fractured teeth, and pad lacerations.",
			Completed:   false,
		},
		{
			ID:          StepThundershirtAcousticDecompress,
			Title:       "Apply Thundershirt Swaddle, White-Noise Acoustic Shielding & Cortisol Decompression",
			Description: "Place recovered pet in a darkened, sound-buffered interior sanctuary with gentle thoracic pressure wrap (Thundershirt), brown-noise masking, and veterinary anxiolytic assessment.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on an acoustic panic operation.
func UpdateProtocolStep(op *domain.AcousticPanicOperation, stepID string, completed bool) error {
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
		if op.Status == domain.AcousticStatusCriticalBolt {
			op.Status = domain.AcousticStatusStagedSniff
		}
	} else {
		if op.Status == domain.AcousticStatusDecompressedSafe {
			op.Status = domain.AcousticStatusStagedSniff
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueAcousticClearance validates that all 4 protocol steps are completed, transitions the operation
// to DECOMPRESSED_CONTAINED_SAFE, and generates a deterministic ACOUSTIC-SAFE-<12HEX> clearance token.
func IssueAcousticClearance(op *domain.AcousticPanicOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue acoustic clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%d", op.ID, op.PetName, op.TriggerType, op.PeakDecibelsDb, op.BarometricDropHpa, op.BoltHeadingDegrees)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("ACOUSTIC-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.AcousticStatusDecompressedSafe
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory acoustic panic & bolt-trajectory operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.AcousticPanicOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (AP-OP-101, AP-OP-102, AP-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.AcousticPanicOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateAcousticPanic(domain.AcousticEvaluationRequest{
		ID:                           "AP-OP-101",
		PetID:                        "PET-AP-101",
		PetName:                      "Boomer",
		Species:                      "Dog - Border Collie",
		WeightKg:                     19.5,
		TriggerType:                  domain.TriggerFireworksBarrage,
		PeakDecibelsDb:               132.0,
		BarometricDropHpa:            2.5,
		BoltHeadingDegrees:           215,
		BarrierBreachInjurySuspected: true,
		NoChasePassiveLureEnforced:   false,
		AdaptilFeliwayStationStaged:  true,
	})
	if err == nil {
		op101.Status = domain.AcousticStatusCriticalBolt
		op101.CreatedAt = "2026-07-04T22:15:00Z"
		op101.UpdatedAt = "2026-07-04T22:20:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateAcousticPanic(domain.AcousticEvaluationRequest{
		ID:                           "AP-OP-102",
		PetID:                        "PET-AP-102",
		PetName:                      "Willow",
		Species:                      "Dog - Whippet",
		WeightKg:                     13.2,
		TriggerType:                  domain.TriggerThunderstormBarometricDrop,
		PeakDecibelsDb:               104.0,
		BarometricDropHpa:            9.5,
		BoltHeadingDegrees:           85,
		BarrierBreachInjurySuspected: false,
		NoChasePassiveLureEnforced:   true,
		AdaptilFeliwayStationStaged:  true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepSilentNoChasePerimeter, true)
		_ = UpdateProtocolStep(op102, StepScentPheromoneRefugeLure, true)
		op102.Status = domain.AcousticStatusStagedSniff
		op102.CreatedAt = "2026-07-04T22:30:00Z"
		op102.UpdatedAt = "2026-07-04T22:40:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateAcousticPanic(domain.AcousticEvaluationRequest{
		ID:                           "AP-OP-103",
		PetID:                        "PET-AP-103",
		PetName:                      "Milo",
		Species:                      "Cat - Russian Blue",
		WeightKg:                     4.8,
		TriggerType:                  domain.TriggerSirenSonicBoomStartle,
		PeakDecibelsDb:               98.0,
		BarometricDropHpa:            1.0,
		BoltHeadingDegrees:           310,
		BarrierBreachInjurySuspected: false,
		NoChasePassiveLureEnforced:   true,
		AdaptilFeliwayStationStaged:  true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueAcousticClearance(op103)
		op103.CreatedAt = "2026-07-04T21:10:00Z"
		op103.UpdatedAt = "2026-07-04T21:55:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.AcousticPanicOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.AcousticPanicOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.AcousticPanicOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.AcousticPanicOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.AcousticEvaluationRequest) (domain.AcousticPanicOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("AP-OP-%d", s.seq+1)
	}

	op, err := EvaluateAcousticPanic(req)
	if err != nil {
		return domain.AcousticPanicOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.AcousticPanicOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.AcousticPanicOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.AcousticPanicOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues an acoustic decompression & containment clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.AcousticPanicOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.AcousticPanicOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueAcousticClearance(op); err != nil {
		return domain.AcousticPanicOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.AcousticPanicOperation) domain.AcousticPanicOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.AcousticProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
