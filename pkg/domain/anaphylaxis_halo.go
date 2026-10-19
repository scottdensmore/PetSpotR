package domain

import (
	"fmt"
	"strings"
	"time"
)

// AnaphylaxisHaloStatus represents the lifecycle state of an Acute Anaphylaxis,
// Hemoabdomen Halo Sign (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation triage operation.
type AnaphylaxisHaloStatus string

const (
	AnaphylaxisStatusShockHaloCritical    AnaphylaxisHaloStatus = "ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL"
	AnaphylaxisStatusEpinephrineFFPStaged AnaphylaxisHaloStatus = "EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED"
	AnaphylaxisStatusBiphasic24HrCleared  AnaphylaxisHaloStatus = "BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED"
)

// IsValid returns true when AnaphylaxisHaloStatus is a supported lifecycle state.
func (s AnaphylaxisHaloStatus) IsValid() bool {
	switch s {
	case AnaphylaxisStatusShockHaloCritical,
		AnaphylaxisStatusEpinephrineFFPStaged,
		AnaphylaxisStatusBiphasic24HrCleared:
		return true
	default:
		return false
	}
}

const (
	AnaphylaxisSpeciesCanineHepaticSplanchnic      = "CANINE_HEPATIC_SPLANCHNIC_SHOCK"
	AnaphylaxisSpeciesCanineHymenopteraHemoabdomen = "CANINE_HYMENOPTERA_HEMOABDOMEN"
	AnaphylaxisSpeciesFelineBronchospasmPulmonary  = "FELINE_BRONCHOSPASM_PULMONARY_SHOCK"
)

// IsValidAnaphylaxisSpecies returns true if species is a recognized anaphylaxis shock shock-organ classification.
func IsValidAnaphylaxisSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case AnaphylaxisSpeciesCanineHepaticSplanchnic,
		AnaphylaxisSpeciesCanineHymenopteraHemoabdomen,
		AnaphylaxisSpeciesFelineBronchospasmPulmonary:
		return true
	default:
		return false
	}
}

// AnaphylaxisPatientProfile captures signalment, ultrasound gallbladder wall thickness & striated Halo Sign,
// mean arterial pressure (MAP), serum ALT, packed cell volume (PCV), and prothrombin time (PT).
type AnaphylaxisPatientProfile struct {
	Species                    string  `json:"species"`
	Breed                      string  `json:"breed,omitempty"`
	WeightKg                   float64 `json:"weight_kg"`
	GallbladderWallThicknessMm float64 `json:"gallbladder_wall_thickness_mm"`
	GallbladderHaloSignPresent bool    `json:"gallbladder_halo_sign_present"`
	MeanArterialPressureMmHg   int     `json:"mean_arterial_pressure_mmhg"`
	AlanineAminotransferaseUL  int     `json:"alanine_aminotransferase_u_l"`
	PackedCellVolumePercent    float64 `json:"packed_cell_volume_percent"`
	ProthrombinTimeSeconds     float64 `json:"prothrombin_time_seconds"`
}

// Validate enforces clinical invariants on AnaphylaxisPatientProfile.
func (p AnaphylaxisPatientProfile) Validate() error {
	if !IsValidAnaphylaxisSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_HEPATIC_SPLANCHNIC_SHOCK, CANINE_HYMENOPTERA_HEMOABDOMEN, or FELINE_BRONCHOSPASM_PULMONARY_SHOCK, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.GallbladderWallThicknessMm < 0.5 || p.GallbladderWallThicknessMm > 15.0 {
		return fmt.Errorf("gallbladder_wall_thickness_mm must be in [0.5, 15.0], got %v", p.GallbladderWallThicknessMm)
	}
	if p.MeanArterialPressureMmHg < 20 || p.MeanArterialPressureMmHg > 200 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [20, 200], got %d", p.MeanArterialPressureMmHg)
	}
	if p.AlanineAminotransferaseUL < 10 || p.AlanineAminotransferaseUL > 25000 {
		return fmt.Errorf("alanine_aminotransferase_u_l must be in [10, 25000], got %d", p.AlanineAminotransferaseUL)
	}
	if p.PackedCellVolumePercent < 10.0 || p.PackedCellVolumePercent > 85.0 {
		return fmt.Errorf("packed_cell_volume_percent must be in [10.0, 85.0], got %v", p.PackedCellVolumePercent)
	}
	if p.ProthrombinTimeSeconds < 5.0 || p.ProthrombinTimeSeconds > 120.0 {
		return fmt.Errorf("prothrombin_time_seconds must be in [5.0, 120.0], got %v", p.ProthrombinTimeSeconds)
	}
	return nil
}

// AnaphylaxisResuscitationPlan captures calculated Epinephrine IM dose (0.01 mg/kg capped at 0.3 mg < 20 kg / 0.5 mg >= 20 kg),
// Epinephrine CRI (0.05 mcg/kg/min for refractory hypotension MAP < 60 mmHg), isotonic crystalloid bolus (20 mL/kg canine vs 10 mL/kg feline),
// Fresh Frozen Plasma (15 mL/kg when PT > 18.0 s for anaphylactic hemoabdomen/hyperfibrinolysis), Diphenhydramine IM (2.0 mg/kg),
// Famotidine IV (0.5 mg/kg), and surgical cholecystectomy / IV undiluted Epinephrine / IV rapid Diphenhydramine guard.
type AnaphylaxisResuscitationPlan struct {
	EpinephrineIM001MgKgDoseMg                     float64 `json:"epinephrine_im_001_mg_kg_dose_mg"`
	EpinephrineCRI005McgKgMin                      float64 `json:"epinephrine_cri_005_mcg_kg_min"`
	IsotonicCrystalloidBolusMl                     float64 `json:"isotonic_crystalloid_bolus_ml"`
	FreshFrozenPlasma15MlKgMl                      float64 `json:"fresh_frozen_plasma_15_ml_kg_ml"`
	DiphenhydramineIM2MgKgMg                       float64 `json:"diphenhydramine_im_2_mg_kg_mg"`
	FamotidineIV05MgKgMg                           float64 `json:"famotidine_iv_05_mg_kg_mg"`
	ZeroIVUndilutedEpiSurgicalCholecystectomyGuard bool    `json:"zero_iv_undiluted_epi_surgical_cholecystectomy_guard"`
	AnaphylaxisPitfallAlert                        string  `json:"anaphylaxis_pitfall_alert"`
	ClinicalRationale                              string  `json:"clinical_rationale"`
}

