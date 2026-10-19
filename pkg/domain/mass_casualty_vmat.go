package domain

import (
	"fmt"
	"strings"
	"time"
)

// MassCasualtyVMATStatus represents the clinical and FEMA/AVMA VMAT Incident Command System (ICS)
// workflow state of an Acute Disaster Mass-Casualty Veterinary Triage (START/SALT Veterinary Color-Tagging:
// Immediate Red, Delayed Yellow, Minimal Green, Expectant Black), Decontamination Corridor,
// Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment operation.
type MassCasualtyVMATStatus string

const (
	MassCasualtyVMATStatusStartSaltTaggedDeconStaged     MassCasualtyVMATStatus = "START_SALT_TAGGED_DECON_STAGED"
	MassCasualtyVMATStatusDamageControlSurgeICUMonitored MassCasualtyVMATStatus = "DAMAGE_CONTROL_SURGE_ICU_MONITORED"
	MassCasualtyVMATStatusReunifiedEvacTransportCleared  MassCasualtyVMATStatus = "REUNIFIED_EVAC_TRANSPORT_CLEARED"
)

// IsValid returns true when MassCasualtyVMATStatus is a recognized lifecycle state.
func (s MassCasualtyVMATStatus) IsValid() bool {
	switch s {
	case MassCasualtyVMATStatusStartSaltTaggedDeconStaged,
		MassCasualtyVMATStatusDamageControlSurgeICUMonitored,
		MassCasualtyVMATStatusReunifiedEvacTransportCleared:
		return true
	default:
		return false
	}
}

// VeterinarySTARTTag classifies the veterinary START/SALT mass-casualty color-tagging priority.
type VeterinarySTARTTag string

const (
	STARTTagRedImmediatePriority1LifeThreat     VeterinarySTARTTag = "RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT"
	STARTTagYellowDelayedPriority2UrgentSurgery VeterinarySTARTTag = "YELLOW_DELAYED_PRIORITY_2_URGENT_SURGERY"
	STARTTagGreenMinimalPriority3WalkingWounded VeterinarySTARTTag = "GREEN_MINIMAL_PRIORITY_3_WALKING_WOUNDED"
	STARTTagBlackExpectantAnalgesiaPalliative   VeterinarySTARTTag = "BLACK_EXPECTANT_ANALGESIA_PALLIATIVE"
)

// IsValid returns true when VeterinarySTARTTag is a recognized START/SALT triage tag color.
func (t VeterinarySTARTTag) IsValid() bool {
	switch t {
	case STARTTagRedImmediatePriority1LifeThreat,
		STARTTagYellowDelayedPriority2UrgentSurgery,
		STARTTagGreenMinimalPriority3WalkingWounded,
		STARTTagBlackExpectantAnalgesiaPalliative:
		return true
	default:
		return false
	}
}

// VMATSurgeProtocol classifies the FEMA/AVMA Veterinary Medical Assistance Team (VMAT)
// decontamination, damage-control resuscitation, ICS-214 cohort isolation, or regional medevac protocol.
type VMATSurgeProtocol string

const (
	VMATProtocolHotWarmColdZoneDawnDeconCorridor       VMATSurgeProtocol = "HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR"
	VMATProtocolDamageControlResuscitationTourniquetO2 VMATSurgeProtocol = "DAMAGE_CONTROL_RESUSCITATION_TOURNIQUET_O2"
	VMATProtocolICS214MicrochipBandCohortIsolation     VMATSurgeProtocol = "ICS_214_MICROCHIP_BAND_COHORT_ISOLATION"
	VMATProtocolRegionalTertiaryVetMedevacTransfer     VMATSurgeProtocol = "REGIONAL_TERTIARY_VET_MEDEVAC_TRANSFER"
)

// IsValid returns true when VMATSurgeProtocol is a recognized VMAT deployment protocol.
func (p VMATSurgeProtocol) IsValid() bool {
	switch p {
	case VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		VMATProtocolDamageControlResuscitationTourniquetO2,
		VMATProtocolICS214MicrochipBandCohortIsolation,
		VMATProtocolRegionalTertiaryVetMedevacTransfer:
		return true
	default:
		return false
	}
}

