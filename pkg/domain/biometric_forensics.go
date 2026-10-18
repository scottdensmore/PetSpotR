package domain

import (
	"fmt"
	"strings"
)

type ForensicResolutionStatus string

const (
	ForensicStatusPendingBiometricScan     ForensicResolutionStatus = "PENDING_BIOMETRIC_SCAN"
	ForensicStatusDermatoglyphicCorrelated ForensicResolutionStatus = "DERMATOGLYPHIC_CORRELATED"
	ForensicStatusIdentityConfirmedSafe    ForensicResolutionStatus = "IDENTITY_CONFIRMED_SAFE_EXTRACTION"
)

func (s ForensicResolutionStatus) IsValid() bool {
	switch s {
	case ForensicStatusPendingBiometricScan,
		ForensicStatusDermatoglyphicCorrelated,
		ForensicStatusIdentityConfirmedSafe:
		return true
	default:
		return false
	}
}

type CognitiveEntrapmentRisk string

const (
	EntrapmentRiskCriticalSundowningHazard CognitiveEntrapmentRisk = "CRITICAL_SUNDOWNING_ENTRAPMENT"
	EntrapmentRiskElevatedSensoryWander    CognitiveEntrapmentRisk = "ELEVATED_SENSORY_DISORIENTATION"
	EntrapmentRiskContainedSafePeriphery   CognitiveEntrapmentRisk = "CONTAINED_SAFE_PERIPHERY"
)

func (r CognitiveEntrapmentRisk) IsValid() bool {
	switch r {
	case EntrapmentRiskCriticalSundowningHazard,
		EntrapmentRiskElevatedSensoryWander,
		EntrapmentRiskContainedSafePeriphery:
		return true
	default:
		return false
	}
}

type ForensicVerificationStep struct {
	StepID      string `json:"stepId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "NASAL_PLANUM_BIOMETRICS", "SUNDOWNING_HAZARD_SWEEP", "DNA_CODIS_CONFIRMATION"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type BiometricForensicCase struct {
	CaseID                   string                     `json:"caseId"`
	PetID                    string                     `json:"petId"`
	PetName                  string                     `json:"petName"`
	Species                  string                     `json:"species"`           // "DOG" or "CAT"
	AgeYears                 int                        `json:"ageYears"`          // 1 to 28
	SensoryCondition         string                     `json:"sensoryCondition"`  // "CCD_SUNDOWNING_DEMENTIA", "BILATERAL_CATARACT_BLIND", "SENILE_DEAFNESS", "INTACT_SENSORY"
	BiometricModality        string                     `json:"biometricModality"` // "NASAL_PLANUM_RIDGE_PRINT", "IRIS_MELANIN_TOPOLOGY", "COAT_PELAGE_EIGENPATCH", "BUCCAL_SWAB_STR_DNA"
	NearestHazardType        string                     `json:"nearestHazardType"` // "RETENTION_POND_CULVERT", "THICK_BLACKBERRY_BRAMBLE", "POOL_UNFENCED_DECK"
	EntrapmentRiskScore      int                        `json:"entrapmentRiskScore"`
	EntrapmentRisk           CognitiveEntrapmentRisk    `json:"entrapmentRisk"`
	ResolutionStatus         ForensicResolutionStatus   `json:"resolutionStatus"`
	SundowningWanderAlert    bool                       `json:"sundowningWanderAlert"`
	NasalPrintMatched        bool                       `json:"nasalPrintMatched"`
	WaterCulvertCleared      bool                       `json:"waterCulvertCleared"`
	BiometricConfidenceScore int                        `json:"biometricConfidenceScore"`
	VerificationSteps        []ForensicVerificationStep `json:"verificationSteps"`
	ForensicCertificateID    string                     `json:"forensicCertificateId,omitempty"`
	CreatedAt                string                     `json:"createdAt"`
	UpdatedAt                string                     `json:"updatedAt"`
}

func validForensicSensoryCondition(cond string) bool {
	switch cond {
	case "CCD_SUNDOWNING_DEMENTIA", "BILATERAL_CATARACT_BLIND", "SENILE_DEAFNESS", "INTACT_SENSORY":
		return true
	default:
		return false
	}
}

func validBiometricModality(mod string) bool {
	switch mod {
	case "NASAL_PLANUM_RIDGE_PRINT", "IRIS_MELANIN_TOPOLOGY", "COAT_PELAGE_EIGENPATCH", "BUCCAL_SWAB_STR_DNA":
		return true
	default:
		return false
	}
}

func validNearestHazardType(hazard string) bool {
	switch hazard {
	case "RETENTION_POND_CULVERT", "THICK_BLACKBERRY_BRAMBLE", "POOL_UNFENCED_DECK":
		return true
	default:
		return false
	}
}

func validForensicStepCategory(cat string) bool {
	switch cat {
	case "NASAL_PLANUM_BIOMETRICS", "SUNDOWNING_HAZARD_SWEEP", "DNA_CODIS_CONFIRMATION":
		return true
	default:
		return false
	}
}

// Validate checks invariants for BiometricForensicCase.
func (c BiometricForensicCase) Validate() error {
	if strings.TrimSpace(c.CaseID) == "" {
		return fmt.Errorf("caseId is required")
	}
	if strings.TrimSpace(c.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(c.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(c.Species)
	if species != "DOG" && species != "CAT" {
		return fmt.Errorf("species must be DOG or CAT, got %q", c.Species)
	}
	if c.AgeYears < 1 || c.AgeYears > 28 {
		return fmt.Errorf("ageYears must be in [1, 28], got %d", c.AgeYears)
	}
	if !validForensicSensoryCondition(strings.TrimSpace(c.SensoryCondition)) {
		return fmt.Errorf("invalid sensoryCondition: %q", c.SensoryCondition)
	}
	if !validBiometricModality(strings.TrimSpace(c.BiometricModality)) {
		return fmt.Errorf("invalid biometricModality: %q", c.BiometricModality)
	}
	if !validNearestHazardType(strings.TrimSpace(c.NearestHazardType)) {
		return fmt.Errorf("invalid nearestHazardType: %q", c.NearestHazardType)
	}
	if c.EntrapmentRiskScore < 0 || c.EntrapmentRiskScore > 100 {
		return fmt.Errorf("entrapmentRiskScore must be in [0, 100], got %d", c.EntrapmentRiskScore)
	}
	if !c.EntrapmentRisk.IsValid() {
		return fmt.Errorf("invalid entrapmentRisk: %q", c.EntrapmentRisk)
	}
	if !c.ResolutionStatus.IsValid() {
		return fmt.Errorf("invalid resolutionStatus: %q", c.ResolutionStatus)
	}
	if c.BiometricConfidenceScore < 0 || c.BiometricConfidenceScore > 100 {
		return fmt.Errorf("biometricConfidenceScore must be in [0, 100], got %d", c.BiometricConfidenceScore)
	}
	if len(c.VerificationSteps) < 1 {
		return fmt.Errorf("at least one verificationStep is required")
	}
	for i, step := range c.VerificationSteps {
		if strings.TrimSpace(step.StepID) == "" ||
			strings.TrimSpace(step.Title) == "" ||
			!validForensicStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("verificationSteps[%d] must have non-empty stepId, title, and valid category", i)
		}
	}
	if c.ResolutionStatus == ForensicStatusIdentityConfirmedSafe && strings.TrimSpace(c.ForensicCertificateID) == "" {
		return fmt.Errorf("forensicCertificateId is required when resolutionStatus is IDENTITY_CONFIRMED_SAFE_EXTRACTION")
	}
	if strings.TrimSpace(c.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(c.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
