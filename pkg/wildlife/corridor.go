package wildlife

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateCorridorInput struct {
	ReportID              string
	PetID                 string
	PetName               string
	PetSpecies            string
	PetWeightKg           float64
	PredatorSpecies       string
	PackCount             int
	SolarPhase            string
	CorridorTerrain       string
	DistanceMeters        float64
	StrobeDeterrentActive bool
	AttractantsCleared    bool
}

type MitigationUpdateInput struct {
	CompleteAllMitigations bool
	DeployDeterrents       bool
	CompleteItemID         string
	StrobeDeterrentActive  *bool
	AttractantsCleared     *bool
}

func EvaluateWildlifeCorridor(input CreateCorridorInput, now time.Time) (*domain.WildlifeCorridorReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	reportID := strings.TrimSpace(input.ReportID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	if reportID == "" {
		return nil, fmt.Errorf("reportId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if input.PetWeightKg < 0.5 || input.PetWeightKg > 90.0 {
		return nil, fmt.Errorf("petWeightKg must be in [0.5, 90.0], got %.2f", input.PetWeightKg)
	}
	if input.PackCount < 1 {
		return nil, fmt.Errorf("packCount must be >= 1, got %d", input.PackCount)
	}
	if input.DistanceMeters < 0 {
		return nil, fmt.Errorf("distanceMeters must be >= 0, got %.2f", input.DistanceMeters)
	}

	petSpecies := strings.ToUpper(strings.TrimSpace(input.PetSpecies))
	if petSpecies == "" {
		petSpecies = "CAT"
	}
	if petSpecies != "CAT" && petSpecies != "DOG" {
		return nil, fmt.Errorf("petSpecies must be CAT or DOG, got %q", input.PetSpecies)
	}

	predatorSpecies := strings.ToUpper(strings.TrimSpace(input.PredatorSpecies))
	if predatorSpecies == "" {
		predatorSpecies = "COYOTE_PACK"
	}

	solarPhase := strings.ToUpper(strings.TrimSpace(input.SolarPhase))
	if solarPhase == "" {
		solarPhase = "NIGHT"
	}
	if solarPhase != "NIGHT" && solarPhase != "DAWN_DUSK_CREPUSCULAR" && solarPhase != "DAYLIGHT" {
		return nil, fmt.Errorf("solarPhase must be NIGHT, DAWN_DUSK_CREPUSCULAR, or DAYLIGHT, got %q", input.SolarPhase)
	}

	corridorTerrain := strings.ToUpper(strings.TrimSpace(input.CorridorTerrain))
	if corridorTerrain == "" {
		corridorTerrain = "URBAN_RAVINE_GREENBELT"
	}

	steps := []domain.CorridorMitigationStep{
		{
			ItemID:   "mit-strobe-hazing",
			Title:    "Deploy motion-activated ultrasonic & amber strobe predator deterrents",
			Category: "ACOUSTIC_STROBE_DETERRENT",
		},
		{
			ItemID:   "mit-attractant-lock",
			Title:    "Remove outdoor pet food bowls, fallen fruit, and unsecured compost",
			Category: "ATTRACTANT_REMOVAL",
		},
		{
			ItemID:   "mit-scent-corridor",
			Title:    "Anchor owner-scent safe-haven box within illuminated garage perimeter",
			Category: "NOCTURNAL_SHELTER_CURFEW",
		},
		{
			ItemID:   "mit-crepuscular-patrol",
			Title:    "Station thermal-optic spotter along ravine choke point at dusk/dawn",
			Category: "NOCTURNAL_SHELTER_CURFEW",
		},
	}

	report := &domain.WildlifeCorridorReport{
		ReportID:              reportID,
		PetID:                 petID,
		PetName:               petName,
		PetSpecies:            petSpecies,
		PetWeightKg:           input.PetWeightKg,
		PredatorSpecies:       predatorSpecies,
		PackCount:             input.PackCount,
		SolarPhase:            solarPhase,
		CorridorTerrain:       corridorTerrain,
		DistanceMeters:        input.DistanceMeters,
		StrobeDeterrentActive: input.StrobeDeterrentActive,
		AttractantsCleared:    input.AttractantsCleared,
		MitigationSteps:       steps,
		CreatedAt:             nowStr,
		UpdatedAt:             nowStr,
	}

	RecomputeCorridorMetrics(report, now)

	if err := report.Validate(); err != nil {
		return nil, err
	}

	return report, nil
}

func RecomputeCorridorMetrics(report *domain.WildlifeCorridorReport, now time.Time) {
	if report == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	report.SmallPreyAlert = report.PetWeightKg <= 11.0 ||
		report.PetSpecies == "CAT" ||
		report.PredatorSpecies == "GREAT_HORNED_OWL"

	readiness := 0
	if report.StrobeDeterrentActive {
		readiness += 50
	}
	if report.AttractantsCleared {
		readiness += 50
	}
	report.DeterrentReadinessScore = readiness

	totalSteps := len(report.MitigationSteps)
	completedSteps := 0
	for _, step := range report.MitigationSteps {
		if step.Completed {
			completedSteps++
		}
	}

	baseVuln := 15
	switch report.SolarPhase {
	case "NIGHT":
		baseVuln = 45
	case "DAWN_DUSK_CREPUSCULAR":
		baseVuln = 35
	case "DAYLIGHT":
		baseVuln = 15
	}

	weightPenalty := 0
	if report.PetWeightKg <= 6.0 {
		weightPenalty = 25
	} else if report.PetWeightKg <= 12.0 {
		weightPenalty = 15
	}

	packPenalty := (report.PackCount - 1) * 8
	if packPenalty < 0 {
		packPenalty = 0
	}
	if packPenalty > 20 {
		packPenalty = 20
	}

	distancePenalty := 0
	if report.DistanceMeters <= 150 {
		distancePenalty = 20
	} else if report.DistanceMeters <= 400 {
		distancePenalty = 10
	}

	mitigationReduction := completedSteps*15 + (report.DeterrentReadinessScore / 4)
	vuln := baseVuln + weightPenalty + packPenalty + distancePenalty - mitigationReduction
	if vuln < 5 {
		vuln = 5
	}
	if vuln > 100 {
		vuln = 100
	}
	report.VulnerabilityIndex = vuln

	if totalSteps > 0 && completedSteps == totalSteps && report.StrobeDeterrentActive && report.AttractantsCleared {
		report.Phase = domain.CorridorPhaseCorridorCleared
		report.ThreatLevel = domain.PredatorThreatMitigatedSafeZone
		report.VulnerabilityIndex = 5
		if report.ClearanceCertificate == "" {
			report.ClearanceCertificate = "CERT-CORRIDOR-" + strings.ToUpper(report.ReportID)
		}
	} else if completedSteps >= 1 || (report.StrobeDeterrentActive && report.AttractantsCleared) {
		report.Phase = domain.CorridorPhaseHazingDeterrentDeployed
		if report.VulnerabilityIndex >= 70 {
			report.ThreatLevel = domain.PredatorThreatExtremeNocturnal
		} else {
			report.ThreatLevel = domain.PredatorThreatElevatedCrepuscular
		}
	} else {
		report.Phase = domain.CorridorPhaseActivePredatorWatch
		if report.VulnerabilityIndex >= 65 || report.SmallPreyAlert {
			report.ThreatLevel = domain.PredatorThreatExtremeNocturnal
		} else {
			report.ThreatLevel = domain.PredatorThreatElevatedCrepuscular
		}
	}

	report.UpdatedAt = now.Format(time.RFC3339)
}

func ApplyCorridorMitigation(report *domain.WildlifeCorridorReport, input MitigationUpdateInput, now time.Time) error {
	if report == nil {
		return fmt.Errorf("report is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.StrobeDeterrentActive != nil {
		report.StrobeDeterrentActive = *input.StrobeDeterrentActive
	}
	if input.AttractantsCleared != nil {
		report.AttractantsCleared = *input.AttractantsCleared
	}

	if itemID := strings.TrimSpace(input.CompleteItemID); itemID != "" {
		found := false
		for i := range report.MitigationSteps {
			if report.MitigationSteps[i].ItemID == itemID {
				report.MitigationSteps[i].Completed = true
				if report.MitigationSteps[i].CompletedAt == "" {
					report.MitigationSteps[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mitigation step %q not found", itemID)
		}
	}

	if input.DeployDeterrents {
		report.StrobeDeterrentActive = true
		report.AttractantsCleared = true
		for i := range report.MitigationSteps {
			if i < 2 {
				report.MitigationSteps[i].Completed = true
				if report.MitigationSteps[i].CompletedAt == "" {
					report.MitigationSteps[i].CompletedAt = nowStr
				}
			}
		}
	}

	if input.CompleteAllMitigations {
		report.StrobeDeterrentActive = true
		report.AttractantsCleared = true
		for i := range report.MitigationSteps {
			report.MitigationSteps[i].Completed = true
			if report.MitigationSteps[i].CompletedAt == "" {
				report.MitigationSteps[i].CompletedAt = nowStr
			}
		}
	}

	RecomputeCorridorMetrics(report, now)
	return nil
}
