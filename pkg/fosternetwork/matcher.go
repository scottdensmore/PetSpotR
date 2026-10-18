package fosternetwork

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateFosterPlacementInput struct {
	PlacementID        string
	PetID              string
	PetName            string
	Species            string
	FosterHostName     string
	ContagionFlag      string
	MedicalNeedLevel   string
	ResidentPetsCount  int
	HEPAIsolationReady bool
	MedCertVerified    bool
}

type FosterPlacementUpdateInput struct {
	VerifyBiosecurityRig    bool
	ActivateFosterPlacement bool
	CompleteCheckID         string
	HEPAIsolationReady      *bool
	MedCertVerified         *bool
}

func EvaluateFosterPlacement(input CreateFosterPlacementInput, now time.Time) (*domain.FosterPlacementMatch, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	placementID := strings.TrimSpace(input.PlacementID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	fosterHostName := strings.TrimSpace(input.FosterHostName)

	if placementID == "" {
		return nil, fmt.Errorf("placementId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if fosterHostName == "" {
		return nil, fmt.Errorf("fosterHostName is required")
	}
	if input.ResidentPetsCount < 0 || input.ResidentPetsCount > 50 {
		return nil, fmt.Errorf("residentPetsCount must be in [0, 50], got %d", input.ResidentPetsCount)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	contagionFlag := strings.ToUpper(strings.TrimSpace(input.ContagionFlag))
	if contagionFlag == "" {
		contagionFlag = "PARVO_SUSPECTED"
	}

	medicalNeedLevel := strings.ToUpper(strings.TrimSpace(input.MedicalNeedLevel))
	if medicalNeedLevel == "" {
		medicalNeedLevel = "INJECTABLE_SUBQ_FLUIDS"
	}

	checks := []domain.FosterBiosecurityCheckItem{
		{
			CheckID:   "bio-chk-bleach-wash",
			Title:     "Complete rescue-grade accelerated hydrogen peroxide (AHP) surface sterilization",
			Category:  "PATHOGEN_ISOLATION",
			Completed: false,
		},
		{
			CheckID:   "bio-chk-airlock-barrier",
			Title:     "Install dedicated footbath, disposable gowning station & HEPA negative-pressure seal",
			Category:  "PATHOGEN_ISOLATION",
			Completed: false,
		},
		{
			CheckID:   "bio-chk-resident-sep",
			Title:     "Verify zero shared HVAC return or yard access with resident household pets",
			Category:  "RESIDENT_PET_SEPARATION",
			Completed: false,
		},
		{
			CheckID:   "bio-chk-med-dosing",
			Title:     "Confirm veterinary technician sign-off on injectable/sub-Q dosing schedule",
			Category:  "MEDICATION_COMPETENCY",
			Completed: false,
		},
	}

	placement := &domain.FosterPlacementMatch{
		PlacementID:        placementID,
		PetID:              petID,
		PetName:            petName,
		Species:            species,
		FosterHostName:     fosterHostName,
		ContagionFlag:      contagionFlag,
		MedicalNeedLevel:   medicalNeedLevel,
		ResidentPetsCount:  input.ResidentPetsCount,
		HEPAIsolationReady: input.HEPAIsolationReady,
		MedCertVerified:    input.MedCertVerified,
		BiosecurityChecks:  checks,
		CreatedAt:          nowStr,
		UpdatedAt:          nowStr,
	}

	RecomputeFosterMetrics(placement, now)

	if err := placement.Validate(); err != nil {
		return nil, err
	}

	return placement, nil
}

func RecomputeFosterMetrics(placement *domain.FosterPlacementMatch, now time.Time) {
	if placement == nil {
		return
	}
	_ = now

	placement.IsoQuarantineAlert = placement.ContagionFlag == "PARVO_SUSPECTED" ||
		placement.ContagionFlag == "RINGWORM_DERMATOPHYTE" ||
		placement.ContagionFlag == "UPPER_RESPIRATORY_URI"

	readiness := 0
	if placement.HEPAIsolationReady {
		readiness += 50
	}
	if placement.MedCertVerified {
		readiness += 50
	}
	placement.HostReadinessScore = readiness

	totalChecks := len(placement.BiosecurityChecks)
	completedChecks := 0
	for _, chk := range placement.BiosecurityChecks {
		if chk.Completed {
			completedChecks++
		}
	}

	baseContagion := 15
	switch placement.ContagionFlag {
	case "PARVO_SUSPECTED":
		baseContagion = 60
	case "RINGWORM_DERMATOPHYTE":
		baseContagion = 50
	case "UPPER_RESPIRATORY_URI":
		baseContagion = 45
	case "NONE_CLEARED":
		baseContagion = 15
	}

	residentPenalty := placement.ResidentPetsCount * 10
	if residentPenalty > 25 {
		residentPenalty = 25
	}

	medPenalty := 0
	switch placement.MedicalNeedLevel {
	case "INJECTABLE_SUBQ_FLUIDS":
		medPenalty = 15
	case "WOUND_DRESSING_BID":
		medPenalty = 10
	}

	risk := baseContagion + residentPenalty + medPenalty - (completedChecks * 15) - (readiness / 4)
	if risk < 5 {
		risk = 5
	}
	if risk > 100 {
		risk = 100
	}
	placement.ContagionRiskScore = risk

	if totalChecks > 0 && completedChecks == totalChecks && placement.HEPAIsolationReady && placement.MedCertVerified {
		placement.PlacementStatus = domain.FosterStatusPlacedInFosterHome
		placement.BiosecurityTier = domain.BiosecurityTierClearedStandard
		placement.ContagionRiskScore = 5
		if placement.PlacementAuthorizationCode == "" {
			placement.PlacementAuthorizationCode = "FOSTER-AUTH-" + strings.ToUpper(placement.PlacementID)
		}
	} else if completedChecks >= 1 || (placement.HEPAIsolationReady && placement.MedCertVerified) {
		placement.PlacementStatus = domain.FosterStatusQuarantineVerified
		if placement.ContagionRiskScore >= 70 {
			placement.BiosecurityTier = domain.BiosecurityTierIsoQuarantine
		} else {
			placement.BiosecurityTier = domain.BiosecurityTierMedicalSinglePet
		}
	} else {
		placement.PlacementStatus = domain.FosterStatusPendingBiosecurity
		if placement.IsoQuarantineAlert || placement.ContagionRiskScore >= 65 {
			placement.BiosecurityTier = domain.BiosecurityTierIsoQuarantine
		} else {
			placement.BiosecurityTier = domain.BiosecurityTierMedicalSinglePet
		}
	}
}

func ApplyFosterPlacementTransition(placement *domain.FosterPlacementMatch, input FosterPlacementUpdateInput, now time.Time) error {
	if placement == nil {
		return fmt.Errorf("placement is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.HEPAIsolationReady != nil {
		placement.HEPAIsolationReady = *input.HEPAIsolationReady
	}
	if input.MedCertVerified != nil {
		placement.MedCertVerified = *input.MedCertVerified
	}

	if checkID := strings.TrimSpace(input.CompleteCheckID); checkID != "" {
		found := false
		for i := range placement.BiosecurityChecks {
			if placement.BiosecurityChecks[i].CheckID == checkID {
				placement.BiosecurityChecks[i].Completed = true
				if placement.BiosecurityChecks[i].CompletedAt == "" {
					placement.BiosecurityChecks[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("biosecurity check %q not found", checkID)
		}
	}

	if input.VerifyBiosecurityRig {
		for i := range placement.BiosecurityChecks {
			if i < 2 {
				placement.BiosecurityChecks[i].Completed = true
				if placement.BiosecurityChecks[i].CompletedAt == "" {
					placement.BiosecurityChecks[i].CompletedAt = nowStr
				}
			}
		}
		placement.HEPAIsolationReady = true
		placement.MedCertVerified = true
	}

	if input.ActivateFosterPlacement {
		for i := range placement.BiosecurityChecks {
			placement.BiosecurityChecks[i].Completed = true
			if placement.BiosecurityChecks[i].CompletedAt == "" {
				placement.BiosecurityChecks[i].CompletedAt = nowStr
			}
		}
		placement.HEPAIsolationReady = true
		placement.MedCertVerified = true
	}

	RecomputeFosterMetrics(placement, now)
	placement.UpdatedAt = nowStr

	return placement.Validate()
}
