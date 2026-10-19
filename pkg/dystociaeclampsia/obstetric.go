package dystociaeclampsia

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
	StepDopplerFetalHRPelvicXray     = "STEP-DOPPLER-FETAL-HR-PELVIC-XRAY"
	StepECGCalciumGluconateInfusion  = "STEP-ECG-CALCIUM-GLUCONATE-INFUSION"
	StepOxytocinGateOrEmergencyCSect = "STEP-OXYTOCIN-GATE-OR-EMERGENCY-CSECTION"
	StepNeonatalResusColostrumLatch  = "STEP-NEONATAL-RESUS-COLOSTRUM-LATCH"
)

// EvaluateDystociaTriage deterministically evaluates a dystocia, maternal C-section, eclampsia (hypocalcemia) & oxytocin contraindication triage request.
func EvaluateDystociaTriage(req domain.DystociaEvaluationRequest) (*domain.DystociaEclampsiaOperation, error) {
	petName := strings.TrimSpace(req.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if req.WeightKg < 1.5 || req.WeightKg > 90.0 {
		return nil, fmt.Errorf("weight_kg must be between 1.5 and 90.0, got %.1f", req.WeightKg)
	}
	if !req.PresentationType.IsValid() {
		return nil, fmt.Errorf("invalid presentation_type: %q", req.PresentationType)
	}
	if req.LowestFetalHeartRateBpm < 80 || req.LowestFetalHeartRateBpm > 280 {
		return nil, fmt.Errorf("lowest_fetal_heart_rate_bpm must be between 80 and 280, got %d", req.LowestFetalHeartRateBpm)
	}
	if req.IonizedCalciumMmolL < 0.4 || req.IonizedCalciumMmolL > 1.8 {
		return nil, fmt.Errorf("ionized_calcium_mmol_l must be between 0.4 and 1.8, got %.2f", req.IonizedCalciumMmolL)
	}
	if req.InterPuppyIntervalMins < 0 || req.InterPuppyIntervalMins > 480 {
		return nil, fmt.Errorf("inter_puppy_interval_mins must be between 0 and 480, got %d", req.InterPuppyIntervalMins)
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog - Mixed Breed Dam"
	}

	opID := strings.TrimSpace(req.ID)
	if opID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%.2f|%d", petName, req.PresentationType, req.LowestFetalHeartRateBpm, req.IonizedCalciumMmolL, req.InterPuppyIntervalMins)))
		opID = fmt.Sprintf("DE-OP-%s", strings.ToUpper(hex.EncodeToString(sum[:3])))
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		sum := sha256.Sum256([]byte(petName + "|" + species))
		petID = fmt.Sprintf("PET-DE-%s", strings.ToUpper(hex.EncodeToString(sum[:2])))
	}

	status := domain.DystociaStatusCalciumECGWhelpingStaged
	if req.LowestFetalHeartRateBpm < 180 ||
		req.IonizedCalciumMmolL < 0.9 ||
		req.GreenBlackUteroverdinNoFetus ||
		!req.PelvicCanalObstructionExcluded ||
		req.InterPuppyIntervalMins >= 120 {
		status = domain.DystociaStatusFetalDistressObstructiveCritical
	}

	protocol := computeRequiredObstetricProtocol(req)
	contraNote := computeOxytocinContraindicationNote(req)
	steps := buildDystociaProtocolSteps(req)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	return &domain.DystociaEclampsiaOperation{
		ID:                             opID,
		PetID:                          petID,
		PetName:                        petName,
		Species:                        species,
		WeightKg:                       req.WeightKg,
		PresentationType:               req.PresentationType,
		Status:                         status,
		LowestFetalHeartRateBpm:        req.LowestFetalHeartRateBpm,
		IonizedCalciumMmolL:            req.IonizedCalciumMmolL,
		InterPuppyIntervalMins:         req.InterPuppyIntervalMins,
		GreenBlackUteroverdinNoFetus:   req.GreenBlackUteroverdinNoFetus,
		PelvicCanalObstructionExcluded: req.PelvicCanalObstructionExcluded,
		ECGMonitoredCalciumInfusion:    req.ECGMonitoredCalciumInfusion,
		RequiredObstetricProtocol:      protocol,
		OxytocinContraindicationNote:   contraNote,
		ProtocolSteps:                  steps,
		CreatedAt:                      nowStr,
		UpdatedAt:                      nowStr,
	}, nil
}

