package akidialysis

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AKIMetrics summarizes operational KPI counts across Acute Kidney Injury (AKI) & Dialysis triage operations.
type AKIMetrics struct {
	TotalOperations               int `json:"total_operations"`
	OliguricAzotemiaCriticalCount int `json:"oliguric_azotemia_critical_count"`
	FomepizoleDiuresisStagedCount int `json:"fomepizole_diuresis_staged_count"`
	PolyuricClearedCount          int `json:"polyuric_cleared_count"`
	AnuricOverloadAlertCount      int `json:"anuric_overload_alert_count"`
}

// AKIService manages Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity,
// Oliguria/Anuria & Furosemide/Diltiazem/Dialysis triage operations in a thread-safe in-memory store.
type AKIService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.AKIDialysisOperation
	order []string
	seq   int
}

// NewAKIService initializes an AKIService with 3 seeded operations (AKI-OP-101, AKI-OP-102, AKI-OP-103).
func NewAKIService() *AKIService {
	s := &AKIService{
		ops:   make(map[string]*domain.AKIDialysisOperation),
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

// BuildDiuresisDialysisPlan deterministically computes an AKIDiuresisDialysisPlan for a patient profile.
func BuildDiuresisDialysisPlan(profile domain.AKIPatientProfile) *domain.AKIDiuresisDialysisPlan {
	var fomepizoleMg float64
	switch strings.ToUpper(strings.TrimSpace(profile.Species)) {
	case domain.AKISpeciesFelineLiliumNephrotoxicity:
		fomepizoleMg = round2(125.0 * profile.WeightKg)
	case domain.AKISpeciesCanineEthyleneGlycolOxalate:
		fomepizoleMg = round2(20.0 * profile.WeightKg)
	default:
		if profile.CalciumOxalateMonohydrateCrystals {
			fomepizoleMg = round2(20.0 * profile.WeightKg)
		}
	}

	insAndOutsMlHr := round2((profile.UrineOutputMlKgHr * profile.WeightKg) + (20.0 * profile.WeightKg / 24.0))

	var furosemideBolusMg float64
	var diltiazemCRI float64
	if profile.UrineOutputMlKgHr < 1.0 {
		furosemideBolusMg = round2(2.0 * profile.WeightKg)
		diltiazemCRI = 0.3
	}

	hemodialysisIndicated := profile.UrineOutputMlKgHr < 0.5 ||
		profile.SerumCreatinineMgDl >= 8.0 ||
		profile.SerumPotassiumMeqL >= 6.5

	pitfallAlert := "CRITICAL AKI, ANURIC FLUID OVERLOAD & FOMEPIZOLE PITFALL ALERT: NEVER force high-rate IV crystalloid fluids into an oliguric/anuric (< 0.5 mL/kg/hr) patient — causes fatal iatrogenic pulmonary/pleural edema and chemosis; switch strictly to 'Ins-and-Outs' fluid matching (urine output + 20 mL/kg/day insensible loss); NEVER use canine-dose Fomepizole (20 mg/kg) in cats (cats require 125 mg/kg IV high-dose Fomepizole within 3 hours due to lower alcohol dehydrogenase affinity); and NEVER delay intermittent hemodialysis when oliguria + K+ > 6.5 mEq/L persists."

	rationaleParts := []string{
		fmt.Sprintf("Ins-and-Outs hourly IV fluid matching rate %.2f mL/hr (closed-system UOP %.1f mL/kg/hr × %.1f kg + 20 mL/kg/day insensible loss) for IRIS Grade %d %s (Cr %.1f mg/dL, K+ %.1f mEq/L, weight gain %.1f%%).", insAndOutsMlHr, profile.UrineOutputMlKgHr, profile.WeightKg, profile.IRISGrade, profile.Species, profile.SerumCreatinineMgDl, profile.SerumPotassiumMeqL, profile.BodyWeightGainPercent),
	}
	if fomepizoleMg > 0 {
		if strings.EqualFold(profile.Species, domain.AKISpeciesFelineLiliumNephrotoxicity) {
			rationaleParts = append(rationaleParts, fmt.Sprintf("Feline high-dose Fomepizole (4-MP) 125 mg/kg IV loading dose (%.1f mg IV) for suspected co-ingestion / alcohol dehydrogenase blockade.", fomepizoleMg))
		} else {
			rationaleParts = append(rationaleParts, fmt.Sprintf("Canine Ethylene Glycol Fomepizole (4-MP) 20 mg/kg IV loading dose (%.1f mg IV) to halt glycolic/oxalic acid nephrotoxic metabolism.", fomepizoleMg))
		}
	}
	if furosemideBolusMg > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Oliguric-to-polyuric conversion trial (UOP %.1f mL/kg/hr < 1.0 mL/kg/hr): Furosemide 2.0 mg/kg IV bolus (%.1f mg IV) + Diltiazem CRI %.1f mcg/kg/min renal afferent arteriolar vasodilation.", profile.UrineOutputMlKgHr, furosemideBolusMg, diltiazemCRI))
	}
	if hemodialysisIndicated {
		rationaleParts = append(rationaleParts, "Intermittent Hemodialysis (IHD) / CRRT indicated for refractory oliguria/anuria (< 0.5 mL/kg/hr), severe IRIS Grade IV-V uremia (Cr >= 8.0 mg/dL), or life-threatening hyperkalemia (K+ >= 6.5 mEq/L).")
	}

	return &domain.AKIDiuresisDialysisPlan{
		FomepizoleLoadingDoseMg:     fomepizoleMg,
		InsAndOutsFluidRateMlHr:     insAndOutsMlHr,
		FurosemideConversionBolusMg: furosemideBolusMg,
		DiltiazemCRI03McgKgMin:      diltiazemCRI,
		HemodialysisIndicated:       hemodialysisIndicated,
		ZeroForceFluidInAnuriaGuard: true,
		AKIPitfallAlert:             pitfallAlert,
		ClinicalRationale:           strings.Join(rationaleParts, " "),
	}
}

func needsAnuricFluidOverloadFomepizoleAlert(p domain.AKIPatientProfile) bool {
	return p.UrineOutputMlKgHr < 1.0 ||
		p.SerumCreatinineMgDl >= 3.5 ||
		p.SerumPotassiumMeqL >= 6.0 ||
		p.BodyWeightGainPercent >= 5.0 ||
		p.CalciumOxalateMonohydrateCrystals ||
		p.IRISGrade >= 3
}

func (s *AKIService) seed() {
	t101 := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	p101 := domain.AKIPatientProfile{
		Species:                           domain.AKISpeciesFelineLiliumNephrotoxicity,
		Breed:                             "Domestic Shorthair",
		WeightKg:                          4.5,
		IRISGrade:                         5,
		SerumCreatinineMgDl:               11.4,
		SerumPotassiumMeqL:                7.4,
		UrineOutputMlKgHr:                 0.2,
		BodyWeightGainPercent:             9.5,
		CalciumOxalateMonohydrateCrystals: false,
	}
	op101 := &domain.AKIDialysisOperation{
		ID:                                 "AKI-OP-101",
		IncidentID:                         "INC-AKI-9931",
		PetName:                            "Lotus",
		Status:                             domain.AKIStatusOliguricAzotemiaCritical,
		AnuricFluidOverloadFomepizoleAlert: true,
		PatientProfile:                     p101,
		DiuresisPlan:                       BuildDiuresisDialysisPlan(p101),
		RenalChecks: []domain.AKIRenalTelemetryCheck{
			{
				Timestamp:           t101.Add(10 * time.Minute),
				UrineOutputMlKgHr:   0.2,
				SerumCreatinineMgDl: 11.4,
				SerumPotassiumMeqL:  7.4,
				BodyWeightKg:        4.5,
				FluidInOutBalanceMl: 185.0,
				ClinicianCallsign:   "DR-RENAL-11",
				Notes:               "Feline Easter lily (Lilium longiflorum) pollen ingestion 36h prior; IRIS Grade V oliguric/anuric AKI (UOP 0.2 mL/kg/hr, Cr 11.4 mg/dL, K+ 7.4 mEq/L, +9.5% fluid overload weight gain); forced IV crystalloids halted, switched to strict Ins-and-Outs (4.65 mL/hr) + Furosemide/Diltiazem trial & urgent intermittent hemodialysis prep.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 4, 15, 0, 0, time.UTC)
	p102 := domain.AKIPatientProfile{
		Species:                           domain.AKISpeciesCanineEthyleneGlycolOxalate,
		Breed:                             "German Shepherd",
		WeightKg:                          28.0,
		IRISGrade:                         3,
		SerumCreatinineMgDl:               4.2,
		SerumPotassiumMeqL:                5.4,
		UrineOutputMlKgHr:                 1.4,
		BodyWeightGainPercent:             2.0,
		CalciumOxalateMonohydrateCrystals: true,
	}
	op102 := &domain.AKIDialysisOperation{
		ID:                                 "AKI-OP-102",
		IncidentID:                         "INC-AKI-9932",
		PetName:                            "Diesel",
		Status:                             domain.AKIStatusFomepizoleDiuresisStaged,
		AnuricFluidOverloadFomepizoleAlert: true,
		PatientProfile:                     p102,
		DiuresisPlan:                       BuildDiuresisDialysisPlan(p102),
		RenalChecks: []domain.AKIRenalTelemetryCheck{
			{
				Timestamp:           t102.Add(20 * time.Minute),
				UrineOutputMlKgHr:   1.4,
				SerumCreatinineMgDl: 4.2,
				SerumPotassiumMeqL:  5.4,
				BodyWeightKg:        28.0,
				FluidInOutBalanceMl: 40.0,
				ClinicianCallsign:   "DR-TOX-22",
				Notes:               "Canine antifreeze (ethylene glycol) ingestion with pathognomonic calcium oxalate monohydrate ('picket-fence') crystalluria; Fomepizole (4-MP) 20 mg/kg (560.0 mg IV) loading dose + Ins-and-Outs fluid matching (62.53 mL/hr) staged.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(20 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC)
	p103 := domain.AKIPatientProfile{
		Species:                           domain.AKISpeciesCanineVitisGrapeLeptospirosis,
		Breed:                             "Beagle",
		WeightKg:                          16.0,
		IRISGrade:                         1,
		SerumCreatinineMgDl:               1.4,
		SerumPotassiumMeqL:                4.3,
		UrineOutputMlKgHr:                 2.8,
		BodyWeightGainPercent:             0.0,
		CalciumOxalateMonohydrateCrystals: false,
	}
	clearance103 := &domain.AKIDischargeClearance{
		ClearedAt:                    t103.Add(48 * time.Hour),
		FinalUrineOutputMlKgHr:       2.8,
		FinalSerumCreatinineMgDl:     1.4,
		FinalSerumPotassiumMeqL:      4.3,
		EuvolemicNoPulmonaryEdema:    true,
		OutpatientSubQFluidsEducated: true,
		VetLicense:                   "VET-RENAL-99303",
		Summary:                      "48h post-Vitis vinifera (raisin/grape) tartaric acid nephrotoxicity diuresis completed; polyuric phase stabilized at UOP 2.8 mL/kg/hr, serum creatinine plateaued at 1.4 mg/dL, normokalemic 4.3 mEq/L, euvolemic; owner educated on outpatient SubQ fluid taper and renal monitoring.",
	}
	op103 := &domain.AKIDialysisOperation{
		ID:                                 "AKI-OP-103",
		IncidentID:                         "INC-AKI-9933",
		PetName:                            "Merlot",
		Status:                             domain.AKIStatusPolyuricNonAzotemicCleared,
		AnuricFluidOverloadFomepizoleAlert: false,
		PatientProfile:                     p103,
		DiuresisPlan:                       BuildDiuresisDialysisPlan(p103),
		RenalChecks: []domain.AKIRenalTelemetryCheck{
			{
				Timestamp:           t103.Add(46 * time.Hour),
				UrineOutputMlKgHr:   2.8,
				SerumCreatinineMgDl: 1.4,
				SerumPotassiumMeqL:  4.3,
				BodyWeightKg:        16.0,
				FluidInOutBalanceMl: 0.0,
				ClinicianCallsign:   "DR-RENAL-09",
				Notes:               "46h post-grape ingestion diuresis: polyuric UOP 2.8 mL/kg/hr, Cr 1.4 mg/dL non-azotemic plateau, K+ 4.3 mEq/L, euvolemic.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(48 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *AKIService) ListOperations(statusFilter string) []domain.AKIDialysisOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.AKIDialysisOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneAKIOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *AKIService) GetOperation(id string) (*domain.AKIDialysisOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("aki dialysis operation %q not found", id)
	}
	cp := cloneAKIOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Acute Kidney Injury (AKI) & Dialysis triage operation.
func (s *AKIService) CreateOperation(incidentID, petName string, profile domain.AKIPatientProfile) (*domain.AKIDialysisOperation, error) {
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
	opID := fmt.Sprintf("AKI-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsAnuricFluidOverloadFomepizoleAlert(profile)
	plan := BuildDiuresisDialysisPlan(profile)

	op := &domain.AKIDialysisOperation{
		ID:                                 opID,
		IncidentID:                         trimmedIncident,
		PetName:                            trimmedPet,
		Status:                             domain.AKIStatusOliguricAzotemiaCritical,
		AnuricFluidOverloadFomepizoleAlert: alert,
		PatientProfile:                     profile,
		DiuresisPlan:                       plan,
		RenalChecks:                        []domain.AKIRenalTelemetryCheck{},
		CreatedAt:                          now,
		UpdatedAt:                          now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneAKIOperation(op)
	return &cp, nil
}

// StageDiuresisPlan computes Fomepizole loading dose (20 mg/kg canine vs 125 mg/kg feline),
// Ins-and-Outs fluid rate ((UOP * WeightKg) + (20.0 * WeightKg / 24.0)), Furosemide 2.0 mg/kg + Diltiazem CRI 0.3 mcg/kg/min
// when oliguric (< 1.0 mL/kg/hr), and HemodialysisIndicated when UOP < 0.5 or Cr >= 8.0 or K+ >= 6.5,
// transitioning the operation to FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED.
func (s *AKIService) StageDiuresisPlan(id string, profile domain.AKIPatientProfile) (*domain.AKIDialysisOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("aki dialysis operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.DiuresisPlan = BuildDiuresisDialysisPlan(profile)
	op.AnuricFluidOverloadFomepizoleAlert = needsAnuricFluidOverloadFomepizoleAlert(profile)
	op.Status = domain.AKIStatusFomepizoleDiuresisStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneAKIOperation(op)
	return &cp, nil
}

// RecordRenalCheck appends a serial closed-system UOP, serum creatinine, serum potassium, body weight, and fluid balance check.
func (s *AKIService) RecordRenalCheck(id string, check domain.AKIRenalTelemetryCheck) (*domain.AKIDialysisOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("aki dialysis operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.RenalChecks = append(op.RenalChecks, check)
	op.PatientProfile.UrineOutputMlKgHr = round2(check.UrineOutputMlKgHr)
	op.PatientProfile.SerumCreatinineMgDl = round1(check.SerumCreatinineMgDl)
	op.PatientProfile.SerumPotassiumMeqL = round1(check.SerumPotassiumMeqL)
	op.PatientProfile.WeightKg = round1(check.BodyWeightKg)
	op.DiuresisPlan = BuildDiuresisDialysisPlan(op.PatientProfile)

	if check.UrineOutputMlKgHr < 0.5 || check.SerumPotassiumMeqL >= 6.5 || check.SerumCreatinineMgDl >= 8.0 {
		op.AnuricFluidOverloadFomepizoleAlert = true
		op.Status = domain.AKIStatusOliguricAzotemiaCritical
		op.DischargeClearance = nil
	} else if check.UrineOutputMlKgHr >= 1.5 && check.SerumCreatinineMgDl <= 2.5 && check.SerumPotassiumMeqL <= 5.5 {
		op.AnuricFluidOverloadFomepizoleAlert = false
		op.PatientProfile.BodyWeightGainPercent = 0.0
		op.PatientProfile.IRISGrade = 1
	}

	op.UpdatedAt = now
	cp := cloneAKIOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates polyuric/normouric UOP >= 1.5 mL/kg/hr, final serum creatinine <= 2.5 mg/dL,
// normokalemia 3.5-5.5 mEq/L, euvolemia without pulmonary edema, and owner outpatient SubQ fluid education,
// transitioning the operation to POLYURIC_CREATININE_PLATEAU_CLEARED.
func (s *AKIService) IssueDischargeClearance(id string, clearance domain.AKIDischargeClearance) (*domain.AKIDialysisOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("aki dialysis operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("Polyuric recovery & non-azotemic creatinine plateau achieved (UOP %.1f mL/kg/hr, Cr %.1f mg/dL, K+ %.1f mEq/L); euvolemic without pulmonary edema and owner educated on outpatient SubQ fluids.", clearance.FinalUrineOutputMlKgHr, clearance.FinalSerumCreatinineMgDl, clearance.FinalSerumPotassiumMeqL)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.AKIStatusPolyuricNonAzotemicCleared
	op.AnuricFluidOverloadFomepizoleAlert = false
	op.PatientProfile.UrineOutputMlKgHr = round2(clearance.FinalUrineOutputMlKgHr)
	op.PatientProfile.SerumCreatinineMgDl = round1(clearance.FinalSerumCreatinineMgDl)
	op.PatientProfile.SerumPotassiumMeqL = round1(clearance.FinalSerumPotassiumMeqL)
	op.PatientProfile.BodyWeightGainPercent = 0.0
	op.PatientProfile.IRISGrade = 1
	op.DiuresisPlan = BuildDiuresisDialysisPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneAKIOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all AKI Dialysis operations in the service.
func (s *AKIService) ComputeMetrics() AKIMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m AKIMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.AKIStatusOliguricAzotemiaCritical:
			m.OliguricAzotemiaCriticalCount++
		case domain.AKIStatusFomepizoleDiuresisStaged:
			m.FomepizoleDiuresisStagedCount++
		case domain.AKIStatusPolyuricNonAzotemicCleared:
			m.PolyuricClearedCount++
		}
		if op.AnuricFluidOverloadFomepizoleAlert {
			m.AnuricOverloadAlertCount++
		}
	}
	return m
}

func cloneAKIOperation(op *domain.AKIDialysisOperation) domain.AKIDialysisOperation {
	cp := *op
	if op.DiuresisPlan != nil {
		plan := *op.DiuresisPlan
		cp.DiuresisPlan = &plan
	}
	if op.RenalChecks != nil {
		cp.RenalChecks = make([]domain.AKIRenalTelemetryCheck, len(op.RenalChecks))
		copy(cp.RenalChecks, op.RenalChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
