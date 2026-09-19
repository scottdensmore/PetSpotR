package analytics_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/analytics"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestComputeAnalytics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	intakeTime1 := now.Add(-48 * time.Hour)
	intakeTime2 := now.Add(-24 * time.Hour)
	intakeTime3 := now.Add(-12 * time.Hour)

	foundPets := []domain.FoundPetRecord{
		{
			PetID:             "found-1",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-101",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusFound,
			FoundAt:           intakeTime1,
			MicrochipID:       "985141000123456",
			MicrochipRegistry: "HomeAgain",
		},
		{
			PetID:             "found-2",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-102",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusResolved,
			FoundAt:           intakeTime2,
			MicrochipID:       "981010000999999",
			MicrochipRegistry: "AKC Reunite",
		},
		{
			PetID:         "found-3",
			ShelterID:     "shelter-bel-02",
			ShelterName:   "Bellevue Humane Society",
			IntakeID:      "BEL-501",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       intakeTime3,
		},
	}

	matches := []domain.MatchRecord{
		{
			MatchID:            "match-1",
			FoundPetID:         "found-1",
			MatchedPetID:       "lost-1",
			Status:             domain.MatchStatusPendingReview,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          intakeTime1.Add(2 * time.Hour),
		},
		{
			MatchID:            "match-2",
			FoundPetID:         "found-2",
			MatchedPetID:       "lost-2",
			Status:             domain.MatchStatusReunited,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          intakeTime2.Add(4 * time.Hour),
		},
	}

	t.Run("Computes overall KPIs correctly across all shelters", func(t *testing.T) {
		report := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{})

		if report.OverallKPIs.TotalIntakes != 3 {
			t.Errorf("expected 3 total intakes, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.ActiveInShelterCare != 2 {
			t.Errorf("expected 2 active in shelter care, got %d", report.OverallKPIs.ActiveInShelterCare)
		}
		if report.OverallKPIs.ReunitedCount != 1 {
			t.Errorf("expected 1 reunited, got %d", report.OverallKPIs.ReunitedCount)
		}
		expectedRTORate := 1.0 / 3.0
		if report.OverallKPIs.ReturnToOwnerRate < expectedRTORate-0.01 || report.OverallKPIs.ReturnToOwnerRate > expectedRTORate+0.01 {
			t.Errorf("expected RTO rate ~0.33, got %f", report.OverallKPIs.ReturnToOwnerRate)
		}
		if report.OverallKPIs.MicrochippedCount != 2 {
			t.Errorf("expected 2 microchipped pets, got %d", report.OverallKPIs.MicrochippedCount)
		}
		expectedScanRate := 2.0 / 3.0
		if report.OverallKPIs.MicrochipScanRate < expectedScanRate-0.01 || report.OverallKPIs.MicrochipScanRate > expectedScanRate+0.01 {
			t.Errorf("expected microchip scan rate ~0.67, got %f", report.OverallKPIs.MicrochipScanRate)
		}
		if report.OverallKPIs.DeterministicMatchCount != 2 {
			t.Errorf("expected 2 deterministic matches, got %d", report.OverallKPIs.DeterministicMatchCount)
		}
		if report.OverallKPIs.MultimodalMatchCount != 0 {
			t.Errorf("expected 0 multimodal matches, got %d", report.OverallKPIs.MultimodalMatchCount)
		}
		if report.OverallKPIs.DeterministicMatchRatio != 1.0 {
			t.Errorf("expected deterministic match ratio 1.0, got %f", report.OverallKPIs.DeterministicMatchRatio)
		}
		if report.OverallKPIs.MedianIntakeToMatchHours != 3.0 {
			t.Errorf("expected median match velocity 3.0 hours (midpoint of 2.0 and 4.0), got %f", report.OverallKPIs.MedianIntakeToMatchHours)
		}
		if report.OverallKPIs.MedianIntakeToReunionHours != 4.0 {
			t.Errorf("expected median reunion velocity 4.0 hours, got %f", report.OverallKPIs.MedianIntakeToReunionHours)
		}

		if len(report.AvailableShelters) != 2 {
			t.Fatalf("expected 2 available shelters, got %d", len(report.AvailableShelters))
		}
		if len(report.ShelterBreakdown) != 2 {
			t.Fatalf("expected 2 shelters in breakdown, got %d", len(report.ShelterBreakdown))
		}
	})

	t.Run("Filters by ShelterID correctly", func(t *testing.T) {
		report := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{
			ShelterID: "shelter-bel-02",
		})

		if report.OverallKPIs.TotalIntakes != 1 {
			t.Errorf("expected 1 intake for Bellevue, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.ActiveInShelterCare != 1 {
			t.Errorf("expected 1 active care for Bellevue, got %d", report.OverallKPIs.ActiveInShelterCare)
		}
		if report.OverallKPIs.MicrochippedCount != 0 {
			t.Errorf("expected 0 microchipped pets for Bellevue, got %d", report.OverallKPIs.MicrochippedCount)
		}
		if report.OverallKPIs.DeterministicMatchCount != 0 {
			t.Errorf("expected 0 deterministic matches for Bellevue, got %d", report.OverallKPIs.DeterministicMatchCount)
		}
		if report.OverallKPIs.MedianIntakeToMatchHours != 0.0 {
			t.Errorf("expected 0.0 median match velocity for Bellevue, got %f", report.OverallKPIs.MedianIntakeToMatchHours)
		}
		if len(report.ShelterBreakdown) != 1 {
			t.Fatalf("expected 1 shelter in breakdown, got %d", len(report.ShelterBreakdown))
		}
		if report.ShelterBreakdown[0].ShelterID != "shelter-bel-02" {
			t.Errorf("expected breakdown shelter ID 'shelter-bel-02', got %s", report.ShelterBreakdown[0].ShelterID)
		}
		// Available shelters should still contain all shelters from dataset
		if len(report.AvailableShelters) != 2 {
			t.Errorf("expected 2 available shelters regardless of filter, got %d", len(report.AvailableShelters))
		}
	})

	t.Run("Filters by date range correctly", func(t *testing.T) {
		// StartDate filter: only intakes on or after now - 30h
		reportAfter := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{
			StartDate: now.Add(-30 * time.Hour),
		})
		if reportAfter.OverallKPIs.TotalIntakes != 2 {
			t.Errorf("expected 2 intakes after -30h, got %d", reportAfter.OverallKPIs.TotalIntakes)
		}

		// EndDate filter: only intakes on or before now - 30h
		reportBefore := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{
			EndDate: now.Add(-30 * time.Hour),
		})
		if reportBefore.OverallKPIs.TotalIntakes != 1 {
			t.Errorf("expected 1 intake before -30h, got %d", reportBefore.OverallKPIs.TotalIntakes)
		}
	})

	t.Run("Handles zero/empty inputs safely without panic or zero division", func(t *testing.T) {
		report := analytics.ComputeAnalytics(nil, nil, analytics.FilterOptions{})

		if report.OverallKPIs.TotalIntakes != 0 {
			t.Errorf("expected 0 intakes, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.ReturnToOwnerRate != 0.0 {
			t.Errorf("expected 0.0 RTO rate, got %f", report.OverallKPIs.ReturnToOwnerRate)
		}
		if report.OverallKPIs.MicrochipScanRate != 0.0 {
			t.Errorf("expected 0.0 microchip scan rate, got %f", report.OverallKPIs.MicrochipScanRate)
		}
		if report.OverallKPIs.DeterministicMatchRatio != 0.0 {
			t.Errorf("expected 0.0 deterministic ratio, got %f", report.OverallKPIs.DeterministicMatchRatio)
		}
		if report.OverallKPIs.MedianIntakeToMatchHours != 0.0 {
			t.Errorf("expected 0.0 median match velocity, got %f", report.OverallKPIs.MedianIntakeToMatchHours)
		}
		if report.OverallKPIs.MedianIntakeToReunionHours != 0.0 {
			t.Errorf("expected 0.0 median reunion velocity, got %f", report.OverallKPIs.MedianIntakeToReunionHours)
		}
		if report.ShelterBreakdown == nil {
			t.Error("expected non-nil slice for ShelterBreakdown")
		}
		if report.AvailableShelters == nil {
			t.Error("expected non-nil slice for AvailableShelters")
		}
	})

	t.Run("Excludes non-shelter intakes", func(t *testing.T) {
		finderPet := domain.FoundPetRecord{
			PetID:         "found-finder-1",
			CustodyStatus: domain.CustodyFinderHome,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-5 * time.Hour),
		}
		report := analytics.ComputeAnalytics([]domain.FoundPetRecord{finderPet}, nil, analytics.FilterOptions{})
		if report.OverallKPIs.TotalIntakes != 0 {
			t.Errorf("expected finder intake to be excluded from shelter analytics, got %d", report.OverallKPIs.TotalIntakes)
		}
	})

	t.Run("Computes odd count median correctly", func(t *testing.T) {
		oddMatches := []domain.MatchRecord{
			{
				MatchID:    "m1",
				FoundPetID: "found-1",
				MatchedAt:  intakeTime1.Add(1 * time.Hour),
			},
			{
				MatchID:    "m2",
				FoundPetID: "found-1",
				MatchedAt:  intakeTime1.Add(5 * time.Hour),
			},
			{
				MatchID:    "m3",
				FoundPetID: "found-1",
				MatchedAt:  intakeTime1.Add(10 * time.Hour),
			},
		}
		report := analytics.ComputeAnalytics([]domain.FoundPetRecord{foundPets[0]}, oddMatches, analytics.FilterOptions{})
		if report.OverallKPIs.MedianIntakeToMatchHours != 5.0 {
			t.Errorf("expected median 5.0 for [1, 5, 10], got %f", report.OverallKPIs.MedianIntakeToMatchHours)
		}
	})

	t.Run("Computes detailed shelter breakdown metrics", func(t *testing.T) {
		report := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{})
		var seattleStats, bellevueStats *analytics.MunicipalShelterStats
		for i := range report.ShelterBreakdown {
			if report.ShelterBreakdown[i].ShelterID == "shelter-sea-01" {
				seattleStats = &report.ShelterBreakdown[i]
			}
			if report.ShelterBreakdown[i].ShelterID == "shelter-bel-02" {
				bellevueStats = &report.ShelterBreakdown[i]
			}
		}

		if seattleStats == nil {
			t.Fatal("expected Seattle stats in breakdown")
		}
		if seattleStats.IntakeCount != 2 {
			t.Errorf("expected Seattle intake count 2, got %d", seattleStats.IntakeCount)
		}
		if seattleStats.ActiveCareCount != 1 {
			t.Errorf("expected Seattle active care count 1, got %d", seattleStats.ActiveCareCount)
		}
		if seattleStats.ReunitedCount != 1 {
			t.Errorf("expected Seattle reunited count 1, got %d", seattleStats.ReunitedCount)
		}
		if seattleStats.ReturnToOwnerRate != 0.5 {
			t.Errorf("expected Seattle RTO rate 0.5, got %f", seattleStats.ReturnToOwnerRate)
		}
		if seattleStats.MicrochipScanRate != 1.0 {
			t.Errorf("expected Seattle scan rate 1.0, got %f", seattleStats.MicrochipScanRate)
		}

		if bellevueStats == nil {
			t.Fatal("expected Bellevue stats in breakdown")
		}
		if bellevueStats.IntakeCount != 1 {
			t.Errorf("expected Bellevue intake count 1, got %d", bellevueStats.IntakeCount)
		}
		if bellevueStats.ActiveCareCount != 1 {
			t.Errorf("expected Bellevue active care count 1, got %d", bellevueStats.ActiveCareCount)
		}
		if bellevueStats.ReunitedCount != 0 {
			t.Errorf("expected Bellevue reunited count 0, got %d", bellevueStats.ReunitedCount)
		}
		if bellevueStats.ReturnToOwnerRate != 0.0 {
			t.Errorf("expected Bellevue RTO rate 0.0, got %f", bellevueStats.ReturnToOwnerRate)
		}
		if bellevueStats.MicrochipScanRate != 0.0 {
			t.Errorf("expected Bellevue scan rate 0.0, got %f", bellevueStats.MicrochipScanRate)
		}
	})

	t.Run("Computes multimodal and deterministic match ratio with mixed matches", func(t *testing.T) {
		mixedMatches := []domain.MatchRecord{
			{
				MatchID:            "m1",
				FoundPetID:         "found-1",
				DeterministicMatch: true,
				MatchedAt:          intakeTime1.Add(1 * time.Hour),
			},
			{
				MatchID:            "m2",
				FoundPetID:         "found-1",
				DeterministicMatch: false,
				MatchedAt:          intakeTime1.Add(2 * time.Hour),
			},
			{
				MatchID:            "m3",
				FoundPetID:         "found-1",
				DeterministicMatch: false,
				MatchedAt:          intakeTime1.Add(3 * time.Hour),
			},
		}
		report := analytics.ComputeAnalytics([]domain.FoundPetRecord{foundPets[0]}, mixedMatches, analytics.FilterOptions{})
		if report.OverallKPIs.DeterministicMatchCount != 1 {
			t.Errorf("expected 1 deterministic match, got %d", report.OverallKPIs.DeterministicMatchCount)
		}
		if report.OverallKPIs.MultimodalMatchCount != 2 {
			t.Errorf("expected 2 multimodal matches, got %d", report.OverallKPIs.MultimodalMatchCount)
		}
		expectedRatio := 1.0 / 3.0
		if report.OverallKPIs.DeterministicMatchRatio < expectedRatio-0.01 || report.OverallKPIs.DeterministicMatchRatio > expectedRatio+0.01 {
			t.Errorf("expected deterministic ratio ~0.33, got %f", report.OverallKPIs.DeterministicMatchRatio)
		}
	})

	t.Run("Reunion via confirmed match status and lifecycle audit timestamp", func(t *testing.T) {
		petWithAudit := domain.FoundPetRecord{
			PetID:         "found-audit",
			ShelterID:     "shelter-sea-01",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusResolved,
			FoundAt:       intakeTime1,
			LifecycleAudit: &domain.FoundPetLifecycleAudit{
				ChangedAt: intakeTime1.Add(6 * time.Hour),
			},
		}
		petWithConfirmedMatch := domain.FoundPetRecord{
			PetID:         "found-confirmed",
			ShelterID:     "shelter-sea-01",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       intakeTime1,
		}
		confirmedMatch := domain.MatchRecord{
			MatchID:    "m-conf",
			FoundPetID: "found-confirmed",
			Status:     domain.MatchStatusConfirmed,
			MatchedAt:  intakeTime1.Add(8 * time.Hour),
		}

		report := analytics.ComputeAnalytics(
			[]domain.FoundPetRecord{petWithAudit, petWithConfirmedMatch},
			[]domain.MatchRecord{confirmedMatch},
			analytics.FilterOptions{},
		)
		if report.OverallKPIs.TotalIntakes != 2 {
			t.Errorf("expected 2 intakes, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.ReunitedCount != 2 {
			t.Errorf("expected 2 reunited, got %d", report.OverallKPIs.ReunitedCount)
		}
		if report.OverallKPIs.ActiveInShelterCare != 0 {
			t.Errorf("expected 0 active care, got %d", report.OverallKPIs.ActiveInShelterCare)
		}
		// 6.0 and 8.0 hours -> median is (6.0 + 8.0) / 2 = 7.0
		if report.OverallKPIs.MedianIntakeToReunionHours != 7.0 {
			t.Errorf("expected median reunion velocity 7.0 hours, got %f", report.OverallKPIs.MedianIntakeToReunionHours)
		}
	})
}