func computeRequiredObstetricProtocol(req domain.DystociaEvaluationRequest) string {
	var parts []string
	caGluconateMl := req.WeightKg * 1.0 // 0.5-1.5 mL/kg of 10% calcium gluconate slow IV

	switch req.PresentationType {
	case domain.DystociaObstructiveBrachycephalicFetalOversize:
		parts = append(parts, fmt.Sprintf("EMERGENCY MATERNAL C-SECTION (Hysterotomy): Brachycephalic Fetal Oversize / Pelvic Disproportion (Fetal HR %d bpm) — Oxytocin Strictly Contraindicated; Propofol/Alfaxalone + Isoflurane/Sevoflurane Rapid-Sequence Induction", req.LowestFetalHeartRateBpm))
	case domain.DystociaPrimaryUterineInertiaNormocalcemic:
		parts = append(parts, fmt.Sprintf("Medical Whelping Management for Primary Uterine Inertia: Verify Patent Pelvic Canal on Radiographs + Slow 10%% Calcium Gluconate %.1f mL IV before Low-Dose Oxytocin (0.1–0.5 IU/dam IM/SC)", caGluconateMl))
	case domain.DystociaPuerperalEclampsiaHypocalcemiaTetany:
		parts = append(parts, fmt.Sprintf("Puerperal Eclampsia Hypocalcemic Tetany Reversal (iCa %.2f mmol/L): 10%% Calcium Gluconate %.1f mL Slow IV over 15–20 min with Continuous Lead-II ECG + Dextrose Check & Temporary 24h Weaning", req.IonizedCalciumMmolL, caGluconateMl))
	case domain.DystociaPlacentalDetachmentUteroverdin:
		parts = append(parts, fmt.Sprintf("URGENT SURGICAL INTERVENTION: Dark Green/Black Uteroverdin Lochnia Without Expelled Fetus Indicates Placental Separation & Acute Fetal Hypoxia (Fetal HR %d bpm)", req.LowestFetalHeartRateBpm))
	case domain.DystociaMalpresentationBreechTransverse:
		parts = append(parts, fmt.Sprintf("Obstetric Malpresentation (Breech / Transverse Posture, Interval %d min): Sterile Lubricated Digital Feathering or Immediate Hysterotomy if Uncorrectable", req.InterPuppyIntervalMins))
	default:
		parts = append(parts, "Transabdominal Doppler Fetal Heart Rate Monitoring + Lateral/VD Abdominal Radiographs")
	}

	if req.LowestFetalHeartRateBpm < 180 {
		parts = append(parts, fmt.Sprintf("CRITICAL FETAL BRADYCARDIA (%d bpm < 180 bpm threshold): Immediate 100%% Flow-By Maternal O2 & Surgical C-Section Prep", req.LowestFetalHeartRateBpm))
	}
	if req.IonizedCalciumMmolL < 0.9 || !req.ECGMonitoredCalciumInfusion {
		parts = append(parts, fmt.Sprintf("10%% Calcium Gluconate (%.1f mL Slow IV over 15-20 min) under Continuous ECG Monitoring (Halt Infusion if Bradycardia or QT Shortening Occurs)", caGluconateMl))
	}
	if !req.PelvicCanalObstructionExcluded {
		parts = append(parts, "HOLD ALL ECBOLIC OXYTOCIN: Pelvic Canal Obstruction / Fetal Oversize NOT Excluded on Radiographs")
	}
	return strings.Join(parts, " | ")
}

