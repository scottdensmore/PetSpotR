package hyperthermiaheat

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
	StepConvectiveCoolingStart = "STEP-CONVECTIVE-COOLING-START"
	StepStopCooling394CCutoff  = "STEP-STOP-COOLING-394C-CUTOFF"
	StepPawPadHydrogelDebride  = "STEP-PAW-PAD-HYDROGEL-DEBRIDE"
	StepDICCoagCrystalloidIV   = "STEP-DIC-COAG-CRYSTALLOID-IV"
)

// EvaluateHeatTriage deterministically evaluates a hyperthermia, heatstroke & hot-asphalt paw pad burn triage request.
func EvaluateHeatTriage(req domain.HeatEvaluationRequest) (*domain.HeatTriageOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg <= 0 {
		return nil, fmt.Errorf("weight_kg must be greater than 0")
	}
	if !req.ExposureType.IsValid() {
		return nil, fmt.Errorf("invalid exposure_type: %q", req.ExposureType)
	}
	if req.RectalCoreTempCelsius < 37.0 || req.RectalCoreTempCelsius > 45.0 {
		return nil, fmt.Errorf("rectal_core_temp_celsius must be between 37.0 and 45.0, got %.1f", req.RectalCoreTempCelsius)
	}
	if req.SurfaceAsphaltTempCelsius < 15.0 || req.SurfaceAsphaltTempCelsius > 95.0 {
		return nil, fmt.Errorf("surface_asphalt_temp_celsius must be between 15.0 and 95.0, got %.1f", req.SurfaceAsphaltTempCelsius)
	}
	if req.ExposureDurationMins <= 0 {
		return nil, fmt.Errorf("exposure_duration_mins must be greater than 0, got %d", req.ExposureDurationMins)
	}
	if req.PawPadSloughingGrade < 0 || req.PawPadSloughingGrade > 4 {
		return nil, fmt.Errorf("paw_pad_sloughing_grade must be between 0 and 4, got %d", req.PawPadSloughingGrade)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%d", petName, req.ExposureType, req.RectalCoreTempCelsius, req.SurfaceAsphaltTempCelsius, req.ExposureDurationMins)))
		opID = fmt.Sprintf("HT-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-HT-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.HeatStatusStagedCooling
	if req.RectalCoreTempCelsius >= 41.0 ||
		req.BrachycephalicAirwayObstruction ||
		req.PawPadSloughingGrade >= 3 ||
		!req.IceBathVasoconstrictionAvoided ||
		req.ExposureDurationMins >= 30 {
		status = domain.HeatStatusCriticalDIC
	}

	coolingProtocol := computeRequiredCoolingProtocol(req)
	cutoffNote := computeCoagulopathyCutoffNote(req)
	steps := buildHeatProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.HeatTriageOperation{
		ID:                              opID,
		PetID:                           petID,
		PetName:                         petName,
		Species:                         species,
		WeightKg:                        req.WeightKg,
		ExposureType:                    req.ExposureType,
		Status:                          status,
		RectalCoreTempCelsius:           req.RectalCoreTempCelsius,
		SurfaceAsphaltTempCelsius:       req.SurfaceAsphaltTempCelsius,
		ExposureDurationMins:            req.ExposureDurationMins,
		BrachycephalicAirwayObstruction: req.BrachycephalicAirwayObstruction,
		PawPadSloughingGrade:            req.PawPadSloughingGrade,
		CoolWaterConvectionFanActive:    req.CoolWaterConvectionFanActive,
		IceBathVasoconstrictionAvoided:  req.IceBathVasoconstrictionAvoided,
		RequiredCoolingProtocol:         coolingProtocol,
		CoagulopathyCutoffNote:          cutoffNote,
		ProtocolSteps:                   steps,
		CreatedAt:                       nowStr,
		UpdatedAt:                       nowStr,
	}, nil
}

func computeRequiredCoolingProtocol(req domain.HeatEvaluationRequest) string {
	var parts []string
	switch req.ExposureType {
	case domain.HeatExposureEnclosedVehicleSolarCabin:
		parts = append(parts, "Solar Cabin Extraction + Tepid Cool-Water (15–20°C) Drenching & High-Velocity Axial Fan Evaporative Convection")
	case domain.HeatExposureHotAsphaltPawNecrosis:
		parts = append(parts, "Immediate Off-Asphalt Carry + Sterile Saline Lavage, Silver Sulfadiazine / Hydrogel Dressings & Four-Paw Bootie Immobilization")
	case domain.HeatExposureExertionalBrachycephalicStroke:
		parts = append(parts, "Emergency Brachycephalic Upper-Airway Oxygenation / Intubation + Evaporative Mist & Jugular/Inguinal Conduction Cooling")
	case domain.HeatExposureUnshadedYardTetherDome:
		parts = append(parts, "Shade Canopy Transfer + Evaporative Cool-Water Wetting, Fan Convection & Isotonic Crystalloid Resuscitation")
	case domain.HeatExposureGarageAtticVentilationFailure:
		parts = append(parts, "Positive-Pressure Fresh-Air Extraction + Evaporative Cool-Water Spray, Fan Convection & Core Thermometry")
	default:
		parts = append(parts, "Evaporative Cool-Water Spray & High-Velocity Convection Fan Cooling")
	}

	if req.BrachycephalicAirwayObstruction {
		parts = append(parts, "BOAS Laryngeal Edema Suction/Dexamethasone/Tracheostomy Standby")
	}
	if req.PawPadSloughingGrade >= 2 {
		parts = append(parts, fmt.Sprintf("Grade %d Digital/Metacarpal Pad Epidermal Sloughing Hydrogel Debridement & Opioid Analgesia", req.PawPadSloughingGrade))
	}
	if !req.IceBathVasoconstrictionAvoided {
		parts = append(parts, "URGENT: Discontinue Ice-Water Immersion Immediately to Reverse Peripheral Vasoconstriction & Shivering Thermogenesis")
	}
	return strings.Join(parts, " | ")
}

func computeCoagulopathyCutoffNote(req domain.HeatEvaluationRequest) string {
	base := "CRITICAL CUTOFF: Stop active evaporative cooling strictly at 39.4°C (103.0°F) rectal core temperature to prevent iatrogenic rebound hypothermia; avoid ice-water baths that induce peripheral vasoconstriction and trap visceral core heat."
	if req.RectalCoreTempCelsius >= 41.0 || req.ExposureDurationMins >= 30 {
		return base + " High risk of Disseminated Intravascular Coagulation (DIC), thermal rhabdomyolysis, and hemorrhagic gastroenteritis: initiate room-temperature buffered crystalloid IV bolus, PT/aPTT & D-dimer serial panel, and fresh frozen plasma standby."
	}
	return base + " Monitor continuous rectal thermistor probe every 5 minutes and dry coat thoroughly once 39.4°C (103.0°F) threshold is reached."
}

func buildHeatProtocolSteps(req domain.HeatEvaluationRequest) []domain.HeatProtocolStep {
	return []domain.HeatProtocolStep{
		{
			ID:          StepConvectiveCoolingStart,
			Title:       "Initiate Tepid Cool-Water Evaporative Wetting & Axial Convection Fan",
			Description: "Saturate inguinal, axillary, and cervical regions with cool water (15–20°C) under continuous high-velocity fan airflow while avoiding ice-water peripheral vasoconstriction.",
			Completed:   false,
		},
		{
			ID:          StepStopCooling394CCutoff,
			Title:       "Enforce 39.4°C (103.0°F) Active Cooling Cutoff & Rebound Hypothermia Guard",
			Description: "Continuously monitor rectal probe temperature and halt all active water/fan cooling immediately at 39.4°C (103.0°F) to prevent iatrogenic afterdrop hypothermia.",
			Completed:   false,
		},
		{
			ID:          StepPawPadHydrogelDebride,
			Title:       fmt.Sprintf("Hot-Asphalt Paw Pad Burn Lavage & Hydrogel Dressing (Sloughing Grade %d, Surface %.1f°C)", req.PawPadSloughingGrade, req.SurfaceAsphaltTempCelsius),
			Description: "Irrigate blistered or sloughed metacarpal/metatarsal and digital pads with sterile 0.9% saline, apply non-adherent hydrogel / silver sulfadiazine dressings, and wrap protective paw bandages.",
			Completed:   false,
		},
		{
			ID:          StepDICCoagCrystalloidIV,
			Title:       "DIC Coagulopathy, Renal Perfusion & Airway Stabilization Panel",
			Description: "Administer shock-rate room-temperature isotonic crystalloids, secure brachycephalic airway patency, and evaluate PT/aPTT, platelets, lactate, and nucleated RBCs for heatstroke DIC.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on an operation.
func UpdateProtocolStep(op *domain.HeatTriageOperation, stepID string, completed bool) error {
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
		if op.Status == domain.HeatStatusCriticalDIC {
			op.Status = domain.HeatStatusStagedCooling
		}
	} else {
		if op.Status == domain.HeatStatusStabilizedCooled {
			op.Status = domain.HeatStatusStagedCooling
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueHeatClearance validates that all 4 protocol steps are completed, transitions the operation
// to STABILIZED_NORMOTHERMIC_COOLED, and generates a deterministic HEAT-SAFE-<12HEX> clearance token.
func IssueHeatClearance(op *domain.HeatTriageOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue heat clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%.1f|%.1f", op.ID, op.PetName, op.ExposureType, op.RectalCoreTempCelsius, op.SurfaceAsphaltTempCelsius)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("HEAT-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.HeatStatusStabilizedCooled
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory hyperthermia & hot-asphalt triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.HeatTriageOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (HT-OP-101, HT-OP-102, HT-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.HeatTriageOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateHeatTriage(domain.HeatEvaluationRequest{
		ID:                              "HT-OP-101",
		PetID:                           "PET-HT-101",
		PetName:                         "Apollo",
		Species:                         "Dog - French Bulldog",
		WeightKg:                        12.8,
		ExposureType:                    domain.HeatExposureEnclosedVehicleSolarCabin,
		RectalCoreTempCelsius:           41.8,
		SurfaceAsphaltTempCelsius:       64.0,
		ExposureDurationMins:            35,
		BrachycephalicAirwayObstruction: true,
		PawPadSloughingGrade:            2,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	})
	if err == nil {
		op101.Status = domain.HeatStatusCriticalDIC
		op101.CreatedAt = "2026-07-19T14:10:00Z"
		op101.UpdatedAt = "2026-07-19T14:15:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateHeatTriage(domain.HeatEvaluationRequest{
		ID:                              "HT-OP-102",
		PetID:                           "PET-HT-102",
		PetName:                         "Biscuit",
		Species:                         "Dog - Golden Retriever",
		WeightKg:                        31.0,
		ExposureType:                    domain.HeatExposureHotAsphaltPawNecrosis,
		RectalCoreTempCelsius:           40.1,
		SurfaceAsphaltTempCelsius:       68.5,
		ExposureDurationMins:            20,
		BrachycephalicAirwayObstruction: false,
		PawPadSloughingGrade:            2,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepConvectiveCoolingStart, true)
		_ = UpdateProtocolStep(op102, StepPawPadHydrogelDebride, true)
		op102.Status = domain.HeatStatusStagedCooling
		op102.CreatedAt = "2026-07-19T14:25:00Z"
		op102.UpdatedAt = "2026-07-19T14:35:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateHeatTriage(domain.HeatEvaluationRequest{
		ID:                              "HT-OP-103",
		PetID:                           "PET-HT-103",
		PetName:                         "Daisy",
		Species:                         "Dog - Australian Shepherd",
		WeightKg:                        21.4,
		ExposureType:                    domain.HeatExposureUnshadedYardTetherDome,
		RectalCoreTempCelsius:           39.3,
		SurfaceAsphaltTempCelsius:       42.0,
		ExposureDurationMins:            15,
		BrachycephalicAirwayObstruction: false,
		PawPadSloughingGrade:            1,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueHeatClearance(op103)
		op103.CreatedAt = "2026-07-19T13:40:00Z"
		op103.UpdatedAt = "2026-07-19T14:05:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.HeatTriageOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.HeatTriageOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.HeatTriageOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.HeatTriageOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.HeatEvaluationRequest) (domain.HeatTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("HT-OP-%d", s.seq+1)
	}

	op, err := EvaluateHeatTriage(req)
	if err != nil {
		return domain.HeatTriageOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.HeatTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.HeatTriageOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.HeatTriageOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a normothermic heat clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.HeatTriageOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.HeatTriageOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueHeatClearance(op); err != nil {
		return domain.HeatTriageOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.HeatTriageOperation) domain.HeatTriageOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.HeatProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
