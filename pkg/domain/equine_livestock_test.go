package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleEquineLivestockDispatch() domain.EquineLivestockDispatch {
	return domain.EquineLivestockDispatch{
		DispatchID:             "eq-dsp-101",
		PetID:                  "pet-2026-001",
		AnimalName:             "Valegro & Starlight",
		AnimalCategory:         "EQUINE_WARMBLOOD",
		HeadCount:              2,
		EstimatedWeightKg:      540,
		BreachLocation:         "SR-202 Snoqualmie Valley Bridle Corridor",
		CollisionRiskScore:     81,
		CollisionRisk:          domain.CollisionRiskCriticalHighwayHerd,
		DispatchStatus:         domain.LivestockStatusStampedeLoose,
		HighwayStampedeAlert:   true,
		StockTrailerDispatched: false,
		CorralPanelsErected:    false,
		TrailerReadinessScore:  0,
		ContainmentSteps: []domain.LargeAnimalContainmentStep{
			{
				StepID:    "eq-step-siren-blackout",
				Title:     "Enforce responder siren/air-horn blackout & deploy calm lead-mare sweet-feed bucket lure",
				Category:  "HERD_FLANK_CONTAINMENT",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestEquineLivestockDispatch_Validate(t *testing.T) {
	base := validSampleEquineLivestockDispatch()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample equine livestock dispatch, got error: %v", err)
	}

	if !domain.LivestockStatusStampedeLoose.IsValid() ||
		!domain.LivestockStatusPanelsCorralled.IsValid() ||
		!domain.LivestockStatusLoadedSafeHaul.IsValid() {
		t.Errorf("expected canonical LargeAnimalDispatchStatus values to be valid")
	}
	if domain.LargeAnimalDispatchStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown LargeAnimalDispatchStatus to be invalid")
	}

	if !domain.CollisionRiskCriticalHighwayHerd.IsValid() ||
		!domain.CollisionRiskElevatedPastureBreach.IsValid() ||
		!domain.CollisionRiskContainedPaddockSafe.IsValid() {
		t.Errorf("expected canonical StampedeCollisionRisk values to be valid")
	}
	if domain.StampedeCollisionRisk("UNKNOWN").IsValid() {
		t.Errorf("expected unknown StampedeCollisionRisk to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.EquineLivestockDispatch)
	}{
		{
			name:   "empty dispatchId",
			mutate: func(d *domain.EquineLivestockDispatch) { d.DispatchID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(d *domain.EquineLivestockDispatch) { d.PetID = "" },
		},
		{
			name:   "empty animalName",
			mutate: func(d *domain.EquineLivestockDispatch) { d.AnimalName = "" },
		},
		{
			name:   "invalid animalCategory",
			mutate: func(d *domain.EquineLivestockDispatch) { d.AnimalCategory = "UNKNOWN_CATEGORY" },
		},
		{
			name:   "headCount too low",
			mutate: func(d *domain.EquineLivestockDispatch) { d.HeadCount = 0 },
		},
		{
			name:   "headCount too high",
			mutate: func(d *domain.EquineLivestockDispatch) { d.HeadCount = 51 },
		},
		{
			name:   "estimatedWeightKg too low",
			mutate: func(d *domain.EquineLivestockDispatch) { d.EstimatedWeightKg = 20 },
		},
		{
			name:   "estimatedWeightKg too high",
			mutate: func(d *domain.EquineLivestockDispatch) { d.EstimatedWeightKg = 1205 },
		},
		{
			name:   "empty breachLocation",
			mutate: func(d *domain.EquineLivestockDispatch) { d.BreachLocation = "" },
		},
		{
			name:   "collisionRiskScore out of range",
			mutate: func(d *domain.EquineLivestockDispatch) { d.CollisionRiskScore = 105 },
		},
		{
			name:   "invalid collisionRisk",
			mutate: func(d *domain.EquineLivestockDispatch) { d.CollisionRisk = "BAD_RISK" },
		},
		{
			name:   "invalid dispatchStatus",
			mutate: func(d *domain.EquineLivestockDispatch) { d.DispatchStatus = "BAD_STATUS" },
		},
		{
			name:   "trailerReadinessScore out of range",
			mutate: func(d *domain.EquineLivestockDispatch) { d.TrailerReadinessScore = -5 },
		},
		{
			name:   "empty containmentSteps",
			mutate: func(d *domain.EquineLivestockDispatch) { d.ContainmentSteps = nil },
		},
		{
			name: "invalid step category",
			mutate: func(d *domain.EquineLivestockDispatch) {
				d.ContainmentSteps[0].Category = "BAD_CAT"
			},
		},
		{
			name: "loaded safe haul without paddockClearanceCode",
			mutate: func(d *domain.EquineLivestockDispatch) {
				d.DispatchStatus = domain.LivestockStatusLoadedSafeHaul
				d.PaddockClearanceCode = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(d *domain.EquineLivestockDispatch) { d.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(d *domain.EquineLivestockDispatch) { d.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleEquineLivestockDispatch()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