func computeOxytocinContraindicationNote(req domain.DystociaEvaluationRequest) string {
	base := "CRITICAL OBSTETRIC CONTRAINDICATION: NEVER administer oxytocin when obstructive dystocia or fetal oversize is present (causes catastrophic uterine rupture, placental separation, and fetal asphyxiation); ALWAYS monitor continuous ECG during slow 10% calcium gluconate IV infusion and pause immediately if bradycardia, S-T elevation, or QT shortening develops."
	if !req.PelvicCanalObstructionExcluded || req.LowestFetalHeartRateBpm < 180 || req.GreenBlackUteroverdinNoFetus {
		return base + fmt.Sprintf(" Urgent surgical C-section indicated (Fetal HR %d bpm, Inter-Puppy Interval %d mins, iCa %.2f mmol/L): withhold oxytocin and prepare warmed neonatal resuscitation station.", req.LowestFetalHeartRateBpm, req.InterPuppyIntervalMins, req.IonizedCalciumMmolL)
	}
	return base + fmt.Sprintf(" Patent pelvic canal verified and fetal HR stable (%d bpm, iCa %.2f mmol/L): titrate calcium gluconate before low-dose oxytocin and monitor inter-puppy interval (%d mins).", req.LowestFetalHeartRateBpm, req.IonizedCalciumMmolL, req.InterPuppyIntervalMins)
}

func buildDystociaProtocolSteps(req domain.DystociaEvaluationRequest) []domain.DystociaProtocolStep {
	caGluconateMl := req.WeightKg * 1.0
	return []domain.DystociaProtocolStep{
		{
			ID:          StepDopplerFetalHRPelvicXray,
			Title:       fmt.Sprintf("Transabdominal Ultrasound Doppler Fetal HR (%d bpm) & Lateral/VD Pelvic Radiographs", req.LowestFetalHeartRateBpm),
			Description: "Quantify fetal heart rates (< 180 bpm indicates acute fetal distress) and obtain orthogonal abdominal radiographs to count fetal skulls/spines and rule out pelvic canal obstruction or malpresentation.",
			Completed:   false,
		},
		{
			ID:          StepECGCalciumGluconateInfusion,
			Title:       fmt.Sprintf("Continuous ECG-Monitored 10%% Calcium Gluconate Slow IV Infusion (iCa %.2f mmol/L, %.1f mL)", req.IonizedCalciumMmolL, caGluconateMl),
			Description: "Administer 0.5–1.5 mL/kg of 10% calcium gluconate slowly IV over 15–20 minutes under continuous Lead-II ECG auscultation, pausing immediately on bradycardia or QT shortening.",
			Completed:   false,
		},
		{
			ID:          StepOxytocinGateOrEmergencyCSect,
			Title:       "Oxytocin Contraindication Safety Gate or Emergency Maternal Hysterotomy (C-Section)",
			Description: "Strictly withhold oxytocin if obstructive dystocia, fetal oversize, uteroverdin placental detachment, or fetal HR < 180 bpm is present; proceed directly to emergency C-section or titrated micro-dose oxytocin if non-obstructive.",
			Completed:   false,
		},
		{
			ID:          StepNeonatalResusColostrumLatch,
			Title:       "Warm Neonatal Airway Suction, Umbilical Ligation & Normocalcemic Colostrum Latch",
			Description: "Clear neonatal oropharyngeal membranes via bulb syringe (avoid swinging), maintain 32°C whelping box warmth, verify maternal normocalcemia (iCa >= 1.1 mmol/L), and confirm oral calcium supplementation & colostrum nursing.",
			Completed:   false,
		},
	}
}