// VMATFieldTriageStaging captures warm-zone gross decontamination verification prior to cold-zone ICU admission,
// damage-control resuscitation (direct pressure/tourniquet, thoracocentesis, flow-by O2, hypertonic saline),
// waterproof RFID triage collar + ICS-214 chain-of-custody tracking, contagious cohort biosecurity segregation,
// and reverse-triage surge bed allocation when hospital capacity reaches >= 85%.
type VMATFieldTriageStaging struct {
	Protocol                                    VMATSurgeProtocol  `json:"protocol"`
	TriageTag                                   VeterinarySTARTTag `json:"triage_tag"`
	GrossDecontaminationPriorToColdZoneVerified bool               `json:"gross_decontamination_prior_to_cold_zone_verified"`
	WarmZoneDecontaminationCorridorPlan         string             `json:"warm_zone_decontamination_corridor_plan"`
	ColdZoneCrossContaminationGuardNote         string             `json:"cold_zone_cross_contamination_guard_note"`
	DamageControlResuscitationPlan              string             `json:"damage_control_resuscitation_plan"`
	WaterproofRFIDTriageCollarNumber            string             `json:"waterproof_rfid_triage_collar_number"`
	ICS214IncidentNumber                        string             `json:"ics214_incident_number"`
	ICS214ChainOfCustodyPlan                    string             `json:"ics214_chain_of_custody_plan"`
	ContagiousCohortSegregated                  bool               `json:"contagious_cohort_segregated"`
	BiosecurityCohortIsolationPlan              string             `json:"biosecurity_cohort_isolation_plan"`
	SurgeBedOccupancyPercent                    float64            `json:"surge_bed_occupancy_percent"`
	ReverseTriageSurgeTriggered                 bool               `json:"reverse_triage_surge_triggered"`
	ReverseTriageSurgeAllocationPlan            string             `json:"reverse_triage_surge_allocation_plan"`
	StagedBy                                    string             `json:"staged_by"`
	StagedAt                                    time.Time          `json:"staged_at"`
}

// VMATSurgeReassessmentCheck records a serial field hospital ICU reassessment check
// (START/SALT Triage Tag re-categorization, Animal Trauma Triage Score ATS 0-25, Lactate mmol/L,
// SpO2 %, Surge Bed Occupancy %, Warm-Zone Decon Verified, and Reverse-Triage status).
type VMATSurgeReassessmentCheck struct {
	ID                                          string             `json:"id"`
	TriageTag                                   VeterinarySTARTTag `json:"triage_tag"`
	AnimalTriageScoreATS                        int                `json:"animal_triage_score_ats"`
	LactateMmolL                                float64            `json:"lactate_mmol_l"`
	SpO2Percent                                 float64            `json:"spo2_percent"`
	SurgeBedOccupancyPercent                    float64            `json:"surge_bed_occupancy_percent"`
	GrossDecontaminationPriorToColdZoneVerified bool               `json:"gross_decontamination_prior_to_cold_zone_verified"`
	ContagiousCohortSegregated                  bool               `json:"contagious_cohort_segregated"`
	ReverseTriageSurgeTriggered                 bool               `json:"reverse_triage_surge_triggered"`
	RecordedBy                                  string             `json:"recorded_by"`
	ClinicalNote                                string             `json:"clinical_note"`
	RecordedAt                                  time.Time          `json:"recorded_at"`
}

// MassCasualtyVMATClearance captures owner reunification or regional tertiary veterinary medevac
// transport clearance once perfusion is restored (lactate_mmol_l <= 2.5), oxygenation is normalized
// (spo2_percent >= 95.0), waterproof RFID collar & microchip match is verified, and ICS-214 chain of custody is signed.
type MassCasualtyVMATClearance struct {
	LactateMmolL                  float64   `json:"lactate_mmol_l"`
	SpO2Percent                   float64   `json:"spo2_percent"`
	WaterproofRFIDMatchedVerified bool      `json:"waterproof_rfid_matched_verified"`
	ICS214ChainOfCustodySigned    bool      `json:"ics214_chain_of_custody_signed"`
	GrossDecontaminationVerified  bool      `json:"gross_decontamination_verified"`
	ClearedBy                     string    `json:"cleared_by"`
	ClearanceSummary              string    `json:"clearance_summary"`
	ClearedAt                     time.Time `json:"cleared_at"`
}

