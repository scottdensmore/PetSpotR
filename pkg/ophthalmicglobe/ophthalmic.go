package ophthalmicglobe

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// OphthalmicMetrics summarizes operational KPI counts across Acute Ophthalmic & Globe Preservation triage operations.
type OphthalmicMetrics struct {
	TotalOperations                    int `json:"total_operations"`
	VisionThreateningCriticalCount     int `json:"vision_threatening_critical_count"`
	LatanoprostTarsorrhaphyStagedCount int `json:"latanoprost_tarsorrhaphy_staged_count"`
	IOPNormalizedClearedCount          int `json:"iop_normalized_cleared_count"`
	LatanoprostSteroidAlertCount       int `json:"latanoprost_steroid_alert_count"`
}

// OphthalmicService manages Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe,
// Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost triage operations in a thread-safe in-memory store.
type OphthalmicService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.OphthalmicGlobeOperation
	order []string
	seq   int
}

// NewOphthalmicService initializes an OphthalmicService with 3 seeded operations (OG-OP-101, OG-OP-102, OG-OP-103).
func NewOphthalmicService() *OphthalmicService {
	s := &OphthalmicService{
		ops:   make(map[string]*domain.OphthalmicGlobeOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildPreservationPlan deterministically computes an OphthalmicPreservationPlan for a patient profile.
func BuildPreservationPlan(profile domain.OphthalmicPatientProfile) *domain.OphthalmicPreservationPlan {
	latanoprostAllowed := profile.IntraocularPressureMmHg > 30 &&
		!profile.AnteriorLensLuxationPresent &&
		!strings.EqualFold(profile.Species, domain.OphthalmicSpeciesFelineMeltingDescemetoceleUlcer)

	dorzolamideFreq := "PRN / Normal IOP"
	if profile.IntraocularPressureMmHg > 25 {
		dorzolamideFreq = "q8h"
	}

	var mannitolGrams float64
	if profile.IntraocularPressureMmHg > 40 {
		mannitolGrams = round1(1.0 * profile.WeightKg)
	}

	stainUpper := strings.ToUpper(strings.TrimSpace(profile.FluoresceinStainPattern))
	autologousSerumQ1H := stainUpper == domain.OphthalmicFluoresceinStromalPositiveMelting ||
		stainUpper == domain.OphthalmicFluoresceinDonutRingDescemetocele ||
		stainUpper == domain.OphthalmicFluoresceinSeidelPositiveLeak ||
		strings.EqualFold(profile.Species, domain.OphthalmicSpeciesFelineMeltingDescemetoceleUlcer)

	tarsorrhaphyIndicated := strings.EqualFold(profile.Species, domain.OphthalmicSpeciesCanineBrachycephalicGlobeProptosis) &&
		profile.ExtraocularMusclesAvulsedCount < 3

	pitfallAlert := "CRITICAL OPHTHALMIC GLOBE & GLAUCOMA PITFALL ALERT: NEVER administer Latanoprost (0.005% prostaglandin F2-alpha analogue) when anterior lens luxation is present or in feline uveitic glaucoma (causes severe pupillary miosis that traps the luxated lens in the pupil, blocks aqueous humor outflow, and spikes IOP further); NEVER apply topical corticosteroids or petroleum-based ointments to deep/melting corneal ulcers or perforated descemetoceles (steroids activate corneal collagenases/MMPs causing rapid perforation within hours, and ointment petrolatum causes severe granulomatous uveitis inside the anterior chamber); and NEVER perform manual jugular compression or neck-lead restraint (spikes IOP by 15–25 mmHg and ruptures fragile descemetoceles)."

	var rationaleParts []string
	if mannitolGrams > 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Acute congestive glaucoma (IOP %d mmHg > 40 mmHg): infuse Mannitol 1.0 g/kg slow IV over 20 min (%.1f g IV via 0.22-micron inline filter for %.1f kg patient; withhold water 2h to dehydrate vitreous humor).", profile.IntraocularPressureMmHg, mannitolGrams, profile.WeightKg))
	}
	if latanoprostAllowed {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Primary open/narrow-angle glaucoma without anterior lens luxation: instill Latanoprost 0.005%% 1 drop topically + Dorzolamide 2%%/Timolol 0.5%% %s.", dorzolamideFreq))
	} else if profile.AnteriorLensLuxationPresent {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Anterior lens luxation present (IOP %d mmHg): Latanoprost 0.005%% is STRICTLY CONTRAINDICATED (miosis traps lens); rely on Mannitol IV + Dorzolamide 2%%/Timolol 0.5%% %s and emergent trans-corneal lens reduction/intracapsular lens extraction.", profile.IntraocularPressureMmHg, dorzolamideFreq))
	} else if dorzolamideFreq == "q8h" {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Elevated IOP (%d mmHg): instill Dorzolamide 2%%/Timolol 0.5%% %s.", profile.IntraocularPressureMmHg, dorzolamideFreq))
	}
	if autologousSerumQ1H {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Corneal defect (%s): instill anticollagenase Autologous Serum drops q1h (alpha-2-macroglobulin inhibits matrix metalloproteinases) + topical fluoroquinolone q2h (space drops >= 5 min apart).", profile.FluoresceinStainPattern))
	}
	if tarsorrhaphyIndicated {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Proptosed brachycephalic globe with %d (< 3) extraocular muscles avulsed and consensual PLR %v: lubricate copiously with sterile water-soluble gel and perform emergent globe reduction + temporary lateral tarsorrhaphy.", profile.ExtraocularMusclesAvulsedCount, profile.ConsensualPLRPresent))
	}
	if len(rationaleParts) == 0 {
		rationaleParts = append(rationaleParts, fmt.Sprintf("Normotensive IOP (%d mmHg) and intact corneal epithelium (%s): maintain hard plastic Elizabethan collar and tear film lubrication.", profile.IntraocularPressureMmHg, profile.FluoresceinStainPattern))
	}

	return &domain.OphthalmicPreservationPlan{
		Latanoprost0005PctAllowed:               latanoprostAllowed,
		DorzolamideTimololDropsFreq:             dorzolamideFreq,
		Mannitol1GKgIVGrams:                     mannitolGrams,
		AutologousSerumDropsQ1H:                 autologousSerumQ1H,
		TemporaryTarsorrhaphyIndicated:          tarsorrhaphyIndicated,
		ZeroTopicalSteroidOintmentNeckLeadGuard: true,
		OphthalmicPitfallAlert:                  pitfallAlert,
		ClinicalRationale:                       strings.Join(rationaleParts, " "),
	}
}

func needsLatanoprostLuxationSteroidAlert(p domain.OphthalmicPatientProfile) bool {
	stainUpper := strings.ToUpper(strings.TrimSpace(p.FluoresceinStainPattern))
	return p.IntraocularPressureMmHg > 25 ||
		p.AnteriorLensLuxationPresent ||
		stainUpper != domain.OphthalmicFluoresceinNegativeIntact ||
		p.ExtraocularMusclesAvulsedCount > 0 ||
		!p.ConsensualPLRPresent
}

func (s *OphthalmicService) seed() {
	t101 := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	p101 := domain.OphthalmicPatientProfile{
		Species:                        domain.OphthalmicSpeciesCaninePrimaryCongestiveGlaucoma,
		Breed:                          "Cocker Spaniel",
		WeightKg:                       12.5,
		IntraocularPressureMmHg:        64,
		FluoresceinStainPattern:        domain.OphthalmicFluoresceinNegativeIntact,
		AnteriorLensLuxationPresent:    true,
		ExtraocularMusclesAvulsedCount: 0,
		ConsensualPLRPresent:           true,
	}
	op101 := &domain.OphthalmicGlobeOperation{
		ID:                              "OG-OP-101",
		IncidentID:                      "INC-OG-9951",
		PetName:                         "Buster",
		Status:                          domain.OphthalmicStatusVisionThreateningCritical,
		LatanoprostLuxationSteroidAlert: true,
		PatientProfile:                  p101,
		PreservationPlan:                BuildPreservationPlan(p101),
		TonometryChecks: []domain.OphthalmicTonometryCheck{
			{
				Timestamp:               t101.Add(10 * time.Minute),
				IntraocularPressureMmHg: 64,
				SeidelTestNegative:      true,
				DazzleReflexPresent:     true,
				RigidECollarVerified:    true,
				ClinicianCallsign:       "DR-OPHTHO-11",
				Notes:                   "Acute congestive glaucoma IOP 64 mmHg in Cocker Spaniel with anterior lens luxation; Latanoprost 0.005% strictly contraindicated (miosis traps lens); Mannitol 12.5 g IV + Dorzolamide/Timolol q8h initiated.",
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(10 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 4, 15, 0, 0, time.UTC)
	p102 := domain.OphthalmicPatientProfile{
		Species:                        domain.OphthalmicSpeciesCanineBrachycephalicGlobeProptosis,
		Breed:                          "Pug",
		WeightKg:                       8.4,
		IntraocularPressureMmHg:        28,
		FluoresceinStainPattern:        domain.OphthalmicFluoresceinStromalPositiveMelting,
		AnteriorLensLuxationPresent:    false,
		ExtraocularMusclesAvulsedCount: 1,
		ConsensualPLRPresent:           true,
	}
	op102 := &domain.OphthalmicGlobeOperation{
		ID:                              "OG-OP-102",
		IncidentID:                      "INC-OG-9952",
		PetName:                         "Pugsley",
		Status:                          domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged,
		LatanoprostLuxationSteroidAlert: true,
		PatientProfile:                  p102,
		PreservationPlan:                BuildPreservationPlan(p102),
		TonometryChecks: []domain.OphthalmicTonometryCheck{
			{
				Timestamp:               t102.Add(15 * time.Minute),
				IntraocularPressureMmHg: 28,
				SeidelTestNegative:      true,
				DazzleReflexPresent:     true,
				RigidECollarVerified:    true,
				ClinicianCallsign:       "DR-ECC-22",
				Notes:                   "Brachycephalic traumatic globe proptosis in Pug with 1 medial rectus muscle avulsed (< 3) and intact consensual PLR; temporary tarsorrhaphy + autologous serum q1h staged.",
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(15 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC)
	p103 := domain.OphthalmicPatientProfile{
		Species:                        domain.OphthalmicSpeciesFelineMeltingDescemetoceleUlcer,
		Breed:                          "Persian",
		WeightKg:                       3.8,
		IntraocularPressureMmHg:        16,
		FluoresceinStainPattern:        domain.OphthalmicFluoresceinNegativeIntact,
		AnteriorLensLuxationPresent:    false,
		ExtraocularMusclesAvulsedCount: 0,
		ConsensualPLRPresent:           true,
	}
	clearance103 := &domain.OphthalmicDischargeClearance{
		ClearedAt:                       t103.Add(18 * time.Hour),
		FinalIOPMmHg:                    16,
		GlobeTectonicIntegrityConfirmed: true,
		HardPlasticECollarFitted:        true,
		TopicalDrop5MinSpacingEducated:  true,
		VetLicense:                      "VET-OPHTHO-99503",
		Summary:                         "Conjunctival pedicle graft tectonic integrity confirmed; Seidel test negative; IOP normalized at 16 mmHg; hard plastic E-collar fitted and owner educated on 5-minute topical drop spacing.",
	}
	op103 := &domain.OphthalmicGlobeOperation{
		ID:                              "OG-OP-103",
		IncidentID:                      "INC-OG-9953",
		PetName:                         "Misty",
		Status:                          domain.OphthalmicStatusIOPNormalCorneaProtected,
		LatanoprostLuxationSteroidAlert: false,
		PatientProfile:                  p103,
		PreservationPlan:                BuildPreservationPlan(p103),
		TonometryChecks: []domain.OphthalmicTonometryCheck{
			{
				Timestamp:               t103.Add(16 * time.Hour),
				IntraocularPressureMmHg: 16,
				SeidelTestNegative:      true,
				DazzleReflexPresent:     true,
				RigidECollarVerified:    true,
				ClinicianCallsign:       "DR-OPHTHO-04",
				Notes:                   "Post-graft corneal re-epithelialization complete; Seidel negative; IOP 16 mmHg; dazzle and consensual PLR intact.",
			},
		},
		DischargeClearance: clearance103,
		CreatedAt:          t103,
		UpdatedAt:          t103.Add(18 * time.Hour),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of operations, optionally filtered by statusFilter.
func (s *OphthalmicService) ListOperations(statusFilter string) []domain.OphthalmicGlobeOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.OphthalmicGlobeOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneOphthalmicOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an operation by ID.
func (s *OphthalmicService) GetOperation(id string) (*domain.OphthalmicGlobeOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("ophthalmic globe operation %q not found", id)
	}
	cp := cloneOphthalmicOperation(op)
	return &cp, nil
}

// CreateOperation validates and creates a new Acute Ophthalmic & Globe Preservation triage operation.
func (s *OphthalmicService) CreateOperation(incidentID, petName string, profile domain.OphthalmicPatientProfile) (*domain.OphthalmicGlobeOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.FluoresceinStainPattern = strings.ToUpper(strings.TrimSpace(profile.FluoresceinStainPattern))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("OG-OP-%d", s.seq)
	now := time.Now().UTC()

	alert := needsLatanoprostLuxationSteroidAlert(profile)
	plan := BuildPreservationPlan(profile)

	op := &domain.OphthalmicGlobeOperation{
		ID:                              opID,
		IncidentID:                      trimmedIncident,
		PetName:                         trimmedPet,
		Status:                          domain.OphthalmicStatusVisionThreateningCritical,
		LatanoprostLuxationSteroidAlert: alert,
		PatientProfile:                  profile,
		PreservationPlan:                plan,
		TonometryChecks:                 []domain.OphthalmicTonometryCheck{},
		CreatedAt:                       now,
		UpdatedAt:                       now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneOphthalmicOperation(op)
	return &cp, nil
}

// StagePreservationPlan computes Latanoprost 0.005% eligibility (IOP > 30, no anterior lens luxation,
// non-feline melting ulcer), Dorzolamide/Timolol q8h when IOP > 25 mmHg, Mannitol 1.0 g/kg IV when IOP > 40 mmHg,
// Autologous Serum q1h for melting/descemetocele ulcers, and Temporary Tarsorrhaphy for globe proptosis with < 3 avulsed muscles,
// transitioning the operation to LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED.
func (s *OphthalmicService) StagePreservationPlan(id string, profile domain.OphthalmicPatientProfile) (*domain.OphthalmicGlobeOperation, error) {
	profile.Species = strings.ToUpper(strings.TrimSpace(profile.Species))
	profile.FluoresceinStainPattern = strings.ToUpper(strings.TrimSpace(profile.FluoresceinStainPattern))
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("ophthalmic globe operation %q not found", id)
	}

	if profile.Breed == "" && op.PatientProfile.Breed != "" {
		profile.Breed = op.PatientProfile.Breed
	}

	op.PatientProfile = profile
	op.PreservationPlan = BuildPreservationPlan(profile)
	op.LatanoprostLuxationSteroidAlert = needsLatanoprostLuxationSteroidAlert(profile)
	op.Status = domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged
	op.DischargeClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOphthalmicOperation(op)
	return &cp, nil
}

// RecordTonometryCheck appends a serial rebound/applanation tonometry, Seidel test, dazzle reflex, and E-collar check.
func (s *OphthalmicService) RecordTonometryCheck(id string, check domain.OphthalmicTonometryCheck) (*domain.OphthalmicGlobeOperation, error) {
	check.ClinicianCallsign = strings.TrimSpace(check.ClinicianCallsign)
	check.Notes = strings.TrimSpace(check.Notes)
	if err := check.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("ophthalmic globe operation %q not found", id)
	}

	now := time.Now().UTC()
	if check.Timestamp.IsZero() {
		check.Timestamp = now
	}

	op.TonometryChecks = append(op.TonometryChecks, check)
	op.PatientProfile.IntraocularPressureMmHg = check.IntraocularPressureMmHg
	if !check.SeidelTestNegative {
		op.PatientProfile.FluoresceinStainPattern = domain.OphthalmicFluoresceinSeidelPositiveLeak
	}
	op.PreservationPlan = BuildPreservationPlan(op.PatientProfile)

	if check.IntraocularPressureMmHg > 35 || !check.SeidelTestNegative || !check.DazzleReflexPresent || !check.RigidECollarVerified {
		op.LatanoprostLuxationSteroidAlert = true
		if check.IntraocularPressureMmHg > 40 || !check.SeidelTestNegative {
			op.Status = domain.OphthalmicStatusVisionThreateningCritical
			op.DischargeClearance = nil
		}
	} else if check.IntraocularPressureMmHg >= 10 && check.IntraocularPressureMmHg <= 25 && check.SeidelTestNegative && check.DazzleReflexPresent && check.RigidECollarVerified && !op.PatientProfile.AnteriorLensLuxationPresent {
		op.LatanoprostLuxationSteroidAlert = false
	}

	op.UpdatedAt = now
	cp := cloneOphthalmicOperation(op)
	return &cp, nil
}

// IssueDischargeClearance validates FinalIOPMmHg in [10, 25], GlobeTectonicIntegrityConfirmed,
// HardPlasticECollarFitted, and TopicalDrop5MinSpacingEducated, transitioning the operation to
// IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED and clearing LatanoprostLuxationSteroidAlert.
func (s *OphthalmicService) IssueDischargeClearance(id string, clearance domain.OphthalmicDischargeClearance) (*domain.OphthalmicGlobeOperation, error) {
	clearance.VetLicense = strings.TrimSpace(clearance.VetLicense)
	clearance.Summary = strings.TrimSpace(clearance.Summary)
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("ophthalmic globe operation %q not found", id)
	}

	now := time.Now().UTC()
	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = now
	}
	if clearance.Summary == "" {
		clearance.Summary = fmt.Sprintf("IOP normalized at %d mmHg; globe tectonic integrity confirmed; hard plastic E-collar fitted and owner educated on >= 5-minute spacing between topical ophthalmic drops.", clearance.FinalIOPMmHg)
	}

	op.DischargeClearance = &clearance
	op.Status = domain.OphthalmicStatusIOPNormalCorneaProtected
	op.LatanoprostLuxationSteroidAlert = false
	op.PatientProfile.IntraocularPressureMmHg = clearance.FinalIOPMmHg
	if op.PatientProfile.FluoresceinStainPattern == domain.OphthalmicFluoresceinSeidelPositiveLeak {
		op.PatientProfile.FluoresceinStainPattern = domain.OphthalmicFluoresceinNegativeIntact
	}
	op.PreservationPlan = BuildPreservationPlan(op.PatientProfile)
	op.UpdatedAt = now

	cp := cloneOphthalmicOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI counts across all Ophthalmic Globe operations in the service.
func (s *OphthalmicService) ComputeMetrics() OphthalmicMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m OphthalmicMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.OphthalmicStatusVisionThreateningCritical:
			m.VisionThreateningCriticalCount++
		case domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged:
			m.LatanoprostTarsorrhaphyStagedCount++
		case domain.OphthalmicStatusIOPNormalCorneaProtected:
			m.IOPNormalizedClearedCount++
		}
		if op.LatanoprostLuxationSteroidAlert {
			m.LatanoprostSteroidAlertCount++
		}
	}
	return m
}

func cloneOphthalmicOperation(op *domain.OphthalmicGlobeOperation) domain.OphthalmicGlobeOperation {
	cp := *op
	if op.PreservationPlan != nil {
		plan := *op.PreservationPlan
		cp.PreservationPlan = &plan
	}
	if op.TonometryChecks != nil {
		cp.TonometryChecks = make([]domain.OphthalmicTonometryCheck, len(op.TonometryChecks))
		copy(cp.TonometryChecks, op.TonometryChecks)
	}
	if op.DischargeClearance != nil {
		clr := *op.DischargeClearance
		cp.DischargeClearance = &clr
	}
	return cp
}
