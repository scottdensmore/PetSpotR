package pericardialtamponade

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TamponadeMetrics summarizes operational KPI counts across Pericardial Effusion & Cardiac Tamponade operations.
type TamponadeMetrics struct {
	TotalOperations                    int `json:"total_operations"`
	CriticalTamponadeCount             int `json:"critical_tamponade_count"`
	RightICS45TapStagedCount           int `json:"right_ics_4_5_tap_staged_count"`
	DecompressedSinusClearedCount      int `json:"decompressed_sinus_cleared_count"`
	FurosemideLeftCoronaryHazardAlerts int `json:"furosemide_left_coronary_hazard_alerts"`
}

// TamponadeService manages Pericardial Effusion, Cardiac Tamponade, Electrical Alternans
// & Right-Sided 4th–5th ICS Pericardiocentesis operations in a thread-safe in-memory store.
type TamponadeService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.PericardialTamponadeOperation
	order []string
	seq   int
}

// NewTamponadeService initializes a TamponadeService with 3 seeded operations (PT-OP-101, PT-OP-102, PT-OP-103).
func NewTamponadeService() *TamponadeService {
	s := &TamponadeService{
		ops:   make(map[string]*domain.PericardialTamponadeOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildPericardiocentesisPlan deterministically computes a PericardiocentesisPlan for a patient profile.
func BuildPericardiocentesisPlan(profile domain.TamponadePatientProfile) *domain.PericardiocentesisPlan {
	lidocaineInfiltMl := round2(0.1 * profile.WeightKg)
	if lidocaineInfiltMl > 4.0 {
		lidocaineInfiltMl = 4.0
	}
	lidocaineIVMg := round1(2.0 * profile.WeightKg)
	crystalloidBolusMl := round1(10.0 * profile.WeightKg)

	pitfallAlert := "CRITICAL PERICARDIAL TAMPONADE, FUROSEMIDE PRELOAD & LEFT CORONARY ARTERY PITFALL ALERT: NEVER administer Furosemide or diuretics in cardiac tamponade (diuretics deplete right ventricular preload and precipitate immediate fatal cardiovascular collapse); NEVER perform left-sided pericardiocentesis (which risks lacerating the left paraconal interventricular coronary artery — always use the RIGHT 4th–5th ICS cardiac notch); and ALWAYS test the first aspirated aliquot in a red-top clot tube to confirm non-clotting defibrinated fluid and rule out iatrogenic intracardiac ventricular puncture."

	rationaleParts := []string{
		fmt.Sprintf("Approach: Right 4th-5th ICS Cardiac Notch (Cranial to 5th Rib at Costochondral Junction) for %.1f kg %s.", profile.WeightKg, profile.Species),
		fmt.Sprintf("Preload Support: %.1f mL Isotonic Crystalloid (10 mL/kg) with strict ZERO Furosemide/Diuretic Guard (HR %d bpm, MAP %d mmHg, Pulsus Paradoxus %d mmHg).", crystalloidBolusMl, profile.HeartRateBpm, profile.MeanArterialPressureMmHg, profile.PulsusParadoxusMmHg),
		fmt.Sprintf("Local & Antiarrhythmic Prep: 2%% Lidocaine %.2f mL intercostal block + IV Lidocaine %.1f mg (2.0 mg/kg) drawn up on standby for catheter-induced VPCs + Red-Top Clot Tube non-clotting verification (Effusion PCV %.1f%%).", lidocaineInfiltMl, lidocaineIVMg, profile.EffusionPCVPercent),
	}

	return &domain.PericardiocentesisPlan{
		ApproachSideAndICS:              "Right 4th-5th ICS Cardiac Notch",
		CostochondralJunctionLandmark:   "Cranial to 5th Rib at Costochondral Junction",
		Lidocaine2PctInfiltrationMl:     lidocaineInfiltMl,
		LidocaineIV2MgKgArrhythmiaMg:    lidocaineIVMg,
		CrystalloidPreloadExpansionMl:   crystalloidBolusMl,
		ClotTubeIatrogenicPunctureCheck: true,
		ZeroFurosemideDiureticGuard:     true,
		TamponadePitfallAlert:           pitfallAlert,
		ClinicalRationale:               strings.Join(rationaleParts, " "),
	}
}

func needsTamponadeHazardAlert(p domain.TamponadePatientProfile) bool {
	return p.RightAtrialDiastolicCollapse ||
		p.ElectricalAlternansPresent ||
		p.MeanArterialPressureMmHg < 75 ||
		p.HeartRateBpm > 140 ||
		p.PulsusParadoxusMmHg >= 10
}

func (s *TamponadeService) seed() {
	t101 := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)
	p101 := domain.TamponadePatientProfile{
		Species:                      domain.TamponadeSpeciesCanineRAHemangiosarcoma,
		Breed:                        "Golden Retriever",
		WeightKg:                     33.0,
		HeartRateBpm:                 185,
		MeanArterialPressureMmHg:     52,
		RightAtrialDiastolicCollapse: true,
		ElectricalAlternansPresent:   true,
		PulsusParadoxusMmHg:          18,
		EffusionPCVPercent:           28.0,
	}
	op101 := &domain.PericardialTamponadeOperation{
		ID:                                "PT-OP-101",
		IncidentID:                        "INC-PT-9921",
		PetName:                           "Brutus",
		Status:                            domain.TamponadeStatusRADiastolicCollapseCritical,
		FurosemideLeftCorpnaryHazardAlert: true,
		PatientProfile:                    p101,
		CentresisPlan:                     BuildPericardiocentesisPlan(p101),
		TapChecks: []domain.PericardiocentesisTapCheck{
			{
				Timestamp:                   t101.Add(10 * time.Minute),
				VolumeAspiratedMl:           45.0,
				SampleNonClottingVerified:   true,
				VentricularArrhythmiasNoted: false,
				PostTapMAPMmHg:              58,
				PostTapHeartRateBpm:         172,
				ClinicianCallsign:           "DR-CARDIO-11",
				Notes:                       "Initial right 4th-5th ICS diagnostic aliquot (45 mL port-wine hemorrhagic fluid, PCV 28%); verified non-clotting in red-top tube; electrical alternans and RA diastolic collapse ongoing.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 2, 20, 0, 0, time.UTC)
	p102 := domain.TamponadePatientProfile{
		Species:                      domain.TamponadeSpeciesCanineHeartBaseChemodectoma,
		Breed:                        "French Bulldog",
		WeightKg:                     13.5,
		HeartRateBpm:                 145,
		MeanArterialPressureMmHg:     72,
		RightAtrialDiastolicCollapse: true,
		ElectricalAlternansPresent:   false,
		PulsusParadoxusMmHg:          12,
		EffusionPCVPercent:           22.0,
	}
	op102 := &domain.PericardialTamponadeOperation{
		ID:                                "PT-OP-102",
		IncidentID:                        "INC-PT-9922",
		PetName:                           "Mugsy",
		Status:                            domain.TamponadeStatusRightICS45TapStaged,
		FurosemideLeftCorpnaryHazardAlert: true,
		PatientProfile:                    p102,
		CentresisPlan:                     BuildPericardiocentesisPlan(p102),
		TapChecks: []domain.PericardiocentesisTapCheck{
			{
				Timestamp:                   t102.Add(15 * time.Minute),
				VolumeAspiratedMl:           95.0,
				SampleNonClottingVerified:   true,
				VentricularArrhythmiasNoted: false,
				PostTapMAPMmHg:              78,
				PostTapHeartRateBpm:         132,
				ClinicianCallsign:           "DR-ECC-24",
				Notes:                       "French Bulldog with heart-base chemodectoma; 95 mL non-clotting serosanguinous fluid aspirated via right 4th-5th ICS cardiac notch; Lidocaine 27.0 mg IV on standby.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	p103 := domain.TamponadePatientProfile{
		Species:                      domain.TamponadeSpeciesCanineIdiopathicPericarditis,
		Breed:                        "Saint Bernard",
		WeightKg:                     58.0,
		HeartRateBpm:                 92,
		MeanArterialPressureMmHg:     96,
		RightAtrialDiastolicCollapse: false,
		ElectricalAlternansPresent:   false,
		PulsusParadoxusMmHg:          4,
		EffusionPCVPercent:           18.0,
	}
	clearance103 := &domain.TamponadeDischargeClearance{
		ClearedAt:                          t103.Add(26 * time.Hour),
		FinalMAPMmHg:                       96,
		RightAtrialCollapseResolved:        true,
		RepeatTfast24HrNoReaccumulation:    true,
		SinusRhythmNoVPCs:                  true,
		YunnanBaiyaoOrOncologyPlanEducated: true,
		VetLicense:                         "VET-CARDIO-99204",
		Summary:                            "Pericardial sac decompressed (420 mL aspirated); right atrial diastolic collapse and electrical alternans resolved; 24h repeat TFAST negative for reaccumulation; sinus rhythm 92 bpm without VPCs.",
	}
	op103 := &domain.PericardialTamponadeOperation{
		ID:                                "PT-OP-103",
		IncidentID:                        "INC-PT-9923",
		PetName:                           "Samson",
		Status:                            domain.TamponadeStatusDecompressedSinusCleared,
		FurosemideLeftCorpnaryHazardAlert: false,
		PatientProfile:                    p103,
		CentresisPlan:                     BuildPericardiocentesisPlan(p103),
		TapChecks: []domain.PericardiocentesisTapCheck{
			{
				Timestamp:                   t103.Add(2 * time.Hour),
				VolumeAspiratedMl:           420.0,
				SampleNonClottingVerified:   true,
				VentricularArrhythmiasNoted: false,
				PostTapMAPMmHg:              96,
				PostTapHeartRateBpm:         92,
				ClinicianCallsign:           "DR-CARDIO-07",
				Notes:                       "Complete right 4th-5th ICS pericardiocentesis (420 mL non-clotting effusion); immediate resolution of right atrial collapse and restoration of normal QRS amplitude.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(26 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *TamponadeService) ListOperations(statusFilter string) []domain.PericardialTamponadeOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.PericardialTamponadeOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneTamponadeOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *TamponadeService) GetOperation(id string) (*domain.PericardialTamponadeOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("pericardial tamponade operation %q not found", id)
	}
	cp := cloneTamponadeOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Pericardial Effusion & Cardiac Tamponade triage operation.
func (s *TamponadeService) CreateOperation(incidentID, petName string, profile domain.TamponadePatientProfile) (*domain.PericardialTamponadeOperation, error) {
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
	opID := fmt.Sprintf("PT-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsTamponadeHazardAlert(profile)
	plan := BuildPericardiocentesisPlan(profile)

	op := &domain.PericardialTamponadeOperation{
		ID:                                opID,
		IncidentID:                        trimmedIncident,
		PetName:                           trimmedPet,
		Status:                            domain.TamponadeStatusRADiastolicCollapseCritical,
		FurosemideLeftCorpnaryHazardAlert: alert,
		PatientProfile:                    profile,
		CentresisPlan:                     plan,
		TapChecks:                         []domain.PericardiocentesisTapCheck{},
		CreatedAt:                         now,
		UpdatedAt:                         now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneTamponadeOperation(op)
	return &cp, nil
}

// StagePericardiocentesisPlan computes the Right 4th–5th ICS cardiac notch approach,
// cranial to 5th rib at costochondral junction landmark, 2% Lidocaine infiltration,
// IV Lidocaine 2 mg/kg arrhythmia standby, and 10 mL/kg crystalloid preload expansion,
// transitioning the operation to RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED.
func (s *TamponadeService) StagePericardiocentesisPlan(id string, profile domain.TamponadePatientProfile) (*domain.PericardialTamponadeOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pericardial tamponade operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.CentresisPlan = BuildPericardiocentesisPlan(profile)
	op.FurosemideLeftCorpnaryHazardAlert = needsTamponadeHazardAlert(profile)
	op.Status = domain.TamponadeStatusRightICS45TapStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneTamponadeOperation(op)
	return &cp, nil
}

// RecordTapCheck appends a pericardiocentesis tap aspiration, non-clotting clot-tube check,
// and post-tap hemodynamic response.
func (s *TamponadeService) RecordTapCheck(id string, check domain.PericardiocentesisTapCheck) (*domain.PericardialTamponadeOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pericardial tamponade operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.TapChecks = append(op.TapChecks, check)
	op.PatientProfile.MeanArterialPressureMmHg = check.PostTapMAPMmHg
	op.PatientProfile.HeartRateBpm = check.PostTapHeartRateBpm
	op.CentresisPlan = BuildPericardiocentesisPlan(op.PatientProfile)

	if check.PostTapMAPMmHg < 65 || check.PostTapHeartRateBpm > 170 || check.VentricularArrhythmiasNoted {
		op.FurosemideLeftCorpnaryHazardAlert = true
		if check.PostTapMAPMmHg < 60 || check.PostTapHeartRateBpm > 175 {
			op.Status = domain.TamponadeStatusRADiastolicCollapseCritical
			op.DischargeClearance = nil
		}
	} else if check.PostTapMAPMmHg >= 80 && check.PostTapHeartRateBpm <= 130 && !check.VentricularArrhythmiasNoted {
		op.PatientProfile.RightAtrialDiastolicCollapse = false
		op.PatientProfile.ElectricalAlternansPresent = false
		op.FurosemideLeftCorpnaryHazardAlert = false
	}

	op.UpdatedAt = now
	cp := cloneTamponadeOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates resolved right atrial collapse, MAP 75–120 mmHg,
// 24h repeat TFAST negative for reaccumulation, sinus rhythm without VPCs, and oncology/Yunnan Baiyao education,
// transitioning the operation to DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED.
func (s *TamponadeService) IssueDischargeClearance(id string, clearance domain.TamponadeDischargeClearance) (*domain.PericardialTamponadeOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pericardial tamponade operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Right 4th-5th ICS pericardiocentesis decompression complete (Final MAP %d mmHg); right atrial diastolic collapse and electrical alternans resolved; 24h TFAST negative for reaccumulation; Yunnan Baiyao / oncology plan reviewed.", clearance.FinalMAPMmHg)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.TamponadeStatusDecompressedSinusCleared
	op.FurosemideLeftCorpnaryHazardAlert = false
	op.PatientProfile.MeanArterialPressureMmHg = clearance.FinalMAPMmHg
	op.PatientProfile.RightAtrialDiastolicCollapse = false
	op.PatientProfile.ElectricalAlternansPresent = false
	op.CentresisPlan = BuildPericardiocentesisPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneTamponadeOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all pericardial tamponade operations in the service.
func (s *TamponadeService) ComputeMetrics() TamponadeMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m TamponadeMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.TamponadeStatusRADiastolicCollapseCritical:
			m.CriticalTamponadeCount++
		case domain.TamponadeStatusRightICS45TapStaged:
			m.RightICS45TapStagedCount++
		case domain.TamponadeStatusDecompressedSinusCleared:
			m.DecompressedSinusClearedCount++
		}
		if op.FurosemideLeftCorpnaryHazardAlert {
			m.FurosemideLeftCoronaryHazardAlerts++
		}
	}
	return m
}

func cloneTamponadeOperation(op *domain.PericardialTamponadeOperation) domain.PericardialTamponadeOperation {
	cp := *op
	if op.CentresisPlan != nil {
		plan := *op.CentresisPlan
		cp.CentresisPlan = &plan
	}
	if op.TapChecks != nil {
		cp.TapChecks = make([]domain.PericardiocentesisTapCheck, len(op.TapChecks))
		copy(cp.TapChecks, op.TapChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
