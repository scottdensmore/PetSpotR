package maritimecoastal_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/maritimecoastal"
)

func TestEvaluateMaritimeRescue_ValidationAndZones(t *testing.T) {
	invalidCases := []struct {
		name string
		req  domain.MaritimeEvaluationRequest
	}{
		{
			name: "empty pet name",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "non-positive weight",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "invalid maritime zone",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneType("INVALID_ZONE"),
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "negative tide surge",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: -1.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "negative swell height",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: -0.2,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "water temp too cold",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  -3.5,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "water temp too hot",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  42.0,
				MudSuctionDepthCm: 10.0,
			},
		},
		{
			name: "negative mud suction depth",
			req: domain.MaritimeEvaluationRequest{
				PetName:           "Anchor",
				WeightKg:          20.0,
				MaritimeZone:      domain.MaritimeZoneTidalMudflatEstuary,
				TideSurgeRateCmHr: 10.0,
				SwellHeightMeters: 0.5,
				WaterTempCelsius:  15.0,
				MudSuctionDepthCm: -5.0,
			},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := maritimecoastal.EvaluateMaritimeRescue(tc.req); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}

	zones := []domain.MaritimeZoneType{
		domain.MaritimeZoneTidalMudflatEstuary,
		domain.MaritimeZoneRipCurrentSurfBreak,
		domain.MaritimeZoneJettyTetrapodBreakwater,
		domain.MaritimeZoneOffshoreVesselOverboard,
		domain.MaritimeZoneSaltMarshCordgrassSlough,
	}
	for _, z := range zones {
		op, err := maritimecoastal.EvaluateMaritimeRescue(domain.MaritimeEvaluationRequest{
			PetName:           "Mariner",
			WeightKg:          18.0,
			MaritimeZone:      z,
			TideSurgeRateCmHr: 20.0,
			SwellHeightMeters: 1.2,
			WaterTempCelsius:  15.0,
			MudSuctionDepthCm: 10.0,
		})
		if err != nil {
			t.Fatalf("unexpected error evaluating zone %s: %v", z, err)
		}
		if op.Status != domain.MaritimeStatusHovercraftMudSledStaged {
			t.Errorf("expected HOVERCRAFT_MUD_SLED_STAGED for zone %s, got %s", z, op.Status)
		}
		if op.RequiredVessel == "" {
			t.Errorf("expected non-empty RequiredVessel for zone %s", z)
		}
		if len(op.ProtocolSteps) != 4 {
			t.Errorf("expected 4 protocol steps for zone %s, got %d", z, len(op.ProtocolSteps))
		}
		if err := op.Validate(); err != nil {
			t.Errorf("domain Validate failed for zone %s: %v", z, err)
		}
	}
}

