package biometrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateForensicCaseInput struct {
	CaseID              string
	PetID               string
	PetName             string
	Species             string
	AgeYears            int
	SensoryCondition    string
	BiometricModality   string
	NearestHazardType   string
	NasalPrintMatched   bool
	WaterCulvertCleared bool
}

type ForensicCaseUpdateInput struct {
	CorrelateBiometricAndSweepHazard bool
	ConfirmForensicExtraction        bool
	CompleteStepID                   string
	NasalPrintMatched                *bool
	WaterCulvertCleared              *bool
}

func EvaluateBiometricForensicCase(input CreateForensicCaseInput, now time.Time) (*domain.BiometricForensicCase, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	caseID := strings.TrimSpace(input.CaseID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)

	if caseID == "" {
		return nil, fmt.Errorf("caseId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if input.AgeYears < 1 || input.AgeYears > 28 {
		return nil, fmt.Errorf("ageYears must be in [1, 28], got %d", input.AgeYears)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	sensoryCondition := strings.ToUpper(strings.TrimSpace(input.SensoryCondition))
	if sensoryCondition == "" {
		sensoryCondition = "CCD_SUNDOWNING_DEMENTIA"
	}

	biometricModality := strings.ToUpper(strings.TrimSpace(input.BiometricModality))
	if biometricModality == "" {
		biometricModality = "NASAL_PLANUM_RIDGE_PRINT"
	}

	nearestHazardType := strings.ToUpper(strings.TrimSpace(input.NearestHazardType))
	if nearestHazardType == "" {
		nearestHazardType = "RETENTION_POND_CULVERT"
	}

	steps := []domain.ForensicVerificationStep{
		{
			StepID:    "for-step-nasal-ridge",
			Title:     "Execute Gabor-wavelet ridge matching on nasal planum dermatoglyphic macro photo",
			Category:  "NASAL_PLANUM_BIOMETRICS",
			Completed: false,
		},
		{
			StepID:    "for-step-culvert-sweep",
			Title:     "Sweep 150m dead-end corners, drainage culverts, retention ponds & pool perimeters",
			Category:  "SUNDOWNING_HAZARD_SWEEP",
			Completed: false,
		},
		{
			StepID:    "for-step-olfactory-beacon",
			Title:     "Deploy heated owner-bedding olfactory beacon & low-frequency tactile vibration lure",
			Category:  "SUNDOWNING_HAZARD_SWEEP",
			Completed: false,
		},
		{
			StepID:    "for-step-str-dna",
			Title:     "Verify 18-loci ISAG canine/feline STR DNA or coat eigenpatch biometric parity",
			Category:  "DNA_CODIS_CONFIRMATION",
			Completed: false,
		},
	}

	forensicCase := &domain.BiometricForensicCase{
		CaseID:              caseID,
		PetID:               petID,
		PetName:             petName,
		Species:             species,
		AgeYears:            input.AgeYears,
		SensoryCondition:    sensoryCondition,
		BiometricModality:   biometricModality,
		NearestHazardType:   nearestHazardType,
		NasalPrintMatched:   input.NasalPrintMatched,
		WaterCulvertCleared: input.WaterCulvertCleared,
		VerificationSteps:   steps,
		CreatedAt:           nowStr,
		UpdatedAt:           nowStr,
	}

	RecomputeForensicMetrics(forensicCase, now)

	if err := forensicCase.Validate(); err != nil {
		return nil, err
	}

	return forensicCase, nil
}

func RecomputeForensicMetrics(fc *domain.BiometricForensicCase, now time.Time) {
	if fc == nil {
		return
	}
	_ = now

	fc.SundowningWanderAlert = fc.AgeYears >= 11 ||
		fc.SensoryCondition == "CCD_SUNDOWNING_DEMENTIA" ||
		fc.SensoryCondition == "BILATERAL_CATARACT_BLIND"

	confidence := 0
	if fc.NasalPrintMatched {
		confidence += 50
	}
	if fc.WaterCulvertCleared {
		confidence += 50
	}
	fc.BiometricConfidenceScore = confidence

	totalSteps := len(fc.VerificationSteps)
	completedSteps := 0
	for _, step := range fc.VerificationSteps {
		if step.Completed {
			completedSteps++
		}
	}

	baseRisk := 20
	switch fc.SensoryCondition {
	case "CCD_SUNDOWNING_DEMENTIA":
		baseRisk = 55
	case "BILATERAL_CATARACT_BLIND":
		baseRisk = 50
	case "SENILE_DEAFNESS":
		baseRisk = 40
	case "INTACT_SENSORY":
		baseRisk = 20
	}

	agePenalty := 0
	if fc.AgeYears >= 12 {
		agePenalty = 20
	} else if fc.AgeYears >= 9 {
		agePenalty = 10
	}

	hazardPenalty := 0
	switch fc.NearestHazardType {
	case "RETENTION_POND_CULVERT", "POOL_UNFENCED_DECK":
		hazardPenalty = 20
	case "THICK_BLACKBERRY_BRAMBLE":
		hazardPenalty = 15
	}

	risk := baseRisk + agePenalty + hazardPenalty - (completedSteps * 15) - (confidence / 4)
	if risk < 5 {
		risk = 5
	}
	if risk > 100 {
		risk = 100
	}
	fc.EntrapmentRiskScore = risk

	if totalSteps > 0 && completedSteps == totalSteps && fc.NasalPrintMatched && fc.WaterCulvertCleared {
		fc.ResolutionStatus = domain.ForensicStatusIdentityConfirmedSafe
		fc.EntrapmentRisk = domain.EntrapmentRiskContainedSafePeriphery
		fc.EntrapmentRiskScore = 5
		if fc.ForensicCertificateID == "" {
			fc.ForensicCertificateID = "CERT-FORENSIC-" + strings.ToUpper(fc.CaseID)
		}
	} else if completedSteps >= 1 || (fc.NasalPrintMatched && fc.WaterCulvertCleared) {
		fc.ResolutionStatus = domain.ForensicStatusDermatoglyphicCorrelated
		if fc.EntrapmentRiskScore >= 70 {
			fc.EntrapmentRisk = domain.EntrapmentRiskCriticalSundowningHazard
		} else {
			fc.EntrapmentRisk = domain.EntrapmentRiskElevatedSensoryWander
		}
	} else {
		fc.ResolutionStatus = domain.ForensicStatusPendingBiometricScan
		if fc.SundowningWanderAlert || fc.EntrapmentRiskScore >= 65 {
			fc.EntrapmentRisk = domain.EntrapmentRiskCriticalSundowningHazard
		} else {
			fc.EntrapmentRisk = domain.EntrapmentRiskElevatedSensoryWander
		}
	}
}

func ApplyForensicCaseUpdate(fc *domain.BiometricForensicCase, input ForensicCaseUpdateInput, now time.Time) error {
	if fc == nil {
		return fmt.Errorf("forensic case is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.NasalPrintMatched != nil {
		fc.NasalPrintMatched = *input.NasalPrintMatched
	}
	if input.WaterCulvertCleared != nil {
		fc.WaterCulvertCleared = *input.WaterCulvertCleared
	}

	if stepID := strings.TrimSpace(input.CompleteStepID); stepID != "" {
		found := false
		for i := range fc.VerificationSteps {
			if fc.VerificationSteps[i].StepID == stepID {
				fc.VerificationSteps[i].Completed = true
				if fc.VerificationSteps[i].CompletedAt == "" {
					fc.VerificationSteps[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("verification step %q not found", stepID)
		}
	}

	if input.CorrelateBiometricAndSweepHazard {
		for i := range fc.VerificationSteps {
			if i < 2 {
				fc.VerificationSteps[i].Completed = true
				if fc.VerificationSteps[i].CompletedAt == "" {
					fc.VerificationSteps[i].CompletedAt = nowStr
				}
			}
		}
		fc.NasalPrintMatched = true
		fc.WaterCulvertCleared = true
	}

	if input.ConfirmForensicExtraction {
		for i := range fc.VerificationSteps {
			fc.VerificationSteps[i].Completed = true
			if fc.VerificationSteps[i].CompletedAt == "" {
				fc.VerificationSteps[i].CompletedAt = nowStr
			}
		}
		fc.NasalPrintMatched = true
		fc.WaterCulvertCleared = true
	}

	RecomputeForensicMetrics(fc, now)
	fc.UpdatedAt = nowStr

	return fc.Validate()
}