// MassCasualtyVMATOperation represents an Acute Disaster Mass-Casualty Veterinary Triage
// (START/SALT Veterinary Color-Tagging: Immediate Red, Delayed Yellow, Minimal Green, Expectant Black),
// Decontamination Corridor, Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment operation.
type MassCasualtyVMATOperation struct {
	ID                                          string                       `json:"id"`
	CaseID                                      string                       `json:"case_id"`
	PetName                                     string                       `json:"pet_name"`
	Species                                     string                       `json:"species"`
	Breed                                       string                       `json:"breed"`
	WeightKg                                    float64                      `json:"weight_kg"`
	Status                                      MassCasualtyVMATStatus       `json:"status"`
	TriageTag                                   VeterinarySTARTTag           `json:"triage_tag"`
	Protocol                                    VMATSurgeProtocol            `json:"protocol"`
	AnimalTriageScoreATS                        int                          `json:"animal_triage_score_ats"`
	LactateMmolL                                float64                      `json:"lactate_mmol_l"`
	SpO2Percent                                 float64                      `json:"spo2_percent"`
	SurgeBedOccupancyPercent                    float64                      `json:"surge_bed_occupancy_percent"`
	GrossDecontaminationPriorToColdZoneVerified bool                         `json:"gross_decontamination_prior_to_cold_zone_verified"`
	WaterproofRFIDTriageCollarNumber            string                       `json:"waterproof_rfid_triage_collar_number"`
	ICS214IncidentNumber                        string                       `json:"ics214_incident_number"`
	ContagiousCohortSegregated                  bool                         `json:"contagious_cohort_segregated"`
	ReverseTriageSurgeTriggered                 bool                         `json:"reverse_triage_surge_triggered"`
	StartSaltTriageSummary                      string                       `json:"start_salt_triage_summary"`
	DeconCorridorBiosecurityNote                string                       `json:"decon_corridor_biosecurity_note"`
	SurgeCapacityICSCommandNote                 string                       `json:"surge_capacity_ics_command_note"`
	FieldTriageStaging                          VMATFieldTriageStaging       `json:"field_triage_staging"`
	ReassessmentChecks                          []VMATSurgeReassessmentCheck `json:"reassessment_checks"`
	Clearance                                   *MassCasualtyVMATClearance   `json:"clearance,omitempty"`
	DischargedBy                                string                       `json:"discharged_by,omitempty"`
	DischargeSummary                            string                       `json:"discharge_summary,omitempty"`
	DischargedAt                                *time.Time                   `json:"discharged_at,omitempty"`
	CreatedAt                                   time.Time                    `json:"created_at"`
	UpdatedAt                                   time.Time                    `json:"updated_at"`
}

// Validate checks domain invariants on MassCasualtyVMATOperation.
func (op MassCasualtyVMATOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be positive, got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.TriageTag.IsValid() {
		return fmt.Errorf("invalid triage_tag: %q", op.TriageTag)
	}
	if !op.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %q", op.Protocol)
	}
	if op.AnimalTriageScoreATS < 0 || op.AnimalTriageScoreATS > 25 {
		return fmt.Errorf("animal_triage_score_ats must be in [0, 25], got %d", op.AnimalTriageScoreATS)
	}
	return nil
}

// MassCasualtyVMATOverview summarizes aggregate telemetry counts across Mass-Casualty VMAT operations.
type MassCasualtyVMATOverview struct {
	TotalOperations                  int `json:"total_operations"`
	StartSaltTaggedDeconStagedCount  int `json:"start_salt_tagged_decon_staged_count"`
	DamageControlSurgeMonitoredCount int `json:"damage_control_surge_monitored_count"`
	ReunifiedEvacClearedCount        int `json:"reunified_evac_cleared_count"`
	RedImmediatePriority1Count       int `json:"red_immediate_priority_1_count"`
	ReverseTriageSurgeTriggeredCount int `json:"reverse_triage_surge_triggered_count"`
	ColdZoneDeconVerifiedCount       int `json:"cold_zone_decon_verified_count"`
}
