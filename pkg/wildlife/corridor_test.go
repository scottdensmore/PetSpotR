package wildlife

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateWildlifeCorridor_ValidationAndDefaults(t *testing.T) {
	now := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   CreateCorridorInput
		wantErr bool
	}{
		{
			name: "missing reportId",
			input: CreateCorridorInput{
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      2,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "missing petId",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      2,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "missing petName",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetWeightKg:    4.2,
				PackCount:      2,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "petWeightKg out of range",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetWeightKg:    0.1,
				PackCount:      2,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "packCount < 1",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      0,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "negative distanceMeters",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      1,
				DistanceMeters: -10,
			},
			wantErr: true,
		},
		{
			name: "invalid petSpecies",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetSpecies:     "RABBIT",
				PetWeightKg:    4.2,
				PackCount:      1,
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid solarPhase",
			input: CreateCorridorInput{
				ReportID:       "rep-1",
				PetID:          "pet-1",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      1,
				SolarPhase:     "TWILIGHT",
				DistanceMeters: 100,
			},
			wantErr: true,
		},
		{
			name: "valid defaults when optional strings omitted",
			input: CreateCorridorInput{
				ReportID:       "corridor-rep-default",
				PetID:          "pet-2026-002",
				PetName:        "Mochi",
				PetWeightKg:    4.2,
				PackCount:      3,
				DistanceMeters: 85,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := EvaluateWildlifeCorridor(tc.input, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rep.PetSpecies != "CAT" {
				t.Errorf("expected default PetSpecies CAT, got %q", rep.PetSpecies)
			}
			if rep.PredatorSpecies != "COYOTE_PACK" {
				t.Errorf("expected default PredatorSpecies COYOTE_PACK, got %q", rep.PredatorSpecies)
			}
			if rep.SolarPhase != "NIGHT" {
				t.Errorf("expected default SolarPhase NIGHT, got %q", rep.SolarPhase)
			}
			if rep.CorridorTerrain != "URBAN_RAVINE_GREENBELT" {
				t.Errorf("expected default CorridorTerrain URBAN_RAVINE_GREENBELT, got %q", rep.CorridorTerrain)
			}
			if !rep.SmallPreyAlert {
				t.Errorf("expected SmallPreyAlert=true for 4.2kg cat")
			}
			if rep.Phase != domain.CorridorPhaseActivePredatorWatch {
				t.Errorf("expected ACTIVE_PREDATOR_WATCH, got %q", rep.Phase)
			}
			if rep.ThreatLevel != domain.PredatorThreatExtremeNocturnal {
				t.Errorf("expected EXTREME_NOCTURNAL_PREDATION_RISK, got %q", rep.ThreatLevel)
			}
			if len(rep.MitigationSteps) != 4 {
				t.Fatalf("expected 4 mitigation steps, got %d", len(rep.MitigationSteps))
			}
		})
	}
}

func TestApplyCorridorMitigation_LifecycleAndClearance(t *testing.T) {
	now := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

	rep, err := EvaluateWildlifeCorridor(CreateCorridorInput{
		ReportID:        "corridor-rep-101",
		PetID:           "pet-2026-002",
		PetName:         "Mochi",
		PetSpecies:      "CAT",
		PetWeightKg:     4.2,
		PredatorSpecies: "COYOTE_PACK",
		PackCount:       3,
		SolarPhase:      "NIGHT",
		CorridorTerrain: "URBAN_RAVINE_GREENBELT",
		DistanceMeters:  85,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 1: DeployDeterrents -> HAZING_DETERRENT_DEPLOYED & ELEVATED_CREPUSCULAR_CORRIDOR
	t1 := now.Add(10 * time.Minute)
	if err := ApplyCorridorMitigation(rep, MitigationUpdateInput{
		DeployDeterrents: true,
	}, t1); err != nil {
		t.Fatalf("DeployDeterrents unexpected error: %v", err)
	}
	if rep.Phase != domain.CorridorPhaseHazingDeterrentDeployed {
		t.Errorf("expected phase HAZING_DETERRENT_DEPLOYED, got %q", rep.Phase)
	}
	if rep.ThreatLevel != domain.PredatorThreatElevatedCrepuscular {
		t.Errorf("expected threat ELEVATED_CREPUSCULAR_CORRIDOR, got %q (vuln=%d)", rep.ThreatLevel, rep.VulnerabilityIndex)
	}
	if rep.DeterrentReadinessScore != 100 {
		t.Errorf("expected DeterrentReadinessScore 100, got %d", rep.DeterrentReadinessScore)
	}

	// Step 2: Complete single item by ID
	t2 := now.Add(20 * time.Minute)
	if err := ApplyCorridorMitigation(rep, MitigationUpdateInput{
		CompleteItemID: "mit-scent-corridor",
	}, t2); err != nil {
		t.Fatalf("CompleteItemID unexpected error: %v", err)
	}
	if !rep.MitigationSteps[2].Completed {
		t.Errorf("expected mit-scent-corridor completed")
	}

	// Step 3: CompleteAllMitigations -> CORRIDOR_CLEARED & MITIGATED_SAFE_ZONE & CERT-CORRIDOR-...
	t3 := now.Add(30 * time.Minute)
	if err := ApplyCorridorMitigation(rep, MitigationUpdateInput{
		CompleteAllMitigations: true,
	}, t3); err != nil {
		t.Fatalf("CompleteAllMitigations unexpected error: %v", err)
	}
	if rep.Phase != domain.CorridorPhaseCorridorCleared {
		t.Errorf("expected phase CORRIDOR_CLEARED, got %q", rep.Phase)
	}
	if rep.ThreatLevel != domain.PredatorThreatMitigatedSafeZone {
		t.Errorf("expected threat MITIGATED_SAFE_ZONE, got %q", rep.ThreatLevel)
	}
	if rep.VulnerabilityIndex != 5 {
		t.Errorf("expected VulnerabilityIndex 5, got %d", rep.VulnerabilityIndex)
	}
	if rep.ClearanceCertificate != "CERT-CORRIDOR-CORRIDOR-REP-101" {
		t.Errorf("expected ClearanceCertificate CERT-CORRIDOR-CORRIDOR-REP-101, got %q", rep.ClearanceCertificate)
	}
	if err := rep.Validate(); err != nil {
		t.Errorf("expected valid cleared report, got error: %v", err)
	}
}

func TestApplyCorridorMitigation_Errors(t *testing.T) {
	now := time.Now().UTC()
	if err := ApplyCorridorMitigation(nil, MitigationUpdateInput{}, now); err == nil {
		t.Fatalf("expected error for nil report")
	}

	rep, err := EvaluateWildlifeCorridor(CreateCorridorInput{
		ReportID:       "corridor-rep-err",
		PetID:          "pet-1",
		PetName:        "Shadow",
		PetSpecies:     "DOG",
		PetWeightKg:    22.0,
		PackCount:      1,
		SolarPhase:     "DAYLIGHT",
		DistanceMeters: 500,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ApplyCorridorMitigation(rep, MitigationUpdateInput{CompleteItemID: "unknown-step"}, now); err == nil {
		t.Errorf("expected error for unknown CompleteItemID")
	}
}
