package trustsafety

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const earthRadiusKm = 6371.0

var extortionKeywords = []string{
	"zelle",
	"venmo",
	"cashapp",
	"cash app",
	"bitcoin",
	"btc",
	"usdt",
	"crypto",
	"gift card",
	"steam card",
	"wire transfer",
	"western union",
	"send money",
	"pay upfront",
	"ransom",
}

var pinPhishingKeywords = []string{
	"verification code",
	"6-digit code",
	"google voice",
	"send me the code",
	"prove you are the owner",
}

func HaversineDistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	toRad := func(deg float64) float64 {
		return deg * math.Pi / 180.0
	}

	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	d := earthRadiusKm * c

	return math.Round(d*100) / 100
}

func EvaluateSightingTrust(item domain.SightingTrustScreening) domain.SightingTrustScreening {
	item.DistanceFromPrevKm = HaversineDistanceKm(item.PrevRefLat, item.PrevRefLng, item.SightingLat, item.SightingLng)

	if item.ElapsedMinutes > 0 {
		hours := item.ElapsedMinutes / 60.0
		item.ImpliedVelocityKmh = math.Round((item.DistanceFromPrevKm/hours)*10) / 10
	} else if item.DistanceFromPrevKm > 0 {
		item.ImpliedVelocityKmh = 999.9
	} else {
		item.ImpliedVelocityKmh = 0
	}

	risk := 0
	flags := []domain.ThreatAnomalyFlag{}
	notesLower := strings.ToLower(item.SubmittedNotes)

	// Check 1 — Extortion / Advance-Fee Ransom Keywords
	if containsAnyKeyword(notesLower, extortionKeywords) {
		risk += 65
		flags = append(flags, domain.FlagExtortionPaymentDemand)
	}

	// Check 2 — 2FA / Google Voice PIN Phishing
	if containsAnyKeyword(notesLower, pinPhishingKeywords) {
		risk += 55
		flags = append(flags, domain.FlagPinPhishingAttempt)
	}

	// Check 3 — Kinematic Velocity / Biological Teleportation Anomaly
	if item.ImpliedVelocityKmh > 35.0 && item.DistanceFromPrevKm > 3.0 {
		risk += 45
		flags = append(flags, domain.FlagTeleportationVelocity)
	}

	// Check 4 — Synthetic / Tampered Image Integrity
	tag := strings.ToUpper(strings.TrimSpace(item.ImageIntegrityTag))
	if tag == "" {
		tag = "CAMERA_EXIF_INTACT"
	}
	item.ImageIntegrityTag = tag

	switch tag {
	case "SYNTHETIC_GAN_SIGNATURE", "STOCK_IMAGE_REUSE":
		risk += 50
		flags = append(flags, domain.FlagSyntheticImageArtifact)
	case "STRIPPED_EXIF":
		risk += 15
	}

	// Check 5 — Reporter Reputation
	if item.ReporterTrustScore < 35 {
		risk += 25
		flags = append(flags, domain.FlagUntrustedNewReporter)
	} else if item.ReporterTrustScore >= 80 && len(flags) == 0 {
		risk -= 15
	}

	if risk < 0 {
		risk = 0
	}
	if risk > 100 {
		risk = 100
	}

	item.RiskScore = risk
	item.TrustConfidenceScore = 100 - risk
	item.AnomalyFlags = flags

	if risk >= 70 {
		item.Verdict = domain.TrustVerdictQuarantinedScam
		item.ExplanationSummary = fmt.Sprintf(
			"QUARANTINED (Risk %d/100): Blocked owner alert — triggered %v (%.1f km in %.0f min = %.1f km/h)",
			risk, flags, item.DistanceFromPrevKm, item.ElapsedMinutes, item.ImpliedVelocityKmh,
		)
	} else if risk >= 35 {
		item.Verdict = domain.TrustVerdictHoldModeration
		item.ExplanationSummary = fmt.Sprintf(
			"HELD FOR MODERATION (Risk %d/100): Requires SAR review — triggered %v (%.1f km in %.0f min = %.1f km/h)",
			risk, flags, item.DistanceFromPrevKm, item.ElapsedMinutes, item.ImpliedVelocityKmh,
		)
	} else {
		item.Verdict = domain.TrustVerdictAuthentic
		item.ExplanationSummary = fmt.Sprintf(
			"VERIFIED AUTHENTIC (Confidence %d%%): Plausible kinematics (%.1f km/h) & clean telemetry",
			item.TrustConfidenceScore, item.ImpliedVelocityKmh,
		)
	}

	return item
}

func ApplyModeratorDecision(
	item domain.SightingTrustScreening,
	verdict domain.TrustScreeningVerdict,
	moderatorNotes string,
	now time.Time,
) (domain.SightingTrustScreening, error) {
	if verdict != domain.TrustVerdictAuthentic && verdict != domain.TrustVerdictRejectedHoax {
		return item, errors.New("moderator verdict must be VERIFIED_AUTHENTIC or REJECTED_CONFIRMED_HOAX")
	}

	item.Verdict = verdict
	item.ModeratorNotes = strings.TrimSpace(moderatorNotes)
	item.UpdatedAt = now.UTC()

	if verdict == domain.TrustVerdictAuthentic {
		item.RiskScore = 5
		item.TrustConfidenceScore = 95
		item.ReporterTrustScore += 15
		if item.ReporterTrustScore > 100 {
			item.ReporterTrustScore = 100
		}
		item.ExplanationSummary = "MODERATOR CLEARED: Verified authentic field sighting — promoted to live SAR map"
	} else {
		item.RiskScore = 100
		item.TrustConfidenceScore = 0
		item.ReporterTrustScore = 0
		item.ExplanationSummary = "MODERATOR REJECTED: Confirmed scam/hoax actor — reporter trust zeroed & blocked"
	}

	return item, nil
}

func containsAnyKeyword(textLower string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(textLower, kw) {
			return true
		}
	}
	return false
}
