package avianrecovery

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

// DefaultProtocolSteps constructs the 5 standard avian telemetry, acoustic lure & capture protocol steps.
func DefaultProtocolSteps(completedCount int, completedAt string) []domain.AvianProtocolStep {
	steps := []domain.AvianProtocolStep{
		{
			ID:       "step-vhf-triangulate",
			Label:    "Triangulate 216 MHz VHF/GPS leg-band telemetry beacon & establish 45° descent glide-path corridor",
			Category: "VHF_BAND_TRIANGULATION",
			Required: true,
		},
		{
			ID:       "step-raptor-overwatch",
			Label:    "Establish anti-raptor aerial predator watch & clear canopy branch obstacles along landing vector",
			Category: "VHF_BAND_TRIANGULATION",
			Required: true,
		},
		{
			ID:       "step-flock-acoustic-lure",
			Label:    "Broadcast directional conspecific flock contact call & owner vocalization loop at low-angle perch",
			Category: "FLOCK_ACOUSTIC_LURE",
			Required: true,
		},
		{
			ID:       "step-forage-perch-rig",
			Label:    "Stage high-visibility familiar travel cage, warm electrolyte mash & high-value nut/millet bait tray",
			Category: "FLOCK_ACOUSTIC_LURE",
			Required: true,
		},
		{
			ID:       "step-kevlar-towel-transfer",
			Label:    "Execute zero-keel-compression Kevlar/towel restraint, crop hydration check & 30°C thermal carrier transfer",
			Category: "KEVLAR_TOWEL_CAGE_TRANSFER",
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

// EvaluateAvianRecovery deterministically evaluates flight-corridor thermal drift hazards,
// crop starvation/hypothermia risks, acoustic lure recommendations, and capture readiness.
func EvaluateAvianRecovery(req domain.AvianEvaluationRequest) (domain.AvianRecoveryOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("petName is required")
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Avian"
	}

	if !req.AvianClass.IsValid() {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("invalid avianClass: %q", req.AvianClass)
	}
	if req.WeightGrams <= 0 || req.WeightGrams > 25000 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("weightGrams must be in (0, 25000], got %v", req.WeightGrams)
	}
	if req.PerchAltitudeMeters < 0 || req.PerchAltitudeMeters > 500 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("perchAltitudeMeters must be in [0, 500], got %v", req.PerchAltitudeMeters)
	}
	if req.ThermalUpdraftKnots < 0 || req.ThermalUpdraftKnots > 120 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("thermalUpdraftKnots must be in [0, 120], got %v", req.ThermalUpdraftKnots)
	}
	if req.AmbientTempCelsius < -40 || req.AmbientTempCelsius > 60 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("ambientTempCelsius must be in [-40, 60], got %v", req.AmbientTempCelsius)
	}
	if req.HoursAloft < 0 || req.HoursAloft > 720 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("hoursAloft must be in [0, 720], got %v", req.HoursAloft)
	}
	if req.TelemetrySignalDbm < -140 || req.TelemetrySignalDbm > 0 {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("telemetrySignalDbm must be in [-140, 0], got %d", req.TelemetrySignalDbm)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	opID := strings.TrimSpace(req.OperationID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f", petName, species, req.AvianClass, req.PerchAltitudeMeters, req.ThermalUpdraftKnots)))
		opID = "AV-OP-" + strings.ToUpper(hex.EncodeToString(sum[:])[:6])
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = "PET-" + opID
	}

	corridorBearing := strings.TrimSpace(req.FlightCorridorBearing)
	if corridorBearing == "" {
		corridorBearing = "CORRIDOR-SSE-155° (1.8 km Ridge Thermal)"
	}

	callsign := strings.TrimSpace(req.LeadAvianTechCallsign)
	if callsign == "" {
		callsign = "AVIAN-TECH-01"
	}

	thermalDriftAlert := req.ThermalUpdraftKnots >= 18.0 || req.PerchAltitudeMeters >= 20.0
	cropStarvationColdAlert := req.HoursAloft >= 16.0 || req.AmbientTempCelsius <= 10.0

	riskFlags := make([]string, 0, 4)
	if thermalDriftAlert {
		riskFlags = append(riskFlags, "EXTREME_THERMAL_DOWNWIND_DRIFT_RISK")
	}
	if cropStarvationColdAlert {
		riskFlags = append(riskFlags, "CRITICAL_AVIAN_CROP_DEPLETION_HYPOTHERMIA")
	}
	if req.PerchAltitudeMeters >= 20.0 {
		riskFlags = append(riskFlags, "STEEP_DESCENT_WING_STALL_HAZARD")
	}
	if req.TelemetrySignalDbm <= -75 {
		riskFlags = append(riskFlags, "WEAK_VHF_BEACON_MULTIPATH_ATTENUATION")
	}
	if len(riskFlags) == 0 {
		riskFlags = append(riskFlags, "AERIAL_RAPTOR_CORVID_MOBBING_EXPOSURE")
	}

	baseHazard := 28
	switch req.AvianClass {
	case domain.AvianClassPsittacineParrotMacaw:
		baseHazard = 36
	case domain.AvianClassCockatielParakeetSmall:
		baseHazard = 34
	case domain.AvianClassFalconryRaptorTelemetry:
		baseHazard = 32
	case domain.AvianClassWaterfowlDomesticGoose:
		baseHazard = 26
	case domain.AvianClassColumbiformHomingPigeon:
		baseHazard = 28
	}

	if thermalDriftAlert {
		baseHazard += 28
	}
	if cropStarvationColdAlert {
		baseHazard += 22
	}
	if req.PerchAltitudeMeters >= 20.0 {
		baseHazard += 6
	}
	if req.TelemetrySignalDbm <= -70 {
		baseHazard += 5
	}

	if baseHazard > 98 {
		baseHazard = 98
	}
	if baseHazard < 15 {
		baseHazard = 15
	}

	lureAudio := "Directional Parabolic Speaker (85 dB Conspecific Flock Contact Call + Bonded Caregiver Voice Loop)"
	switch req.AvianClass {
	case domain.AvianClassFalconryRaptorTelemetry:
		lureAudio = "216 MHz Yagi Directional Receiver + High-Pitch Falconer Whistle & Leather Garnished Lure Swing"
	case domain.AvianClassCockatielParakeetSmall:
		lureAudio = "Near-Field Soft Contact Chirp Transducer + Familiar Cage Bell & Millet Spray Visual Beacon"
	}

	captureRig := "45° Telescopic Perch Pole + Zero-Keel-Compression Fleece Towel & 30°C Heated Isolette Carrier"
	if req.AvianClass == domain.AvianClassFalconryRaptorTelemetry {
		captureRig = "Kevlar Gauntlet + Bewit/Jess Tether Clip, Falconry Hood & Darkened Mew Transport Box"
	} else if req.PerchAltitudeMeters >= 20.0 {
		captureRig = "Elevated Neighbor Balcony 45° Sightline Perch Rig + Drop-Door Travel Aviary & Warm Dextrose/Electrolyte Crop Syringe"
	}

	initialCompleted := 2
	status := domain.AvianStatusThermalDriftPredationCritical
	if !thermalDriftAlert && !cropStarvationColdAlert && baseHazard < 60 {
		initialCompleted = 3
		status = domain.AvianStatusAcousticLurePerchStaged
	}

	steps := DefaultProtocolSteps(initialCompleted, nowStr)
	readiness := computeLureReadinessScore(steps)

	op := domain.AvianRecoveryOperation{
		OperationID:             opID,
		PetID:                   petID,
		PetName:                 petName,
		Species:                 species,
		WeightGrams:             req.WeightGrams,
		AvianClass:              req.AvianClass,
		Status:                  status,
		PerchAltitudeMeters:     req.PerchAltitudeMeters,
		ThermalUpdraftKnots:     req.ThermalUpdraftKnots,
		AmbientTempCelsius:      req.AmbientTempCelsius,
		HoursAloft:              req.HoursAloft,
		TelemetrySignalDbm:      req.TelemetrySignalDbm,
		HazardScore:             baseHazard,
		LureReadinessScore:      readiness,
		ThermalDriftAlert:       thermalDriftAlert,
		CropStarvationColdAlert: cropStarvationColdAlert,
		RecommendedLureAudio:    lureAudio,
		RecommendedCaptureRig:   captureRig,
		FlightCorridorBearing:   corridorBearing,
		RiskFlags:               riskFlags,
		ProtocolSteps:           steps,
		LeadAvianTechCallsign:   callsign,
		Notes:                   strings.TrimSpace(req.Notes),
		UpdatedAt:               nowStr,
	}

	if err := op.Validate(); err != nil {
		return domain.AvianRecoveryOperation{}, err
	}
	return op, nil
}

