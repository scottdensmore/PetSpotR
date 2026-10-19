package domain

import (
	"fmt"
	"strings"
	"time"
)

// IMHAThrombosisStatus represents the lifecycle state of an Immune-Mediated Hemolytic Anemia (IMHA)
// & Thrombocytopenia (IMTP), Saline Agglutination, Spherocytosis & Thromboprophylaxis triage operation.
type IMHAThrombosisStatus string

const (
	IMHAStatusHemolyticCrisisCritical         IMHAThrombosisStatus = "HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL"
	IMHAStatusImmunosuppressantAnticoagStaged IMHAThrombosisStatus = "PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED"
	IMHAStatusPCVStableReticulocytosisCleared IMHAThrombosisStatus = "PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE"
)

// IsValid returns true when IMHAThrombosisStatus is a supported lifecycle state.
func (s IMHAThrombosisStatus) IsValid() bool {
	switch s {
	case IMHAStatusHemolyticCrisisCritical,
		IMHAStatusImmunosuppressantAnticoagStaged,
		IMHAStatusPCVStableReticulocytosisCleared:
		return true
	default:
		return false
	}
}

const (
	IMHASpeciesCaninePrimaryIMHA             = "CANINE_PRIMARY_IMHA"
	IMHASpeciesCanineEvansSyndromeIMTP       = "CANINE_EVANS_SYNDROME_IMTP"
	IMHASpeciesFelineMycoplasmaSecondaryIMHA = "FELINE_MYCOPLASMA_SECONDARY_IMHA"
)

// IsValidIMHASpecies returns true if species is a recognized IMHA/IMTP presentation classification.
func IsValidIMHASpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case IMHASpeciesCaninePrimaryIMHA,
		IMHASpeciesCanineEvansSyndromeIMTP,
		IMHASpeciesFelineMycoplasmaSecondaryIMHA:
		return true
	default:
		return false
	}
}

// IMHAPatientProfile captures signalment, hematology, bilirubin, platelet, spherocytosis,
// 1:4 saline agglutination, and intravascular hemoglobinuria telemetry.
type IMHAPatientProfile struct {
	Species                         string  `json:"species"`
	Breed                           string  `json:"breed,omitempty"`
	WeightKg                        float64 `json:"weight_kg"`
	PackedCellVolumePercent         float64 `json:"packed_cell_volume_percent"`
	TotalBilirubinMgDl              float64 `json:"total_bilirubin_mg_dl"`
	PlateletCountKUl                float64 `json:"platelet_count_k_ul"`
	SpherocytesPerHPF               int     `json:"spherocytes_per_hpf"`
	SalineAgglutination4To1Positive bool    `json:"saline_agglutination_4_to_1_positive"`
	IntravascularHemoglobinuria     bool    `json:"intravascular_hemoglobinuria"`
}

// Validate enforces clinical invariants on IMHAPatientProfile.
func (p IMHAPatientProfile) Validate() error {
	if !IsValidIMHASpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_PRIMARY_IMHA, CANINE_EVANS_SYNDROME_IMTP, or FELINE_MYCOPLASMA_SECONDARY_IMHA, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.PackedCellVolumePercent < 4.0 || p.PackedCellVolumePercent > 65.0 {
		return fmt.Errorf("packed_cell_volume_percent must be in [4.0, 65.0], got %v", p.PackedCellVolumePercent)
	}
	if p.TotalBilirubinMgDl < 0.0 || p.TotalBilirubinMgDl > 50.0 {
		return fmt.Errorf("total_bilirubin_mg_dl must be in [0.0, 50.0], got %v", p.TotalBilirubinMgDl)
	}
	if p.PlateletCountKUl < 0.0 || p.PlateletCountKUl > 1000.0 {
		return fmt.Errorf("platelet_count_k_ul must be in [0.0, 1000.0], got %v", p.PlateletCountKUl)
	}
	if p.SpherocytesPerHPF < 0 || p.SpherocytesPerHPF > 100 {
		return fmt.Errorf("spherocytes_per_hpf must be in [0, 100], got %d", p.SpherocytesPerHPF)
	}
	return nil
}

// IMHAImmunosuppressiveRegimen captures calculated Prednisolone, Mycophenolate mofetil,
// Clopidogrel + Rivaroxaban thromboprophylaxis, Vincristine IMTP platelet-release bolus,
// pRBC transfusion volume, and zero-jugular venipuncture guards.
type IMHAImmunosuppressiveRegimen struct {
	Prednisolone2MgKgDayMg       float64 `json:"prednisolone_2_mg_kg_day_mg"`
	Mycophenolate10MgKgBIDMg     float64 `json:"mycophenolate_10_mg_kg_bid_mg"`
	Clopidogrel2MgKgDayMg        float64 `json:"clopidogrel_2_mg_kg_day_mg"`
	Rivaroxaban1MgKgDayMg        float64 `json:"rivaroxaban_1_mg_kg_day_mg"`
	Vincristine002MgKgIMTPMg     float64 `json:"vincristine_002_mg_kg_imtp_mg"`
	PackedRBCTransfusionMl       float64 `json:"packed_rbc_transfusion_ml"`
	ZeroJugularVenipunctureGuard bool    `json:"zero_jugular_venipuncture_guard"`
	IMHAPitfallAlert             string  `json:"imha_pitfall_alert"`
	ClinicalRationale            string  `json:"clinical_rationale"`
}

