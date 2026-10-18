package domain

import (
	"testing"
)

func validWildlifeCorridorReport() WildlifeCorridorReport {
	return WildlifeCorridorReport{
		ReportID:                "corridor-rep-101",
		PetID:                   "pet-2026-002",
		PetName:                 "Mochi",
		PetSpecies:              "CAT",
		PetWeightKg:             4.2,
		PredatorSpecies:         "COYOTE_PACK",
		PackCount:               3,
		SolarPhase:              "NIGHT",
		CorridorTerrain:         "URBAN_RAVINE_GREENBELT",
		DistanceMeters:          85,
		VulnerabilityIndex:      95,
		ThreatLevel:             PredatorThreatExtremeNocturnal,
		Phase:                   CorridorPhaseActivePredatorWatch,
		SmallPreyAlert:          true,
		DeterrentReadinessScore: 0,
		StrobeDeterrentActive:   false,
		AttractantsCleared:      false,
		MitigationSteps: []CorridorMitigationStep{
			{
				ItemID:    "mit-strobe-hazing",
				Title:     "Deploy motion-activated ultrasonic & amber strobe predator deterrents",
				Category:  "ACOUSTIC_STROBE_DETERRENT",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T18:00:00Z",
		UpdatedAt: "2026-10-18T18:00:00Z",
	}
}

func TestWildlifeCorridorValidate(t *testing.T) {
	t.Run("valid active predator watch report", func(t *testing.T) {
		r := validWildlifeCorridorReport()
		if err := r.Validate(); err != nil {
			t.Fatalf("expected valid report, got error: %v", err)
		}
	})

	t.Run("valid cleared corridor report with certificate", func(t *testing.T) {
		r := validWildlifeCorridorReport()
		r.Phase = CorridorPhaseCorridorCleared
		r.ThreatLevel = PredatorThreatMitigatedSafeZone
		r.VulnerabilityIndex = 5
		r.DeterrentReadinessScore = 100
		r.StrobeDeterrentActive = true
		r.AttractantsCleared = true
		r.MitigationSteps[0].Completed = true
		r.MitigationSteps[0].CompletedAt = "2026-10-18T18:30:00Z"
		r.ClearanceCertificate = "CERT-CORRIDOR-CORRIDOR-REP-101"
		if err := r.Validate(); err != nil {
			t.Fatalf("expected valid cleared report, got error: %v", err)
		}
	})

	t.Run("rejects invalid fields", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*WildlifeCorridorReport)
		}{
			{"empty reportId", func(r *WildlifeCorridorReport) { r.ReportID = " " }},
			{"empty petId", func(r *WildlifeCorridorReport) { r.PetID = "" }},
			{"empty petName", func(r *WildlifeCorridorReport) { r.PetName = " " }},
			{"invalid petSpecies", func(r *WildlifeCorridorReport) { r.PetSpecies = "BIRD" }},
			{"petWeightKg too low", func(r *WildlifeCorridorReport) { r.PetWeightKg = 0.2 }},
			{"petWeightKg too high", func(r *WildlifeCorridorReport) { r.PetWeightKg = 95.0 }},
			{"empty predatorSpecies", func(r *WildlifeCorridorReport) { r.PredatorSpecies = "" }},
			{"packCount < 1", func(r *WildlifeCorridorReport) { r.PackCount = 0 }},
			{"invalid solarPhase", func(r *WildlifeCorridorReport) { r.SolarPhase = "MIDDAY" }},
			{"empty corridorTerrain", func(r *WildlifeCorridorReport) { r.CorridorTerrain = " " }},
			{"negative distanceMeters", func(r *WildlifeCorridorReport) { r.DistanceMeters = -5.0 }},
			{"negative vulnerabilityIndex", func(r *WildlifeCorridorReport) { r.VulnerabilityIndex = -1 }},
			{"excessive vulnerabilityIndex", func(r *WildlifeCorridorReport) { r.VulnerabilityIndex = 101 }},
			{"invalid threatLevel", func(r *WildlifeCorridorReport) { r.ThreatLevel = PredatorThreatLevel("UNKNOWN") }},
			{"invalid phase", func(r *WildlifeCorridorReport) { r.Phase = CorridorPhase("UNKNOWN") }},
			{"negative deterrentReadinessScore", func(r *WildlifeCorridorReport) { r.DeterrentReadinessScore = -1 }},
			{"excessive deterrentReadinessScore", func(r *WildlifeCorridorReport) { r.DeterrentReadinessScore = 105 }},
			{"empty mitigationSteps", func(r *WildlifeCorridorReport) { r.MitigationSteps = nil }},
			{"mitigationStep missing fields", func(r *WildlifeCorridorReport) { r.MitigationSteps[0].Title = "" }},
			{"cleared phase without certificate", func(r *WildlifeCorridorReport) {
				r.Phase = CorridorPhaseCorridorCleared
				r.ClearanceCertificate = ""
			}},
			{"empty createdAt", func(r *WildlifeCorridorReport) { r.CreatedAt = "" }},
			{"empty updatedAt", func(r *WildlifeCorridorReport) { r.UpdatedAt = " " }},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := validWildlifeCorridorReport()
				tc.mutate(&r)
				if err := r.Validate(); err == nil {
					t.Fatalf("expected validation error for %s, got nil", tc.name)
				}
			})
		}
	})
}
