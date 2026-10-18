package domain

import (
	"fmt"
	"strings"
)

type PredatorThreatLevel string

const (
	PredatorThreatExtremeNocturnal    PredatorThreatLevel = "EXTREME_NOCTURNAL_PREDATION_RISK"
	PredatorThreatElevatedCrepuscular PredatorThreatLevel = "ELEVATED_CREPUSCULAR_CORRIDOR"
	PredatorThreatMitigatedSafeZone   PredatorThreatLevel = "MITIGATED_SAFE_ZONE"
)

func (t PredatorThreatLevel) IsValid() bool {
	switch t {
	case PredatorThreatExtremeNocturnal,
		PredatorThreatElevatedCrepuscular,
		PredatorThreatMitigatedSafeZone:
		return true
	default:
		return false
	}
}

type CorridorPhase string

const (
	CorridorPhaseActivePredatorWatch     CorridorPhase = "ACTIVE_PREDATOR_WATCH"
	CorridorPhaseHazingDeterrentDeployed CorridorPhase = "HAZING_DETERRENT_DEPLOYED"
	CorridorPhaseCorridorCleared         CorridorPhase = "CORRIDOR_CLEARED"
)

func (p CorridorPhase) IsValid() bool {
	switch p {
	case CorridorPhaseActivePredatorWatch,
		CorridorPhaseHazingDeterrentDeployed,
		CorridorPhaseCorridorCleared:
		return true
	default:
		return false
	}
}

type CorridorMitigationStep struct {
	ItemID      string `json:"itemId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "ACOUSTIC_STROBE_DETERRENT", "ATTRACTANT_REMOVAL", "NOCTURNAL_SHELTER_CURFEW"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type WildlifeCorridorReport struct {
	ReportID                string                   `json:"reportId"`
	PetID                   string                   `json:"petId"`
	PetName                 string                   `json:"petName"`
	PetSpecies              string                   `json:"petSpecies"`      // "CAT" or "DOG"
	PetWeightKg             float64                  `json:"petWeightKg"`     // 0.5 to 90.0
	PredatorSpecies         string                   `json:"predatorSpecies"` // e.g. "COYOTE_PACK", "GREAT_HORNED_OWL", "BOBCAT", "COUGAR_PUMA"
	PackCount               int                      `json:"packCount"`       // >= 1
	SolarPhase              string                   `json:"solarPhase"`      // "NIGHT", "DAWN_DUSK_CREPUSCULAR", "DAYLIGHT"
	CorridorTerrain         string                   `json:"corridorTerrain"` // e.g. "URBAN_RAVINE_GREENBELT", "BRUSH_CANYON_INTERFACE", "SUBURBAN_CUL_DE_SAC"
	DistanceMeters          float64                  `json:"distanceMeters"`  // >= 0
	VulnerabilityIndex      int                      `json:"vulnerabilityIndex"`
	ThreatLevel             PredatorThreatLevel      `json:"threatLevel"`
	Phase                   CorridorPhase            `json:"phase"`
	SmallPreyAlert          bool                     `json:"smallPreyAlert"`
	DeterrentReadinessScore int                      `json:"deterrentReadinessScore"`
	StrobeDeterrentActive   bool                     `json:"strobeDeterrentActive"`
	AttractantsCleared      bool                     `json:"attractantsCleared"`
	MitigationSteps         []CorridorMitigationStep `json:"mitigationSteps"`
	ClearanceCertificate    string                   `json:"clearanceCertificate,omitempty"`
	CreatedAt               string                   `json:"createdAt"`
	UpdatedAt               string                   `json:"updatedAt"`
}

// Validate checks invariants for WildlifeCorridorReport.
func (r WildlifeCorridorReport) Validate() error {
	if strings.TrimSpace(r.ReportID) == "" {
		return fmt.Errorf("reportId is required")
	}
	if strings.TrimSpace(r.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(r.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(r.PetSpecies)
	if species != "CAT" && species != "DOG" {
		return fmt.Errorf("petSpecies must be CAT or DOG, got %q", r.PetSpecies)
	}
	if r.PetWeightKg < 0.5 || r.PetWeightKg > 90.0 {
		return fmt.Errorf("petWeightKg must be in [0.5, 90.0], got %.2f", r.PetWeightKg)
	}
	if strings.TrimSpace(r.PredatorSpecies) == "" {
		return fmt.Errorf("predatorSpecies is required")
	}
	if r.PackCount < 1 {
		return fmt.Errorf("packCount must be >= 1, got %d", r.PackCount)
	}
	solar := strings.TrimSpace(r.SolarPhase)
	if solar != "NIGHT" && solar != "DAWN_DUSK_CREPUSCULAR" && solar != "DAYLIGHT" {
		return fmt.Errorf("solarPhase must be NIGHT, DAWN_DUSK_CREPUSCULAR, or DAYLIGHT, got %q", r.SolarPhase)
	}
	if strings.TrimSpace(r.CorridorTerrain) == "" {
		return fmt.Errorf("corridorTerrain is required")
	}
	if r.DistanceMeters < 0 {
		return fmt.Errorf("distanceMeters must be >= 0, got %.2f", r.DistanceMeters)
	}
	if r.VulnerabilityIndex < 0 || r.VulnerabilityIndex > 100 {
		return fmt.Errorf("vulnerabilityIndex must be in [0, 100], got %d", r.VulnerabilityIndex)
	}
	if !r.ThreatLevel.IsValid() {
		return fmt.Errorf("invalid threatLevel: %q", r.ThreatLevel)
	}
	if !r.Phase.IsValid() {
		return fmt.Errorf("invalid phase: %q", r.Phase)
	}
	if r.DeterrentReadinessScore < 0 || r.DeterrentReadinessScore > 100 {
		return fmt.Errorf("deterrentReadinessScore must be in [0, 100], got %d", r.DeterrentReadinessScore)
	}
	if len(r.MitigationSteps) < 1 {
		return fmt.Errorf("at least one mitigationStep is required")
	}
	for i, step := range r.MitigationSteps {
		if strings.TrimSpace(step.ItemID) == "" ||
			strings.TrimSpace(step.Title) == "" ||
			strings.TrimSpace(step.Category) == "" {
			return fmt.Errorf("mitigationSteps[%d] must have non-empty itemId, title, and category", i)
		}
	}
	if r.Phase == CorridorPhaseCorridorCleared && strings.TrimSpace(r.ClearanceCertificate) == "" {
		return fmt.Errorf("clearanceCertificate is required when phase is CORRIDOR_CLEARED")
	}
	if strings.TrimSpace(r.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(r.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
