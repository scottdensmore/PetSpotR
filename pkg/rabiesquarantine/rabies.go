package rabiesquarantine

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// RabiesQuarantineService manages Acute Rabies Quarantine, Lyssavirus Post-Exposure Prophylaxis (PEP),
// Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month Unvaccinated Quarantine, Biosecurity
// Double-Barrier Isolation & Public Health Rabies Control Officer (RCO) Statutory Reporting operations.
type RabiesQuarantineService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.RabiesQuarantineOperation
	order []string
	seq   int
}

// NewRabiesQuarantineService initializes a RabiesQuarantineService with 3 seeded operations
// (RBQ-OP-101, RBQ-OP-102, RBQ-OP-103).
func NewRabiesQuarantineService() *RabiesQuarantineService {
	s := &RabiesQuarantineService{
		ops:   make(map[string]*domain.RabiesQuarantineOperation),
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

// DetermineRequiredQuarantineDays returns the NASPHV Compendium statutory quarantine or observation
// duration in days (10, 45, 120, or 180 days) for a given RabiesStatutoryCategory.
func DetermineRequiredQuarantineDays(category domain.RabiesStatutoryCategory) int {
	switch category {
	case domain.RabiesCategoryTenDayDomesticBiteConfinement:
		return 10
	case domain.RabiesCategoryFortyFiveDayVaccinatedObservation:
		return 45
	case domain.RabiesCategoryFourMonthStrictDoubleBarrier:
		return 120
	case domain.RabiesCategorySixMonthOverdueSerologyQuarantine:
		return 180
	default:
		return 10
	}
}

// IsValidStatutoryQuarantineDays returns true if days is a recognized NASPHV statutory duration (10, 45, 120, or 180).
func IsValidStatutoryQuarantineDays(days int) bool {
	return days == 10 || days == 45 || days == 120 || days == 180
}

// BuildClinicalNarratives constructs the Lyssavirus neuro-pathophysiology summary,
// NASPHV statutory quarantine & RCO reporting note, and cranial nerve / salivary shedding observation note.
func BuildClinicalNarratives(
	category domain.RabiesStatutoryCategory,
	rcoCaseFilingNumber string,
	quarantineDayElapsed, requiredQuarantineDays int,
	rffitTiterIUML float64,
	noVaccineDuring10DayBiteHoldVerified, doubleBarrierLockedEnclosureVerified bool,
	cranialNerveBulbarPalsyAbsent, hypersalivationDysphagiaAbsent bool,
) (patho, rcoNote, neuroNote string) {
	switch category {
	case domain.RabiesCategoryTenDayDomesticBiteConfinement:
		patho = "NASPHV 10-Day Domestic Dog/Cat/Ferret Human-Bite Confinement — Lyssavirus centrifugal salivary gland shedding in dogs, cats, and ferrets begins at most 3–5 days before clinical onset and is invariably fatal within 10 days; survival at Day 10 without encephalitis/bulbar palsy rules out infectious rabies virus in saliva at the time of the bite."
	case domain.RabiesCategoryFortyFiveDayVaccinatedObservation:
		patho = "NASPHV 45-Day Currently Vaccinated Exposure Observation — Domestic animal with current USDA-licensed rabies vaccination exposed to confirmed/suspected rabid wildlife vector (bat, raccoon, skunk, fox, coyote); immediate 15-minute virucidal povidone-iodine wound lavage + IM rabies vaccine booster within 96 hours induces rapid anamnestic neutralizing antibody response (>= 0.5 IU/mL)."
	case domain.RabiesCategoryFourMonthStrictDoubleBarrier:
		patho = "NASPHV 4-Month (120-Day) Strict Double-Barrier Unvaccinated Quarantine — Never-vaccinated dog, cat, or ferret exposed to rabid wildlife vector; requires immediate virucidal wound cleansing, rabies vaccination upon entry (within 96 hours of exposure), and 120 days of strict double-barrier locked enclosure isolation to span the variable centripetal retrograde axonal incubation window."
	case domain.RabiesCategorySixMonthOverdueSerologyQuarantine:
		patho = "Overdue Booster / Undocumented Vaccination Exposure (Prospective Serologic Monitoring vs. 4–6 Month Strict Quarantine) — Paired Day 0 and Day 5–7 Rapid Fluorescent Focus Inhibition Test (RFFIT) serum titers following IM rabies booster verify anamnestic immunological memory (>= 0.5 IU/mL WHO/NASPHV threshold and >= 2-fold rise)."
	default:
		patho = "Acute Lyssavirus Biosecurity Quarantine & Public Health Rabies Control Officer (RCO) Statutory Surveillance."
	}

	titerStatus := "BELOW PROTECTIVE THRESHOLD (< 0.50 IU/mL)"
	if rffitTiterIUML >= 0.5 {
		titerStatus = "SEROPROTECTIVE (>= 0.50 IU/mL WHO/NASPHV Neutralizing Threshold)"
	}
	enclosureStatus := "UNVERIFIED (CRITICAL BIOSECURITY BREACH RISK)"
	if doubleBarrierLockedEnclosureVerified {
		enclosureStatus = "VERIFIED (Padlocked Double-Door Negative-Pressure / Escape-Proof Isolation Kennel + Dedicated PPE & Catch-Pole)"
	}
	vaccineHoldStr := "VERIFIED (No Rabies Vaccine Administered During 10-Day Bite Hold)"
	if !noVaccineDuring10DayBiteHoldVerified {
		vaccineHoldStr = "VIOLATION RISK (Vaccine Must Be Withheld Until Day 10 Release)"
	}

	rcoNote = fmt.Sprintf(
		"Public Health RCO Statutory Case %s | Progress: Day %d of %d Required Days | Double-Barrier Enclosure: %s | RFFIT Neutralizing Titer: %.2f IU/mL (%s) | 10-Day Bite Hold Vaccine Guard: %s.",
		rcoCaseFilingNumber, quarantineDayElapsed, requiredQuarantineDays, enclosureStatus, rffitTiterIUML, titerStatus, vaccineHoldStr,
	)

	if cranialNerveBulbarPalsyAbsent && hypersalivationDysphagiaAbsent {
		neuroNote = fmt.Sprintf(
			"Day %d/%d Serial Contactless Neurological & Salivary Shedding Telemetry: Cranial nerves V, VII, IX, X, XII intact — NO dropped jaw, NO laryngeal/pharyngeal dysphagia, NO foamy hypersalivation, NO photophobia/hydrophobia, NO ascending ataxia, and NO furious/paralytic behavioral shift.",
			quarantineDayElapsed, requiredQuarantineDays,
		)
	} else {
		neuroNote = fmt.Sprintf(
			"CRITICAL LYSSAVIRUS NEUROLOGICAL PROGRESSION ALERT (Day %d/%d): Bulbar cranial nerve palsy, pharyngeal dysphagia/hypersalivation, or acute encephalitic behavioral change detected — notify Public Health RCO (%s) immediately; if humane euthanasia is mandated, preserve intact brainstem, cerebellum, and hippocampus refrigerated at 4°C (NEVER freeze with dry ice or traumatize cranium) for state lab Direct Fluorescent Antibody (DFA) testing.",
			quarantineDayElapsed, requiredQuarantineDays, rcoCaseFilingNumber,
		)
	}

	return patho, rcoNote, neuroNote
}

// BuildDefaultQuarantineStaging constructs an evidence-based RabiesQuarantineStaging plan.
func BuildDefaultQuarantineStaging(
	weightKg float64,
	category domain.RabiesStatutoryCategory,
	protocol domain.RabiesBiosecurityProtocol,
	rcoCaseFilingNumber string,
	requiredDays int,
	rffitTiterIUML float64,
	noVaccineDuring10DayBiteHoldVerified, doubleBarrierLockedEnclosureVerified bool,
	stagedBy string,
	now time.Time,
) domain.RabiesQuarantineStaging {
	if requiredDays <= 0 {
		requiredDays = DetermineRequiredQuarantineDays(category)
	}
	if strings.TrimSpace(rcoCaseFilingNumber) == "" {
		rcoCaseFilingNumber = "RCO-STAT-2026-PENDING"
	}

	tenDayGuard := "NASPHV COMPENDIUM 10-DAY BITE HOLD VACCINE PROHIBITION: Do NOT administer rabies vaccine to any dog, cat, or ferret during the 10-day human-bite observation period — adverse post-vaccinal reactions (fever, lethargy, transient cranial/peripheral neuropathy) can mimic prodromal rabies and force unnecessary euthanasia and brain DFA testing. Administer USDA-licensed rabies vaccine ONLY upon Day 10 veterinary clearance and release."
	if category != domain.RabiesCategoryTenDayDomesticBiteConfinement {
		tenDayGuard = "Post-Exposure Wildlife Vector Quarantine: Immediate IM USDA-licensed rabies vaccine booster within 96 hours of exposure IS indicated (10-day human-bite vaccine withholding applies strictly to animals confined for biting a human)."
	}

	ppePlan := "Double-Barrier Biosecurity Isolation: Padlocked primary stainless-steel run inside a locked secondary anteroom with 'RABIES QUARANTINE — AUTHORIZED VACCINATED PERSONNEL ONLY' signage; mandatory pre-exposure immunized handlers (RFFIT >= 0.5 IU/mL), face shield, heavy Kevlar bite-resistant gauntlets, fluid-impervious gown, contactless catch-pole/transfer door, and 1:32 sodium hypochlorite / quaternary ammonium virucidal disinfection."

	lavagePlan := fmt.Sprintf(
		"Immediate Virucidal Wound Lavage & PEP Protocol (%.1f kg patient): Copious high-volume irrigation and gentle mechanical cleansing of all bite/saliva-contaminated wounds with soap/water and virucidal 1%% povidone-iodine solution for >= 15 minutes; for post-exposure quarantine categories, administer 1.0 mL USDA-licensed inactivated rabies vaccine IM in the right pelvic limb within 96 hours of exposure.",
		weightKg,
	)
	if category == domain.RabiesCategoryTenDayDomesticBiteConfinement {
		lavagePlan = fmt.Sprintf(
			"10-Day Human-Bite Confinement Protocol (%.1f kg patient): Coordinate immediately with local Public Health RCO (%s) for human victim PEP evaluation (HRIG 20 IU/kg infiltrated around wound + Days 0, 3, 7, 14 human diploid cell/PVRV rabies vaccine if animal develops clinical signs); withhold canine/feline rabies vaccine until Day 10 survival clearance.",
			weightKg, rcoCaseFilingNumber,
		)
	}

	rffitPlan := fmt.Sprintf(
		"Rapid Fluorescent Focus Inhibition Test (RFFIT) Serology (Current Titer: %.2f IU/mL; WHO/NASPHV Protective Threshold >= 0.50 IU/mL): Collect paired baseline Day 0 serum prior to IM rabies booster and Day 5–7 post-booster serum to confirm >= 2-fold anamnestic rise and >= 0.50 IU/mL neutralizing antibody concentration.",
		rffitTiterIUML,
	)

	dfaNote := "DFA Neuropathology Preservation Guard: If progressive cranial nerve/bulbar palsy or encephalitis necessitates euthanasia during quarantine, NEVER shoot/crush the skull and NEVER freeze tissues on dry ice — submit intact head or whole cross-sections of brain stem, cerebellum, and hippocampus chilled on wet ice (4°C) to the State Public Health Reference Laboratory for Direct Fluorescent Antibody (DFA) testing."

	if strings.TrimSpace(stagedBy) == "" {
		stagedBy = "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)"
	}

	return domain.RabiesQuarantineStaging{
		Protocol:                             protocol,
		RCOCaseFilingNumber:                  rcoCaseFilingNumber,
		RequiredQuarantineDays:               requiredDays,
		NoVaccineDuring10DayBiteHoldVerified: noVaccineDuring10DayBiteHoldVerified,
		TenDayBiteHoldVaccineGuardNote:       tenDayGuard,
		DoubleBarrierLockedEnclosureVerified: doubleBarrierLockedEnclosureVerified,
		DoubleBarrierBiosecurityPPEPlan:      ppePlan,
		VirucidalWoundLavageBoosterPEPPlan:   lavagePlan,
		ProspectiveSerologicRFFITPlan:        rffitPlan,
		BrainStemHippocampusPreservedGuard:   true,
		DFANeuropathologyPreservationNote:    dfaNote,
		StagedBy:                             stagedBy,
		StagedAt:                             now,
	}
}

func (s *RabiesQuarantineService) seed() {
	now := time.Now().UTC()

	// RBQ-OP-101: Koda — Canine, Alaskan Malamute, 34.0 kg, BIOSECURITY_ISOLATION_RCO_HOLD_STAGED
	patho101, rco101, neuro101 := BuildClinicalNarratives(
		domain.RabiesCategoryTenDayDomesticBiteConfinement,
		"RCO-STAT-2026-101",
		2, 10,
		0.25,
		true, true,
		true, true,
	)
	stage101 := BuildDefaultQuarantineStaging(
		34.0,
		domain.RabiesCategoryTenDayDomesticBiteConfinement,
		domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		"RCO-STAT-2026-101",
		10,
		0.25,
		true, true,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		now.Add(-40*time.Minute),
	)
	op101 := &domain.RabiesQuarantineOperation{
		ID:                                   "RBQ-OP-101",
		CaseID:                               "CASE-RBQ-101",
		PetName:                              "Koda",
		Species:                              "Canine",
		Breed:                                "Alaskan Malamute",
		WeightKg:                             34.0,
		Status:                               domain.RabiesStatusBiosecurityIsolationRCOHoldStaged,
		StatutoryCategory:                    domain.RabiesCategoryTenDayDomesticBiteConfinement,
		Protocol:                             domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		RCOCaseFilingNumber:                  "RCO-STAT-2026-101",
		QuarantineDayElapsed:                 2,
		RequiredQuarantineDays:               10,
		RFFITTiterIUML:                       0.25,
		NoVaccineDuring10DayBiteHoldVerified: true,
		DoubleBarrierLockedEnclosureVerified: true,
		CranialNerveBulbarPalsyAbsent:        true,
		HypersalivationDysphagiaAbsent:       true,
		BrainStemHippocampusPreservedGuard:   true,
		SeroprotectiveTiterVerified:          false,
		PathophysiologySummary:               patho101,
		StatutoryQuarantineRCONote:           rco101,
		NeuroSalivarySheddingNote:            neuro101,
		QuarantineStaging:                    stage101,
		NeuroObservationChecks: []domain.RabiesNeuroObservationCheck{
			{
				ID:                                   "RBQ-CHK-101-1",
				QuarantineDayElapsed:                 2,
				RFFITTiterIUML:                       0.25,
				CranialNerveBulbarPalsyAbsent:        true,
				HypersalivationDysphagiaAbsent:       true,
				DoubleBarrierLockedEnclosureVerified: true,
				NoVaccineDuring10DayBiteHoldVerified: true,
				RecordedBy:                           "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
				ClinicalNote:                         "Day 2 of 10 statutory human-bite confinement (RCO-STAT-2026-101): bright, alert, drinking water normally without dysphagia or hypersalivation, cranial nerves V/VII/IX/X/XII intact; rabies vaccine strictly withheld during 10-day observation per NASPHV Compendium.",
				RecordedAt:                           now.Add(-40 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-40 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// RBQ-OP-102: Vesper — Feline, Domestic Shorthair, 4.6 kg, SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED
	patho102, rco102, neuro102 := BuildClinicalNarratives(
		domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		"RCO-STAT-2026-102",
		42, 120,
		0.85,
		true, true,
		true, true,
	)
	stage102 := BuildDefaultQuarantineStaging(
		4.6,
		domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		domain.RabiesProtocolDoubleBarrierLockedCagePPEHandling,
		"RCO-STAT-2026-102",
		120,
		0.85,
		true, true,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		now.Add(-90*time.Minute),
	)
	op102 := &domain.RabiesQuarantineOperation{
		ID:                                   "RBQ-OP-102",
		CaseID:                               "CASE-RBQ-102",
		PetName:                              "Vesper",
		Species:                              "Feline",
		Breed:                                "Domestic Shorthair",
		WeightKg:                             4.6,
		Status:                               domain.RabiesStatusSerialNeuroSalivarySheddingMonitored,
		StatutoryCategory:                    domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		Protocol:                             domain.RabiesProtocolDoubleBarrierLockedCagePPEHandling,
		RCOCaseFilingNumber:                  "RCO-STAT-2026-102",
		QuarantineDayElapsed:                 42,
		RequiredQuarantineDays:               120,
		RFFITTiterIUML:                       0.85,
		NoVaccineDuring10DayBiteHoldVerified: true,
		DoubleBarrierLockedEnclosureVerified: true,
		CranialNerveBulbarPalsyAbsent:        true,
		HypersalivationDysphagiaAbsent:       true,
		BrainStemHippocampusPreservedGuard:   true,
		SeroprotectiveTiterVerified:          true,
		PathophysiologySummary:               patho102,
		StatutoryQuarantineRCONote:           rco102,
		NeuroSalivarySheddingNote:            neuro102,
		QuarantineStaging:                    stage102,
		NeuroObservationChecks: []domain.RabiesNeuroObservationCheck{
			{
				ID:                                   "RBQ-CHK-102-1",
				QuarantineDayElapsed:                 42,
				RFFITTiterIUML:                       0.85,
				CranialNerveBulbarPalsyAbsent:        true,
				HypersalivationDysphagiaAbsent:       true,
				DoubleBarrierLockedEnclosureVerified: true,
				NoVaccineDuring10DayBiteHoldVerified: true,
				RecordedBy:                           "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
				ClinicalNote:                         "Day 42 of 120 strict 4-month unvaccinated wildlife-exposure double-barrier quarantine (bat bite to right pinna; 15-min povidone-iodine lavage + entry IM rabies vaccine within 96h): RFFIT seroconversion verified at 0.85 IU/mL (>= 0.50 IU/mL), no bulbar palsy or behavioral aggression.",
				RecordedAt:                           now.Add(-75 * time.Minute),
			},
		},
		CreatedAt: now.Add(-4 * time.Hour),
		UpdatedAt: now.Add(-75 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// RBQ-OP-103: Artemis — Canine, German Shepherd Dog, 31.5 kg, STATUTORY_QUARANTINE_SURVIVED_CLEARED
	patho103, rco103, neuro103 := BuildClinicalNarratives(
		domain.RabiesCategoryFortyFiveDayVaccinatedObservation,
		"RCO-STAT-2026-103",
		45, 45,
		2.40,
		true, true,
		true, true,
	)
	stage103 := BuildDefaultQuarantineStaging(
		31.5,
		domain.RabiesCategoryFortyFiveDayVaccinatedObservation,
		domain.RabiesProtocolImmediateIMBooster96HrWoundLavage,
		"RCO-STAT-2026-103",
		45,
		2.40,
		true, true,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		now.Add(-12*time.Hour),
	)
	clearedAt := now.Add(-25 * time.Minute)
	clearance103 := &domain.RabiesQuarantineClearance{
		QuarantineDayElapsed:          45,
		RequiredQuarantineDays:        45,
		CranialNerveBulbarPalsyAbsent: true,
		RCOWrittenReleaseSigned:       true,
		ExitRabiesVaccinationVerified: true,
		RFFITTiterIUML:                2.40,
		ClearedBy:                     "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		ClearanceSummary:              "Completed full 45-day NASPHV post-exposure observation following rabid raccoon altercation and immediate <96h IM rabies booster; Day 45/45 neurological exam 100% normal (cranial nerve/bulbar palsy absent), RFFIT 2.40 IU/mL (>= 0.50 IU/mL), RCO statutory release RCO-STAT-2026-103 signed and exit vaccination verified.",
		ClearedAt:                     clearedAt,
	}
	op103 := &domain.RabiesQuarantineOperation{
		ID:                                   "RBQ-OP-103",
		CaseID:                               "CASE-RBQ-103",
		PetName:                              "Artemis",
		Species:                              "Canine",
		Breed:                                "German Shepherd Dog",
		WeightKg:                             31.5,
		Status:                               domain.RabiesStatusStatutoryQuarantineSurvivedCleared,
		StatutoryCategory:                    domain.RabiesCategoryFortyFiveDayVaccinatedObservation,
		Protocol:                             domain.RabiesProtocolImmediateIMBooster96HrWoundLavage,
		RCOCaseFilingNumber:                  "RCO-STAT-2026-103",
		QuarantineDayElapsed:                 45,
		RequiredQuarantineDays:               45,
		RFFITTiterIUML:                       2.40,
		NoVaccineDuring10DayBiteHoldVerified: true,
		DoubleBarrierLockedEnclosureVerified: true,
		CranialNerveBulbarPalsyAbsent:        true,
		HypersalivationDysphagiaAbsent:       true,
		BrainStemHippocampusPreservedGuard:   true,
		SeroprotectiveTiterVerified:          true,
		PathophysiologySummary:               patho103,
		StatutoryQuarantineRCONote:           rco103,
		NeuroSalivarySheddingNote:            neuro103,
		QuarantineStaging:                    stage103,
		NeuroObservationChecks: []domain.RabiesNeuroObservationCheck{
			{
				ID:                                   "RBQ-CHK-103-1",
				QuarantineDayElapsed:                 45,
				RFFITTiterIUML:                       2.40,
				CranialNerveBulbarPalsyAbsent:        true,
				HypersalivationDysphagiaAbsent:       true,
				DoubleBarrierLockedEnclosureVerified: true,
				NoVaccineDuring10DayBiteHoldVerified: true,
				RecordedBy:                           "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
				ClinicalNote:                         "Day 45 of 45 statutory observation completed: zero neurological deficits, normal deglutition and vocalization, RFFIT neutralizing titer 2.40 IU/mL; signed Public Health RCO statutory clearance.",
				RecordedAt:                           now.Add(-35 * time.Minute),
			},
		},
		Clearance:        clearance103,
		DischargedBy:     clearance103.ClearedBy,
		DischargeSummary: clearance103.ClearanceSummary,
		DischargedAt:     &clearedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Rabies Quarantine operations, optionally filtered by statusFilter.
func (s *RabiesQuarantineService) ListOperations(statusFilter string) []domain.RabiesQuarantineOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.RabiesQuarantineOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneRabiesOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Rabies Quarantine operation by ID.
func (s *RabiesQuarantineService) GetOperation(id string) (*domain.RabiesQuarantineOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("rabies quarantine operation %q not found", id)
	}
	cp := cloneRabiesOperation(op)
	return &cp, nil
}

// CalculateOverview computes summary telemetry metrics across all Rabies Quarantine operations.
func (s *RabiesQuarantineService) CalculateOverview() domain.RabiesQuarantineOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.RabiesQuarantineOverview
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.RabiesStatusBiosecurityIsolationRCOHoldStaged:
			m.BiosecurityIsolationRCOHoldStagedCount++
		case domain.RabiesStatusSerialNeuroSalivarySheddingMonitored:
			m.SerialNeuroSalivaryMonitoredCount++
		case domain.RabiesStatusStatutoryQuarantineSurvivedCleared:
			m.StatutoryQuarantineSurvivedClearedCount++
		}
		if op.StatutoryCategory == domain.RabiesCategoryTenDayDomesticBiteConfinement {
			m.TenDayDomesticBiteHoldCount++
		}
		if op.StatutoryCategory == domain.RabiesCategoryFourMonthStrictDoubleBarrier {
			m.FourMonthStrictUnvaccinatedCount++
		}
		if op.RFFITTiterIUML >= 0.5 {
			m.ProtectiveRFFITTiterCount++
		}
	}
	return m
}

// GetOverviewMetrics is an alias for CalculateOverview for handler consistency.
func (s *RabiesQuarantineService) GetOverviewMetrics() domain.RabiesQuarantineOverview {
	return s.CalculateOverview()
}

// CreateOperation validates and creates a new Acute Rabies Quarantine & PEP triage operation.
func (s *RabiesQuarantineService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	statutoryCategory domain.RabiesStatutoryCategory,
	protocol domain.RabiesBiosecurityProtocol,
	rcoCaseFilingNumber string,
	quarantineDayElapsed, requiredQuarantineDays int,
	rffitTiterIUML float64,
	noVaccineDuring10DayBiteHoldVerified, doubleBarrierLockedEnclosureVerified bool,
) (*domain.RabiesQuarantineOperation, error) {
	trimmedCase := strings.TrimSpace(caseID)
	if trimmedCase == "" {
		return nil, fmt.Errorf("case_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	trimmedSpecies := strings.TrimSpace(species)
	if trimmedSpecies == "" {
		trimmedSpecies = "Canine"
	}
	trimmedBreed := strings.TrimSpace(breed)
	if trimmedBreed == "" {
		trimmedBreed = "Mixed Breed"
	}
	if weightKg <= 0 || weightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %v", weightKg)
	}
	if !statutoryCategory.IsValid() {
		return nil, fmt.Errorf("invalid statutory_category: %q", statutoryCategory)
	}
	if protocol == "" {
		protocol = domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	trimmedRCO := strings.TrimSpace(rcoCaseFilingNumber)
	if trimmedRCO == "" {
		return nil, fmt.Errorf("rco_case_filing_number is required")
	}
	if requiredQuarantineDays == 0 {
		requiredQuarantineDays = DetermineRequiredQuarantineDays(statutoryCategory)
	}
	if !IsValidStatutoryQuarantineDays(requiredQuarantineDays) {
		return nil, fmt.Errorf("required_quarantine_days must be 10, 45, 120, or 180 per NASPHV Compendium, got %d", requiredQuarantineDays)
	}
	if quarantineDayElapsed < 0 || quarantineDayElapsed > 200 {
		return nil, fmt.Errorf("quarantine_day_elapsed must be in [0, 200], got %d", quarantineDayElapsed)
	}
	if rffitTiterIUML < 0 || rffitTiterIUML > 50.0 {
		return nil, fmt.Errorf("rffit_titer_iu_ml must be in [0, 50.0], got %v", rffitTiterIUML)
	}
	// Safety Guard: NASPHV 10-Day Domestic Bite Confinement strictly prohibits rabies vaccination during the 10-day hold
	if statutoryCategory == domain.RabiesCategoryTenDayDomesticBiteConfinement && !noVaccineDuring10DayBiteHoldVerified {
		return nil, fmt.Errorf("NASPHV Compendium 10-Day Bite Hold Violation: a dog, cat, or ferret confined for biting a human MUST NOT receive a rabies vaccine during the 10-day observation period (no_vaccine_during_10day_bite_hold_verified must be true)")
	}
	if !doubleBarrierLockedEnclosureVerified {
		return nil, fmt.Errorf("double_barrier_locked_enclosure_verified must be true to admit a rabies quarantine patient")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("RBQ-OP-%d", s.seq)
	now := time.Now().UTC()

	patho, rcoNote, neuroNote := BuildClinicalNarratives(
		statutoryCategory,
		trimmedRCO,
		quarantineDayElapsed,
		requiredQuarantineDays,
		rffitTiterIUML,
		noVaccineDuring10DayBiteHoldVerified,
		doubleBarrierLockedEnclosureVerified,
		true,
		true,
	)
	staging := BuildDefaultQuarantineStaging(
		weightKg,
		statutoryCategory,
		protocol,
		trimmedRCO,
		requiredQuarantineDays,
		rffitTiterIUML,
		noVaccineDuring10DayBiteHoldVerified,
		doubleBarrierLockedEnclosureVerified,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		now,
	)

	op := &domain.RabiesQuarantineOperation{
		ID:                                   opID,
		CaseID:                               trimmedCase,
		PetName:                              trimmedPet,
		Species:                              trimmedSpecies,
		Breed:                                trimmedBreed,
		WeightKg:                             round1(weightKg),
		Status:                               domain.RabiesStatusBiosecurityIsolationRCOHoldStaged,
		StatutoryCategory:                    statutoryCategory,
		Protocol:                             protocol,
		RCOCaseFilingNumber:                  trimmedRCO,
		QuarantineDayElapsed:                 quarantineDayElapsed,
		RequiredQuarantineDays:               requiredQuarantineDays,
		RFFITTiterIUML:                       round2(rffitTiterIUML),
		NoVaccineDuring10DayBiteHoldVerified: noVaccineDuring10DayBiteHoldVerified,
		DoubleBarrierLockedEnclosureVerified: doubleBarrierLockedEnclosureVerified,
		CranialNerveBulbarPalsyAbsent:        true,
		HypersalivationDysphagiaAbsent:       true,
		BrainStemHippocampusPreservedGuard:   true,
		SeroprotectiveTiterVerified:          rffitTiterIUML >= 0.5,
		PathophysiologySummary:               patho,
		StatutoryQuarantineRCONote:           rcoNote,
		NeuroSalivarySheddingNote:            neuroNote,
		QuarantineStaging:                    staging,
		NeuroObservationChecks:               []domain.RabiesNeuroObservationCheck{},
		CreatedAt:                            now,
		UpdatedAt:                            now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneRabiesOperation(op)
	return &cp, nil
}

// StageQuarantine stages biosecurity double-barrier isolation, Public Health RCO statutory case number,
// NASPHV 10-day bite-hold vaccine withholding guard, virucidal povidone-iodine wound lavage & PEP plan,
// prospective serologic RFFIT monitoring plan, and DFA brainstem/hippocampus preservation guard.
func (s *RabiesQuarantineService) StageQuarantine(
	id string,
	protocol domain.RabiesBiosecurityProtocol,
	rcoCaseFilingNumber string,
	requiredQuarantineDays int,
	noVaccineDuring10DayBiteHoldVerified, doubleBarrierLockedEnclosureVerified bool,
	doubleBarrierPPEPlan, woundLavagePEPPlan, rffitPlan string,
	brainStemHippocampusPreservedGuard bool,
	dfaPreservationNote, stagedBy string,
) (*domain.RabiesQuarantineOperation, error) {
	if protocol == "" {
		protocol = domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	trimmedRCO := strings.TrimSpace(rcoCaseFilingNumber)
	if trimmedRCO == "" {
		return nil, fmt.Errorf("rco_case_filing_number is required")
	}
	if !doubleBarrierLockedEnclosureVerified {
		return nil, fmt.Errorf("double_barrier_locked_enclosure_verified must be true (padlocked double-barrier isolation enclosure + PPE required)")
	}
	if !brainStemHippocampusPreservedGuard {
		return nil, fmt.Errorf("brain_stem_hippocampus_preserved_guard must be true (intact brainstem/cerebellum/hippocampus chilled at 4°C without dry-ice freezing required for DFA testing)")
	}
	trimmedPPE := strings.TrimSpace(doubleBarrierPPEPlan)
	if trimmedPPE == "" {
		return nil, fmt.Errorf("double_barrier_biosecurity_ppe_plan is required")
	}
	trimmedLavage := strings.TrimSpace(woundLavagePEPPlan)
	if trimmedLavage == "" {
		return nil, fmt.Errorf("virucidal_wound_lavage_booster_pep_plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("rabies quarantine operation %q not found", id)
	}

	if requiredQuarantineDays == 0 {
		requiredQuarantineDays = op.RequiredQuarantineDays
		if requiredQuarantineDays == 0 {
			requiredQuarantineDays = DetermineRequiredQuarantineDays(op.StatutoryCategory)
		}
	}
	if !IsValidStatutoryQuarantineDays(requiredQuarantineDays) {
		return nil, fmt.Errorf("required_quarantine_days must be 10, 45, 120, or 180 per NASPHV Compendium, got %d", requiredQuarantineDays)
	}

	// Safety Guard: NASPHV Compendium 10-Day Bite Hold Vaccine Prohibition
	if op.StatutoryCategory == domain.RabiesCategoryTenDayDomesticBiteConfinement &&
		(!noVaccineDuring10DayBiteHoldVerified || protocol == domain.RabiesProtocolImmediateIMBooster96HrWoundLavage) {
		if !noVaccineDuring10DayBiteHoldVerified {
			return nil, fmt.Errorf("NASPHV Compendium 10-Day Bite Hold Violation: a dog, cat, or ferret confined for biting a human MUST NOT receive a rabies vaccine during the 10-day observation period; vaccinate only upon Day 10 release")
		}
	}

	now := time.Now().UTC()
	defaults := BuildDefaultQuarantineStaging(
		op.WeightKg,
		op.StatutoryCategory,
		protocol,
		trimmedRCO,
		requiredQuarantineDays,
		op.RFFITTiterIUML,
		noVaccineDuring10DayBiteHoldVerified,
		doubleBarrierLockedEnclosureVerified,
		stagedBy,
		now,
	)

	trimmedRFFIT := strings.TrimSpace(rffitPlan)
	if trimmedRFFIT == "" {
		trimmedRFFIT = defaults.ProspectiveSerologicRFFITPlan
	}
	trimmedDFA := strings.TrimSpace(dfaPreservationNote)
	if trimmedDFA == "" {
		trimmedDFA = defaults.DFANeuropathologyPreservationNote
	}
	trimmedBy := strings.TrimSpace(stagedBy)
	if trimmedBy == "" {
		trimmedBy = defaults.StagedBy
	}

	op.Protocol = protocol
	op.RCOCaseFilingNumber = trimmedRCO
	op.RequiredQuarantineDays = requiredQuarantineDays
	op.NoVaccineDuring10DayBiteHoldVerified = noVaccineDuring10DayBiteHoldVerified
	op.DoubleBarrierLockedEnclosureVerified = doubleBarrierLockedEnclosureVerified
	op.BrainStemHippocampusPreservedGuard = brainStemHippocampusPreservedGuard
	op.QuarantineStaging = domain.RabiesQuarantineStaging{
		Protocol:                             protocol,
		RCOCaseFilingNumber:                  trimmedRCO,
		RequiredQuarantineDays:               requiredQuarantineDays,
		NoVaccineDuring10DayBiteHoldVerified: noVaccineDuring10DayBiteHoldVerified,
		TenDayBiteHoldVaccineGuardNote:       defaults.TenDayBiteHoldVaccineGuardNote,
		DoubleBarrierLockedEnclosureVerified: doubleBarrierLockedEnclosureVerified,
		DoubleBarrierBiosecurityPPEPlan:      trimmedPPE,
		VirucidalWoundLavageBoosterPEPPlan:   trimmedLavage,
		ProspectiveSerologicRFFITPlan:        trimmedRFFIT,
		BrainStemHippocampusPreservedGuard:   brainStemHippocampusPreservedGuard,
		DFANeuropathologyPreservationNote:    trimmedDFA,
		StagedBy:                             trimmedBy,
		StagedAt:                             now,
	}

	patho, rcoNote, neuroNote := BuildClinicalNarratives(
		op.StatutoryCategory,
		op.RCOCaseFilingNumber,
		op.QuarantineDayElapsed,
		op.RequiredQuarantineDays,
		op.RFFITTiterIUML,
		op.NoVaccineDuring10DayBiteHoldVerified,
		op.DoubleBarrierLockedEnclosureVerified,
		op.CranialNerveBulbarPalsyAbsent,
		op.HypersalivationDysphagiaAbsent,
	)
	op.PathophysiologySummary = patho
	op.StatutoryQuarantineRCONote = rcoNote
	op.NeuroSalivarySheddingNote = neuroNote
	op.Status = domain.RabiesStatusSerialNeuroSalivarySheddingMonitored
	op.Clearance = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = now

	cp := cloneRabiesOperation(op)
	return &cp, nil
}

// RecordNeuroObservationCheck appends a serial RabiesNeuroObservationCheck and updates
// quarantine day elapsed, RFFIT neutralizing titer, cranial nerve / bulbar palsy status,
// and salivary shedding telemetry on the operation.
func (s *RabiesQuarantineService) RecordNeuroObservationCheck(
	id string,
	quarantineDayElapsed int,
	rffitTiterIUML float64,
	cranialNerveBulbarPalsyAbsent, hypersalivationDysphagiaAbsent bool,
	doubleBarrierLockedEnclosureVerified, noVaccineDuring10DayBiteHoldVerified bool,
	recordedBy, clinicalNote string,
) (*domain.RabiesQuarantineOperation, error) {
	if quarantineDayElapsed < 0 || quarantineDayElapsed > 200 {
		return nil, fmt.Errorf("quarantine_day_elapsed must be in [0, 200], got %d", quarantineDayElapsed)
	}
	if rffitTiterIUML < 0 || rffitTiterIUML > 50.0 {
		return nil, fmt.Errorf("rffit_titer_iu_ml must be in [0, 50.0], got %v", rffitTiterIUML)
	}
	if !doubleBarrierLockedEnclosureVerified {
		return nil, fmt.Errorf("double_barrier_locked_enclosure_verified must be true during serial neuro observation")
	}
	trimmedBy := strings.TrimSpace(recordedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("recorded_by is required")
	}
	trimmedNote := strings.TrimSpace(clinicalNote)

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("rabies quarantine operation %q not found", id)
	}

	if op.StatutoryCategory == domain.RabiesCategoryTenDayDomesticBiteConfinement &&
		quarantineDayElapsed < op.RequiredQuarantineDays &&
		!noVaccineDuring10DayBiteHoldVerified {
		return nil, fmt.Errorf("NASPHV Compendium 10-Day Bite Hold Violation: no_vaccine_during_10day_bite_hold_verified must remain true throughout the 10-day observation period")
	}

	now := time.Now().UTC()
	chk := domain.RabiesNeuroObservationCheck{
		ID:                                   fmt.Sprintf("RBQ-CHK-%s-%d", op.ID, len(op.NeuroObservationChecks)+1),
		QuarantineDayElapsed:                 quarantineDayElapsed,
		RFFITTiterIUML:                       round2(rffitTiterIUML),
		CranialNerveBulbarPalsyAbsent:        cranialNerveBulbarPalsyAbsent,
		HypersalivationDysphagiaAbsent:       hypersalivationDysphagiaAbsent,
		DoubleBarrierLockedEnclosureVerified: doubleBarrierLockedEnclosureVerified,
		NoVaccineDuring10DayBiteHoldVerified: noVaccineDuring10DayBiteHoldVerified,
		RecordedBy:                           trimmedBy,
		ClinicalNote:                         trimmedNote,
		RecordedAt:                           now,
	}

	op.NeuroObservationChecks = append(op.NeuroObservationChecks, chk)
	op.QuarantineDayElapsed = chk.QuarantineDayElapsed
	op.RFFITTiterIUML = chk.RFFITTiterIUML
	op.CranialNerveBulbarPalsyAbsent = chk.CranialNerveBulbarPalsyAbsent
	op.HypersalivationDysphagiaAbsent = chk.HypersalivationDysphagiaAbsent
	op.DoubleBarrierLockedEnclosureVerified = chk.DoubleBarrierLockedEnclosureVerified
	op.NoVaccineDuring10DayBiteHoldVerified = chk.NoVaccineDuring10DayBiteHoldVerified
	op.SeroprotectiveTiterVerified = chk.RFFITTiterIUML >= 0.5

	patho, rcoNote, neuroNote := BuildClinicalNarratives(
		op.StatutoryCategory,
		op.RCOCaseFilingNumber,
		op.QuarantineDayElapsed,
		op.RequiredQuarantineDays,
		op.RFFITTiterIUML,
		op.NoVaccineDuring10DayBiteHoldVerified,
		op.DoubleBarrierLockedEnclosureVerified,
		op.CranialNerveBulbarPalsyAbsent,
		op.HypersalivationDysphagiaAbsent,
	)
	op.PathophysiologySummary = patho
	op.StatutoryQuarantineRCONote = rcoNote
	op.NeuroSalivarySheddingNote = neuroNote
	if op.Status == domain.RabiesStatusBiosecurityIsolationRCOHoldStaged {
		op.Status = domain.RabiesStatusSerialNeuroSalivarySheddingMonitored
	}
	op.UpdatedAt = now

	cp := cloneRabiesOperation(op)
	return &cp, nil
}

// EvaluateClearance verifies that statutory quarantine survival duration is complete
// (quarantine_day_elapsed >= required_quarantine_days), cranial nerve / bulbar palsy is absent
// (cranial_nerve_bulbar_palsy_absent == true), Public Health RCO written release is signed
// (rco_written_release_signed == true), and exit rabies vaccination is verified
// (exit_rabies_vaccination_verified == true), transitioning status to STATUTORY_QUARANTINE_SURVIVED_CLEARED.
func (s *RabiesQuarantineService) EvaluateClearance(
	id string,
	quarantineDayElapsed int,
	cranialNerveBulbarPalsyAbsent, rcoWrittenReleaseSigned, exitRabiesVaccinationVerified bool,
	rffitTiterIUML float64,
	clearedBy, clearanceSummary string,
) (*domain.RabiesQuarantineOperation, error) {
	if !cranialNerveBulbarPalsyAbsent {
		return nil, fmt.Errorf("cannot clear rabies quarantine operation: cranial_nerve_bulbar_palsy_absent must be true (active neurological/bulbar signs require RCO hold or DFA brain testing)")
	}
	if !rcoWrittenReleaseSigned {
		return nil, fmt.Errorf("cannot clear rabies quarantine operation: rco_written_release_signed must be true (Public Health Rabies Control Officer statutory release signature required)")
	}
	if !exitRabiesVaccinationVerified {
		return nil, fmt.Errorf("cannot clear rabies quarantine operation: exit_rabies_vaccination_verified must be true (USDA-licensed rabies vaccination upon release required)")
	}
	if rffitTiterIUML < 0 || rffitTiterIUML > 50.0 {
		return nil, fmt.Errorf("rffit_titer_iu_ml must be in [0, 50.0], got %v", rffitTiterIUML)
	}
	trimmedBy := strings.TrimSpace(clearedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("cleared_by is required")
	}
	trimmedSummary := strings.TrimSpace(clearanceSummary)
	if trimmedSummary == "" {
		return nil, fmt.Errorf("clearance_summary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("rabies quarantine operation %q not found", id)
	}

	reqDays := op.RequiredQuarantineDays
	if reqDays <= 0 {
		reqDays = DetermineRequiredQuarantineDays(op.StatutoryCategory)
	}
	if quarantineDayElapsed < reqDays {
		return nil, fmt.Errorf("cannot clear rabies quarantine operation: quarantine_day_elapsed (%d) must be >= required_quarantine_days (%d)", quarantineDayElapsed, reqDays)
	}

	now := time.Now().UTC()
	op.QuarantineDayElapsed = quarantineDayElapsed
	op.RequiredQuarantineDays = reqDays
	op.CranialNerveBulbarPalsyAbsent = true
	op.HypersalivationDysphagiaAbsent = true
	if rffitTiterIUML > 0 {
		op.RFFITTiterIUML = round2(rffitTiterIUML)
	}
	op.SeroprotectiveTiterVerified = op.RFFITTiterIUML >= 0.5

	patho, rcoNote, neuroNote := BuildClinicalNarratives(
		op.StatutoryCategory,
		op.RCOCaseFilingNumber,
		op.QuarantineDayElapsed,
		op.RequiredQuarantineDays,
		op.RFFITTiterIUML,
		op.NoVaccineDuring10DayBiteHoldVerified,
		op.DoubleBarrierLockedEnclosureVerified,
		op.CranialNerveBulbarPalsyAbsent,
		op.HypersalivationDysphagiaAbsent,
	)
	op.PathophysiologySummary = patho
	op.StatutoryQuarantineRCONote = rcoNote
	op.NeuroSalivarySheddingNote = neuroNote

	clearance := &domain.RabiesQuarantineClearance{
		QuarantineDayElapsed:          op.QuarantineDayElapsed,
		RequiredQuarantineDays:        op.RequiredQuarantineDays,
		CranialNerveBulbarPalsyAbsent: true,
		RCOWrittenReleaseSigned:       true,
		ExitRabiesVaccinationVerified: true,
		RFFITTiterIUML:                op.RFFITTiterIUML,
		ClearedBy:                     trimmedBy,
		ClearanceSummary:              trimmedSummary,
		ClearedAt:                     now,
	}
	op.Clearance = clearance
	op.Status = domain.RabiesStatusStatutoryQuarantineSurvivedCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneRabiesOperation(op)
	return &cp, nil
}

func cloneRabiesOperation(op *domain.RabiesQuarantineOperation) domain.RabiesQuarantineOperation {
	cp := *op
	if op.NeuroObservationChecks != nil {
		cp.NeuroObservationChecks = make([]domain.RabiesNeuroObservationCheck, len(op.NeuroObservationChecks))
		copy(cp.NeuroObservationChecks, op.NeuroObservationChecks)
	}
	if op.Clearance != nil {
		cl := *op.Clearance
		cp.Clearance = &cl
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
