package municipalcad

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestIngestMunicipal311Call_ValidationAndDefaults(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   CreateCADIncidentInput
		wantErr bool
	}{
		{
			name: "missing incidentId",
			input: CreateCADIncidentInput{
				PetID:         "pet-1",
				PetName:       "Cooper",
				RoadwayName:   "I-5 Median",
				SpeedLimitMPH: 60,
				LaneCount:     4,
			},
			wantErr: true,
		},
		{
			name: "missing petId",
			input: CreateCADIncidentInput{
				IncidentID:    "cad-1",
				PetName:       "Cooper",
				RoadwayName:   "I-5 Median",
				SpeedLimitMPH: 60,
				LaneCount:     4,
			},
			wantErr: true,
		},
		{
			name: "missing petName",
			input: CreateCADIncidentInput{
				IncidentID:    "cad-1",
				PetID:         "pet-1",
				RoadwayName:   "I-5 Median",
				SpeedLimitMPH: 60,
				LaneCount:     4,
			},
			wantErr: true,
		},
		{
			name: "missing roadwayName",
			input: CreateCADIncidentInput{
				IncidentID:    "cad-1",
				PetID:         "pet-1",
				PetName:       "Cooper",
				SpeedLimitMPH: 60,
				LaneCount:     4,
			},
			wantErr: true,
		},
		{
			name: "speedLimitMph out of range",
			input: CreateCADIncidentInput{
				IncidentID:    "cad-1",
				PetID:         "pet-1",
				PetName:       "Cooper",
				RoadwayName:   "I-5 Median",
				SpeedLimitMPH: 10,
				LaneCount:     4,
			},
			wantErr: true,
		},
		{
			name: "laneCount out of range",
			input: CreateCADIncidentInput{
				IncidentID:    "cad-1",
				PetID:         "pet-1",
				PetName:       "Cooper",
				RoadwayName:   "I-5 Median",
				SpeedLimitMPH: 60,
				LaneCount:     0,
			},
			wantErr: true,
		},
		{
			name: "valid freeway median panic call with defaults",
			input: CreateCADIncidentInput{
				IncidentID:       "cad-inc-101",
				PetID:            "pet-2026-001",
				PetName:          "Cooper",
				RoadwayName:      "I-5 Express Lanes Milepost 167 Median",
				SpeedLimitMPH:    60,
				LaneCount:        4,
				PetBehaviorState: "BOLTING_INTO_TRAFFIC",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inc, err := IngestMunicipal311Call(tc.input, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inc.Species != "DOG" {
				t.Errorf("expected default species DOG, got %q", inc.Species)
			}
			if inc.CADCallNumber != "311-CAD-CAD-INC-101" {
				t.Errorf("expected default CADCallNumber 311-CAD-CAD-INC-101, got %q", inc.CADCallNumber)
			}
			if !inc.DoNotChaseAlert {
				t.Errorf("expected DoNotChaseAlert=true for 60 mph BOLTING_INTO_TRAFFIC")
			}
			if inc.DispatchStatus != domain.CADStatusInbound311Triage {
				t.Errorf("expected status INBOUND_311_TRIAGE, got %s", inc.DispatchStatus)
			}
			if inc.HazardSeverity != domain.RoadwayHazardFreewayCritical {
				t.Errorf("expected severity FREEWAY_MEDIAN_CRITICAL, got %s", inc.HazardSeverity)
			}
			if len(inc.ProtocolSteps) != 4 {
				t.Fatalf("expected 4 protocol steps, got %d", len(inc.ProtocolSteps))
			}
		})
	}
}

func TestApplyCADDispatchAction_ContainmentAndSafeCustodyResolution(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	inc, err := IngestMunicipal311Call(CreateCADIncidentInput{
		IncidentID:       "cad-inc-102",
		CADCallNumber:    "311-SEA-8835",
		PetID:            "pet-2026-002",
		PetName:          "Hazel",
		Species:          "DOG",
		RoadwayName:      "SR-99 Aurora Bridge North Approach",
		SpeedLimitMPH:    45,
		LaneCount:        3,
		PetBehaviorState: "HUDDLED_IN_MEDIAN",
	}, now)
	if err != nil {
		t.Fatalf("unexpected ingest error: %v", err)
	}

	// Step 1: Deploy highway containment
	t1 := now.Add(5 * time.Minute)
	if err := ApplyCADDispatchAction(inc, CADDispatchUpdateInput{
		DeployHighwayContainment: true,
	}, t1); err != nil {
		t.Fatalf("unexpected deploy error: %v", err)
	}
	if inc.DispatchStatus != domain.CADStatusHighwayContainment {
		t.Errorf("expected HIGHWAY_CONTAINMENT_DISPATCHED, got %s", inc.DispatchStatus)
	}
	if inc.HazardSeverity != domain.RoadwayHazardArterialHigh {
		t.Errorf("expected ARTERIAL_TRAFFIC_HIGH, got %s", inc.HazardSeverity)
	}
	if !inc.DOTSignAdvisoryActive || inc.ACOUnitAssigned != "ACO-UNIT-14" || inc.ContainmentReadinessScore != 100 {
		t.Errorf("expected DOTSignAdvisoryActive=true, ACO-UNIT-14, readiness=100, got %+v", inc)
	}

	// Step 2: Complete single step cad-step-l-barrier
	t2 := now.Add(10 * time.Minute)
	if err := ApplyCADDispatchAction(inc, CADDispatchUpdateInput{
		CompleteStepID: "cad-step-l-barrier",
	}, t2); err != nil {
		t.Fatalf("unexpected complete step error: %v", err)
	}
	if !inc.ProtocolSteps[2].Completed {
		t.Errorf("expected step 2 (cad-step-l-barrier) completed")
	}

	// Step 3: Resolve safe custody
	t3 := now.Add(15 * time.Minute)
	if err := ApplyCADDispatchAction(inc, CADDispatchUpdateInput{
		ResolveSafeCustody: true,
	}, t3); err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if inc.DispatchStatus != domain.CADStatusResolvedSafeCustody {
		t.Errorf("expected RESOLVED_SAFE_CUSTODY, got %s", inc.DispatchStatus)
	}
	if inc.HazardSeverity != domain.RoadwayHazardContainedSafe {
		t.Errorf("expected CONTAINED_CORRIDOR_SAFE, got %s", inc.HazardSeverity)
	}
	if inc.TrafficStrikeRiskScore != 5 {
		t.Errorf("expected TrafficStrikeRiskScore 5, got %d", inc.TrafficStrikeRiskScore)
	}
	if inc.CustodyResolutionCode != "CAD-SAFE-CAD-INC-102" {
		t.Errorf("expected CustodyResolutionCode CAD-SAFE-CAD-INC-102, got %q", inc.CustodyResolutionCode)
	}
}

func TestApplyCADDispatchAction_Errors(t *testing.T) {
	now := time.Now().UTC()
	if err := ApplyCADDispatchAction(nil, CADDispatchUpdateInput{}, now); err == nil {
		t.Fatalf("expected error for nil incident")
	}

	inc, err := IngestMunicipal311Call(CreateCADIncidentInput{
		IncidentID:    "cad-err",
		PetID:         "pet-1",
		PetName:       "Max",
		RoadwayName:   "Main St",
		SpeedLimitMPH: 30,
		LaneCount:     2,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ApplyCADDispatchAction(inc, CADDispatchUpdateInput{CompleteStepID: "unknown-step"}, now); err == nil {
		t.Errorf("expected error for unknown CompleteStepID")
	}
}
