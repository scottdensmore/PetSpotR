package hemoabdomen

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// HemoabdomenMetrics summarizes operational KPI counts across hemoabdomen & autotransfusion triage operations.
type HemoabdomenMetrics struct {
	TotalOperations                int `json:"total_operations"`
	HemorrhagicShockCriticalCount  int `json:"hemorrhagic_shock_critical_count"`
	AutotransfusionTXAStagedCount  int `json:"autotransfusion_txa_staged_count"`
	HemostasisClearedCount         int `json:"hemostasis_cleared_count"`
	ClotPoppingNeoplasiaAlertCount int `json:"clot_popping_neoplasia_alert_count"`
}

// HemoabdomenService manages Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting
// & Norepinephrine/TXA Resuscitation operations in a thread-safe in-memory store.
type HemoabdomenService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.HemoabdomenOperation
	order []string
	seq   int
}

// NewHemoabdomenService initializes a HemoabdomenService with 3 seeded operations (HA-OP-101, HA-OP-102, HA-OP-103).
func NewHemoabdomenService() *HemoabdomenService {
	s := &HemoabdomenService{
		ops:   make(map[string]*domain.HemoabdomenOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildResuscitationPlan deterministically computes a HemoabdomenResuscitationPlan for a patient profile.
func BuildResuscitationPlan(profile domain.HemoabdomenPatientProfile) *domain.HemoabdomenResuscitationPlan {
	targetMAP := 65
	crystalloidAliquot := 10.0
	txaDoseMg := round2(profile.WeightKg * 10.0)

	autotransfusionIndicated := !profile.SuspectedNeoplasia &&
		profile.Species != domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma &&
		profile.EffusionNonClottingVerified &&
		profile.EffusionPCVPercent >= 25.0

	var maxCollectMl float64
	if autotransfusionIndicated {
		maxCollectMl = round2(profile.WeightKg * 15.0)
	}

	var norepiCRI float64
	if profile.MeanArterialPressureMmHg < 55 {
		norepiCRI = 0.1
	}

	pitfallAlert := "CRITICAL HEMOABDOMEN RESUSCITATION PITFALL ALERT: NEVER administer full-shock 90 mL/kg crystalloid boluses (pops nascent clots via hydrostatic hypertension and dilutional coagulopathy); enforce ZERO compressive abdominal wraps (impairs diaphragmatic excursion, caudal vena cava return, and splanchnic perfusion); and NEVER autotransfuse unverified neoplastic or hollow-viscus-contaminated effusions or infuse shed blood without an inline 170–260 µm blood filter."

	rationaleParts := []string{
		fmt.Sprintf("Permissive hypotensive resuscitation targeting MAP %d mmHg using conservative %.1f mL/kg isotonic crystalloid aliquots + Tranexamic Acid (TXA) 10 mg/kg IV slow over 15 min (%.1f mg antifibrinolytic bolus) + Zero Abdominal Compression Wrap Guard enforced.", targetMAP, crystalloidAliquot, txaDoseMg),
	}
	if autotransfusionIndicated {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Autologous cell salvage / autotransfusion INDICATED (non-clotting traumatic effusion PCV %.1f%% vs peripheral PCV %.1f%%, AFS %d/4): collect up to %.1f mL (15 mL/kg) aseptically and reinfuse strictly through an inline 170 µm (170–260 µm) blood administration filter.", profile.EffusionPCVPercent, profile.PeripheralPCVPercent, profile.AfastAFS4Score, maxCollectMl))
	} else if profile.SuspectedNeoplasia || profile.Species == domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma {
		rationaleParts = append(rationaleParts, "Autotransfusion CONTRAINDICATED due to suspected splenic hemangiosarcoma / abdominal neoplasia (risk of hematogenous metastatic seeding); use crossmatched allogeneic DEA 1.1-negative pRBCs + FFP prior to emergency splenectomy.")
	} else {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Autotransfusion deferred (effusion PCV %.1f%%, non-clotting verified=%t); monitor serial AFAST AFS score and peripheral PCV/TS.", profile.EffusionPCVPercent, profile.EffusionNonClottingVerified))
	}
	if norepiCRI > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Refractory hemorrhagic hypotension (MAP %d mmHg < 55 mmHg): initiate Norepinephrine vasopressor CRI at %.2f mcg/kg/min alongside blood product volume restoration.", profile.MeanArterialPressureMmHg, norepiCRI))
	}

	return &domain.HemoabdomenResuscitationPlan{
		PermissiveHypotensionTargetMAPMmHg: targetMAP,
		CrystalloidAliquotMlKg:             crystalloidAliquot,
		TranexamicAcid10MgKgDoseMg:         txaDoseMg,
		AutotransfusionIndicated:           autotransfusionIndicated,
		AutotransfusionMaxCollectMl:        maxCollectMl,
		MicronFilterSizeUm:                 170,
		NorepinephrineCRIMcgKgMin:          norepiCRI,
		ZeroAbdominalCompressionWrapGuard:  true,
		HemoabdomenPitfallAlert:            pitfallAlert,
		ClinicalRationale:                  strings.Join(rationaleParts, " "),
	}
}

func computeClotPoppingNeoplasiaAlert(profile domain.HemoabdomenPatientProfile) bool {
	return profile.MeanArterialPressureMmHg < 65 ||
		profile.PlasmaLactateMmolL >= 4.0 ||
		profile.AfastAFS4Score >= 3 ||
		profile.SuspectedNeoplasia
}

func (s *HemoabdomenService) seed() {
	t101 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p101 := domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineBluntTraumaHepatic,
		Breed:                       "German Shepherd",
		WeightKg:                    34.0,
		AfastAFS4Score:              4,
		EffusionPCVPercent:          38.0,
		PeripheralPCVPercent:        32.0,
		MeanArterialPressureMmHg:    48,
		PlasmaLactateMmolL:          7.4,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          false,
	}
	op101 := &domain.HemoabdomenOperation{
		ID:                        "HA-OP-101",
		IncidentID:                "INC-HA-9401",
		PetName:                   "Apollo",
		Status:                    domain.HemoabdomenStatusHemorrhagicShockCritical,
		ClotPoppingNeoplasiaAlert: true,
		PatientProfile:            p101,
		ResuscitationPlan:         BuildResuscitationPlan(p101),
		HemodynamicChecks: []domain.HemoabdomenHemodynamicCheck{
			{
				Timestamp:                t101.Add(5 * time.Minute),
				MeanArterialPressureMmHg: 48,
				HeartRateBpm:             168,
				PeripheralPCVPercent:     32.0,
				TotalSolidsGdl:           4.6,
				PlasmaLactateMmolL:       7.4,
				AutotransfusedVolumeMl:   180.0,
				ClinicianCallsign:        "DR-TRAUMA-14",
				Notes:                    "Blunt vehicular hepatic fracture hemoabdomen (AFAST AFS 4/4); non-clotting centesis PCV 38%; TXA 340 mg IV + autologous cell salvage via 170 µm filter + Norepinephrine 0.1 mcg/kg/min initiated.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(5 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 1, 15, 0, 0, time.UTC)
	p102 := domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma,
		Breed:                       "Golden Retriever",
		WeightKg:                    29.5,
		AfastAFS4Score:              3,
		EffusionPCVPercent:          34.0,
		PeripheralPCVPercent:        28.0,
		MeanArterialPressureMmHg:    64,
		PlasmaLactateMmolL:          4.1,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          true,
	}
	op102 := &domain.HemoabdomenOperation{
		ID:                        "HA-OP-102",
		IncidentID:                "INC-HA-9402",
		PetName:                   "Sadie",
		Status:                    domain.HemoabdomenStatusAutotransfusionTXAStaged,
		ClotPoppingNeoplasiaAlert: true,
		PatientProfile:            p102,
		ResuscitationPlan:         BuildResuscitationPlan(p102),
		HemodynamicChecks: []domain.HemoabdomenHemodynamicCheck{
			{
				Timestamp:                t102.Add(10 * time.Minute),
				MeanArterialPressureMmHg: 64,
				HeartRateBpm:             142,
				PeripheralPCVPercent:     28.0,
				TotalSolidsGdl:           5.2,
				PlasmaLactateMmolL:       4.1,
				AutotransfusedVolumeMl:   0.0,
				ClinicianCallsign:        "DR-SURG-22",
				Notes:                    "Ruptured cavitated splenic mass on AFAST (AFS 3/4); autotransfusion withheld due to suspected hemangiosarcoma; TXA 295 mg IV + DEA 1.1-negative pRBCs staged for emergency splenectomy.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(10 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)
	p103 := domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineBluntTraumaHepatic,
		Breed:                       "Labrador Retriever",
		WeightKg:                    26.0,
		AfastAFS4Score:              0,
		EffusionPCVPercent:          0.0,
		PeripheralPCVPercent:        31.0,
		MeanArterialPressureMmHg:    82,
		PlasmaLactateMmolL:          1.6,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          false,
	}
	clearance103 := &domain.HemoabdomenHemostasisClearance{
		ClearedAt:                    t103.Add(6 * time.Hour),
		FinalMAPMmHg:                 82,
		FinalPeripheralPCVPercent:    31.0,
		FinalPlasmaLactateMmolL:      1.6,
		SurgicalHemostasisConfirmed:  true,
		RepeatAfastFreeFluidResolved: true,
		VetLicense:                   "VET-CA-94810",
		Summary:                      "Definitive hepatic lobe hemostasis confirmed; post-autotransfusion peripheral PCV 31.0%, MAP 82 mmHg, lactate 1.6 mmol/L; repeat AFAST AFS 0/4 free fluid resolved.",
	}
	op103 := &domain.HemoabdomenOperation{
		ID:                        "HA-OP-103",
		IncidentID:                "INC-HA-9403",
		PetName:                   "Ranger",
		Status:                    domain.HemoabdomenStatusHemostasisRestoredCleared,
		ClotPoppingNeoplasiaAlert: false,
		PatientProfile:            p103,
		ResuscitationPlan:         BuildResuscitationPlan(p103),
		HemodynamicChecks: []domain.HemoabdomenHemodynamicCheck{
			{
				Timestamp:                t103.Add(5 * time.Hour),
				MeanArterialPressureMmHg: 82,
				HeartRateBpm:             96,
				PeripheralPCVPercent:     31.0,
				TotalSolidsGdl:           5.8,
				PlasmaLactateMmolL:       1.6,
				AutotransfusedVolumeMl:   320.0,
				ClinicianCallsign:        "DR-TRAUMA-09",
				Notes:                    "Normotensive MAP 82 mmHg off vasopressors; 320 mL autologous shed blood reinfused via 170 µm filter; repeat AFAST negative.",
			},
		},
		HemostasisClearance: clearance103,
		CreatedAt:           t103,
		UpdatedAt:           t103.Add(6 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *HemoabdomenService) ListOperations(statusFilter string) []domain.HemoabdomenOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.HemoabdomenOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneHemoabdomenOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *HemoabdomenService) GetOperation(id string) (*domain.HemoabdomenOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("hemoabdomen operation %q not found", id)
	}
	cp := cloneHemoabdomenOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new hemoabdomen & autotransfusion triage operation.
func (s *HemoabdomenService) CreateOperation(incidentID, petName string, profile domain.HemoabdomenPatientProfile) (*domain.HemoabdomenOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if profile.Species == domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma {
		profile.SuspectedNeoplasia = true
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("HA-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := computeClotPoppingNeoplasiaAlert(profile)
	plan := BuildResuscitationPlan(profile)

	op := &domain.HemoabdomenOperation{
		ID:                        opID,
		IncidentID:                trimmedIncident,
		PetName:                   trimmedPet,
		Status:                    domain.HemoabdomenStatusHemorrhagicShockCritical,
		ClotPoppingNeoplasiaAlert: alert,
		PatientProfile:            profile,
		ResuscitationPlan:         plan,
		HemodynamicChecks:         []domain.HemoabdomenHemodynamicCheck{},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneHemoabdomenOperation(op)
	return &cp, nil
}

// StageResuscitationPlan computes the permissive hypotensive, TXA, and autotransfusion plan
// and transitions the operation to AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED.
func (s *HemoabdomenService) StageResuscitationPlan(id string, profile domain.HemoabdomenPatientProfile) (*domain.HemoabdomenOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if profile.Species == domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma {
		profile.SuspectedNeoplasia = true
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("hemoabdomen operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.ResuscitationPlan = BuildResuscitationPlan(profile)
	op.ClotPoppingNeoplasiaAlert = computeClotPoppingNeoplasiaAlert(profile)
	op.Status = domain.HemoabdomenStatusAutotransfusionTXAStaged
	op.HemostasisClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneHemoabdomenOperation(op)
	return &cp, nil
}

// RecordHemodynamicCheck appends a serial hemodynamic check to an existing operation.
func (s *HemoabdomenService) RecordHemodynamicCheck(id string, check domain.HemoabdomenHemodynamicCheck) (*domain.HemoabdomenOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("hemoabdomen operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.HemodynamicChecks = append(op.HemodynamicChecks, check)
	op.PatientProfile.MeanArterialPressureMmHg = check.MeanArterialPressureMmHg
	op.PatientProfile.PeripheralPCVPercent = check.PeripheralPCVPercent
	op.PatientProfile.PlasmaLactateMmolL = check.PlasmaLactateMmolL

	if check.MeanArterialPressureMmHg < 55 || check.PlasmaLactateMmolL >= 6.0 || check.PeripheralPCVPercent < 18.0 {
		op.ClotPoppingNeoplasiaAlert = true
		if check.MeanArterialPressureMmHg < 50 {
			op.Status = domain.HemoabdomenStatusHemorrhagicShockCritical
			op.HemostasisClearance = nil
		}
	} else if check.MeanArterialPressureMmHg >= 65 && check.PlasmaLactateMmolL < 3.0 && !op.PatientProfile.SuspectedNeoplasia {
		op.ClotPoppingNeoplasiaAlert = false
	}

	op.UpdatedAt = now
	cp := cloneHemoabdomenOperation(op)
	return &cp, nil
}

// IssueHemostasisClearance validates surgical/spontaneous hemostasis, normotensive MAP, restored PCV,
// and normalized lactate, transitioning the operation to HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE.
func (s *HemoabdomenService) IssueHemostasisClearance(id string, clearance domain.HemoabdomenHemostasisClearance) (*domain.HemoabdomenOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("hemoabdomen operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Surgical/spontaneous hemostasis confirmed; normotensive MAP %d mmHg, peripheral PCV %.1f%%, lactate %.1f mmol/L; repeat AFAST AFS 0/4 free fluid resolved.", clearance.FinalMAPMmHg, clearance.FinalPeripheralPCVPercent, clearance.FinalPlasmaLactateMmolL)
	}

	op.HemostasisClearance = &clearance
	op.Status = domain.HemoabdomenStatusHemostasisRestoredCleared
	op.ClotPoppingNeoplasiaAlert = false
	op.PatientProfile.AfastAFS4Score = 0
	op.PatientProfile.MeanArterialPressureMmHg = clearance.FinalMAPMmHg
	op.PatientProfile.PeripheralPCVPercent = clearance.FinalPeripheralPCVPercent
	op.PatientProfile.PlasmaLactateMmolL = clearance.FinalPlasmaLactateMmolL
	op.UpdatedAt = now

	cp := cloneHemoabdomenOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all operations in the service.
func (s *HemoabdomenService) ComputeMetrics() HemoabdomenMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m HemoabdomenMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.HemoabdomenStatusHemorrhagicShockCritical:
			m.HemorrhagicShockCriticalCount++
		case domain.HemoabdomenStatusAutotransfusionTXAStaged:
			m.AutotransfusionTXAStagedCount++
		case domain.HemoabdomenStatusHemostasisRestoredCleared:
			m.HemostasisClearedCount++
		}
		if op.ClotPoppingNeoplasiaAlert {
			m.ClotPoppingNeoplasiaAlertCount++
		}
	}
	return m
}

func cloneHemoabdomenOperation(op *domain.HemoabdomenOperation) domain.HemoabdomenOperation {
	cp := *op
	if op.ResuscitationPlan != nil {
		plan := *op.ResuscitationPlan
		cp.ResuscitationPlan = &plan
	}
	if op.HemodynamicChecks != nil {
		cp.HemodynamicChecks = make([]domain.HemoabdomenHemodynamicCheck, len(op.HemodynamicChecks))
		copy(cp.HemodynamicChecks, op.HemodynamicChecks)
	}
	if op.HemostasisClearance != nil {
		clr := *op.HemostasisClearance
		cp.HemostasisClearance = &clr
	}
	return cp
}
