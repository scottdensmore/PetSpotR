# Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Engine) — Design Specification

- **Milestone:** 13.2
- **Date:** 2026-10-17
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

Families of lost pets are frequently targeted by reward-extortion scams (demanding advance crypto/gift-card/wire payments before revealing a pet's location, or sending AI-generated/stock photos), while search-and-rescue teams can be diverted miles away by physically impossible "teleportation" hoax sightings. PetSpotR requires an automated **Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Engine)** that:
1. **Detects Reward-Extortion & Advance-Fee Scam Patterns:** Scans inbound sighting notes and contact metadata for coercive payment demands (`Zelle`, `Venmo`, `CashApp`, `Bitcoin`, `USDT`, `gift card`, `wire transfer`, `pay upfront`, `verification code`, `Google Voice pin`).
2. **Evaluates Kinematic Velocity / Teleportation Anomalies:** Computes the Haversine distance (`km`), elapsed time (`hours`), and implied ground velocity (`km/h`) from the pet's Last Known Point (LKP) or prior verified sighting—flagging physically impossible biological travel speeds (`> 35 km/h` for dogs/cats without vehicle transport) as `TELEPORTATION_ANOMALY`.
3. **Detects Synthetic / Tampered Image & Metadata Anomalies:** Checks whether submitted image metadata/hashes indicate stripped EXIF/synthetic generation (`SYNTHETIC_AI_ARTIFACT`, `STOCK_IMAGE_REUSE`) and incorporates the reporter's historical `ReporterTrustScore` (`0–100`).
4. **Quarantines High-Risk Reports & Provides a Moderator Verification Cockpit (`/trust-safety`):** Automatically routes suspicious reports to `QUARANTINED_SCAM_RISK` or `HOLD_FOR_MODERATION` so distressed pet owners are shielded from extortion while SAR moderators can inspect the multi-factor risk breakdown, confirm & ban scam actors (`REJECTED_CONFIRMED_HOAX`), or clear false-positives (`VERIFIED_AUTHENTIC`) to promote them to live Community Sightings.

---

## 2. Domain Models & State Store Collections

### 2.1 State Store Collections (`pkg/store/store.go`)
- `CollectionSightingTrustScreening = "sighting_trust_screenings"`

### 2.2 Domain Models (`pkg/domain/trust_safety.go`)

```go
package domain

import "time"

type TrustScreeningVerdict string

const (
	TrustVerdictAuthentic        TrustScreeningVerdict = "VERIFIED_AUTHENTIC"
	TrustVerdictHoldModeration   TrustScreeningVerdict = "HOLD_FOR_MODERATION"
	TrustVerdictQuarantinedScam  TrustScreeningVerdict = "QUARANTINED_SCAM_RISK"
	TrustVerdictRejectedHoax     TrustScreeningVerdict = "REJECTED_CONFIRMED_HOAX"
)

type ThreatAnomalyFlag string

const (
	FlagExtortionPaymentDemand ThreatAnomalyFlag = "EXTORTION_PAYMENT_DEMAND"
	FlagPinPhishingAttempt     ThreatAnomalyFlag = "PIN_PHISHING_ATTEMPT"
	FlagTeleportationVelocity  ThreatAnomalyFlag = "TELEPORTATION_VELOCITY_ANOMALY"
	FlagSyntheticImageArtifact ThreatAnomalyFlag = "SYNTHETIC_IMAGE_ARTIFACT"
	FlagUntrustedNewReporter   ThreatAnomalyFlag = "UNTRUSTED_REPORTER_HISTORY"
)

type SightingTrustScreening struct {
	ScreeningID          string                `json:"screeningId"`
	PetID                string                `json:"petId"`
	PetName              string                `json:"petName"`
	ReporterHandle       string                `json:"reporterHandle"`
	ReporterTrustScore   int                   `json:"reporterTrustScore"` // 0 - 100
	SightingLat          float64               `json:"sightingLat"`
	SightingLng          float64               `json:"sightingLng"`
	PrevRefLat           float64               `json:"prevRefLat"`
	PrevRefLng           float64               `json:"prevRefLng"`
	ElapsedMinutes       float64               `json:"elapsedMinutes"`
	DistanceFromPrevKm   float64               `json:"distanceFromPrevKm"`
	ImpliedVelocityKmh   float64               `json:"impliedVelocityKmh"`
	SubmittedNotes       string                `json:"submittedNotes"`
	ImageIntegrityTag    string                `json:"imageIntegrityTag"` // "CAMERA_EXIF_INTACT", "STRIPPED_EXIF", "SYNTHETIC_GAN_SIGNATURE"
	RiskScore            int                   `json:"riskScore"`         // 0 - 100 (higher = riskier)
	TrustConfidenceScore int                   `json:"trustConfidenceScore"` // 100 - RiskScore
	Verdict              TrustScreeningVerdict `json:"verdict"`
	AnomalyFlags         []ThreatAnomalyFlag   `json:"anomalyFlags"`
	ExplanationSummary   string                `json:"explanationSummary"`
	ModeratorNotes       string                `json:"moderatorNotes,omitempty"`
	PromotedSightingID   string                `json:"promotedSightingId,omitempty"`
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt"`
}
```

---

## 3. Pure-Go Multi-Modal Scam, Velocity & Synthetic Heuristics Engine (`pkg/trustsafety`)

### 3.1 Scam Keyword, Phishing & Image Integrity Analyzer (`pkg/trustsafety/shield.go`)
- `EvaluateSightingTrust(item domain.SightingTrustScreening) domain.SightingTrustScreening`:
  - Computes Haversine distance `DistanceFromPrevKm` between `(item.PrevRefLat, item.PrevRefLng)` and `(item.SightingLat, item.SightingLng)`.
  - Computes `ImpliedVelocityKmh`: if `item.ElapsedMinutes > 0`, `DistanceFromPrevKm / (item.ElapsedMinutes / 60.0)` rounded to 1 decimal place.
  - Starts with `risk := 0` and empty `flags := []domain.ThreatAnomalyFlag{}`:
    1. **Extortion / Advance-Fee Scam Check (`FlagExtortionPaymentDemand`):**
       - Case-insensitive scan of `item.SubmittedNotes` for payment/ransom terms (`"zelle"`, `"venmo"`, `"cashapp"`, `"bitcoin"`, `"btc"`, `"usdt"`, `"crypto"`, `"gift card"`, `"steam card"`, `"wire transfer"`, `"western union"`, `"send money"`, `"pay upfront"`, `"ransom"`).
       - If matched: `risk += 65`, append `FlagExtortionPaymentDemand`.
    2. **2FA / Google Voice PIN Phishing Check (`FlagPinPhishingAttempt`):**
       - Case-insensitive scan of `item.SubmittedNotes` for (`"verification code"`, `"6-digit code"`, `"google voice"`, `"send me the code"`, `"prove you are the owner"`).
       - If matched: `risk += 55`, append `FlagPinPhishingAttempt`.
    3. **Kinematic Velocity / Biological Teleportation Check (`FlagTeleportationVelocity`):**
       - If `ImpliedVelocityKmh > 35.0` and `DistanceFromPrevKm > 3.0`:
         - `risk += 45`, append `FlagTeleportationVelocity`.
    4. **Synthetic / Tampered Image Check (`FlagSyntheticImageArtifact`):**
       - If `item.ImageIntegrityTag == "SYNTHETIC_GAN_SIGNATURE"` or `item.ImageIntegrityTag == "STOCK_IMAGE_REUSE"`:
         - `risk += 50`, append `FlagSyntheticImageArtifact`.
       - Else if `item.ImageIntegrityTag == "STRIPPED_EXIF"`:
         - `risk += 15`.
    5. **Reporter Reputation Check (`FlagUntrustedNewReporter`):**
       - If `item.ReporterTrustScore < 35`:
         - `risk += 25`, append `FlagUntrustedNewReporter`.
       - Else if `item.ReporterTrustScore >= 80` and `len(flags) == 0`:
         - `risk -= 15`.
  - Clamp `risk` to `[0, 100]`.
  - `item.RiskScore = risk`
  - `item.TrustConfidenceScore = 100 - risk`
  - Determine `Verdict`:
    - If `risk >= 70`: `item.Verdict = domain.TrustVerdictQuarantinedScam`
    - Else if `risk >= 35`: `item.Verdict = domain.TrustVerdictHoldModeration`
    - Else: `item.Verdict = domain.TrustVerdictAuthentic`
  - Build a clear human-readable `item.ExplanationSummary` listing triggered flags, implied velocity (`km/h`), and whether owner notifications were blocked to prevent extortion.

---

## 4. REST API & Trust & Safety Moderator Cockpit (`internal/app/webfrontend/trust_safety_handlers.go`)

1. `GET /trust-safety` — Renders the Multi-Modal False-Positive & Scam/Hoax Sighting Shield Cockpit (`templates/trust_safety.html`).
2. `GET /api/v1/trust-safety/screenings` — Lists all sighting trust screenings sorted by `UpdatedAt` descending (seeds 3 default screenings on first lookup: 1 `QUARANTINED_SCAM_RISK` extortion attempt, 1 `HOLD_FOR_MODERATION` high-velocity anomaly, and 1 `VERIFIED_AUTHENTIC` EXIF-verified sighting).
3. `POST /api/v1/trust-safety/screenings` — Evaluates an inbound sighting submission through `trustsafety.EvaluateSightingTrust`, persists to `CollectionSightingTrustScreening`, automatically promotes `VERIFIED_AUTHENTIC` reports to `store.SightingsCollection`, and broadcasts `"trust_screening_updated"` via SSE (`/api/v1/trust-safety/stream`) and WebRTC mesh.
4. `POST /api/v1/trust-safety/screenings/{id}/decision` — Allows a SAR Moderator to adjudicate a quarantined or held screening (`verdict: "VERIFIED_AUTHENTIC"` or `"REJECTED_CONFIRMED_HOAX"`):
   - When adjudicated as `"VERIFIED_AUTHENTIC"`, adjusts `ReporterTrustScore` upward (`+15`, max `100`) and promotes the sighting into `store.SightingsCollection` (`PromotedSightingID`).
   - When adjudicated as `"REJECTED_CONFIRMED_HOAX"`, penalizes `ReporterTrustScore` to `0`, sets `RiskScore = 100`, and permanently blocks promotion.
5. `GET /api/v1/trust-safety/stream` — Public SSE stream on topic `"trust_safety"`.
