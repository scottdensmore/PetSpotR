package domain

import (
	"fmt"
	"strings"
)

type LargeAnimalDispatchStatus string

const (
	LivestockStatusStampedeLoose   LargeAnimalDispatchStatus = "LOOSE_HERD_CORRIDOR_ALERT"
	LivestockStatusPanelsCorralled LargeAnimalDispatchStatus = "PORTABLE_CORRAL_STAGED"
	LivestockStatusLoadedSafeHaul  LargeAnimalDispatchStatus = "LOADED_SAFE_PADDOCK_CUSTODY"
)

func (s LargeAnimalDispatchStatus) IsValid() bool {
	switch s {
	case LivestockStatusStampedeLoose,
		LivestockStatusPanelsCorralled,
		LivestockStatusLoadedSafeHaul:
		return true
	default:
		return false
	}
}

type StampedeCollisionRisk string

const (
	CollisionRiskCriticalHighwayHerd   StampedeCollisionRisk = "CRITICAL_HIGHWAY_STAMPEDE_RISK"
	CollisionRiskElevatedPastureBreach StampedeCollisionRisk = "ELEVATED_PASTURE_PERIMETER_RISK"
	CollisionRiskContainedPaddockSafe  StampedeCollisionRisk = "CONTAINED_PADDOCK_SAFE"
)

func (r StampedeCollisionRisk) IsValid() bool {
	switch r {
	case CollisionRiskCriticalHighwayHerd,
		CollisionRiskElevatedPastureBreach,
		CollisionRiskContainedPaddockSafe:
		return true
	default:
		return false
	}
}

type LargeAnimalContainmentStep struct {
	StepID      string `json:"stepId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "HERD_FLANK_CONTAINMENT", "GOOSENECK_TRAILER_RIGGING", "BRAND_COGGINS_VERIFICATION"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type EquineLivestockDispatch struct {
	DispatchID             string                       `json:"dispatchId"`
	PetID                  string                       `json:"petId"`
	AnimalName             string                       `json:"animalName"`
	AnimalCategory         string                       `json:"animalCategory"`    // "EQUINE_WARMBLOOD", "CAMELID_ALPACA_HERD", "CAPRINE_OVINE_FLOCK", "TORTUOUS_EXOTIC_GIANT"
	HeadCount              int                          `json:"headCount"`         // 1 to 50
	EstimatedWeightKg      int                          `json:"estimatedWeightKg"` // 25 to 1200
	BreachLocation         string                       `json:"breachLocation"`
	CollisionRiskScore     int                          `json:"collisionRiskScore"` // 0 to 100
	CollisionRisk          StampedeCollisionRisk        `json:"collisionRisk"`
	DispatchStatus         LargeAnimalDispatchStatus    `json:"dispatchStatus"`
	HighwayStampedeAlert   bool                         `json:"highwayStampedeAlert"`
	StockTrailerDispatched bool                         `json:"stockTrailerDispatched"`
	CorralPanelsErected    bool                         `json:"corralPanelsErected"`
	TrailerReadinessScore  int                          `json:"trailerReadinessScore"` // 0 to 100
	ContainmentSteps       []LargeAnimalContainmentStep `json:"containmentSteps"`
	PaddockClearanceCode   string                       `json:"paddockClearanceCode,omitempty"`
	CreatedAt              string                       `json:"createdAt"`
	UpdatedAt              string                       `json:"updatedAt"`
}

func validLivestockAnimalCategory(cat string) bool {
	switch cat {
	case "EQUINE_WARMBLOOD", "CAMELID_ALPACA_HERD", "CAPRINE_OVINE_FLOCK", "TORTUOUS_EXOTIC_GIANT":
		return true
	default:
		return false
	}
}

func validLivestockContainmentCategory(cat string) bool {
	switch cat {
	case "HERD_FLANK_CONTAINMENT", "GOOSENECK_TRAILER_RIGGING", "BRAND_COGGINS_VERIFICATION":
		return true
	default:
		return false
	}
}

// Validate checks invariants for EquineLivestockDispatch.
func (d EquineLivestockDispatch) Validate() error {
	if strings.TrimSpace(d.DispatchID) == "" {
		return fmt.Errorf("dispatchId is required")
	}
	if strings.TrimSpace(d.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(d.AnimalName) == "" {
		return fmt.Errorf("animalName is required")
	}
	if !validLivestockAnimalCategory(strings.TrimSpace(d.AnimalCategory)) {
		return fmt.Errorf("invalid animalCategory: %q", d.AnimalCategory)
	}
	if d.HeadCount < 1 || d.HeadCount > 50 {
		return fmt.Errorf("headCount must be in [1, 50], got %d", d.HeadCount)
	}
	if d.EstimatedWeightKg < 25 || d.EstimatedWeightKg > 1200 {
		return fmt.Errorf("estimatedWeightKg must be in [25, 1200], got %d", d.EstimatedWeightKg)
	}
	if strings.TrimSpace(d.BreachLocation) == "" {
		return fmt.Errorf("breachLocation is required")
	}
	if d.CollisionRiskScore < 0 || d.CollisionRiskScore > 100 {
		return fmt.Errorf("collisionRiskScore must be in [0, 100], got %d", d.CollisionRiskScore)
	}
	if !d.CollisionRisk.IsValid() {
		return fmt.Errorf("invalid collisionRisk: %q", d.CollisionRisk)
	}
	if !d.DispatchStatus.IsValid() {
		return fmt.Errorf("invalid dispatchStatus: %q", d.DispatchStatus)
	}
	if d.TrailerReadinessScore < 0 || d.TrailerReadinessScore > 100 {
		return fmt.Errorf("trailerReadinessScore must be in [0, 100], got %d", d.TrailerReadinessScore)
	}
	if len(d.ContainmentSteps) < 1 {
		return fmt.Errorf("at least one containmentStep is required")
	}
	for i, step := range d.ContainmentSteps {
		if strings.TrimSpace(step.StepID) == "" ||
			strings.TrimSpace(step.Title) == "" ||
			!validLivestockContainmentCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("containmentSteps[%d] must have non-empty stepId, title, and valid category", i)
		}
	}
	if d.DispatchStatus == LivestockStatusLoadedSafeHaul && strings.TrimSpace(d.PaddockClearanceCode) == "" {
		return fmt.Errorf("paddockClearanceCode is required when dispatchStatus is LOADED_SAFE_PADDOCK_CUSTODY")
	}
	if strings.TrimSpace(d.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(d.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