// IMHAHemolysisSerialCheck records a serial q6h PCV, reticulocyte count, bilirubin,
// 1:4 saline agglutination, and resting respiratory rate (PTE screen) check.
type IMHAHemolysisSerialCheck struct {
	Timestamp                     time.Time `json:"timestamp"`
	PackedCellVolumePercent       float64   `json:"packed_cell_volume_percent"`
	ReticulocyteCountKUl          float64   `json:"reticulocyte_count_k_ul"`
	TotalBilirubinMgDl            float64   `json:"total_bilirubin_mg_dl"`
	SalineAgglutinationPersistent bool      `json:"saline_agglutination_persistent"`
	RestingRespiratoryRateBpm     int       `json:"resting_respiratory_rate_bpm"`
	ClinicianCallsign             string    `json:"clinician_callsign"`
	Notes                         string    `json:"notes"`
}

// Validate enforces clinical invariants on IMHAHemolysisSerialCheck.
func (c IMHAHemolysisSerialCheck) Validate() error {
	if c.PackedCellVolumePercent < 4.0 || c.PackedCellVolumePercent > 65.0 {
		return fmt.Errorf("packed_cell_volume_percent must be in [4.0, 65.0], got %v", c.PackedCellVolumePercent)
	}
	if c.ReticulocyteCountKUl < 0.0 || c.ReticulocyteCountKUl > 1000.0 {
		return fmt.Errorf("reticulocyte_count_k_ul must be in [0.0, 1000.0], got %v", c.ReticulocyteCountKUl)
	}
	if c.TotalBilirubinMgDl < 0.0 || c.TotalBilirubinMgDl > 50.0 {
		return fmt.Errorf("total_bilirubin_mg_dl must be in [0.0, 50.0], got %v", c.TotalBilirubinMgDl)
	}
	if c.RestingRespiratoryRateBpm < 6 || c.RestingRespiratoryRateBpm > 140 {
		return fmt.Errorf("resting_respiratory_rate_bpm must be in [6, 140], got %d", c.RestingRespiratoryRateBpm)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// IMHADischargeClearance certifies 48h stable PCV (>= 25%), regenerative reticulocytosis (>= 60 K/uL),
// resolution of saline autoagglutination, absence of pulmonary thromboembolism dyspnea,
// and dispensed outpatient thromboprophylaxis + slow immunosuppressive taper schedule.
type IMHADischargeClearance struct {
	ClearedAt                         time.Time `json:"cleared_at"`
	FinalPCVStable48HrPercent         float64   `json:"final_pcv_stable_48_hr_percent"`
	FinalReticulocyteCountKUl         float64   `json:"final_reticulocyte_count_k_ul"`
	SalineAgglutinationResolved       bool      `json:"saline_agglutination_resolved"`
	NoPulmonaryThromboembolismDyspnea bool      `json:"no_pulmonary_thromboembolism_dyspnea"`
	ThromboprophylaxisDispensed       bool      `json:"thromboprophylaxis_dispensed"`
	VetLicense                        string    `json:"vet_license"`
	Summary                           string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on IMHADischargeClearance.
func (d IMHADischargeClearance) Validate() error {
	if d.FinalPCVStable48HrPercent < 25.0 || d.FinalPCVStable48HrPercent > 65.0 {
		return fmt.Errorf("final_pcv_stable_48_hr_percent must be in [25.0, 65.0], got %v", d.FinalPCVStable48HrPercent)
	}
	if d.FinalReticulocyteCountKUl < 60.0 || d.FinalReticulocyteCountKUl > 1000.0 {
		return fmt.Errorf("final_reticulocyte_count_k_ul must be in [60.0, 1000.0], got %v", d.FinalReticulocyteCountKUl)
	}
	if !d.SalineAgglutinationResolved {
		return fmt.Errorf("saline_agglutination_resolved must be true for discharge clearance")
	}
	if !d.NoPulmonaryThromboembolismDyspnea {
		return fmt.Errorf("no_pulmonary_thromboembolism_dyspnea must be true for discharge clearance")
	}
	if !d.ThromboprophylaxisDispensed {
		return fmt.Errorf("thromboprophylaxis_dispensed must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// IMHAThrombosisOperation represents an Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP),
// Saline Agglutination, Spherocytosis & Thromboprophylaxis triage operation.
type IMHAThrombosisOperation struct {
	ID                            string                        `json:"id"`
	IncidentID                    string                        `json:"incident_id"`
	PetName                       string                        `json:"pet_name"`
	Status                        IMHAThrombosisStatus          `json:"status"`
	PulmonaryThromboembolismAlert bool                          `json:"pulmonary_thromboembolism_alert"`
	PatientProfile                IMHAPatientProfile            `json:"patient_profile"`
	ImmunosuppressiveRegimen      *IMHAImmunosuppressiveRegimen `json:"immunosuppressive_regimen,omitempty"`
	HemolysisChecks               []IMHAHemolysisSerialCheck    `json:"hemolysis_checks"`
	DischargeClearance            *IMHADischargeClearance       `json:"discharge_clearance,omitempty"`
	CreatedAt                     time.Time                     `json:"created_at"`
	UpdatedAt                     time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on IMHAThrombosisOperation.
func (op IMHAThrombosisOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.IncidentID) == "" {
		return fmt.Errorf("incident_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if err := op.PatientProfile.Validate(); err != nil {
		return fmt.Errorf("invalid patient_profile: %w", err)
	}
	return nil
}