func computeLureReadinessScore(steps []domain.AvianProtocolStep) int {
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
func UpdateProtocolStep(op domain.AvianRecoveryOperation, req domain.AvianStepUpdateRequest) (domain.AvianRecoveryOperation, error) {
	stepID := strings.TrimSpace(req.StepID)
	if stepID == "" {
		return op, fmt.Errorf("stepId is required")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	found := false
	steps := make([]domain.AvianProtocolStep, len(op.ProtocolSteps))
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
	op.LureReadinessScore = computeLureReadinessScore(steps)
	if trimmedNotes := strings.TrimSpace(req.Notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	if op.Status != domain.AvianStatusRecoveredAviarySafe {
		if op.LureReadinessScore >= 60 {
			op.Status = domain.AvianStatusAcousticLurePerchStaged
		} else {
			op.Status = domain.AvianStatusThermalDriftPredationCritical
		}
	}

	op.UpdatedAt = nowStr
	return op, nil
}

// IssueAvianClearance verifies protocol readiness, generates a deterministic SHA-256 clearance
// certificate formatted AVIAN-SAFE-<12HEX>, and transitions the operation to RECOVERED_AVIARY_SAFE.
func IssueAvianClearance(op domain.AvianRecoveryOperation, leadAvianTechCallsign, notes string) (domain.AvianRecoveryOperation, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return op, fmt.Errorf("operationId is required")
	}

	for _, step := range op.ProtocolSteps {
		if step.Required && !step.Completed {
			return op, fmt.Errorf("cannot issue avian clearance: required protocol step %q is not completed", step.ID)
		}
	}

	if trimmedCallsign := strings.TrimSpace(leadAvianTechCallsign); trimmedCallsign != "" {
		op.LeadAvianTechCallsign = trimmedCallsign
	}
	if trimmedNotes := strings.TrimSpace(notes); trimmedNotes != "" {
		op.Notes = trimmedNotes
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payload := fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%s",
		op.OperationID,
		op.PetID,
		op.AvianClass,
		op.PerchAltitudeMeters,
		op.ThermalUpdraftKnots,
		op.LeadAvianTechCallsign,
	)
	digest := sha256.Sum256([]byte(payload))
	op.ClearanceCertificateHash = "AVIAN-SAFE-" + strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	op.Status = domain.AvianStatusRecoveredAviarySafe
	op.HazardScore = 10
	op.LureReadinessScore = 100
	op.ThermalDriftAlert = false
	op.CropStarvationColdAlert = false
	op.UpdatedAt = nowStr

	return op, nil
}

// SeedOperations returns the 3 realistic seeded avian recovery operations (AV-OP-101, AV-OP-102, AV-OP-103).
func SeedOperations() []domain.AvianRecoveryOperation {
	baseTime := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)

	op101 := domain.AvianRecoveryOperation{
		OperationID:             "AV-OP-101",
		PetID:                   "PET-AV-101",
		PetName:                 "Rio",
		Species:                 "Avian",
		WeightGrams:             950.0,
		AvianClass:              domain.AvianClassPsittacineParrotMacaw,
		Status:                  domain.AvianStatusThermalDriftPredationCritical,
		PerchAltitudeMeters:     26.0,
		ThermalUpdraftKnots:     22.5,
		AmbientTempCelsius:      7.5,
		HoursAloft:              24.0,
		TelemetrySignalDbm:      -78,
		HazardScore:             95,
		LureReadinessScore:      40,
		ThermalDriftAlert:       true,
		CropStarvationColdAlert: true,
		RecommendedLureAudio:    "Directional Parabolic Speaker (85 dB Conspecific Flock Contact Call + Bonded Caregiver Voice Loop)",
		RecommendedCaptureRig:   "Elevated Neighbor Balcony 45° Sightline Perch Rig + Drop-Door Travel Aviary & Warm Dextrose/Electrolyte Crop Syringe",
		FlightCorridorBearing:   "CORRIDOR-SSE-162° (Discovery Park Bluff Ridge Thermal)",
		RiskFlags: []string{
			"EXTREME_THERMAL_DOWNWIND_DRIFT_RISK",
			"CRITICAL_AVIAN_CROP_DEPLETION_HYPOTHERMIA",
			"STEEP_DESCENT_WING_STALL_HAZARD",
		},
		ProtocolSteps:         DefaultProtocolSteps(2, baseTime.Add(-15*time.Minute).Format(time.RFC3339)),
		LeadAvianTechCallsign: "AVIAN-TECH-01 (Ortega)",
		Notes:                 "Blue-and-gold macaw perched 26m high in Douglas fir crown; 22.5 kt ridge thermal updraft and 7.5°C chill after 24h aloft. 45° descent perch staged on adjacent balcony.",
		UpdatedAt:             baseTime.Add(-5 * time.Minute).Format(time.RFC3339),
	}

	op102 := domain.AvianRecoveryOperation{
		OperationID:             "AV-OP-102",
		PetID:                   "PET-AV-102",
		PetName:                 "Zephyr",
		Species:                 "Avian",
		WeightGrams:             780.0,
		AvianClass:              domain.AvianClassFalconryRaptorTelemetry,
		Status:                  domain.AvianStatusAcousticLurePerchStaged,
		PerchAltitudeMeters:     14.0,
		ThermalUpdraftKnots:     11.0,
		AmbientTempCelsius:      16.0,
		HoursAloft:              5.5,
		TelemetrySignalDbm:      -52,
		HazardScore:             44,
		LureReadinessScore:      100,
		ThermalDriftAlert:       false,
		CropStarvationColdAlert: false,
		RecommendedLureAudio:    "216 MHz Yagi Directional Receiver + High-Pitch Falconer Whistle & Leather Garnished Lure Swing",
		RecommendedCaptureRig:   "Kevlar Gauntlet + Bewit/Jess Tether Clip, Falconry Hood & Darkened Mew Transport Box",
		FlightCorridorBearing:   "CORRIDOR-ENE-074° (Skagit Valley Windbreak Line)",
		RiskFlags: []string{
			"AERIAL_RAPTOR_CORVID_MOBBING_EXPOSURE",
		},
		ProtocolSteps:         DefaultProtocolSteps(5, baseTime.Add(-25*time.Minute).Format(time.RFC3339)),
		LeadAvianTechCallsign: "AVIAN-TECH-04 (Ashford)",
		Notes:                 "Peregrine tiercel localized via 216.045 MHz tail-mount VHF transmitter (-52 dBm); garnished lure swung in open meadow clear of corvid mobbing.",
		UpdatedAt:             baseTime.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	op103 := domain.AvianRecoveryOperation{
		OperationID:             "AV-OP-103",
		PetID:                   "PET-AV-103",
		PetName:                 "Sunny",
		Species:                 "Avian",
		WeightGrams:             92.0,
		AvianClass:              domain.AvianClassCockatielParakeetSmall,
		Status:                  domain.AvianStatusRecoveredAviarySafe,
		PerchAltitudeMeters:     4.5,
		ThermalUpdraftKnots:     5.0,
		AmbientTempCelsius:      21.0,
		HoursAloft:              2.0,
		TelemetrySignalDbm:      -40,
		HazardScore:             10,
		LureReadinessScore:      100,
		ThermalDriftAlert:       false,
		CropStarvationColdAlert: false,
		RecommendedLureAudio:    "Near-Field Soft Contact Chirp Transducer + Familiar Cage Bell & Millet Spray Visual Beacon",
		RecommendedCaptureRig:   "45° Telescopic Perch Pole + Zero-Keel-Compression Fleece Towel & 30°C Heated Isolette Carrier",
		FlightCorridorBearing:   "CORRIDOR-WNW-290° (Green Lake Courtyard Maples)",
		RiskFlags: []string{
			"AERIAL_RAPTOR_CORVID_MOBBING_EXPOSURE",
		},
		ProtocolSteps:            DefaultProtocolSteps(5, baseTime.Add(-40*time.Minute).Format(time.RFC3339)),
		ClearanceCertificateHash: "AVIAN-SAFE-4D8E12A9F603",
		LeadAvianTechCallsign:    "AVIAN-TECH-02 (lin)",
		Notes:                    "Lutino cockatiel descended to familiar travel cage top on millet spray lure; zero keel compression transfer to 30°C warmed carrier verified.",
		UpdatedAt:                baseTime.Add(-20 * time.Minute).Format(time.RFC3339),
	}

	return []domain.AvianRecoveryOperation{op101, op102, op103}
}

// Store provides thread-safe in-memory management for avian recovery operations.
type Store struct {
	mu      sync.RWMutex
	ops     map[string]domain.AvianRecoveryOperation
	order   []string
	counter int
}

// NewStore creates and seeds an AvianRecoveryOperation store with AV-OP-101, AV-OP-102, and AV-OP-103.
func NewStore() *Store {
	s := &Store{
		ops:     make(map[string]domain.AvianRecoveryOperation),
		order:   make([]string, 0, 8),
		counter: 103,
	}
	for _, op := range SeedOperations() {
		s.ops[op.OperationID] = cloneOperation(op)
		s.order = append(s.order, op.OperationID)
	}
	return s
}

func cloneOperation(op domain.AvianRecoveryOperation) domain.AvianRecoveryOperation {
	flags := make([]string, len(op.RiskFlags))
	copy(flags, op.RiskFlags)
	steps := make([]domain.AvianProtocolStep, len(op.ProtocolSteps))
	copy(steps, op.ProtocolSteps)
	op.RiskFlags = flags
	op.ProtocolSteps = steps
	return op
}

// List returns all operations in deterministic order (AV-OP-101, AV-OP-102, AV-OP-103, followed by newly evaluated ops).
func (s *Store) List() []domain.AvianRecoveryOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.AvianRecoveryOperation, 0, len(s.ops))
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
func (s *Store) Get(id string) (domain.AvianRecoveryOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.AvianRecoveryOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a new avian recovery operation and persists it in the store.
func (s *Store) EvaluateAndSave(req domain.AvianEvaluationRequest) (domain.AvianRecoveryOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.OperationID) == ""
	if autoID {
		req.OperationID = fmt.Sprintf("AV-OP-%d", s.counter+1)
	}

	op, err := EvaluateAvianRecovery(req)
	if err != nil {
		return domain.AvianRecoveryOperation{}, err
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
func (s *Store) UpdateStep(id string, req domain.AvianStepUpdateRequest) (domain.AvianRecoveryOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("avian recovery operation %q not found", key)
	}

	updated, err := UpdateProtocolStep(existing, req)
	if err != nil {
		return domain.AvianRecoveryOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}

// IssueClearance issues avian recovery clearance for an existing operation in the store.
func (s *Store) IssueClearance(id, leadAvianTechCallsign, notes string) (domain.AvianRecoveryOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	existing, ok := s.ops[key]
	if !ok {
		return domain.AvianRecoveryOperation{}, fmt.Errorf("avian recovery operation %q not found", key)
	}

	updated, err := IssueAvianClearance(existing, leadAvianTechCallsign, notes)
	if err != nil {
		return domain.AvianRecoveryOperation{}, err
	}
	s.ops[key] = cloneOperation(updated)
	return cloneOperation(updated), nil
}