func TestEvaluateMaritimeRescue_CriticalThresholdsAndClearanceLifecycle(t *testing.T) {
	op, err := maritimecoastal.EvaluateMaritimeRescue(domain.MaritimeEvaluationRequest{
		PetName:                      "Anchor",
		Species:                      "Dog - Newfoundland",
		WeightKg:                     54.0,
		MaritimeZone:                 domain.MaritimeZoneTidalMudflatEstuary,
		TideSurgeRateCmHr:            62.0,
		SwellHeightMeters:            2.8,
		WaterTempCelsius:             8.5,
		SaltwaterAspirationSuspected: true,
		MudSuctionDepthCm:            48.0,
		MudWalkersAirLanceDeployed:   false,
		CaninePFDHarnessSecured:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Status != domain.MaritimeStatusTidalFloodAspirationCritical {
		t.Fatalf("expected TIDAL_FLOOD_ASPIRATION_CRITICAL, got %s", op.Status)
	}

	// Issuing clearance before completing steps must fail
	if err := maritimecoastal.IssueMaritimeClearance(op); err == nil {
		t.Fatalf("expected error issuing clearance before completing all 4 steps")
	}

	// Unknown step ID must fail
	if err := maritimecoastal.UpdateProtocolStep(op, "UNKNOWN-STEP", true); err == nil {
		t.Fatalf("expected error for unknown step ID")
	}
	if err := maritimecoastal.UpdateProtocolStep(nil, "STEP-TIDE-SUCTION-RELIEF", true); err == nil {
		t.Fatalf("expected error for nil operation")
	}
	if err := maritimecoastal.UpdateProtocolStep(op, "", true); err == nil {
		t.Fatalf("expected error for empty step ID")
	}

	// Complete all 4 steps
	for _, stepID := range []string{
		"STEP-TIDE-SUCTION-RELIEF",
		"STEP-VESSEL-PFD-RIGGING",
		"STEP-SALTWATER-ASPIRATION-O2",
		"STEP-HARBOR-DECK-DECON-WARM",
	} {
		if err := maritimecoastal.UpdateProtocolStep(op, stepID, true); err != nil {
			t.Fatalf("failed updating step %s: %v", stepID, err)
		}
	}
	if op.Status != domain.MaritimeStatusHovercraftMudSledStaged {
		t.Errorf("expected HOVERCRAFT_MUD_SLED_STAGED after completing steps prior to clearance, got %s", op.Status)
	}

	// Issue clearance
	if err := maritimecoastal.IssueMaritimeClearance(op); err != nil {
		t.Fatalf("unexpected error issuing clearance: %v", err)
	}
	if op.Status != domain.MaritimeStatusExtractedHarborDeckSafe {
		t.Errorf("expected EXTRACTED_HARBOR_DECK_SAFE, got %s", op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "MARITIME-SAFE-") || len(op.ClearanceToken) != len("MARITIME-SAFE-")+12 {
		t.Errorf("unexpected clearance token format: %q", op.ClearanceToken)
	}

	// Unchecking a step clears token and reverts status
	if err := maritimecoastal.UpdateProtocolStep(op, "STEP-HARBOR-DECK-DECON-WARM", false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.ClearanceToken != "" || op.Status != domain.MaritimeStatusHovercraftMudSledStaged {
		t.Errorf("expected empty clearance token and HOVERCRAFT_MUD_SLED_STAGED after unchecking step, got token=%q status=%s", op.ClearanceToken, op.Status)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := maritimecoastal.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("MC-OP-101")
	if !ok || op101.PetName != "Anchor" || op101.Status != domain.MaritimeStatusTidalFloodAspirationCritical {
		t.Errorf("unexpected MC-OP-101: %+v", op101)
	}
	op102, ok := st.Get("MC-OP-102")
	if !ok || op102.PetName != "Coral" || op102.Status != domain.MaritimeStatusHovercraftMudSledStaged {
		t.Errorf("unexpected MC-OP-102: %+v", op102)
	}
	op103, ok := st.Get("MC-OP-103")
	if !ok || op103.PetName != "Skipper" || op103.Status != domain.MaritimeStatusExtractedHarborDeckSafe || !strings.HasPrefix(op103.ClearanceToken, "MARITIME-SAFE-") {
		t.Errorf("unexpected MC-OP-103: %+v", op103)
	}

	created, err := st.EvaluateAndSave(domain.MaritimeEvaluationRequest{
		PetName:           "Barnaby",
		Species:           "Dog - Labrador Retriever",
		WeightKg:          31.0,
		MaritimeZone:      domain.MaritimeZoneOffshoreVesselOverboard,
		TideSurgeRateCmHr: 30.0,
		SwellHeightMeters: 2.9,
		WaterTempCelsius:  11.5,
		MudSuctionDepthCm: 0.0,
	})
	if err != nil {
		t.Fatalf("unexpected error saving operation: %v", err)
	}
	if created.ID != "MC-OP-104" || created.Status != domain.MaritimeStatusTidalFloodAspirationCritical {
		t.Errorf("unexpected created operation: %+v", created)
	}

	for _, stepID := range []string{
		"STEP-TIDE-SUCTION-RELIEF",
		"STEP-VESSEL-PFD-RIGGING",
		"STEP-SALTWATER-ASPIRATION-O2",
		"STEP-HARBOR-DECK-DECON-WARM",
	} {
		if _, err := st.UpdateStep(created.ID, domain.MaritimeStepUpdateRequest{StepID: stepID, Completed: true}); err != nil {
			t.Fatalf("unexpected error updating step %s: %v", stepID, err)
		}
	}

	cleared, err := st.IssueClearance(created.ID)
	if err != nil {
		t.Fatalf("unexpected error issuing clearance: %v", err)
	}
	if cleared.Status != domain.MaritimeStatusExtractedHarborDeckSafe || !strings.HasPrefix(cleared.ClearanceToken, "MARITIME-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}

	if _, ok := st.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for non-existent ID")
	}
	if _, err := st.UpdateStep("NON-EXISTENT", domain.MaritimeStepUpdateRequest{StepID: "STEP-TIDE-SUCTION-RELIEF", Completed: true}); err == nil {
		t.Errorf("expected error updating step on non-existent ID")
	}
	if _, err := st.IssueClearance("NON-EXISTENT"); err == nil {
		t.Errorf("expected error issuing clearance on non-existent ID")
	}
}
