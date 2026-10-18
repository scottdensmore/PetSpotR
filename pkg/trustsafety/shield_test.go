package trustsafety_test

import (
	"slices"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/trustsafety"
)

func TestEvaluateSightingTrust_ExtortionAndSyntheticGAN_Quarantined(t *testing.T) {
	input := domain.SightingTrustScreening{
		ScreeningID:        "ts-001",
		PetID:              "pet-101",
		PetName:            "Barnaby",
		ReporterHandle:     "@sketchy_actor",
		ReporterTrustScore: 20,
		PrevRefLat:         37.7749,
		PrevRefLng:         -122.4194,
		SightingLat:        37.7849,
		SightingLng:        -122.4094,
		ElapsedMinutes:     30,
		SubmittedNotes:     "I have Barnaby locked up. Send $300 via Zelle or Bitcoin upfront if you want your dog back.",
		ImageIntegrityTag:  "SYNTHETIC_GAN_SIGNATURE",
	}

	got := trustsafety.EvaluateSightingTrust(input)

	if got.Verdict != domain.TrustVerdictQuarantinedScam {
		t.Fatalf("expected verdict %q, got %q", domain.TrustVerdictQuarantinedScam, got.Verdict)
	}
	if got.RiskScore < 70 || got.RiskScore > 100 {
		t.Fatalf("expected risk score in [70, 100], got %d", got.RiskScore)
	}
	if got.TrustConfidenceScore != 100-got.RiskScore {
		t.Fatalf("expected confidence score %d, got %d", 100-got.RiskScore, got.TrustConfidenceScore)
	}
	if !slices.Contains(got.AnomalyFlags, domain.FlagExtortionPaymentDemand) {
		t.Errorf("expected flag %q in %v", domain.FlagExtortionPaymentDemand, got.AnomalyFlags)
	}
	if !slices.Contains(got.AnomalyFlags, domain.FlagSyntheticImageArtifact) {
		t.Errorf("expected flag %q in %v", domain.FlagSyntheticImageArtifact, got.AnomalyFlags)
	}
	if !slices.Contains(got.AnomalyFlags, domain.FlagUntrustedNewReporter) {
		t.Errorf("expected flag %q in %v", domain.FlagUntrustedNewReporter, got.AnomalyFlags)
	}
}

func TestEvaluateSightingTrust_GoogleVoicePinPhishing(t *testing.T) {
	input := domain.SightingTrustScreening{
		ScreeningID:        "ts-002",
		PetID:              "pet-102",
		PetName:            "Luna",
		ReporterHandle:     "@caller_id_spoof",
		ReporterTrustScore: 45,
		PrevRefLat:         37.7749,
		PrevRefLng:         -122.4194,
		SightingLat:        37.7755,
		SightingLng:        -122.4185,
		ElapsedMinutes:     40,
		SubmittedNotes:     "Found your cat! Before I share my address, prove you are the owner and send me the 6-digit code from Google Voice.",
		ImageIntegrityTag:  "STRIPPED_EXIF",
	}

	got := trustsafety.EvaluateSightingTrust(input)

	if !slices.Contains(got.AnomalyFlags, domain.FlagPinPhishingAttempt) {
		t.Fatalf("expected flag %q in %v", domain.FlagPinPhishingAttempt, got.AnomalyFlags)
	}
	if got.Verdict != domain.TrustVerdictQuarantinedScam {
		t.Fatalf("expected verdict %q (55 + 15 = 70), got %q (risk=%d)", domain.TrustVerdictQuarantinedScam, got.Verdict, got.RiskScore)
	}
}

func TestEvaluateSightingTrust_BiologicalTeleportationVelocity(t *testing.T) {
	// ~50 km north in 15 minutes (~200 km/h)
	input := domain.SightingTrustScreening{
		ScreeningID:        "ts-003",
		PetID:              "pet-103",
		PetName:            "Milo",
		ReporterHandle:     "@confused_tipster",
		ReporterTrustScore: 60,
		PrevRefLat:         37.7749,
		PrevRefLng:         -122.4194,
		SightingLat:        38.2249,
		SightingLng:        -122.4194,
		ElapsedMinutes:     15,
		SubmittedNotes:     "Saw a golden retriever running near the highway exit.",
		ImageIntegrityTag:  "CAMERA_EXIF_INTACT",
	}

	got := trustsafety.EvaluateSightingTrust(input)

	if !slices.Contains(got.AnomalyFlags, domain.FlagTeleportationVelocity) {
		t.Fatalf("expected flag %q in %v", domain.FlagTeleportationVelocity, got.AnomalyFlags)
	}
	if got.DistanceFromPrevKm < 45 || got.DistanceFromPrevKm > 55 {
		t.Fatalf("expected distance around 50 km, got %.2f km", got.DistanceFromPrevKm)
	}
	if got.ImpliedVelocityKmh < 180 {
		t.Fatalf("expected implied velocity > 180 km/h, got %.1f km/h", got.ImpliedVelocityKmh)
	}
	if got.Verdict != domain.TrustVerdictHoldModeration {
		t.Fatalf("expected verdict %q, got %q (risk=%d)", domain.TrustVerdictHoldModeration, got.Verdict, got.RiskScore)
	}
}