// UpdateProtocolStep toggles completion of a specific protocol step on a dystocia & eclampsia operation.
func UpdateProtocolStep(op *domain.DystociaEclampsiaOperation, stepID string, completed bool) error {
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
		if op.Status == domain.DystociaStatusFetalDistressObstructiveCritical {
			op.Status = domain.DystociaStatusCalciumECGWhelpingStaged
		}
	} else {
		if op.Status == domain.DystociaStatusDeliveredNormocalcemicSafe {
			op.Status = domain.DystociaStatusCalciumECGWhelpingStaged
			op.ClearanceToken = ""
		}
	}

	op.UpdatedAt = nowStr
	return nil
}

// IssueDystociaClearance validates that all 4 protocol steps are completed, transitions the operation
// to DELIVERED_NORMOCALCEMIC_LACTATING_SAFE, and generates a deterministic DYSTOCIA-SAFE-<12HEX> clearance token.
func IssueDystociaClearance(op *domain.DystociaEclampsiaOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if len(op.ProtocolSteps) < 4 {
		return fmt.Errorf("operation must have all 4 protocol steps")
	}
	for _, s := range op.ProtocolSteps {
		if !s.Completed {
			return fmt.Errorf("cannot issue dystocia clearance: step %q (%s) is not completed", s.ID, s.Title)
		}
	}

	seed := fmt.Sprintf("%s|%s|%s|%d|%.2f|%d", op.ID, op.PetName, op.PresentationType, op.LowestFetalHeartRateBpm, op.IonizedCalciumMmolL, op.InterPuppyIntervalMins)
	sum := sha256.Sum256([]byte(seed))
	op.ClearanceToken = fmt.Sprintf("DYSTOCIA-SAFE-%s", strings.ToUpper(hex.EncodeToString(sum[:6])))
	op.Status = domain.DystociaStatusDeliveredNormocalcemicSafe
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Store maintains in-memory dystocia, maternal C-section & eclampsia triage operations with deterministic seeding.
type Store struct {
	mu    sync.RWMutex
	ops   map[string]*domain.DystociaEclampsiaOperation
	order []string
	seq   int
}

// NewStore initializes a Store with 3 deterministic seeded operations (DE-OP-101, DE-OP-102, DE-OP-103).
func NewStore() *Store {
	s := &Store{
		ops:   make(map[string]*domain.DystociaEclampsiaOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	op101, err := EvaluateDystociaTriage(domain.DystociaEvaluationRequest{
		ID:                             "DE-OP-101",
		PetID:                          "PET-DE-101",
		PetName:                        "Penelope",
		Species:                        "Dog - English Bulldog Dam",
		WeightKg:                       23.5,
		PresentationType:               domain.DystociaObstructiveBrachycephalicFetalOversize,
		LowestFetalHeartRateBpm:        154,
		IonizedCalciumMmolL:            1.12,
		InterPuppyIntervalMins:         145,
		GreenBlackUteroverdinNoFetus:   true,
		PelvicCanalObstructionExcluded: false,
		ECGMonitoredCalciumInfusion:    true,
	})
	if err == nil {
		op101.Status = domain.DystociaStatusFetalDistressObstructiveCritical
		op101.CreatedAt = "2026-10-19T01:05:00Z"
		op101.UpdatedAt = "2026-10-19T01:10:00Z"
		s.ops[op101.ID] = op101
		s.order = append(s.order, op101.ID)
	}

	op102, err := EvaluateDystociaTriage(domain.DystociaEvaluationRequest{
		ID:                             "DE-OP-102",
		PetID:                          "PET-DE-102",
		PetName:                        "Maple",
		Species:                        "Dog - Cavalier King Charles Spaniel Dam",
		WeightKg:                       8.2,
		PresentationType:               domain.DystociaPrimaryUterineInertiaNormocalcemic,
		LowestFetalHeartRateBpm:        205,
		IonizedCalciumMmolL:            1.18,
		InterPuppyIntervalMins:         75,
		GreenBlackUteroverdinNoFetus:   false,
		PelvicCanalObstructionExcluded: true,
		ECGMonitoredCalciumInfusion:    true,
	})
	if err == nil {
		_ = UpdateProtocolStep(op102, StepDopplerFetalHRPelvicXray, true)
		_ = UpdateProtocolStep(op102, StepECGCalciumGluconateInfusion, true)
		op102.Status = domain.DystociaStatusCalciumECGWhelpingStaged
		op102.CreatedAt = "2026-10-19T01:15:00Z"
		op102.UpdatedAt = "2026-10-19T01:22:00Z"
		s.ops[op102.ID] = op102
		s.order = append(s.order, op102.ID)
	}

	op103, err := EvaluateDystociaTriage(domain.DystociaEvaluationRequest{
		ID:                             "DE-OP-103",
		PetID:                          "PET-DE-103",
		PetName:                        "Rosie",
		Species:                        "Cat - Ragdoll Queen",
		WeightKg:                       4.6,
		PresentationType:               domain.DystociaPuerperalEclampsiaHypocalcemiaTetany,
		LowestFetalHeartRateBpm:        220,
		IonizedCalciumMmolL:            1.28,
		InterPuppyIntervalMins:         30,
		GreenBlackUteroverdinNoFetus:   false,
		PelvicCanalObstructionExcluded: true,
		ECGMonitoredCalciumInfusion:    true,
	})
	if err == nil {
		for _, step := range op103.ProtocolSteps {
			_ = UpdateProtocolStep(op103, step.ID, true)
		}
		_ = IssueDystociaClearance(op103)
		op103.CreatedAt = "2026-10-19T00:40:00Z"
		op103.UpdatedAt = "2026-10-19T01:00:00Z"
		s.ops[op103.ID] = op103
		s.order = append(s.order, op103.ID)
	}
}

// List returns a deep copy of all operations in deterministic insertion order.
func (s *Store) List() []domain.DystociaEclampsiaOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.DystociaEclampsiaOperation, 0, len(s.order))
	for _, id := range s.order {
		if op, ok := s.ops[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	return out
}

// Get returns a deep copy of an operation by ID.
func (s *Store) Get(id string) (domain.DystociaEclampsiaOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return domain.DystociaEclampsiaOperation{}, false
	}
	return cloneOperation(op), true
}

// EvaluateAndSave evaluates a request and stores the resulting operation.
func (s *Store) EvaluateAndSave(req domain.DystociaEvaluationRequest) (domain.DystociaEclampsiaOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoID := strings.TrimSpace(req.ID) == ""
	if autoID {
		req.ID = fmt.Sprintf("DE-OP-%d", s.seq+1)
	}

	op, err := EvaluateDystociaTriage(req)
	if err != nil {
		return domain.DystociaEclampsiaOperation{}, err
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
func (s *Store) UpdateStep(id string, stepID string, completed bool) (domain.DystociaEclampsiaOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.DystociaEclampsiaOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := UpdateProtocolStep(op, stepID, completed); err != nil {
		return domain.DystociaEclampsiaOperation{}, err
	}
	return cloneOperation(op), nil
}

// IssueClearance issues a normocalcemic delivered clearance token for an operation in the store.
func (s *Store) IssueClearance(id string) (domain.DystociaEclampsiaOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.ops[strings.TrimSpace(id)]
	if !exists {
		return domain.DystociaEclampsiaOperation{}, fmt.Errorf("operation %q not found", id)
	}
	if err := IssueDystociaClearance(op); err != nil {
		return domain.DystociaEclampsiaOperation{}, err
	}
	return cloneOperation(op), nil
}

func cloneOperation(op *domain.DystociaEclampsiaOperation) domain.DystociaEclampsiaOperation {
	cp := *op
	if len(op.ProtocolSteps) > 0 {
		cp.ProtocolSteps = make([]domain.DystociaProtocolStep, len(op.ProtocolSteps))
		copy(cp.ProtocolSteps, op.ProtocolSteps)
	}
	return cp
}
