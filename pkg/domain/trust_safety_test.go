package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestTrustSafetyDomainValidationAndSerialization(t *testing.T) {
	if store.CollectionSightingTrustScreening != "sighting_trust_screenings" {
		t.Fatalf("expected CollectionSightingTrustScreening to be sighting_trust_screenings, got %q", store.CollectionSightingTrustScreening)
	}

	validVerdicts := []domain.TrustScreeningVerdict{
		domain.TrustVerdictAuthentic,
		domain.TrustVerdictHoldModeration,
		domain.TrustVerdictQuarantinedScam,
		domain.TrustVerdictRejectedHoax,
	}
	for _, verdict := range validVerdicts {
		if !verdict.IsValid() {
			t.Errorf("expected verdict %q to be valid", verdict)
		}
	}
	if domain.TrustScreeningVerdict("UNKNOWN_VERDICT").IsValid() {
		t.Error("expected UNKNOWN_VERDICT to be invalid")
	}

	validFlags := []domain.ThreatAnomalyFlag{
		domain.FlagExtortionPaymentDemand,
		domain.FlagPinPhishingAttempt,
		domain.FlagTeleportationVelocity,
		domain.FlagSyntheticImageArtifact,
		domain.FlagUntrustedNewReporter,
	}
	for _, flag := range validFlags {
		if !flag.IsValid() {
			t.Errorf("expected threat anomaly flag %q to be valid", flag)
		}
	}
	if domain.ThreatAnomalyFlag("UNKNOWN_FLAG").IsValid() {
		t.Error("expected UNKNOWN_FLAG to be invalid")
	}

	now := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
	validScreening := domain.SightingTrustScreening{
		ScreeningID:          "screen-101",
		PetID:                "pet-901",
		PetName:              "Cooper",
		ReporterHandle:       "@scam_hunter_99",
		ReporterTrustScore:   15,
		SightingLat:          45.5152,
		SightingLng:          -122.6784,
		PrevRefLat:           47.6062,
		PrevRefLng:           -122.3321,
		ElapsedMinutes:       10,
		DistanceFromPrevKm:   233.4,
		ImpliedVelocityKmh:   1400.4,
		SubmittedNotes:       "Send $300 via Zelle or crypto and give me your 6-digit Google Voice verification code to get your dog.",
		ImageIntegrityTag:    "AI_GENERATED_SUSPECT",
		RiskScore:            95,
		TrustConfidenceScore: 5,
		Verdict:              domain.TrustVerdictQuarantinedScam,
		AnomalyFlags: []domain.ThreatAnomalyFlag{
			domain.FlagExtortionPaymentDemand,
			domain.FlagPinPhishingAttempt,
			domain.FlagTeleportationVelocity,
			domain.FlagSyntheticImageArtifact,
			domain.FlagUntrustedNewReporter,
		},
		ExplanationSummary: "Multiple high-severity scam indicators detected",
		ModeratorNotes:     "Auto-quarantined by multi-modal shield",
		PromotedSightingID: "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := validScreening.Validate(); err != nil {
		t.Fatalf("expected valid screening to pass validation, got: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*domain.SightingTrustScreening)
		wantErr string
	}{
		{
			name:    "missing screeningId",
			mutate:  func(s *domain.SightingTrustScreening) { s.ScreeningID = "  " },
			wantErr: "screeningId is required",
		},
		{
			name:    "missing petId",
			mutate:  func(s *domain.SightingTrustScreening) { s.PetID = "" },
			wantErr: "petId is required",
		},
		{
			name:    "missing reporterHandle",
			mutate:  func(s *domain.SightingTrustScreening) { s.ReporterHandle = " " },
			wantErr: "reporterHandle is required",
		},
		{
			name:    "reporterTrustScore negative",
			mutate:  func(s *domain.SightingTrustScreening) { s.ReporterTrustScore = -1 },
			wantErr: "reporterTrustScore must be between 0 and 100",
		},
		{
			name:    "reporterTrustScore above 100",
			mutate:  func(s *domain.SightingTrustScreening) { s.ReporterTrustScore = 101 },
			wantErr: "reporterTrustScore must be between 0 and 100",
		},
		{
			name:    "sightingLat out of bounds low",
			mutate:  func(s *domain.SightingTrustScreening) { s.SightingLat = -90.1 },
			wantErr: "sightingLat must be between -90 and 90",
		},
		{
			name:    "sightingLat out of bounds high",
			mutate:  func(s *domain.SightingTrustScreening) { s.SightingLat = 90.1 },
			wantErr: "sightingLat must be between -90 and 90",
		},
		{
			name:    "sightingLng out of bounds low",
			mutate:  func(s *domain.SightingTrustScreening) { s.SightingLng = -180.1 },
			wantErr: "sightingLng must be between -180 and 180",
		},
		{
			name:    "sightingLng out of bounds high",
			mutate:  func(s *domain.SightingTrustScreening) { s.SightingLng = 180.1 },
			wantErr: "sightingLng must be between -180 and 180",
		},
		{
			name:    "prevRefLat out of bounds low",
			mutate:  func(s *domain.SightingTrustScreening) { s.PrevRefLat = -91 },
			wantErr: "prevRefLat must be between -90 and 90",
		},
		{
			name:    "prevRefLat out of bounds high",
			mutate:  func(s *domain.SightingTrustScreening) { s.PrevRefLat = 91 },
			wantErr: "prevRefLat must be between -90 and 90",
		},
		{
			name:    "prevRefLng out of bounds low",
			mutate:  func(s *domain.SightingTrustScreening) { s.PrevRefLng = -181 },
			wantErr: "prevRefLng must be between -180 and 180",
		},
		{
			name:    "prevRefLng out of bounds high",
			mutate:  func(s *domain.SightingTrustScreening) { s.PrevRefLng = 181 },
			wantErr: "prevRefLng must be between -180 and 180",
		},
		{
			name:    "negative elapsedMinutes",
			mutate:  func(s *domain.SightingTrustScreening) { s.ElapsedMinutes = -0.1 },
			wantErr: "elapsedMinutes cannot be negative",
		},
		{
			name:    "invalid verdict",
			mutate:  func(s *domain.SightingTrustScreening) { s.Verdict = "INVALID" },
			wantErr: "invalid trust screening verdict",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validScreening
			tc.mutate(&s)
			err := s.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	raw, err := json.Marshal(validScreening)
	if err != nil {
		t.Fatalf("failed to marshal validScreening: %v", err)
	}
	var decoded domain.SightingTrustScreening
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal validScreening: %v", err)
	}
	if decoded.ScreeningID != validScreening.ScreeningID ||
		decoded.Verdict != validScreening.Verdict ||
		len(decoded.AnomalyFlags) != len(validScreening.AnomalyFlags) ||
		decoded.RiskScore != validScreening.RiskScore {
		t.Fatalf("round-trip mismatch: got %+v", decoded)
	}
}
