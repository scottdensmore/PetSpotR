package domain

import (
	"fmt"
	"strings"
)

type FosterPlacementStatus string

const (
	FosterStatusPendingBiosecurity FosterPlacementStatus = "PENDING_BIOSECURITY_CLEARANCE"
	FosterStatusQuarantineVerified FosterPlacementStatus = "QUARANTINE_VERIFIED_STAGED"
	FosterStatusPlacedInFosterHome FosterPlacementStatus = "ACTIVE_IN_FOSTER_HOME"
)

func (s FosterPlacementStatus) IsValid() bool {
	switch s {
	case FosterStatusPendingBiosecurity,
		FosterStatusQuarantineVerified,
		FosterStatusPlacedInFosterHome:
		return true
	default:
		return false
	}
}

type FosterBiosecurityTier string

const (
	BiosecurityTierIsoQuarantine    FosterBiosecurityTier = "ISO_QUARANTINE_CONTAGION"
	BiosecurityTierMedicalSinglePet FosterBiosecurityTier = "MEDICAL_RECOVERY_SINGLE_PET"
	BiosecurityTierClearedStandard  FosterBiosecurityTier = "BIOSECURITY_CLEARED_STANDARD"
)

func (t FosterBiosecurityTier) IsValid() bool {
	switch t {
	case BiosecurityTierIsoQuarantine,
		BiosecurityTierMedicalSinglePet,
		BiosecurityTierClearedStandard:
		return true
	default:
		return false
	}
}

type FosterBiosecurityCheckItem struct {
	CheckID     string `json:"checkId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "PATHOGEN_ISOLATION", "RESIDENT_PET_SEPARATION", "MEDICATION_COMPETENCY"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type FosterPlacementMatch struct {
	PlacementID                string                       `json:"placementId"`
	PetID                      string                       `json:"petId"`
	PetName                    string                       `json:"petName"`
	Species                    string                       `json:"species"` // "DOG" or "CAT"
	FosterHostName             string                       `json:"fosterHostName"`
	ContagionFlag              string                       `json:"contagionFlag"`      // "PARVO_SUSPECTED", "RINGWORM_DERMATOPHYTE", "UPPER_RESPIRATORY_URI", "NONE_CLEARED"
	MedicalNeedLevel           string                       `json:"medicalNeedLevel"`   // "INJECTABLE_SUBQ_FLUIDS", "WOUND_DRESSING_BID", "STANDARD_DECOMPRESSION"
	ResidentPetsCount          int                          `json:"residentPetsCount"`  // >= 0
	ContagionRiskScore         int                          `json:"contagionRiskScore"` // 0 to 100
	BiosecurityTier            FosterBiosecurityTier        `json:"biosecurityTier"`
	PlacementStatus            FosterPlacementStatus        `json:"placementStatus"`
	IsoQuarantineAlert         bool                         `json:"isoQuarantineAlert"`
	HEPAIsolationReady         bool                         `json:"hepaIsolationReady"`
	MedCertVerified            bool                         `json:"medCertVerified"`
	HostReadinessScore         int                          `json:"hostReadinessScore"` // 0 to 100
	BiosecurityChecks          []FosterBiosecurityCheckItem `json:"biosecurityChecks"`
	PlacementAuthorizationCode string                       `json:"placementAuthorizationCode,omitempty"`
	CreatedAt                  string                       `json:"createdAt"`
	UpdatedAt                  string                       `json:"updatedAt"`
}

func validFosterContagionFlag(flag string) bool {
	switch flag {
	case "PARVO_SUSPECTED", "RINGWORM_DERMATOPHYTE", "UPPER_RESPIRATORY_URI", "NONE_CLEARED":
		return true
	default:
		return false
	}
}

func validFosterMedicalNeedLevel(level string) bool {
	switch level {
	case "INJECTABLE_SUBQ_FLUIDS", "WOUND_DRESSING_BID", "STANDARD_DECOMPRESSION":
		return true
	default:
		return false
	}
}

func validFosterCheckCategory(cat string) bool {
	switch cat {
	case "PATHOGEN_ISOLATION", "RESIDENT_PET_SEPARATION", "MEDICATION_COMPETENCY":
		return true
	default:
		return false
	}
}

// Validate checks invariants for FosterPlacementMatch.
func (m FosterPlacementMatch) Validate() error {
	if strings.TrimSpace(m.PlacementID) == "" {
		return fmt.Errorf("placementId is required")
	}
	if strings.TrimSpace(m.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(m.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(m.Species)
	if species != "DOG" && species != "CAT" {
		return fmt.Errorf("species must be DOG or CAT, got %q", m.Species)
	}
	if strings.TrimSpace(m.FosterHostName) == "" {
		return fmt.Errorf("fosterHostName is required")
	}
	if !validFosterContagionFlag(strings.TrimSpace(m.ContagionFlag)) {
		return fmt.Errorf("invalid contagionFlag: %q", m.ContagionFlag)
	}
	if !validFosterMedicalNeedLevel(strings.TrimSpace(m.MedicalNeedLevel)) {
		return fmt.Errorf("invalid medicalNeedLevel: %q", m.MedicalNeedLevel)
	}
	if m.ResidentPetsCount < 0 {
		return fmt.Errorf("residentPetsCount must be >= 0, got %d", m.ResidentPetsCount)
	}
	if m.ContagionRiskScore < 0 || m.ContagionRiskScore > 100 {
		return fmt.Errorf("contagionRiskScore must be in [0, 100], got %d", m.ContagionRiskScore)
	}
	if !m.BiosecurityTier.IsValid() {
		return fmt.Errorf("invalid biosecurityTier: %q", m.BiosecurityTier)
	}
	if !m.PlacementStatus.IsValid() {
		return fmt.Errorf("invalid placementStatus: %q", m.PlacementStatus)
	}
	if m.HostReadinessScore < 0 || m.HostReadinessScore > 100 {
		return fmt.Errorf("hostReadinessScore must be in [0, 100], got %d", m.HostReadinessScore)
	}
	if len(m.BiosecurityChecks) < 1 {
		return fmt.Errorf("at least one biosecurityCheck is required")
	}
	for i, chk := range m.BiosecurityChecks {
		if strings.TrimSpace(chk.CheckID) == "" ||
			strings.TrimSpace(chk.Title) == "" ||
			!validFosterCheckCategory(strings.TrimSpace(chk.Category)) {
			return fmt.Errorf("biosecurityChecks[%d] must have non-empty checkId, title, and valid category", i)
		}
	}
	if m.PlacementStatus == FosterStatusPlacedInFosterHome && strings.TrimSpace(m.PlacementAuthorizationCode) == "" {
		return fmt.Errorf("placementAuthorizationCode is required when placementStatus is ACTIVE_IN_FOSTER_HOME")
	}
	if strings.TrimSpace(m.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(m.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