func TestEvaluateSightingTrust_AuthenticSighting(t *testing.T) {
	// ~1.2 km in 45 minutes (~1.6 km/h)
	input := domain.SightingTrustScreening{
		ScreeningID:        "ts-004",
		PetID:              "pet-104",
		PetName:            "Bella",
		ReporterHandle:     "@sar_volunteer_09",
		ReporterTrustScore: 85,
		PrevRefLat:         37.7749,
		PrevRefLng:         -122.4194,
		SightingLat:        37.7857,
		SightingLng:        -122.4194,
		ElapsedMinutes:     45,
		SubmittedNotes:     "Spotted Bella Drinking water near Alamo Square fountain, heading east at a calm trot.",
		ImageIntegrityTag:  "",
	}

	got := trustsafety.EvaluateSightingTrust(input)

	if got.Verdict != domain.TrustVerdictAuthentic {
		t.Fatalf("expected verdict %q, got %q (risk=%d)", domain.TrustVerdictAuthentic, got.Verdict, got.RiskScore)
	}
	if got.RiskScore != 0 {
		t.Fatalf("expected risk score 0, got %d", got.RiskScore)
	}
	if got.TrustConfidenceScore != 100 {
		t.Fatalf("expected confidence score 100, got %d", got.TrustConfidenceScore)
	}
	if got.ImageIntegrityTag != "CAMERA_EXIF_INTACT" {
		t.Fatalf("expected default ImageIntegrityTag CAMERA_EXIF_INTACT, got %q", got.ImageIntegrityTag)
	}
	if len(got.AnomalyFlags) != 0 {
		t.Fatalf("expected 0 anomaly flags, got %v", got.AnomalyFlags)
	}
}

func TestApplyModeratorDecision(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	base := domain.SightingTrustScreening{
		ScreeningID:        "ts-005",
		PetID:              "pet-105",
		PetName:            "Cooper",
		ReporterHandle:     "@neighbor_dan",
		ReporterTrustScore: 60,
		RiskScore:          45,
		Verdict:            domain.TrustVerdictHoldModeration,
	}

	cleared, err := trustsafety.ApplyModeratorDecision(base, domain.TrustVerdictAuthentic, "  Verified via ring camera timestamp  ", now)
	if err != nil {
		t.Fatalf("unexpected error clearing sighting: %v", err)
	}
	if cleared.Verdict != domain.TrustVerdictAuthentic {
		t.Errorf("expected verdict %q, got %q", domain.TrustVerdictAuthentic, cleared.Verdict)
	}
	if cleared.RiskScore != 5 || cleared.TrustConfidenceScore != 95 {
		t.Errorf("expected risk=5, confidence=95, got risk=%d, confidence=%d", cleared.RiskScore, cleared.TrustConfidenceScore)
	}
	if cleared.ReporterTrustScore != 75 {
		t.Errorf("expected reporter trust 75, got %d", cleared.ReporterTrustScore)
	}
	if cleared.ModeratorNotes != "Verified via ring camera timestamp" {
		t.Errorf("expected trimmed moderator notes, got %q", cleared.ModeratorNotes)
	}

	rejected, err := trustsafety.ApplyModeratorDecision(base, domain.TrustVerdictRejectedHoax, "Confirmed extortion botnet number", now)
	if err != nil {
		t.Fatalf("unexpected error rejecting sighting: %v", err)
	}
	if rejected.Verdict != domain.TrustVerdictRejectedHoax {
		t.Errorf("expected verdict %q, got %q", domain.TrustVerdictRejectedHoax, rejected.Verdict)
	}
	if rejected.RiskScore != 100 || rejected.TrustConfidenceScore != 0 || rejected.ReporterTrustScore != 0 {
		t.Errorf("expected risk=100, confidence=0, reporterTrust=0, got %d/%d/%d", rejected.RiskScore, rejected.TrustConfidenceScore, rejected.ReporterTrustScore)
	}

	if _, err := trustsafety.ApplyModeratorDecision(base, domain.TrustVerdictHoldModeration, "invalid", now); err == nil {
		t.Fatal("expected error when applying non-terminal moderator verdict")
	}
}