// AnaphylaxisBiphasicCheck records a serial telemetry check during the mandatory 24-hour biphasic anaphylaxis observation window.
type AnaphylaxisBiphasicCheck struct {
	Timestamp                  time.Time `json:"timestamp"`
	MeanArterialPressureMmHg   int       `json:"mean_arterial_pressure_mmhg"`
	GallbladderWallThicknessMm float64   `json:"gallbladder_wall_thickness_mm"`
	PackedCellVolumePercent    float64   `json:"packed_cell_volume_percent"`
	AlanineAminotransferaseUL  int       `json:"alanine_aminotransferase_u_l"`
	BiphasicRelapseSignsNoted  bool      `json:"biphasic_relapse_signs_noted"`
	ClinicianCallsign          string    `json:"clinician_callsign"`
	Notes                      string    `json:"notes"`
}

// Validate enforces clinical invariants on AnaphylaxisBiphasicCheck.
func (c AnaphylaxisBiphasicCheck) Validate() error {
	if c.MeanArterialPressureMmHg < 20 || c.MeanArterialPressureMmHg > 200 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [20, 200], got %d", c.MeanArterialPressureMmHg)
	}
	if c.GallbladderWallThicknessMm < 0.5 || c.GallbladderWallThicknessMm > 15.0 {
		return fmt.Errorf("gallbladder_wall_thickness_mm must be in [0.5, 15.0], got %v", c.GallbladderWallThicknessMm)
	}
	if c.PackedCellVolumePercent < 10.0 || c.PackedCellVolumePercent > 85.0 {
		return fmt.Errorf("packed_cell_volume_percent must be in [10.0, 85.0], got %v", c.PackedCellVolumePercent)
	}
	if c.AlanineAminotransferaseUL < 10 || c.AlanineAminotransferaseUL > 25000 {
		return fmt.Errorf("alanine_aminotransferase_u_l must be in [10, 25000], got %d", c.AlanineAminotransferaseUL)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// AnaphylaxisDischargeClearance certifies >= 24h biphasic anaphylaxis observation, normotension (MAP 75-120 mmHg),
// gallbladder wall Halo Sign resolution, coagulopathy normalization, and owner EpiPen auto-injector education.
type AnaphylaxisDischargeClearance struct {
	ClearedAt                  time.Time `json:"cleared_at"`
	BiphasicObservationHours   int       `json:"biphasic_observation_hours"`
	FinalMAPMmHg               int       `json:"final_map_mmhg"`
	GallbladderHaloResolved    bool      `json:"gallbladder_halo_resolved"`
	CoagulopathyNormalized     bool      `json:"coagulopathy_normalized"`
	EpiPenAutoInjectorEducated bool      `json:"epipen_auto_injector_educated"`
	VetLicense                 string    `json:"vet_license"`
	Summary                    string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on AnaphylaxisDischargeClearance.
func (d AnaphylaxisDischargeClearance) Validate() error {
	if d.BiphasicObservationHours < 24 || d.BiphasicObservationHours > 168 {
		return fmt.Errorf("biphasic_observation_hours must be >= 24 (in [24, 168]), got %d", d.BiphasicObservationHours)
	}
	if d.FinalMAPMmHg < 75 || d.FinalMAPMmHg > 120 {
		return fmt.Errorf("final_map_mmhg must be in [75, 120], got %d", d.FinalMAPMmHg)
	}
	if !d.GallbladderHaloResolved {
		return fmt.Errorf("gallbladder_halo_resolved must be true for discharge clearance")
	}
	if !d.CoagulopathyNormalized {
		return fmt.Errorf("coagulopathy_normalized must be true for discharge clearance")
	}
	if !d.EpiPenAutoInjectorEducated {
		return fmt.Errorf("epipen_auto_injector_educated must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// AnaphylaxisHaloOperation represents an Acute Anaphylaxis, Hemoabdomen Halo Sign
// (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation triage operation.
type AnaphylaxisHaloOperation struct {
	ID                                 string                         `json:"id"`
	IncidentID                         string                         `json:"incident_id"`
	PetName                            string                         `json:"pet_name"`
	Status                             AnaphylaxisHaloStatus          `json:"status"`
	HepaticCongestionCoagulopathyAlert bool                           `json:"hepatic_congestion_coagulopathy_alert"`
	PatientProfile                     AnaphylaxisPatientProfile      `json:"patient_profile"`
	ResuscitationPlan                  *AnaphylaxisResuscitationPlan  `json:"resuscitation_plan,omitempty"`
	BiphasicChecks                     []AnaphylaxisBiphasicCheck     `json:"biphasic_checks"`
	DischargeClearance                 *AnaphylaxisDischargeClearance `json:"discharge_clearance,omitempty"`
	CreatedAt                          time.Time                      `json:"created_at"`
	UpdatedAt                          time.Time                      `json:"updated_at"`
}

// Validate checks domain invariants on AnaphylaxisHaloOperation.
func (op AnaphylaxisHaloOperation) Validate() error {
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
