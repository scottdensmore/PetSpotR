package equinelivestock

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateLivestockDispatchInput struct {
	DispatchID             string
	PetID                  string
	AnimalName             string
	AnimalCategory         string
	BreachLocation         string
	HeadCount              int
	EstimatedWeightKg      int
	StockTrailerDispatched bool
	CorralPanelsErected    bool
}

type LivestockDispatchUpdateInput struct {
	StageCorralAndTrailer   bool
	CompleteSafeTrailerLoad bool
	CompleteStepID          string
	StockTrailerDispatched  *bool
	CorralPanelsErected     *bool
}

func EvaluateEquineLivestockDispatch(input CreateLivestockDispatchInput, now time.Time) (*domain.EquineLivestockDispatch, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	dispatchID := strings.TrimSpace(input.DispatchID)
	petID := strings.TrimSpace(input.PetID)
	animalName := strings.TrimSpace(input.AnimalName)
	breachLocation := strings.TrimSpace(input.BreachLocation)

	if dispatchID == "" {
		return nil, fmt.Errorf("dispatchId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if animalName == "" {
		return nil, fmt.Errorf("animalName is required")
	}
	if breachLocation == "" {
		return nil, fmt.Errorf("breachLocation is required")
	}
	if input.HeadCount < 1 || input.HeadCount > 50 {
		return nil, fmt.Errorf("headCount must be in [1, 50], got %d", input.HeadCount)
	}
	if input.EstimatedWeightKg < 25 || input.EstimatedWeightKg > 1200 {
		return nil, fmt.Errorf("estimatedWeightKg must be in [25, 1200], got %d", input.EstimatedWeightKg)
	}

	animalCategory := strings.ToUpper(strings.TrimSpace(input.AnimalCategory))
	if animalCategory == "" {
		animalCategory = "EQUINE_WARMBLOOD"
	}

	steps := []domain.LargeAnimalContainmentStep{
		{
			StepID:    "eq-step-siren-blackout",
			Title:     "Enforce responder siren/air-horn blackout & deploy calm lead-mare sweet-feed bucket lure",
			Category:  "HERD_FLANK_CONTAINMENT",
			Completed: false,
		},
		{
			StepID:    "eq-step-wing-panels",
			Title:     "Erect interlocking 12ft portable steel corral wing-panels along breach funnel",
			Category:  "HERD_FLANK_CONTAINMENT",
			Completed: false,
		},
		{
			StepID:    "eq-step-gooseneck-ramp",
			Title:     "Stage non-slip rubberized 4-horse gooseneck stock trailer with low-angle loading ramp",
			Category:  "GOOSENECK_TRAILER_RIGGING",
			Completed: false,
		},
		{
			StepID:    "eq-step-coggins-brand",
			Title:     "Verify EIA Coggins certificate, freeze-brand/RFID ear tag & secure paddock turnout",
			Category:  "BRAND_COGGINS_VERIFICATION",
			Completed: false,
		},
	}

	dispatch := &domain.EquineLivestockDispatch{
		DispatchID:             dispatchID,
		PetID:                  petID,
		AnimalName:             animalName,
		AnimalCategory:         animalCategory,
		HeadCount:              input.HeadCount,
		EstimatedWeightKg:      input.EstimatedWeightKg,
		BreachLocation:         breachLocation,
		StockTrailerDispatched: input.StockTrailerDispatched,
		CorralPanelsErected:    input.CorralPanelsErected,
		ContainmentSteps:       steps,
		CreatedAt:              nowStr,
		UpdatedAt:              nowStr,
	}

	RecomputeLivestockMetrics(dispatch, now)

	if err := dispatch.Validate(); err != nil {
		return nil, err
	}

	return dispatch, nil
}

func RecomputeLivestockMetrics(d *domain.EquineLivestockDispatch, now time.Time) {
	if d == nil {
		return
	}
	_ = now

	d.HighwayStampedeAlert = d.EstimatedWeightKg >= 350 ||
		d.HeadCount >= 3 ||
		d.AnimalCategory == "EQUINE_WARMBLOOD"

	readiness := 0
	if d.StockTrailerDispatched {
		readiness += 50
	}
	if d.CorralPanelsErected {
		readiness += 50
	}
	d.TrailerReadinessScore = readiness

	totalSteps := len(d.ContainmentSteps)
	completedSteps := 0
	for _, step := range d.ContainmentSteps {
		if step.Completed {
			completedSteps++
		}
	}

	baseRisk := 30
	switch d.AnimalCategory {
	case "EQUINE_WARMBLOOD":
		baseRisk = 55
	case "CAMELID_ALPACA_HERD":
		baseRisk = 45
	case "CAPRINE_OVINE_FLOCK":
		baseRisk = 40
	case "TORTUOUS_EXOTIC_GIANT":
		baseRisk = 30
	}

	weightPenalty := 0
	if d.EstimatedWeightKg >= 400 {
		weightPenalty = 20
	} else if d.EstimatedWeightKg >= 150 {
		weightPenalty = 10
	}

	herdPenalty := 0
	if d.HeadCount > 1 {
		herdPenalty = (d.HeadCount - 1) * 6
		if herdPenalty > 20 {
			herdPenalty = 20
		}
	}

	risk := baseRisk + weightPenalty + herdPenalty - (completedSteps * 15) - (readiness / 4)
	if risk < 5 {
		risk = 5
	}
	if risk > 100 {
		risk = 100
	}
	d.CollisionRiskScore = risk

	if totalSteps > 0 && completedSteps == totalSteps && d.StockTrailerDispatched && d.CorralPanelsErected {
		d.DispatchStatus = domain.LivestockStatusLoadedSafeHaul
		d.CollisionRisk = domain.CollisionRiskContainedPaddockSafe
		d.CollisionRiskScore = 5
		if d.PaddockClearanceCode == "" {
			d.PaddockClearanceCode = "PADDOCK-SAFE-" + strings.ToUpper(d.DispatchID)
		}
	} else if completedSteps >= 1 || (d.StockTrailerDispatched && d.CorralPanelsErected) {
		d.DispatchStatus = domain.LivestockStatusPanelsCorralled
		if d.CollisionRiskScore >= 70 {
			d.CollisionRisk = domain.CollisionRiskCriticalHighwayHerd
		} else {
			d.CollisionRisk = domain.CollisionRiskElevatedPastureBreach
		}
	} else {
		d.DispatchStatus = domain.LivestockStatusStampedeLoose
		if d.HighwayStampedeAlert || d.CollisionRiskScore >= 65 {
			d.CollisionRisk = domain.CollisionRiskCriticalHighwayHerd
		} else {
			d.CollisionRisk = domain.CollisionRiskElevatedPastureBreach
		}
	}
}

func ApplyLivestockDispatchUpdate(d *domain.EquineLivestockDispatch, input LivestockDispatchUpdateInput, now time.Time) error {
	if d == nil {
		return fmt.Errorf("equine livestock dispatch is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.StockTrailerDispatched != nil {
		d.StockTrailerDispatched = *input.StockTrailerDispatched
	}
	if input.CorralPanelsErected != nil {
		d.CorralPanelsErected = *input.CorralPanelsErected
	}

	if stepID := strings.TrimSpace(input.CompleteStepID); stepID != "" {
		found := false
		for i := range d.ContainmentSteps {
			if d.ContainmentSteps[i].StepID == stepID {
				d.ContainmentSteps[i].Completed = true
				if d.ContainmentSteps[i].CompletedAt == "" {
					d.ContainmentSteps[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("containment step %q not found", stepID)
		}
	}

	if input.StageCorralAndTrailer {
		for i := range d.ContainmentSteps {
			if i < 2 {
				d.ContainmentSteps[i].Completed = true
				if d.ContainmentSteps[i].CompletedAt == "" {
					d.ContainmentSteps[i].CompletedAt = nowStr
				}
			}
		}
		d.StockTrailerDispatched = true
		d.CorralPanelsErected = true
	}

	if input.CompleteSafeTrailerLoad {
		for i := range d.ContainmentSteps {
			d.ContainmentSteps[i].Completed = true
			if d.ContainmentSteps[i].CompletedAt == "" {
				d.ContainmentSteps[i].CompletedAt = nowStr
			}
		}
		d.StockTrailerDispatched = true
		d.CorralPanelsErected = true
	}

	RecomputeLivestockMetrics(d, now)
	d.UpdatedAt = nowStr

	return d.Validate()
}
