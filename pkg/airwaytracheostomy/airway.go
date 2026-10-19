package airwaytracheostomy

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AirwayCockpitMetrics summarizes aggregate KPI counts across all airway & tracheostomy triage operations.
type AirwayCockpitMetrics struct {
	TotalOperations             int `json:"total_operations"`
	CriticalStridorCyanosis     int `json:"critical_stridor_cyanosis"`
	SedationIntubationStaged    int `json:"sedation_intubation_staged"`
	PatentNormothermicExtubated int `json:"patent_normothermic_extubated"`
	CriticalAirwayAlerts        int `json:"critical_airway_alerts"`
}

// AirwayService manages Brachycephalic Obstructive Airway Syndrome (BOAS),
// Laryngeal Paralysis & Temporary Tracheostomy triage operations.
type AirwayService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.AirwayTracheostomyOperation
	order []string
	seq   int
}

// NewAirwayService creates a thread-safe AirwayService seeded with 3 clinical triage operations.
func NewAirwayService() *AirwayService {
	svc := &AirwayService{
		ops:   make(map[string]*domain.AirwayTracheostomyOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	svc.seed()
	return svc
}

func roundTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}

func computeEndotrachealTubeSizeMm(species string, weightKg float64) float64 {
	switch species {
	case domain.AirwaySpeciesFelineNasopharyngealPolyp:
		if weightKg < 3.5 {
			return 3.0
		}
		if weightKg < 5.5 {
			return 3.5
		}
		return 4.0
	case domain.AirwaySpeciesCanineBrachycephalicBOAS:
		// Brachycephalic dogs have hypoplastic tracheas requiring ~35-50% downsized endotracheal tubes
		switch {
		case weightKg < 7.0:
			return 4.0
		case weightKg < 11.0:
			return 4.5
		case weightKg < 15.0:
			return 5.5
		case weightKg < 22.0:
			return 6.5
		default:
			return 7.5
		}
	default:
		// Standard canine sizing for geriatric laryngeal paralysis (e.g., Labrador / Golden Retriever)
		switch {
		case weightKg < 10.0:
			return 7.0
		case weightKg < 20.0:
			return 8.5
		case weightKg < 30.0:
			return 9.5
		case weightKg < 40.0:
			return 10.5
		default:
			return 11.5
		}
	}
}

// ComputeStabilizationPlan calculates the pharmacological anxiolysis, corticosteroid edema protocol,
// rapid-sequence Propofol induction, downsized endotracheal tube sizing, active cooling ceiling, and hazard guardrails.
func ComputeStabilizationPlan(profile domain.AirwayPatientProfile) (*domain.AirwayStabilizationPlan, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	butorphanolAceDose := roundTwoDecimals(profile.WeightKg * 0.22)
	dexSPDose := roundTwoDecimals(profile.WeightKg * 0.15)
	propofolDose := roundTwoDecimals(profile.WeightKg * 2.0)
	etTubeMm := computeEndotrachealTubeSizeMm(profile.Species, profile.WeightKg)

	trachIndicated := profile.LaryngealEdemaGrade == domain.AirwayEdemaGradeIIIEvertedSacculesCollapse ||
		profile.InspiratoryEffort == domain.AirwayEffortSevereParadoxicalCyanotic ||
		profile.SpO2Percent < 88.0

	hazardAlert := "CRITICAL AIRWAY & TRACHEOSTOMY HAZARD: Never apply forceful physical restraint or neck slip-leads without prior Butorphanol (0.2 mg/kg) + low-dose Acepromazine anxiolysis — struggling triggers fatal negative-pressure laryngeal edema and vagal arrest. Always select downsized endotracheal tubes in brachycephalic BOAS patients (hypoplastic trachea), pre-oxygenate for >=60s before tracheal suctioning, and strictly limit suction duration to <=10 seconds to prevent vagal bradycardia and acute hypoxia."

	trachNote := "Endotracheal intubation sufficient with supraglottic edema monitoring"
	if trachIndicated {
		trachNote = "Temporary tracheostomy tube staged (Grade III everted laryngeal saccules / severe cyanotic paradoxical dyspnea)"
	}

	coolingNote := fmt.Sprintf("Normothermic (%.1f°F); maintain cool ambient oxygen cage", profile.RectalTempFahrenheit)
	if profile.RectalTempFahrenheit > 103.0 {
		coolingNote = fmt.Sprintf("Active evaporative cooling initiated from %.1f°F — DISCONTINUE cooling at 103.0°F target to prevent rebound hypothermia", profile.RectalTempFahrenheit)
	}

	rationale := fmt.Sprintf(
		"Butorphanol 0.2 mg/kg + Acepromazine 0.02 mg/kg IV/IM (%.2f mg combined) to break inspiratory panic & negative-pressure edema cycle; Dexamethasone-SP 0.15 mg/kg IV (%.2f mg) to reduce arytenoid/soft-palate swelling; Propofol 2.0 mg/kg IV (%.2f mg) rapid-sequence titration for atraumatic %.1f mm ET tube placement (%s); %s; %s.",
		butorphanolAceDose,
		dexSPDose,
		propofolDose,
		etTubeMm,
		profile.Species,
		coolingNote,
		trachNote,
	)

	return &domain.AirwayStabilizationPlan{
		ButorphanolAcepromazineDoseMg:  butorphanolAceDose,
		DexamethasoneSP015MgKgDoseMg:   dexSPDose,
		PropofolInduction2MgKgDoseMg:   propofolDose,
		EndotrachealTubeSizeMm:         etTubeMm,
		TemporaryTracheostomyIndicated: trachIndicated,
		ActiveCoolingTargetMaxTempF:    103.0,
		ZeroNeckLeadRestraintGuard:     true,
		AirwayHazardAlert:              hazardAlert,
		ClinicalRationale:              rationale,
	}, nil
}

func isCriticalAirwayProfile(p domain.AirwayPatientProfile) bool {
	return p.SpO2Percent < 92.0 ||
		p.RectalTempFahrenheit >= 104.0 ||
		p.InspiratoryEffort == domain.AirwayEffortSevereParadoxicalCyanotic ||
		p.LaryngealEdemaGrade == domain.AirwayEdemaGradeIIIEvertedSacculesCollapse
}

func (s *AirwayService) seed() {
	baseTime := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)

	// AT-OP-101: "Barnaby" (French Bulldog, Critical BOAS crisis)
	p101 := domain.AirwayPatientProfile{
		Species:              domain.AirwaySpeciesCanineBrachycephalicBOAS,
		Breed:                "French Bulldog",
		WeightKg:             12.4,
		RectalTempFahrenheit: 105.8,
		SpO2Percent:          84.0,
		InspiratoryEffort:    domain.AirwayEffortSevereParadoxicalCyanotic,
		LaryngealEdemaGrade:  domain.AirwayEdemaGradeIIIEvertedSacculesCollapse,
		GagReflexPresent:     false,
	}
	plan101, _ := ComputeStabilizationPlan(p101)
	op101 := &domain.AirwayTracheostomyOperation{
		ID:                  "AT-OP-101",
		IncidentID:          "INC-AT-8901",
		PetName:             "Barnaby",
		Status:              domain.AirwayStatusStridorCyanosisCritical,
		CriticalAirwayAlert: true,
		PatientProfile:      p101,
		StabilizationPlan:   plan101,
		TrachCareLogs: []domain.TracheostomyCareLog{
			{
				Timestamp:                   baseTime.Add(5 * time.Minute),
				SterileSalineInstillationMl: 1.5,
				PreOxygenationDurationSec:   90,
				SuctionDurationMaxSec:       8,
				InnerCannulaCleaned:         true,
				SubcutaneousEmphysemaNoted:  false,
				PostSuctionSpO2Percent:      89.0,
				TechnicianCallsign:          "RVT-CHEN-04",
				Notes:                       "Initial pre-oxygenated aseptic suctioning (8s <= 10s limit); copious tenacious pharyngeal foam cleared.",
			},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime.Add(5 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// AT-OP-102: "Yukon" (Labrador Retriever, Geriatric Laryngeal Paralysis, Staged)
	p102 := domain.AirwayPatientProfile{
		Species:              domain.AirwaySpeciesCanineGeriatricLarPar,
		Breed:                "Labrador Retriever",
		WeightKg:             32.0,
		RectalTempFahrenheit: 104.2,
		SpO2Percent:          91.0,
		InspiratoryEffort:    domain.AirwayEffortModerateStridorOrthopnea,
		LaryngealEdemaGrade:  domain.AirwayEdemaGradeIIElongatedPalateEdema,
		GagReflexPresent:     true,
	}
	plan102, _ := ComputeStabilizationPlan(p102)
	op102 := &domain.AirwayTracheostomyOperation{
		ID:                  "AT-OP-102",
		IncidentID:          "INC-AT-8902",
		PetName:             "Yukon",
		Status:              domain.AirwayStatusSedationDexamethasoneStaged,
		CriticalAirwayAlert: true,
		PatientProfile:      p102,
		StabilizationPlan:   plan102,
		TrachCareLogs: []domain.TracheostomyCareLog{
			{
				Timestamp:                   baseTime.Add(15 * time.Minute),
				SterileSalineInstillationMl: 2.0,
				PreOxygenationDurationSec:   120,
				SuctionDurationMaxSec:       7,
				InnerCannulaCleaned:         true,
				SubcutaneousEmphysemaNoted:  false,
				PostSuctionSpO2Percent:      95.0,
				TechnicianCallsign:          "RVT-MORALES-11",
				Notes:                       "Bilateral arytenoid abduction failure stabilized with Butorphanol + Dex-SP; active cooling halted at 103.0°F.",
			},
		},
		CreatedAt: baseTime.Add(10 * time.Minute),
		UpdatedAt: baseTime.Add(18 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// AT-OP-103: "Pippin" (Pug, Patent Normothermic Extubated)
	p103 := domain.AirwayPatientProfile{
		Species:              domain.AirwaySpeciesCanineBrachycephalicBOAS,
		Breed:                "Pug",
		WeightKg:             8.5,
		RectalTempFahrenheit: 100.8,
		SpO2Percent:          98.0,
		InspiratoryEffort:    domain.AirwayEffortMildStertorEupneic,
		LaryngealEdemaGrade:  domain.AirwayEdemaGradeIStenoticNaresOnly,
		GagReflexPresent:     true,
	}
	plan103, _ := ComputeStabilizationPlan(p103)
	clearance103 := &domain.AirwayExtubationClearance{
		ClearedAt:                    baseTime.Add(45 * time.Minute),
		FinalSpO2OnRoomAirPercent:    98.0,
		FinalRectalTempFahrenheit:    100.8,
		GagSwallowReflexVigorous:     true,
		TrachTubeOccludedTrialPassed: true,
		HarnessOnlyDischargeVerified: true,
		VetLicense:                   "DVM-WA-99412",
		Summary:                      "Extubated smoothly with vigorous gag reflex, SpO2 98% on room air, normothermic 100.8°F, chest harness verified (zero neck leads).",
	}
	op103 := &domain.AirwayTracheostomyOperation{
		ID:                  "AT-OP-103",
		IncidentID:          "INC-AT-8903",
		PetName:             "Pippin",
		Status:              domain.AirwayStatusPatentNormothermicExtubated,
		CriticalAirwayAlert: false,
		PatientProfile:      p103,
		StabilizationPlan:   plan103,
		TrachCareLogs: []domain.TracheostomyCareLog{
			{
				Timestamp:                   baseTime.Add(30 * time.Minute),
				SterileSalineInstillationMl: 1.0,
				PreOxygenationDurationSec:   75,
				SuctionDurationMaxSec:       6,
				InnerCannulaCleaned:         true,
				SubcutaneousEmphysemaNoted:  false,
				PostSuctionSpO2Percent:      98.0,
				TechnicianCallsign:          "RVT-PATEL-07",
				Notes:                       "Pre-extubation airway clear; occluded lumen trial passed with zero inspiratory stridor.",
			},
		},
		ExtubationClearance: clearance103,
		CreatedAt:           baseTime.Add(20 * time.Minute),
		UpdatedAt:           baseTime.Add(45 * time.Minute),
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

func cloneOperation(op *domain.AirwayTracheostomyOperation) domain.AirwayTracheostomyOperation {
	cp := *op
	if op.StabilizationPlan != nil {
		planCopy := *op.StabilizationPlan
		cp.StabilizationPlan = &planCopy
	}
	if op.TrachCareLogs != nil {
		logsCopy := make([]domain.TracheostomyCareLog, len(op.TrachCareLogs))
		copy(logsCopy, op.TrachCareLogs)
		cp.TrachCareLogs = logsCopy
	} else {
		cp.TrachCareLogs = []domain.TracheostomyCareLog{}
	}
	if op.ExtubationClearance != nil {
		clearCopy := *op.ExtubationClearance
		cp.ExtubationClearance = &clearCopy
	}
	return cp
}

// ListOperations returns all airway tracheostomy operations matching the optional status filter.
func (s *AirwayService) ListOperations(statusFilter string) []domain.AirwayTracheostomyOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filter := strings.TrimSpace(statusFilter)
	out := make([]domain.AirwayTracheostomyOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if filter != "" && !strings.EqualFold(string(op.Status), filter) {
			continue
		}
		item := cloneOperation(op)
		out = append(out, item)
	}
	return out
}

// GetOperation retrieves a deep copy of an operation by ID.
func (s *AirwayService) GetOperation(id string) (*domain.AirwayTracheostomyOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("airway tracheostomy operation %q not found", id)
	}
	cp := cloneOperation(op)
	return &cp, nil
}

// CreateOperation validates the patient profile and creates a new airway tracheostomy triage operation.
func (s *AirwayService) CreateOperation(incidentID, petName string, profile domain.AirwayPatientProfile) (*domain.AirwayTracheostomyOperation, error) {
	trimmedIncident := strings.TrimSpace(incidentID)
	if trimmedIncident == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	plan, err := ComputeStabilizationPlan(profile)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("AT-OP-%d", s.seq)
	now := time.Now().UTC()

	op := &domain.AirwayTracheostomyOperation{
		ID:                  opID,
		IncidentID:          trimmedIncident,
		PetName:             trimmedPet,
		Status:              domain.AirwayStatusStridorCyanosisCritical,
		CriticalAirwayAlert: isCriticalAirwayProfile(profile),
		PatientProfile:      profile,
		StabilizationPlan:   plan,
		TrachCareLogs:       []domain.TracheostomyCareLog{},
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneOperation(op)
	return &cp, nil
}

// StageStabilizationPlan computes and stages the Butorphanol/Acepromazine + Dexamethasone-SP + Propofol stabilization plan
// and transitions the operation to BUTORPHANOL_DEX_INTUBATION_STAGED.
func (s *AirwayService) StageStabilizationPlan(id string, profile domain.AirwayPatientProfile) (*domain.AirwayTracheostomyOperation, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	plan, err := ComputeStabilizationPlan(profile)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("airway tracheostomy operation %q not found", id)
	}

	op.PatientProfile = profile
	op.StabilizationPlan = plan
	op.Status = domain.AirwayStatusSedationDexamethasoneStaged
	op.CriticalAirwayAlert = isCriticalAirwayProfile(profile) || plan.TemporaryTracheostomyIndicated
	op.ExtubationClearance = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOperation(op)
	return &cp, nil
}

// RecordTrachCareLog validates and appends a TracheostomyCareLog entry to an existing operation.
func (s *AirwayService) RecordTrachCareLog(id string, log domain.TracheostomyCareLog) (*domain.AirwayTracheostomyOperation, error) {
	if err := log.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("airway tracheostomy operation %q not found", id)
	}

	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	op.TrachCareLogs = append(op.TrachCareLogs, log)
	op.PatientProfile.SpO2Percent = log.PostSuctionSpO2Percent
	if log.SubcutaneousEmphysemaNoted || log.PostSuctionSpO2Percent < 92.0 {
		op.CriticalAirwayAlert = true
	}
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOperation(op)
	return &cp, nil
}

// IssueExtubationClearance validates the extubation/decannulation checklist and transitions the operation
// to PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED with CriticalAirwayAlert cleared.
func (s *AirwayService) IssueExtubationClearance(id string, clearance domain.AirwayExtubationClearance) (*domain.AirwayTracheostomyOperation, error) {
	if err := clearance.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("airway tracheostomy operation %q not found", id)
	}

	if op.StabilizationPlan == nil {
		return nil, fmt.Errorf("cannot issue extubation clearance before staging stabilization plan")
	}
	if len(op.TrachCareLogs) == 0 {
		return nil, fmt.Errorf("cannot issue extubation clearance before recording at least one tracheostomy/airway care log")
	}

	if clearance.ClearedAt.IsZero() {
		clearance.ClearedAt = time.Now().UTC()
	}

	op.ExtubationClearance = &clearance
	op.Status = domain.AirwayStatusPatentNormothermicExtubated
	op.CriticalAirwayAlert = false
	op.PatientProfile.SpO2Percent = clearance.FinalSpO2OnRoomAirPercent
	op.PatientProfile.RectalTempFahrenheit = clearance.FinalRectalTempFahrenheit
	op.PatientProfile.GagReflexPresent = true
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOperation(op)
	return &cp, nil
}

// ComputeMetrics returns aggregate KPI telemetry across all airway & tracheostomy operations.
func (s *AirwayService) ComputeMetrics() AirwayCockpitMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m AirwayCockpitMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.AirwayStatusStridorCyanosisCritical:
			m.CriticalStridorCyanosis++
		case domain.AirwayStatusSedationDexamethasoneStaged:
			m.SedationIntubationStaged++
		case domain.AirwayStatusPatentNormothermicExtubated:
			m.PatentNormothermicExtubated++
		}
		if op.CriticalAirwayAlert {
			m.CriticalAirwayAlerts++
		}
	}
	return m
}
