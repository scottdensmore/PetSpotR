package pneumothorax

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// PneumothoraxMetrics summarizes operational KPI counts across Tension Pneumothorax & Thoracocentesis operations.
type PneumothoraxMetrics struct {
	TotalOperations                int `json:"total_operations"`
	CriticalTensionCount           int `json:"critical_tension_count"`
	ThoracocentesisTubeStagedCount int `json:"thoracocentesis_tube_staged_count"`
	PleuralSealRemovedCount        int `json:"pleural_seal_removed_count"`
	TensionRadiographAlertCount    int `json:"tension_radiograph_alert_count"`
}

// PneumothoraxService manages Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis &
// Heimlich/Pleural Seal Chest Tube operations in a thread-safe in-memory store.
type PneumothoraxService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.PneumothoraxOperation
	order []string
	seq   int
}

// NewPneumothoraxService initializes a PneumothoraxService with 3 seeded operations (PTX-OP-101, PTX-OP-102, PTX-OP-103).
func NewPneumothoraxService() *PneumothoraxService {
	s := &PneumothoraxService{
		ops:   make(map[string]*domain.PneumothoraxOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildDecompressionPlan deterministically computes a PneumothoraxDecompressionPlan for a patient profile.
func BuildDecompressionPlan(profile domain.PneumothoraxPatientProfile) *domain.PneumothoraxDecompressionPlan {
	expectedTapMl := round2(30.0 * profile.WeightKg)
	tubeIndicated := profile.CumulativeAirAspiratedMlKg > 40.0 ||
		profile.TapCountLast24Hr >= 3 ||
		profile.OpenSuckingChestWound

	var bloodPatchMlKg float64
	if strings.EqualFold(profile.Species, domain.PneumothoraxSpeciesCanineSpontaneousBullae) {
		bloodPatchMlKg = 5.0
	}

	pitfallAlert := "CRITICAL TENSION PNEUMOTHORAX & INTERCOSTAL VESSEL PITFALL ALERT: NEVER take thoracic radiographs before needle thoracocentesis in tension pneumothorax (restraint on the X-ray table precipitates fatal obstructive shock cardiac arrest); NEVER insert the needle along the caudal margin of the rib (which lacerates the intercostal artery/vein/nerve bundle — always advance along the CRANIAL rib margin at the 7th–9th ICS dorsal third); and NEVER initiate positive-pressure ventilation before pleural decompression."

	rationaleParts := []string{
		fmt.Sprintf("7th-9th ICS Dorsal Third Thoracocentesis along CRANIAL rib margin (Expected Tap Volume: %.1f mL at 30 mL/kg for %.1f kg %s).", expectedTapMl, profile.WeightKg, profile.Species),
		fmt.Sprintf("Cumulative Air Aspirated: %.1f mL/kg across %d taps/24h; Thoracostomy Chest Tube Indicated: %t; Continuous Pleural Suction: -15 cmH2O.", profile.CumulativeAirAspiratedMlKg, profile.TapCountLast24Hr, tubeIndicated),
	}
	if bloodPatchMlKg > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Autologous Blood Patch Pleurodesis: %.1f mL/kg (%.1f mL non-anticoagulated autologous venous blood instilled via chest tube for refractory spontaneous bullae leak).", bloodPatchMlKg, round2(bloodPatchMlKg*profile.WeightKg)))
	}

	return &domain.PneumothoraxDecompressionPlan{
		NeedleLandmarkIntercostalSpace:      "7th-9th ICS Dorsal Third",
		CranialRibMarginVesselGuard:         true,
		ExpectedNegativePressureTapMl:       expectedTapMl,
		ThoracostomyTubeIndicated:           tubeIndicated,
		ContinuousSuctionCmH2O:              -15,
		AutologousBloodPatchMlKg:            bloodPatchMlKg,
		ZeroPreDecompressionRadiographGuard: true,
		PneumothoraxPitfallAlert:            pitfallAlert,
		ClinicalRationale:                   strings.Join(rationaleParts, " "),
	}
}

func needsTensionRadiographDelayAlert(p domain.PneumothoraxPatientProfile) bool {
	return !p.TfastGlideSignPresent ||
		p.SpO2Percent < 94.0 ||
		p.RespiratoryRateBpm > 45 ||
		p.CumulativeAirAspiratedMlKg > 20.0 ||
		p.TapCountLast24Hr >= 2 ||
		p.OpenSuckingChestWound
}

func (s *PneumothoraxService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.PneumothoraxPatientProfile{
		Species:                    domain.PneumothoraxSpeciesCanineSpontaneousBullae,
		Breed:                      "Siberian Husky",
		WeightKg:                   25.0,
		RespiratoryRateBpm:         76,
		SpO2Percent:                83.0,
		TfastGlideSignPresent:      false,
		CumulativeAirAspiratedMlKg: 62.0,
		TapCountLast24Hr:           3,
		OpenSuckingChestWound:      false,
	}
	op101 := &domain.PneumothoraxOperation{
		ID:                          "PTX-OP-101",
		IncidentID:                  "INC-PTX-9911",
		PetName:                     "Maverick",
		Status:                      domain.PneumothoraxStatusTensionAbsentGlideCritical,
		TensionRadiographDelayAlert: true,
		PatientProfile:              p101,
		DecompressionPlan:           BuildDecompressionPlan(p101),
		DrainageChecks: []domain.PleuralDrainageSerialCheck{
			{
				Timestamp:                    t101.Add(10 * time.Minute),
				AirAspiratedRateMlKgHr:       22.0,
				FluidAspiratedRateMlKgDay:    3.5,
				TfastGlideSignRestored:       false,
				SpO2Percent:                  83.0,
				SubcutaneousEmphysemaPresent: true,
				ClinicianCallsign:            "DR-PULM-11",
				Notes:                        "Siberian Husky with spontaneous pulmonary bullae rupture; absent TFAST glide sign bilaterally, barrel chest, SpO2 83%; 1,550 mL air aspirated over 3 taps (62 mL/kg); immediate 7th-9th ICS cranial rib margin decompression & MILA guidewire thoracostomy tube staged.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 20, 0, 0, time.UTC)
	p102 := domain.PneumothoraxPatientProfile{
		Species:                    domain.PneumothoraxSpeciesFelineTraumaticPneumothorax,
		Breed:                      "Domestic Shorthair",
		WeightKg:                   4.2,
		RespiratoryRateBpm:         52,
		SpO2Percent:                92.0,
		TfastGlideSignPresent:      true,
		CumulativeAirAspiratedMlKg: 28.0,
		TapCountLast24Hr:           2,
		OpenSuckingChestWound:      false,
	}
	op102 := &domain.PneumothoraxOperation{
		ID:                          "PTX-OP-102",
		IncidentID:                  "INC-PTX-9912",
		PetName:                     "Bandit",
		Status:                      domain.PneumothoraxStatusThoracocentesisTubeStaged,
		TensionRadiographDelayAlert: true,
		PatientProfile:              p102,
		DecompressionPlan:           BuildDecompressionPlan(p102),
		DrainageChecks: []domain.PleuralDrainageSerialCheck{
			{
				Timestamp:                    t102.Add(15 * time.Minute),
				AirAspiratedRateMlKgHr:       4.5,
				FluidAspiratedRateMlKgDay:    1.8,
				TfastGlideSignRestored:       true,
				SpO2Percent:                  92.0,
				SubcutaneousEmphysemaPresent: false,
				ClinicianCallsign:            "DR-ECC-24",
				Notes:                        "Feline blunt thoracic trauma; 118 mL air aspirated at 8th ICS dorsal third along cranial rib margin; TFAST glide sign restored on left hemithorax.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC)
	p103 := domain.PneumothoraxPatientProfile{
		Species:                    domain.PneumothoraxSpeciesCanineTraumaticFlailChest,
		Breed:                      "Australian Shepherd",
		WeightKg:                   18.0,
		RespiratoryRateBpm:         24,
		SpO2Percent:                98.0,
		TfastGlideSignPresent:      true,
		CumulativeAirAspiratedMlKg: 0.0,
		TapCountLast24Hr:           1,
		OpenSuckingChestWound:      false,
	}
	clearance103 := &domain.PneumothoraxRemovalClearance{
		ClearedAt:                   t103.Add(30 * time.Hour),
		ZeroAirProductionHours:      28,
		FinalFluidOutputMlKgDay:     1.1,
		BilateralGlideSignConfirmed: true,
		RoomAirSpO2Percent:          98.0,
		ChestTubeAsepticallyRemoved: true,
		VetLicense:                  "VET-CO-99108",
		Summary:                     "Zero air production for 28 hours; serosanguinous fluid output 1.1 mL/kg/day (<= 2.0 mL/kg/day); bilateral TFAST glide sign confirmed; room-air SpO2 98%; thoracostomy tube aseptically removed with purse-string closure.",
	}
	op103 := &domain.PneumothoraxOperation{
		ID:                          "PTX-OP-103",
		IncidentID:                  "INC-PTX-9913",
		PetName:                     "Tango",
		Status:                      domain.PneumothoraxStatusPleuralSealTubeRemoved,
		TensionRadiographDelayAlert: false,
		PatientProfile:              p103,
		DecompressionPlan:           BuildDecompressionPlan(p103),
		DrainageChecks: []domain.PleuralDrainageSerialCheck{
			{
				Timestamp:                    t103.Add(24 * time.Hour),
				AirAspiratedRateMlKgHr:       0.0,
				FluidAspiratedRateMlKgDay:    1.1,
				TfastGlideSignRestored:       true,
				SpO2Percent:                  98.0,
				SubcutaneousEmphysemaPresent: false,
				ClinicianCallsign:            "DR-SURG-07",
				Notes:                        "28h zero air aspiration; intercostal bupivacaine nerve block resolved paradoxical flail segment pain; bilateral TFAST glide sign intact.",
			},
		},
		RemovalClearance: clearance103,
		CreatedAt:        t103,
		UpdatedAt:        t103.Add(30 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *PneumothoraxService) ListOperations(statusFilter string) []domain.PneumothoraxOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.PneumothoraxOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, clonePneumothoraxOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *PneumothoraxService) GetOperation(id string) (*domain.PneumothoraxOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("pneumothorax operation %q not found", id)
	}
	cp := clonePneumothoraxOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Tension Pneumothorax & Thoracocentesis triage operation.
func (s *PneumothoraxService) CreateOperation(incidentID, petName string, profile domain.PneumothoraxPatientProfile) (*domain.PneumothoraxOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("PTX-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsTensionRadiographDelayAlert(profile)
	plan := BuildDecompressionPlan(profile)

	op := &domain.PneumothoraxOperation{
		ID:                          opID,
		IncidentID:                  trimmedIncident,
		PetName:                     trimmedPet,
		Status:                      domain.PneumothoraxStatusTensionAbsentGlideCritical,
		TensionRadiographDelayAlert: alert,
		PatientProfile:              profile,
		DecompressionPlan:           plan,
		DrainageChecks:              []domain.PleuralDrainageSerialCheck{},
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := clonePneumothoraxOperation(op)
	return &cp, nil
}

// StageDecompressionPlan computes the 7th-9th ICS dorsal third needle landmark, cranial rib margin guard,
// expected negative pressure tap volume (30 mL/kg), thoracostomy chest tube indication, continuous suction (-15 cmH2O),
// and autologous blood patch pleurodesis dose, transitioning the operation to ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED.
func (s *PneumothoraxService) StageDecompressionPlan(id string, profile domain.PneumothoraxPatientProfile) (*domain.PneumothoraxOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pneumothorax operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.DecompressionPlan = BuildDecompressionPlan(profile)
	op.TensionRadiographDelayAlert = needsTensionRadiographDelayAlert(profile)
	op.Status = domain.PneumothoraxStatusThoracocentesisTubeStaged
	op.RemovalClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := clonePneumothoraxOperation(op)
	return &cp, nil
}

// RecordDrainageCheck appends a serial pleural air/fluid drainage, TFAST glide sign, and SpO2 check.
func (s *PneumothoraxService) RecordDrainageCheck(id string, check domain.PleuralDrainageSerialCheck) (*domain.PneumothoraxOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pneumothorax operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.DrainageChecks = append(op.DrainageChecks, check)
	op.PatientProfile.TfastGlideSignPresent = check.TfastGlideSignRestored
	op.PatientProfile.SpO2Percent = check.SpO2Percent
	op.DecompressionPlan = BuildDecompressionPlan(op.PatientProfile)

	if !check.TfastGlideSignRestored || check.SpO2Percent < 88.0 || check.AirAspiratedRateMlKgHr >= 20.0 {
		op.TensionRadiographDelayAlert = true
		if !check.TfastGlideSignRestored || check.SpO2Percent < 86.0 {
			op.Status = domain.PneumothoraxStatusTensionAbsentGlideCritical
			op.RemovalClearance = nil
		}
	} else if check.TfastGlideSignRestored && check.SpO2Percent >= 95.0 && check.AirAspiratedRateMlKgHr == 0.0 && !check.SubcutaneousEmphysemaPresent {
		op.TensionRadiographDelayAlert = false
	}

	op.UpdatedAt = now
	cp := clonePneumothoraxOperation(op)
	return &cp, nil
}

// IssueRemovalClearance validates >= 24h zero air production, fluid output <= 2.0 mL/kg/day,
// bilateral TFAST glide sign confirmation, room-air SpO2 >= 95%, and aseptic chest tube removal,
// transitioning the operation to PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED.
func (s *PneumothoraxService) IssueRemovalClearance(id string, clearance domain.PneumothoraxRemovalClearance) (*domain.PneumothoraxOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pneumothorax operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Zero air production for %d hours; fluid output %.1f mL/kg/day (<= 2.0 mL/kg/day); bilateral TFAST glide sign confirmed; room-air SpO2 %.1f%%; thoracostomy chest tube aseptically removed.", clearance.ZeroAirProductionHours, clearance.FinalFluidOutputMlKgDay, clearance.RoomAirSpO2Percent)
	}

	op.RemovalClearance = &clearance
	op.Status = domain.PneumothoraxStatusPleuralSealTubeRemoved
	op.TensionRadiographDelayAlert = false
	op.PatientProfile.TfastGlideSignPresent = true
	op.PatientProfile.SpO2Percent = clearance.RoomAirSpO2Percent
	op.DecompressionPlan = BuildDecompressionPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := clonePneumothoraxOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all pneumothorax operations in the service.
func (s *PneumothoraxService) ComputeMetrics() PneumothoraxMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m PneumothoraxMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.PneumothoraxStatusTensionAbsentGlideCritical:
			m.CriticalTensionCount++
		case domain.PneumothoraxStatusThoracocentesisTubeStaged:
			m.ThoracocentesisTubeStagedCount++
		case domain.PneumothoraxStatusPleuralSealTubeRemoved:
			m.PleuralSealRemovedCount++
		}
		if op.TensionRadiographDelayAlert {
			m.TensionRadiographAlertCount++
		}
	}
	return m
}

func clonePneumothoraxOperation(op *domain.PneumothoraxOperation) domain.PneumothoraxOperation {
	cp := *op
	if op.DecompressionPlan != nil {
		plan := *op.DecompressionPlan
		cp.DecompressionPlan = &plan
	}
	if op.DrainageChecks != nil {
		cp.DrainageChecks = make([]domain.PleuralDrainageSerialCheck, len(op.DrainageChecks))
		copy(cp.DrainageChecks, op.DrainageChecks)
	}
	if op.RemovalClearance != nil {
		clr := *op.RemovalClearance
		cp.RemovalClearance = &clr
	}
	return cp
}
