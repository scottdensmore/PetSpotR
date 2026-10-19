package masscasualtyvmat

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// MassCasualtyVMATService manages Acute Disaster Mass-Casualty Veterinary Triage
// (START/SALT Veterinary Color-Tagging: Immediate Red, Delayed Yellow, Minimal Green, Expectant Black),
// Decontamination Corridor, Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment operations
// in a thread-safe store.
type MassCasualtyVMATService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.MassCasualtyVMATOperation
	order []string
	seq   int
}

// NewMassCasualtyVMATService initializes a MassCasualtyVMATService with 3 seeded operations
// (MCI-OP-101, MCI-OP-102, MCI-OP-103).
func NewMassCasualtyVMATService() *MassCasualtyVMATService {
	s := &MassCasualtyVMATService{
		ops:   make(map[string]*domain.MassCasualtyVMATOperation),
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

// IsReverseTriageSurgeTriggered returns true when field hospital surge bed occupancy is >= 85.0%,
// triggering reverse triage (routing stable Green Minimal and Yellow Delayed patients to secondary
// staging shelters to preserve Red Immediate oxygen cages, ventilators, and surgical tables).
func IsReverseTriageSurgeTriggered(surgeBedOccupancyPercent float64) bool {
	return surgeBedOccupancyPercent >= 85.0
}

// BuildClinicalNarratives constructs the START/SALT triage summary, Hot/Warm/Cold zone decontamination
// corridor & biosecurity note, and FEMA/AVMA VMAT ICS-214 surge capacity command note.
func BuildClinicalNarratives(
	triageTag domain.VeterinarySTARTTag,
	animalTriageScoreATS int,
	lactateMmolL, spo2Percent, surgeBedOccupancyPercent float64,
	grossDeconVerified, contagiousCohortSegregated bool,
	rfidCollar, ics214Number string,
) (startSaltSummary, deconBiosecurityNote, surgeICSNote string) {
	switch triageTag {
	case domain.STARTTagRedImmediatePriority1LifeThreat:
		startSaltSummary = fmt.Sprintf("RED — IMMEDIATE (Priority 1 Life-Threatening Airway/Breathing/Circulation/Hemorrhage/Decompensated Shock | Veterinary ATS %d/25 | Lactate %.2f mmol/L | SpO2 %.1f%%): Immediate life-saving intervention required within < 5 minutes — secure patent airway, thoracocentesis for tension pneumothorax, direct arterial pressure/tourniquet for exsanguinating hemorrhage, flow-by/cage oxygen, and hypertonic saline 4 mL/kg IV or balanced crystalloid shock bolus.", animalTriageScoreATS, lactateMmolL, spo2Percent)
	case domain.STARTTagYellowDelayedPriority2UrgentSurgery:
		startSaltSummary = fmt.Sprintf("YELLOW — DELAYED (Priority 2 Urgent Surgical/Medical Stabilization | Veterinary ATS %d/25 | Lactate %.2f mmol/L | SpO2 %.1f%%): Serious injuries (open long-bone fractures, non-exsanguinating degloving wounds, moderate smoke/thermal burns) that are hemodynamically stable enough to tolerate a 45–120 minute surgical delay while Red Immediate patients occupy resuscitation bays.", animalTriageScoreATS, lactateMmolL, spo2Percent)
	case domain.STARTTagGreenMinimalPriority3WalkingWounded:
		startSaltSummary = fmt.Sprintf("GREEN — MINIMAL (Priority 3 'Walking Wounded' / Minor Ambulatory Trauma | Veterinary ATS %d/25 | Lactate %.2f mmol/L | SpO2 %.1f%%): Superficial abrasions, minor paw pad singeing, or mild disaster stress with normal perfusion and oxygenation; cohort in secondary holding shelter for wound lavage, tetanus/antimicrobial prophylaxis, microchip scan, and owner reunification.", animalTriageScoreATS, lactateMmolL, spo2Percent)
	case domain.STARTTagBlackExpectantAnalgesiaPalliative:
		startSaltSummary = fmt.Sprintf("BLACK — EXPECTANT / PALLIATIVE (Priority 4 Non-Survivable Catastrophic Polytrauma / Agonal | Veterinary ATS %d/25 | Lactate %.2f mmol/L | SpO2 %.1f%%): Unresponsive agonal presentation or catastrophic multi-system trauma incompatible with survival given finite MCI surge resources; provide immediate high-dose full-mu opioid analgesia (Methadone/Fentanyl/Hydromorphone) and humane palliative comfort.", animalTriageScoreATS, lactateMmolL, spo2Percent)
	default:
		startSaltSummary = fmt.Sprintf("Veterinary START/SALT Mass-Casualty Triage Assessment (ATS %d/25 | Lactate %.2f mmol/L | SpO2 %.1f%%).", animalTriageScoreATS, lactateMmolL, spo2Percent)
	}

	deconStr := "PENDING WARM-ZONE GROSS DECONTAMINATION (BLOCKED FROM COLD-ZONE ICU)"
	if grossDeconVerified {
		deconStr = "VERIFIED WARM-ZONE DAWN/SALINE GROSS DECONTAMINATION COMPLETED"
	}
	bioStr := "PENDING BIOSECURITY COHORT SEGREGATION"
	if contagiousCohortSegregated {
		bioStr = "ISOLATION COHORT SEGREGATED (Leptospirosis/Parvovirus/URI Biosecurity Perimeter Enforced)"
	}
	deconBiosecurityNote = fmt.Sprintf("Hot/Warm/Cold Zone Decontamination Corridor: %s | Biosecurity Status: %s — Warm-zone low-pressure tepid water + mild dish detergent (Dawn) dermal/ocular/paw flush removes petrochemical hydrocarbons, organophosphates, caustic wildfire ash, and floodwater sewage toxins prior to Cold-Zone VMAT ICU entry.", deconStr, bioStr)

	reverseSurge := IsReverseTriageSurgeTriggered(surgeBedOccupancyPercent)
	surgeState := "STANDARD VMAT SURGE OPERATIONS (< 85% Bed Occupancy)"
	if reverseSurge {
		surgeState = "REVERSE-TRIAGE SURGE PROTOCOL ACTIVE (>= 85% Bed Occupancy — Diverting Green/Yellow Stable Patients to Secondary Staging Shelter & Regional Tertiary Vet Medevac to Preserve Red Immediate Oxygen/Ventilator Bays)"
	}
	surgeICSNote = fmt.Sprintf("FEMA/AVMA VMAT Incident Command (ICS-214 %s | Waterproof RFID Collar %s): Field Hospital Surge Bed Occupancy %.1f%% — %s.", ics214Number, rfidCollar, surgeBedOccupancyPercent, surgeState)

	return startSaltSummary, deconBiosecurityNote, surgeICSNote
}

// BuildDefaultFieldTriageStaging constructs an evidence-based VMATFieldTriageStaging plan.
func BuildDefaultFieldTriageStaging(
	weightKg float64,
	triageTag domain.VeterinarySTARTTag,
	protocol domain.VMATSurgeProtocol,
	grossDeconVerified, contagiousCohortSegregated bool,
	rfidCollar, ics214Number string,
	surgeBedOccupancyPercent float64,
	stagedBy string,
	now time.Time,
) domain.VMATFieldTriageStaging {
	hypertonicSalineMl := round1(weightKg * 4.0)
	crystalloidShockBolusMl := round1(weightKg * 20.0)
	methadoneAnalgesiaMg := round2(weightKg * 0.2)
	tranexamicAcidMg := round1(weightKg * 10.0)

	if strings.TrimSpace(rfidCollar) == "" {
		rfidCollar = "VMAT-RFID-9100"
	}
	if strings.TrimSpace(ics214Number) == "" {
		ics214Number = "ICS214-FEMA-2026-100"
	}
	if strings.TrimSpace(stagedBy) == "" {
		stagedBy = "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)"
	}

	guardNote := "HOT-TO-COLD ZONE CROSS-CONTAMINATION GUARD VERIFIED: Patient completed warm-zone gross decontamination corridor (tepid water + Dawn surfactant dermal wash, sterile 0.9% NaCl ocular/oral irrigation, and Tyvek/nitrile handler handoff) prior to crossing the Cold-Zone VMAT ICU line."
	if !grossDeconVerified {
		guardNote = "CRITICAL HOT-TO-COLD ZONE CROSS-CONTAMINATION BLOCK: Patient has NOT completed warm-zone gross decontamination; entry into Cold-Zone VMAT surgical/ICU tent is strictly prohibited to prevent secondary responder intoxication and hospital-wide chemical/sewage contamination."
	}

	deconPlan := "Warm-Zone Decontamination Corridor: Establish uphill/upwind Hot-Warm-Cold perimeter; perform 5-minute tepid low-pressure water + Dawn dish detergent dermal degreasing flush (avoid high-pressure spray that drives toxins intradermally), copiously irrigate corneas and oral mucosa with 0.9% sterile saline, dry thoroughly with radiant warmers to prevent post-decon hypothermia, and transfer across the Cold-Zone clean line."

	dcrPlan := fmt.Sprintf("Damage-Control Resuscitation (%.1f kg patient): Control catastrophic hemorrhage via direct pressure dressing or windlass limb tourniquet + Tranexamic Acid (TXA) 10 mg/kg IV (%.1f mg); administer Hypertonic Saline 7.2%% 4 mL/kg IV (%.1f mL over 10 min) or isotropic crystalloid 20 mL/kg bolus (%.1f mL) + Methadone 0.2 mg/kg IV (%.2f mg) analgesia and high-flow oxygen hood/cage.", weightKg, tranexamicAcidMg, hypertonicSalineMl, crystalloidShockBolusMl, methadoneAnalgesiaMg)

	icsPlan := fmt.Sprintf("FEMA/AVMA ICS-214 Activity Log (%s) & Waterproof RFID Triage Collar (%s): Affix color-coded tear-resistant START/SALT neck band, scan 15-digit ISO 11784/11785 microchip, capture lateral/frontal disaster intake photographs, and log GPS extraction sector for owner reunification chain of custody.", ics214Number, rfidCollar)

	bioPlan := "Biosecurity & Contagious Cohort Isolation: Segregate patients with unknown vaccination history, floodwater leptospirosis exposure, hemorrhagic diarrhea (parvovirus/panleukopenia), or coughing/nasal discharge into negative-pressure/downwind isolation tents with dedicated PPE and footbaths."

	reverseTriggered := IsReverseTriageSurgeTriggered(surgeBedOccupancyPercent)
	reversePlan := fmt.Sprintf("Reverse-Triage Surge Allocation (Occupancy %.1f%% >= 85.0%% Threshold): ACTIVE — Transfer stabilized Green Minimal and Yellow Delayed patients to secondary county fairground staging shelters or regional tertiary veterinary medevac partners so Cold-Zone VMAT ICU oxygen cages and anesthesia circuits remain open for Red Immediate arrivals.", surgeBedOccupancyPercent)
	if !reverseTriggered {
		reversePlan = fmt.Sprintf("Surge Capacity Telemetry (Occupancy %.1f%% < 85.0%% Threshold): Adequate Cold-Zone VMAT ICU oxygen cages and surgical tables available; standby regional medevac relay ready if occupancy reaches 85%%.", surgeBedOccupancyPercent)
	}

	return domain.VMATFieldTriageStaging{
		Protocol:  protocol,
		TriageTag: triageTag,
		GrossDecontaminationPriorToColdZoneVerified: grossDeconVerified,
		WarmZoneDecontaminationCorridorPlan:         deconPlan,
		ColdZoneCrossContaminationGuardNote:         guardNote,
		DamageControlResuscitationPlan:              dcrPlan,
		WaterproofRFIDTriageCollarNumber:            rfidCollar,
		ICS214IncidentNumber:                        ics214Number,
		ICS214ChainOfCustodyPlan:                    icsPlan,
		ContagiousCohortSegregated:                  contagiousCohortSegregated,
		BiosecurityCohortIsolationPlan:              bioPlan,
		SurgeBedOccupancyPercent:                    round1(surgeBedOccupancyPercent),
		ReverseTriageSurgeTriggered:                 reverseTriggered,
		ReverseTriageSurgeAllocationPlan:            reversePlan,
		StagedBy:                                    stagedBy,
		StagedAt:                                    now,
	}
}

func (s *MassCasualtyVMATService) seed() {
	now := time.Now().UTC()

	// MCI-OP-101: Kodiak — Canine, Belgian Malinois (USAR K9), 31.5 kg, START_SALT_TAGGED_DECON_STAGED
	start101, decon101, surge101 := BuildClinicalNarratives(
		domain.STARTTagRedImmediatePriority1LifeThreat,
		17, 6.8, 87.5, 91.0,
		true, true,
		"VMAT-RFID-9101", "ICS214-FEMA-2026-101",
	)
	stage101 := BuildDefaultFieldTriageStaging(
		31.5,
		domain.STARTTagRedImmediatePriority1LifeThreat,
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		true, true,
		"VMAT-RFID-9101", "ICS214-FEMA-2026-101",
		91.0,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		now.Add(-40*time.Minute),
	)
	op101 := &domain.MassCasualtyVMATOperation{
		ID:                       "MCI-OP-101",
		CaseID:                   "CASE-MCI-101",
		PetName:                  "Kodiak",
		Species:                  "Canine",
		Breed:                    "Belgian Malinois (USAR K9)",
		WeightKg:                 31.5,
		Status:                   domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged,
		TriageTag:                domain.STARTTagRedImmediatePriority1LifeThreat,
		Protocol:                 domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		AnimalTriageScoreATS:     17,
		LactateMmolL:             6.8,
		SpO2Percent:              87.5,
		SurgeBedOccupancyPercent: 91.0,
		GrossDecontaminationPriorToColdZoneVerified: true,
		WaterproofRFIDTriageCollarNumber:            "VMAT-RFID-9101",
		ICS214IncidentNumber:                        "ICS214-FEMA-2026-101",
		ContagiousCohortSegregated:                  true,
		ReverseTriageSurgeTriggered:                 true,
		StartSaltTriageSummary:                      start101,
		DeconCorridorBiosecurityNote:                decon101,
		SurgeCapacityICSCommandNote:                 surge101,
		FieldTriageStaging:                          stage101,
		ReassessmentChecks: []domain.VMATSurgeReassessmentCheck{
			{
				ID:                       "MCI-CHK-101-1",
				TriageTag:                domain.STARTTagRedImmediatePriority1LifeThreat,
				AnimalTriageScoreATS:     17,
				LactateMmolL:             6.8,
				SpO2Percent:              87.5,
				SurgeBedOccupancyPercent: 91.0,
				GrossDecontaminationPriorToColdZoneVerified: true,
				ContagiousCohortSegregated:                  true,
				ReverseTriageSurgeTriggered:                 true,
				RecordedBy:                                  "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
				ClinicalNote:                                "RED Immediate Priority 1 USAR K9 extracted from floodwater/petrochemical rubble: ATS 17/25, lactate 6.8 mmol/L, SpO2 87.5%, right antebrachial arterial laceration controlled with pressure bandage, warm-zone Dawn surfactant decon verified, RFID collar VMAT-RFID-9101 & ICS-214 logged; reverse-triage surge triggered at 91.0% occupancy.",
				RecordedAt:                                  now.Add(-40 * time.Minute),
			},
		},
		CreatedAt: now.Add(-55 * time.Minute),
		UpdatedAt: now.Add(-40 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// MCI-OP-102: Cinder — Feline, Domestic Shorthair, 4.6 kg, DAMAGE_CONTROL_SURGE_ICU_MONITORED
	start102, decon102, surge102 := BuildClinicalNarratives(
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		11, 3.4, 93.5, 88.0,
		true, true,
		"VMAT-RFID-9102", "ICS214-FEMA-2026-102",
	)
	stage102 := BuildDefaultFieldTriageStaging(
		4.6,
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		domain.VMATProtocolDamageControlResuscitationTourniquetO2,
		true, true,
		"VMAT-RFID-9102", "ICS214-FEMA-2026-102",
		88.0,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		now.Add(-85*time.Minute),
	)
	op102 := &domain.MassCasualtyVMATOperation{
		ID:                       "MCI-OP-102",
		CaseID:                   "CASE-MCI-102",
		PetName:                  "Cinder",
		Species:                  "Feline",
		Breed:                    "Domestic Shorthair",
		WeightKg:                 4.6,
		Status:                   domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored,
		TriageTag:                domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		Protocol:                 domain.VMATProtocolDamageControlResuscitationTourniquetO2,
		AnimalTriageScoreATS:     11,
		LactateMmolL:             3.4,
		SpO2Percent:              93.5,
		SurgeBedOccupancyPercent: 88.0,
		GrossDecontaminationPriorToColdZoneVerified: true,
		WaterproofRFIDTriageCollarNumber:            "VMAT-RFID-9102",
		ICS214IncidentNumber:                        "ICS214-FEMA-2026-102",
		ContagiousCohortSegregated:                  true,
		ReverseTriageSurgeTriggered:                 true,
		StartSaltTriageSummary:                      start102,
		DeconCorridorBiosecurityNote:                decon102,
		SurgeCapacityICSCommandNote:                 surge102,
		FieldTriageStaging:                          stage102,
		ReassessmentChecks: []domain.VMATSurgeReassessmentCheck{
			{
				ID:                       "MCI-CHK-102-1",
				TriageTag:                domain.STARTTagYellowDelayedPriority2UrgentSurgery,
				AnimalTriageScoreATS:     11,
				LactateMmolL:             3.4,
				SpO2Percent:              93.5,
				SurgeBedOccupancyPercent: 88.0,
				GrossDecontaminationPriorToColdZoneVerified: true,
				ContagiousCohortSegregated:                  true,
				ReverseTriageSurgeTriggered:                 true,
				RecordedBy:                                  "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
				ClinicalNote:                                "YELLOW Delayed Priority 2 feline wildfire-ash evacuation intake: warm-zone ocular/dermal decon completed, ATS 11/25, lactate 3.4 mmol/L, SpO2 93.5% on flow-by O2, buprenorphine analgesia and splinted tibial fracture monitored in Cold-Zone VMAT ICU.",
				RecordedAt:                                  now.Add(-75 * time.Minute),
			},
		},
		CreatedAt: now.Add(-3 * time.Hour),
		UpdatedAt: now.Add(-75 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// MCI-OP-103: Shasta — Canine, Golden Retriever, 29.0 kg, REUNIFIED_EVAC_TRANSPORT_CLEARED
	start103, decon103, surge103 := BuildClinicalNarratives(
		domain.STARTTagGreenMinimalPriority3WalkingWounded,
		3, 1.6, 98.0, 74.0,
		true, true,
		"VMAT-RFID-9103", "ICS214-FEMA-2026-103",
	)
	stage103 := BuildDefaultFieldTriageStaging(
		29.0,
		domain.STARTTagGreenMinimalPriority3WalkingWounded,
		domain.VMATProtocolRegionalTertiaryVetMedevacTransfer,
		true, true,
		"VMAT-RFID-9103", "ICS214-FEMA-2026-103",
		74.0,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		now.Add(-10*time.Hour),
	)
	clearedAt := now.Add(-25 * time.Minute)
	clearance103 := &domain.MassCasualtyVMATClearance{
		LactateMmolL:                  1.6,
		SpO2Percent:                   98.0,
		WaterproofRFIDMatchedVerified: true,
		ICS214ChainOfCustodySigned:    true,
		GrossDecontaminationVerified:  true,
		ClearedBy:                     "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		ClearanceSummary:              "Perfusion normalized (lactate 1.6 mmol/L <= 2.5 mmol/L), SpO2 98.0% (>= 95.0% on room air), warm-zone decontamination verified, waterproof RFID collar VMAT-RFID-9103 & ISO microchip matched, and FEMA ICS-214 chain-of-custody signed; cleared for owner reunification.",
		ClearedAt:                     clearedAt,
	}
	op103 := &domain.MassCasualtyVMATOperation{
		ID:                       "MCI-OP-103",
		CaseID:                   "CASE-MCI-103",
		PetName:                  "Shasta",
		Species:                  "Canine",
		Breed:                    "Golden Retriever",
		WeightKg:                 29.0,
		Status:                   domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared,
		TriageTag:                domain.STARTTagGreenMinimalPriority3WalkingWounded,
		Protocol:                 domain.VMATProtocolRegionalTertiaryVetMedevacTransfer,
		AnimalTriageScoreATS:     3,
		LactateMmolL:             1.6,
		SpO2Percent:              98.0,
		SurgeBedOccupancyPercent: 74.0,
		GrossDecontaminationPriorToColdZoneVerified: true,
		WaterproofRFIDTriageCollarNumber:            "VMAT-RFID-9103",
		ICS214IncidentNumber:                        "ICS214-FEMA-2026-103",
		ContagiousCohortSegregated:                  true,
		ReverseTriageSurgeTriggered:                 false,
		StartSaltTriageSummary:                      start103,
		DeconCorridorBiosecurityNote:                decon103,
		SurgeCapacityICSCommandNote:                 surge103,
		FieldTriageStaging:                          stage103,
		ReassessmentChecks: []domain.VMATSurgeReassessmentCheck{
			{
				ID:                       "MCI-CHK-103-1",
				TriageTag:                domain.STARTTagGreenMinimalPriority3WalkingWounded,
				AnimalTriageScoreATS:     3,
				LactateMmolL:             1.6,
				SpO2Percent:              98.0,
				SurgeBedOccupancyPercent: 74.0,
				GrossDecontaminationPriorToColdZoneVerified: true,
				ContagiousCohortSegregated:                  true,
				ReverseTriageSurgeTriggered:                 false,
				RecordedBy:                                  "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
				ClinicalNote:                                "Post-stabilization check: ATS improved to 3/25, lactate 1.6 mmol/L, SpO2 98.0% on room air, RFID collar VMAT-RFID-9103 and microchip verified for family reunification.",
				RecordedAt:                                  now.Add(-35 * time.Minute),
			},
		},
		Clearance:        clearance103,
		DischargedBy:     clearance103.ClearedBy,
		DischargeSummary: clearance103.ClearanceSummary,
		DischargedAt:     &clearedAt,
		CreatedAt:        now.Add(-12 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Mass-Casualty VMAT operations, optionally filtered by statusFilter.
func (s *MassCasualtyVMATService) ListOperations(statusFilter string) []domain.MassCasualtyVMATOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.MassCasualtyVMATOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneMassCasualtyOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Mass-Casualty VMAT operation by ID.
func (s *MassCasualtyVMATService) GetOperation(id string) (*domain.MassCasualtyVMATOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("mass casualty vmat operation %q not found", id)
	}
	cp := cloneMassCasualtyOperation(op)
	return &cp, nil
}

// CalculateOverview computes summary telemetry metrics across all Mass-Casualty VMAT operations.
func (s *MassCasualtyVMATService) CalculateOverview() domain.MassCasualtyVMATOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.MassCasualtyVMATOverview
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged:
			m.StartSaltTaggedDeconStagedCount++
		case domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored:
			m.DamageControlSurgeMonitoredCount++
		case domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared:
			m.ReunifiedEvacClearedCount++
		}
		if op.TriageTag == domain.STARTTagRedImmediatePriority1LifeThreat {
			m.RedImmediatePriority1Count++
		}
		if op.ReverseTriageSurgeTriggered {
			m.ReverseTriageSurgeTriggeredCount++
		}
		if op.GrossDecontaminationPriorToColdZoneVerified {
			m.ColdZoneDeconVerifiedCount++
		}
	}
	return m
}

// GetOverviewMetrics is an alias for CalculateOverview for handler consistency.
func (s *MassCasualtyVMATService) GetOverviewMetrics() domain.MassCasualtyVMATOverview {
	return s.CalculateOverview()
}

// CreateOperation validates and creates a new Acute Disaster Mass-Casualty Veterinary Triage operation.
func (s *MassCasualtyVMATService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	triageTag domain.VeterinarySTARTTag,
	protocol domain.VMATSurgeProtocol,
	animalTriageScoreATS int,
	lactateMmolL, spo2Percent, surgeBedOccupancyPercent float64,
	grossDeconVerified bool,
	rfidCollar, ics214Number string,
	contagiousCohortSegregated bool,
) (*domain.MassCasualtyVMATOperation, error) {
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
	if weightKg <= 0 || weightKg > 150.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 150.0], got %v", weightKg)
	}
	if !triageTag.IsValid() {
		return nil, fmt.Errorf("invalid triage_tag: %q", triageTag)
	}
	if protocol == "" {
		protocol = domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if animalTriageScoreATS < 0 || animalTriageScoreATS > 25 {
		return nil, fmt.Errorf("animal_triage_score_ats must be in [0, 25], got %d", animalTriageScoreATS)
	}
	if lactateMmolL <= 0 || lactateMmolL > 25.0 {
		return nil, fmt.Errorf("lactate_mmol_l must be in (0, 25.0], got %v", lactateMmolL)
	}
	if spo2Percent < 50.0 || spo2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spo2Percent)
	}
	if surgeBedOccupancyPercent < 0 || surgeBedOccupancyPercent > 100.0 {
		return nil, fmt.Errorf("surge_bed_occupancy_percent must be in [0, 100.0], got %v", surgeBedOccupancyPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("MCI-OP-%d", s.seq)
	now := time.Now().UTC()

	trimmedRFID := strings.TrimSpace(rfidCollar)
	if trimmedRFID == "" {
		trimmedRFID = fmt.Sprintf("VMAT-RFID-9%d", s.seq)
	}
	trimmedICS := strings.TrimSpace(ics214Number)
	if trimmedICS == "" {
		trimmedICS = fmt.Sprintf("ICS214-FEMA-2026-%d", s.seq)
	}

	startSummary, deconNote, surgeNote := BuildClinicalNarratives(
		triageTag,
		animalTriageScoreATS,
		lactateMmolL,
		spo2Percent,
		surgeBedOccupancyPercent,
		grossDeconVerified,
		contagiousCohortSegregated,
		trimmedRFID,
		trimmedICS,
	)
	staging := BuildDefaultFieldTriageStaging(
		weightKg,
		triageTag,
		protocol,
		grossDeconVerified,
		contagiousCohortSegregated,
		trimmedRFID,
		trimmedICS,
		surgeBedOccupancyPercent,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		now,
	)

	op := &domain.MassCasualtyVMATOperation{
		ID:                       opID,
		CaseID:                   trimmedCase,
		PetName:                  trimmedPet,
		Species:                  trimmedSpecies,
		Breed:                    trimmedBreed,
		WeightKg:                 round1(weightKg),
		Status:                   domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged,
		TriageTag:                triageTag,
		Protocol:                 protocol,
		AnimalTriageScoreATS:     animalTriageScoreATS,
		LactateMmolL:             round2(lactateMmolL),
		SpO2Percent:              round1(spo2Percent),
		SurgeBedOccupancyPercent: round1(surgeBedOccupancyPercent),
		GrossDecontaminationPriorToColdZoneVerified: grossDeconVerified,
		WaterproofRFIDTriageCollarNumber:            trimmedRFID,
		ICS214IncidentNumber:                        trimmedICS,
		ContagiousCohortSegregated:                  contagiousCohortSegregated,
		ReverseTriageSurgeTriggered:                 IsReverseTriageSurgeTriggered(surgeBedOccupancyPercent),
		StartSaltTriageSummary:                      startSummary,
		DeconCorridorBiosecurityNote:                deconNote,
		SurgeCapacityICSCommandNote:                 surgeNote,
		FieldTriageStaging:                          staging,
		ReassessmentChecks:                          []domain.VMATSurgeReassessmentCheck{},
		CreatedAt:                                   now,
		UpdatedAt:                                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneMassCasualtyOperation(op)
	return &cp, nil
}

// StageFieldTriage stages warm-zone gross decontamination verification, damage-control resuscitation,
// waterproof RFID triage collar + ICS-214 chain-of-custody tracking, contagious cohort biosecurity
// segregation, and reverse-triage surge bed allocation, enforcing the Hot-to-Cold Zone Cross-Contamination Guard.
func (s *MassCasualtyVMATService) StageFieldTriage(
	id string,
	protocol domain.VMATSurgeProtocol,
	triageTag domain.VeterinarySTARTTag,
	grossDeconVerified bool,
	warmZoneDeconPlan, damageControlPlan, rfidCollar, ics214Number, ics214Plan string,
	contagiousCohortSegregated bool,
	biosecurityPlan string,
	surgeBedOccupancyPercent float64,
	reverseTriagePlan, stagedBy string,
) (*domain.MassCasualtyVMATOperation, error) {
	if protocol == "" {
		protocol = domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if triageTag != "" && !triageTag.IsValid() {
		return nil, fmt.Errorf("invalid triage_tag: %q", triageTag)
	}
	// Safety Guard 1: Hot-to-Cold Zone Cross-Contamination Guard
	if !grossDeconVerified {
		return nil, fmt.Errorf("hot-to-cold zone cross-contamination guard: gross_decontamination_prior_to_cold_zone_verified must be true before admitting MCI patient into Cold-Zone VMAT surgical/ICU tent")
	}
	// Safety Guard 2: Contagious Biosecurity Segregation Guard
	if !contagiousCohortSegregated {
		return nil, fmt.Errorf("biosecurity isolation guard: contagious_cohort_segregated must be true to prevent infectious disease outbreak across VMAT surge wards")
	}
	trimmedDeconPlan := strings.TrimSpace(warmZoneDeconPlan)
	if trimmedDeconPlan == "" {
		return nil, fmt.Errorf("warm_zone_decontamination_corridor_plan is required")
	}
	trimmedDCRPlan := strings.TrimSpace(damageControlPlan)
	if trimmedDCRPlan == "" {
		return nil, fmt.Errorf("damage_control_resuscitation_plan is required")
	}
	trimmedRFID := strings.TrimSpace(rfidCollar)
	if trimmedRFID == "" {
		return nil, fmt.Errorf("waterproof_rfid_triage_collar_number is required")
	}
	trimmedICS := strings.TrimSpace(ics214Number)
	if trimmedICS == "" {
		return nil, fmt.Errorf("ics214_incident_number is required")
	}
	if surgeBedOccupancyPercent < 0 || surgeBedOccupancyPercent > 100.0 {
		return nil, fmt.Errorf("surge_bed_occupancy_percent must be in [0, 100.0], got %v", surgeBedOccupancyPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("mass casualty vmat operation %q not found", id)
	}

	if triageTag == "" {
		triageTag = op.TriageTag
	}
	if surgeBedOccupancyPercent == 0 {
		surgeBedOccupancyPercent = op.SurgeBedOccupancyPercent
	}

	now := time.Now().UTC()
	defaults := BuildDefaultFieldTriageStaging(
		op.WeightKg,
		triageTag,
		protocol,
		grossDeconVerified,
		contagiousCohortSegregated,
		trimmedRFID,
		trimmedICS,
		surgeBedOccupancyPercent,
		stagedBy,
		now,
	)

	trimmedICSPlan := strings.TrimSpace(ics214Plan)
	if trimmedICSPlan == "" {
		trimmedICSPlan = defaults.ICS214ChainOfCustodyPlan
	}
	trimmedBioPlan := strings.TrimSpace(biosecurityPlan)
	if trimmedBioPlan == "" {
		trimmedBioPlan = defaults.BiosecurityCohortIsolationPlan
	}
	trimmedReversePlan := strings.TrimSpace(reverseTriagePlan)
	if trimmedReversePlan == "" {
		trimmedReversePlan = defaults.ReverseTriageSurgeAllocationPlan
	}
	trimmedBy := strings.TrimSpace(stagedBy)
	if trimmedBy == "" {
		trimmedBy = defaults.StagedBy
	}

	op.Protocol = protocol
	op.TriageTag = triageTag
	op.GrossDecontaminationPriorToColdZoneVerified = true
	op.WaterproofRFIDTriageCollarNumber = trimmedRFID
	op.ICS214IncidentNumber = trimmedICS
	op.ContagiousCohortSegregated = true
	op.SurgeBedOccupancyPercent = round1(surgeBedOccupancyPercent)
	op.ReverseTriageSurgeTriggered = IsReverseTriageSurgeTriggered(op.SurgeBedOccupancyPercent)

	startSummary, deconNote, surgeNote := BuildClinicalNarratives(
		op.TriageTag,
		op.AnimalTriageScoreATS,
		op.LactateMmolL,
		op.SpO2Percent,
		op.SurgeBedOccupancyPercent,
		op.GrossDecontaminationPriorToColdZoneVerified,
		op.ContagiousCohortSegregated,
		op.WaterproofRFIDTriageCollarNumber,
		op.ICS214IncidentNumber,
	)
	op.StartSaltTriageSummary = startSummary
	op.DeconCorridorBiosecurityNote = deconNote
	op.SurgeCapacityICSCommandNote = surgeNote

	op.FieldTriageStaging = domain.VMATFieldTriageStaging{
		Protocol:  protocol,
		TriageTag: triageTag,
		GrossDecontaminationPriorToColdZoneVerified: true,
		WarmZoneDecontaminationCorridorPlan:         trimmedDeconPlan,
		ColdZoneCrossContaminationGuardNote:         defaults.ColdZoneCrossContaminationGuardNote,
		DamageControlResuscitationPlan:              trimmedDCRPlan,
		WaterproofRFIDTriageCollarNumber:            trimmedRFID,
		ICS214IncidentNumber:                        trimmedICS,
		ICS214ChainOfCustodyPlan:                    trimmedICSPlan,
		ContagiousCohortSegregated:                  true,
		BiosecurityCohortIsolationPlan:              trimmedBioPlan,
		SurgeBedOccupancyPercent:                    op.SurgeBedOccupancyPercent,
		ReverseTriageSurgeTriggered:                 op.ReverseTriageSurgeTriggered,
		ReverseTriageSurgeAllocationPlan:            trimmedReversePlan,
		StagedBy:                                    trimmedBy,
		StagedAt:                                    now,
	}
	op.Status = domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored
	op.Clearance = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = now

	cp := cloneMassCasualtyOperation(op)
	return &cp, nil
}

// RecordReassessmentCheck appends a serial VMATSurgeReassessmentCheck and updates START/SALT triage tag,
// Animal Trauma Triage Score (ATS), blood lactate, SpO2, and surge bed occupancy telemetry on the operation.
func (s *MassCasualtyVMATService) RecordReassessmentCheck(
	id string,
	triageTag domain.VeterinarySTARTTag,
	animalTriageScoreATS int,
	lactateMmolL, spo2Percent, surgeBedOccupancyPercent float64,
	grossDeconVerified, contagiousCohortSegregated bool,
	recordedBy, clinicalNote string,
) (*domain.MassCasualtyVMATOperation, error) {
	if triageTag != "" && !triageTag.IsValid() {
		return nil, fmt.Errorf("invalid triage_tag: %q", triageTag)
	}
	if animalTriageScoreATS < 0 || animalTriageScoreATS > 25 {
		return nil, fmt.Errorf("animal_triage_score_ats must be in [0, 25], got %d", animalTriageScoreATS)
	}
	if lactateMmolL <= 0 || lactateMmolL > 25.0 {
		return nil, fmt.Errorf("lactate_mmol_l must be in (0, 25.0], got %v", lactateMmolL)
	}
	if spo2Percent < 50.0 || spo2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spo2Percent)
	}
	if surgeBedOccupancyPercent < 0 || surgeBedOccupancyPercent > 100.0 {
		return nil, fmt.Errorf("surge_bed_occupancy_percent must be in [0, 100.0], got %v", surgeBedOccupancyPercent)
	}
	if !grossDeconVerified {
		return nil, fmt.Errorf("hot-to-cold zone cross-contamination guard: gross_decontamination_prior_to_cold_zone_verified must be true in Cold-Zone VMAT ICU")
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
		return nil, fmt.Errorf("mass casualty vmat operation %q not found", id)
	}

	if triageTag == "" {
		triageTag = op.TriageTag
	}
	if surgeBedOccupancyPercent == 0 {
		surgeBedOccupancyPercent = op.SurgeBedOccupancyPercent
	}

	now := time.Now().UTC()
	reverseSurge := IsReverseTriageSurgeTriggered(surgeBedOccupancyPercent)
	chk := domain.VMATSurgeReassessmentCheck{
		ID:                       fmt.Sprintf("MCI-CHK-%s-%d", op.ID, len(op.ReassessmentChecks)+1),
		TriageTag:                triageTag,
		AnimalTriageScoreATS:     animalTriageScoreATS,
		LactateMmolL:             round2(lactateMmolL),
		SpO2Percent:              round1(spo2Percent),
		SurgeBedOccupancyPercent: round1(surgeBedOccupancyPercent),
		GrossDecontaminationPriorToColdZoneVerified: grossDeconVerified,
		ContagiousCohortSegregated:                  contagiousCohortSegregated,
		ReverseTriageSurgeTriggered:                 reverseSurge,
		RecordedBy:                                  trimmedBy,
		ClinicalNote:                                trimmedNote,
		RecordedAt:                                  now,
	}

	op.ReassessmentChecks = append(op.ReassessmentChecks, chk)
	op.TriageTag = chk.TriageTag
	op.AnimalTriageScoreATS = chk.AnimalTriageScoreATS
	op.LactateMmolL = chk.LactateMmolL
	op.SpO2Percent = chk.SpO2Percent
	op.SurgeBedOccupancyPercent = chk.SurgeBedOccupancyPercent
	op.GrossDecontaminationPriorToColdZoneVerified = chk.GrossDecontaminationPriorToColdZoneVerified
	op.ContagiousCohortSegregated = chk.ContagiousCohortSegregated
	op.ReverseTriageSurgeTriggered = chk.ReverseTriageSurgeTriggered

	startSummary, deconNote, surgeNote := BuildClinicalNarratives(
		op.TriageTag,
		op.AnimalTriageScoreATS,
		op.LactateMmolL,
		op.SpO2Percent,
		op.SurgeBedOccupancyPercent,
		op.GrossDecontaminationPriorToColdZoneVerified,
		op.ContagiousCohortSegregated,
		op.WaterproofRFIDTriageCollarNumber,
		op.ICS214IncidentNumber,
	)
	op.StartSaltTriageSummary = startSummary
	op.DeconCorridorBiosecurityNote = deconNote
	op.SurgeCapacityICSCommandNote = surgeNote
	if op.Status == domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged {
		op.Status = domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored
	}
	op.UpdatedAt = now

	cp := cloneMassCasualtyOperation(op)
	return &cp, nil
}

// EvaluateClearance verifies that tissue perfusion is restored (lactate_mmol_l <= 2.5),
// arterial oxygenation is normalized (spo2_percent >= 95.0), waterproof RFID collar & microchip
// match is verified (waterproof_rfid_matched_verified == true), and FEMA ICS-214 chain of custody
// is signed (ics214_chain_of_custody_signed == true), transitioning status to REUNIFIED_EVAC_TRANSPORT_CLEARED.
func (s *MassCasualtyVMATService) EvaluateClearance(
	id string,
	lactateMmolL, spo2Percent float64,
	waterproofRFIDMatchedVerified, ics214ChainOfCustodySigned bool,
	clearedBy, clearanceSummary string,
) (*domain.MassCasualtyVMATOperation, error) {
	if lactateMmolL <= 0 || lactateMmolL > 2.5 {
		return nil, fmt.Errorf("cannot clear mass casualty vmat operation: lactate_mmol_l must be in (0, 2.5] mmol/L, got %v", lactateMmolL)
	}
	if spo2Percent < 95.0 || spo2Percent > 100.0 {
		return nil, fmt.Errorf("cannot clear mass casualty vmat operation: spo2_percent must be in [95.0, 100.0]%%, got %v", spo2Percent)
	}
	if !waterproofRFIDMatchedVerified {
		return nil, fmt.Errorf("cannot clear mass casualty vmat operation: waterproof_rfid_matched_verified must be true (RFID collar & ISO microchip verification required)")
	}
	if !ics214ChainOfCustodySigned {
		return nil, fmt.Errorf("cannot clear mass casualty vmat operation: ics214_chain_of_custody_signed must be true (FEMA ICS-214 chain of custody signature required)")
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
		return nil, fmt.Errorf("mass casualty vmat operation %q not found", id)
	}
	if !op.GrossDecontaminationPriorToColdZoneVerified {
		return nil, fmt.Errorf("cannot clear mass casualty vmat operation: warm-zone gross decontamination must be verified prior to transport or reunification")
	}

	now := time.Now().UTC()
	op.LactateMmolL = round2(lactateMmolL)
	op.SpO2Percent = round1(spo2Percent)

	startSummary, deconNote, surgeNote := BuildClinicalNarratives(
		op.TriageTag,
		op.AnimalTriageScoreATS,
		op.LactateMmolL,
		op.SpO2Percent,
		op.SurgeBedOccupancyPercent,
		op.GrossDecontaminationPriorToColdZoneVerified,
		op.ContagiousCohortSegregated,
		op.WaterproofRFIDTriageCollarNumber,
		op.ICS214IncidentNumber,
	)
	op.StartSaltTriageSummary = startSummary
	op.DeconCorridorBiosecurityNote = deconNote
	op.SurgeCapacityICSCommandNote = surgeNote

	clearance := &domain.MassCasualtyVMATClearance{
		LactateMmolL:                  op.LactateMmolL,
		SpO2Percent:                   op.SpO2Percent,
		WaterproofRFIDMatchedVerified: true,
		ICS214ChainOfCustodySigned:    true,
		GrossDecontaminationVerified:  true,
		ClearedBy:                     trimmedBy,
		ClearanceSummary:              trimmedSummary,
		ClearedAt:                     now,
	}
	op.Clearance = clearance
	op.Status = domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneMassCasualtyOperation(op)
	return &cp, nil
}

func cloneMassCasualtyOperation(op *domain.MassCasualtyVMATOperation) domain.MassCasualtyVMATOperation {
	cp := *op
	if op.ReassessmentChecks != nil {
		cp.ReassessmentChecks = make([]domain.VMATSurgeReassessmentCheck, len(op.ReassessmentChecks))
		copy(cp.ReassessmentChecks, op.ReassessmentChecks)
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
