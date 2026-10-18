package equinelivestock_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/equinelivestock"
)

func TestEvaluateEquineLivestockDispatch_SpookedWarmblood(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	d, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             "eq-dsp-101",
		PetID:                  "pet-2026-001",
		AnimalName:             "Valegro & Starlight",
		AnimalCategory:         "EQUINE_WARMBLOOD",
		HeadCount:              2,
		EstimatedWeightKg:      540,
		BreachLocation:         "SR-202 Snoqualmie Valley Bridle Corridor",
		StockTrailerDispatched: false,
		CorralPanelsErected:    false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating livestock dispatch: %v", err)
	}

	if d.DispatchStatus != domain.LivestockStatusStampedeLoose {
		t.Errorf("expected LOOSE_HERD_CORRIDOR_ALERT, got %s", d.DispatchStatus)
	}
	if d.CollisionRisk != domain.CollisionRiskCriticalHighwayHerd {
		t.Errorf("expected CRITICAL_HIGHWAY_STAMPEDE_RISK, got %s", d.CollisionRisk)
	}
	if !d.HighwayStampedeAlert {
		t.Errorf("expected HighwayStampedeAlert=true for 540kg Warmblood")
	}
	if d.CollisionRiskScore != 81 {
		t.Errorf("expected CollisionRiskScore=81, got %d", d.CollisionRiskScore)
	}
	if len(d.ContainmentSteps) != 4 {
		t.Errorf("expected 4 containment steps, got %d", len(d.ContainmentSteps))
	}
}

func TestApplyLivestockDispatchUpdate_StageAndCompleteLoad(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	d, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:             "eq-dsp-102",
		PetID:                  "pet-2026-002",
		AnimalName:             "Andes Alpaca Trio",
		AnimalCategory:         "CAMELID_ALPACA_HERD",
		HeadCount:              3,
		EstimatedWeightKg:      195,
		BreachLocation:         "Vashon Island North Pasture Gate",
		StockTrailerDispatched: false,
		CorralPanelsErected:    false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating livestock dispatch: %v", err)
	}

	// 1. StageCorralAndTrailer -> PORTABLE_CORRAL_STAGED
	if err := equinelivestock.ApplyLivestockDispatchUpdate(d, equinelivestock.LivestockDispatchUpdateInput{
		StageCorralAndTrailer: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error staging corral and trailer: %v", err)
	}

	if d.DispatchStatus != domain.LivestockStatusPanelsCorralled {
		t.Errorf("expected PORTABLE_CORRAL_STAGED, got %s", d.DispatchStatus)
	}
	if d.CollisionRisk != domain.CollisionRiskElevatedPastureBreach {
		t.Errorf("expected ELEVATED_PASTURE_PERIMETER_RISK, got %s", d.CollisionRisk)
	}
	if d.TrailerReadinessScore != 100 {
		t.Errorf("expected TrailerReadinessScore=100, got %d", d.TrailerReadinessScore)
	}

	// 2. Complete individual step
	if err := equinelivestock.ApplyLivestockDispatchUpdate(d, equinelivestock.LivestockDispatchUpdateInput{
		CompleteStepID: "eq-step-gooseneck-ramp",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing step: %v", err)
	}

	// 3. Unknown step ID returns error
	if err := equinelivestock.ApplyLivestockDispatchUpdate(d, equinelivestock.LivestockDispatchUpdateInput{
		CompleteStepID: "unknown-step",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown step ID")
	}

	// 4. CompleteSafeTrailerLoad -> LOADED_SAFE_PADDOCK_CUSTODY
	if err := equinelivestock.ApplyLivestockDispatchUpdate(d, equinelivestock.LivestockDispatchUpdateInput{
		CompleteSafeTrailerLoad: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing safe trailer load: %v", err)
	}

	if d.DispatchStatus != domain.LivestockStatusLoadedSafeHaul {
		t.Errorf("expected LOADED_SAFE_PADDOCK_CUSTODY, got %s", d.DispatchStatus)
	}
	if d.CollisionRisk != domain.CollisionRiskContainedPaddockSafe {
		t.Errorf("expected CONTAINED_PADDOCK_SAFE, got %s", d.CollisionRisk)
	}
	if d.CollisionRiskScore != 5 {
		t.Errorf("expected CollisionRiskScore=5, got %d", d.CollisionRiskScore)
	}
	if d.PaddockClearanceCode != "PADDOCK-SAFE-EQ-DSP-102" {
		t.Errorf("expected PADDOCK-SAFE-EQ-DSP-102, got %s", d.PaddockClearanceCode)
	}
}

func TestEvaluateEquineLivestockDispatch_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:        "",
		PetID:             "pet-1",
		AnimalName:        "Starlight",
		BreachLocation:    "SR-202",
		HeadCount:         1,
		EstimatedWeightKg: 450,
	}, now); err == nil {
		t.Errorf("expected error for missing dispatchId")
	}

	if _, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:        "eq-1",
		PetID:             "pet-1",
		AnimalName:        "Starlight",
		BreachLocation:    "SR-202",
		HeadCount:         0,
		EstimatedWeightKg: 450,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range headCount")
	}

	if _, err := equinelivestock.EvaluateEquineLivestockDispatch(equinelivestock.CreateLivestockDispatchInput{
		DispatchID:        "eq-1",
		PetID:             "pet-1",
		AnimalName:        "Starlight",
		BreachLocation:    "SR-202",
		HeadCount:         1,
		EstimatedWeightKg: 10,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range estimatedWeightKg")
	}

	if err := equinelivestock.ApplyLivestockDispatchUpdate(nil, equinelivestock.LivestockDispatchUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil dispatch")
	}
}
