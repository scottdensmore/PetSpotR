package domain

import (
	"errors"
	"strings"
	"time"
)

type TrustScreeningVerdict string

const (
	TrustVerdictAuthentic       TrustScreeningVerdict = "VERIFIED_AUTHENTIC"
	TrustVerdictHoldModeration  TrustScreeningVerdict = "HOLD_FOR_MODERATION"
	TrustVerdictQuarantinedScam TrustScreeningVerdict = "QUARANTINED_SCAM_RISK"
	TrustVerdictRejectedHoax    TrustScreeningVerdict = "REJECTED_CONFIRMED_HOAX"
)

func (v TrustScreeningVerdict) IsValid() bool {
	switch v {
	case TrustVerdictAuthentic, TrustVerdictHoldModeration, TrustVerdictQuarantinedScam, TrustVerdictRejectedHoax:
		return true
	default:
		return false
	}
}

type ThreatAnomalyFlag string

const (
	FlagExtortionPaymentDemand ThreatAnomalyFlag = "EXTORTION_PAYMENT_DEMAND"
	FlagPinPhishingAttempt     ThreatAnomalyFlag = "PIN_PHISHING_ATTEMPT"
	FlagTeleportationVelocity  ThreatAnomalyFlag = "TELEPORTATION_VELOCITY_ANOMALY"
	FlagSyntheticImageArtifact ThreatAnomalyFlag = "SYNTHETIC_IMAGE_ARTIFACT"
	FlagUntrustedNewReporter   ThreatAnomalyFlag = "UNTRUSTED_REPORTER_HISTORY"
)

func (f ThreatAnomalyFlag) IsValid() bool {
	switch f {
	case FlagExtortionPaymentDemand,
		FlagPinPhishingAttempt,
		FlagTeleportationVelocity,
		FlagSyntheticImageArtifact,
		FlagUntrustedNewReporter:
		return true
	default:
		return false
	}
}

type SightingTrustScreening struct {
	ScreeningID          string                `json:"screeningId"`
	PetID                string                `json:"petId"`
	PetName              string                `json:"petName"`
	ReporterHandle       string                `json:"reporterHandle"`
	ReporterTrustScore   int                   `json:"reporterTrustScore"`
	SightingLat          float64               `json:"sightingLat"`
	SightingLng          float64               `json:"sightingLng"`
	PrevRefLat           float64               `json:"prevRefLat"`
	PrevRefLng           float64               `json:"prevRefLng"`
	ElapsedMinutes       float64               `json:"elapsedMinutes"`
	DistanceFromPrevKm   float64               `json:"distanceFromPrevKm"`
	ImpliedVelocityKmh   float64               `json:"impliedVelocityKmh"`
	SubmittedNotes       string                `json:"submittedNotes"`
	ImageIntegrityTag    string                `json:"imageIntegrityTag"`
	RiskScore            int                   `json:"riskScore"`
	TrustConfidenceScore int                   `json:"trustConfidenceScore"`
	Verdict              TrustScreeningVerdict `json:"verdict"`
	AnomalyFlags         []ThreatAnomalyFlag   `json:"anomalyFlags"`
	ExplanationSummary   string                `json:"explanationSummary"`
	ModeratorNotes       string                `json:"moderatorNotes,omitempty"`
	PromotedSightingID   string                `json:"promotedSightingId,omitempty"`
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt"`
}

func (s SightingTrustScreening) Validate() error {
	if strings.TrimSpace(s.ScreeningID) == "" {
		return errors.New("screeningId is required")
	}
	if strings.TrimSpace(s.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(s.ReporterHandle) == "" {
		return errors.New("reporterHandle is required")
	}
	if s.ReporterTrustScore < 0 || s.ReporterTrustScore > 100 {
		return errors.New("reporterTrustScore must be between 0 and 100")
	}
	if s.SightingLat < -90 || s.SightingLat > 90 {
		return errors.New("sightingLat must be between -90 and 90")
	}
	if s.SightingLng < -180 || s.SightingLng > 180 {
		return errors.New("sightingLng must be between -180 and 180")
	}
	if s.PrevRefLat < -90 || s.PrevRefLat > 90 {
		return errors.New("prevRefLat must be between -90 and 90")
	}
	if s.PrevRefLng < -180 || s.PrevRefLng > 180 {
		return errors.New("prevRefLng must be between -180 and 180")
	}
	if s.ElapsedMinutes < 0 {
		return errors.New("elapsedMinutes cannot be negative")
	}
	if !s.Verdict.IsValid() {
		return errors.New("invalid trust screening verdict")
	}
	return nil
}
