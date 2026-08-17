package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ImageTraitsStatus records whether traits are safe for matcher consumption.
type ImageTraitsStatus string

const (
	// ImageTraitsVerified means the traits came from a finalized private image
	// and a successfully parsed model response.
	ImageTraitsVerified ImageTraitsStatus = "verified"
)

// PetImageTraits is the durable, model-derived visual description of a pet.
type PetImageTraits struct {
	Breed               string   `json:"breed,omitempty"`
	PrimaryColor        string   `json:"primaryColor,omitempty"`
	SecondaryColor      string   `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string `json:"distinctiveMarkings,omitempty"`
	EyeColor            string   `json:"eyeColor,omitempty"`
}

// ImageTraitAnalysis records verified traits and the provenance needed to
// reproduce or supersede the analysis safely.
type ImageTraitAnalysis struct {
	Status            ImageTraitsStatus `json:"status"`
	Traits            PetImageTraits    `json:"traits"`
	Model             string            `json:"model"`
	AnalysisVersion   string            `json:"analysisVersion"`
	SourceEventID     string            `json:"sourceEventId"`
	SourceImageObject string            `json:"sourceImageObject"`
	VerifiedAt        time.Time         `json:"verifiedAt"`
}

// NormalizeImageTraitAnalysis returns an isolated canonical copy.
func NormalizeImageTraitAnalysis(analysis *ImageTraitAnalysis) *ImageTraitAnalysis {
	if analysis == nil {
		return nil
	}
	normalized := *analysis
	normalized.Traits.Breed = strings.TrimSpace(normalized.Traits.Breed)
	normalized.Traits.PrimaryColor = strings.TrimSpace(normalized.Traits.PrimaryColor)
	normalized.Traits.SecondaryColor = strings.TrimSpace(normalized.Traits.SecondaryColor)
	normalized.Traits.EyeColor = strings.TrimSpace(normalized.Traits.EyeColor)
	normalized.Traits.DistinctiveMarkings = normalizeMarkings(normalized.Traits.DistinctiveMarkings)
	normalized.Model = strings.TrimSpace(normalized.Model)
	normalized.AnalysisVersion = strings.TrimSpace(normalized.AnalysisVersion)
	normalized.SourceEventID = strings.TrimSpace(normalized.SourceEventID)
	normalized.SourceImageObject = strings.TrimSpace(normalized.SourceImageObject)
	if !normalized.VerifiedAt.IsZero() {
		normalized.VerifiedAt = normalized.VerifiedAt.UTC()
	}
	return &normalized
}

// Validate checks verified trait and provenance invariants.
func (a ImageTraitAnalysis) Validate() error {
	normalized := NormalizeImageTraitAnalysis(&a)
	if normalized.Status != ImageTraitsVerified {
		return fmt.Errorf("domain: unsupported image traits status %q", normalized.Status)
	}
	if !normalized.Traits.hasAnyValue() {
		return errors.New("domain: verified image analysis requires at least one trait")
	}
	if normalized.Model == "" || normalized.AnalysisVersion == "" || normalized.SourceEventID == "" ||
		normalized.SourceImageObject == "" || normalized.VerifiedAt.IsZero() {
		return errors.New("domain: verified image analysis requires complete provenance")
	}
	fields := []struct {
		name  string
		value string
		limit int
	}{
		{name: "breed", value: normalized.Traits.Breed, limit: 200},
		{name: "primaryColor", value: normalized.Traits.PrimaryColor, limit: 100},
		{name: "secondaryColor", value: normalized.Traits.SecondaryColor, limit: 100},
		{name: "eyeColor", value: normalized.Traits.EyeColor, limit: 100},
		{name: "model", value: normalized.Model, limit: 200},
		{name: "analysisVersion", value: normalized.AnalysisVersion, limit: 100},
		{name: "sourceEventId", value: normalized.SourceEventID, limit: 256},
		{name: "sourceImageObject", value: normalized.SourceImageObject, limit: 1024},
	}
	for _, field := range fields {
		if utf8.RuneCountInString(field.value) > field.limit {
			return fmt.Errorf("domain: image analysis %s exceeds %d characters", field.name, field.limit)
		}
	}
	if len(normalized.Traits.DistinctiveMarkings) > 50 {
		return errors.New("domain: image analysis has too many distinctive markings")
	}
	for _, marking := range normalized.Traits.DistinctiveMarkings {
		if utf8.RuneCountInString(marking) > 500 {
			return errors.New("domain: image analysis distinctive marking exceeds 500 characters")
		}
	}
	return nil
}

func (t PetImageTraits) hasAnyValue() bool {
	return strings.TrimSpace(t.Breed) != "" || strings.TrimSpace(t.PrimaryColor) != "" ||
		strings.TrimSpace(t.SecondaryColor) != "" || strings.TrimSpace(t.EyeColor) != "" ||
		len(normalizeMarkings(t.DistinctiveMarkings)) > 0
}
