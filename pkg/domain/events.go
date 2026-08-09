package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// LostPetEvent represents a report of a lost pet.
type LostPetEvent struct {
	PetID         string    `json:"petId"`
	ReporterEmail string    `json:"reporterEmail"`
	ReportedAt    time.Time `json:"reportedAt"`
	Location      string    `json:"location"`
}

// Validate checks that mandatory fields on LostPetEvent are valid.
func (e *LostPetEvent) Validate() error {
	if strings.TrimSpace(e.PetID) == "" {
		return errors.New("domain: petId is required")
	}
	email := strings.TrimSpace(e.ReporterEmail)
	if email == "" {
		return errors.New("domain: reporterEmail is required")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return fmt.Errorf("domain: invalid reporterEmail address: %s", email)
	}
	return nil
}

// ToJSON serializes LostPetEvent to JSON bytes.
func (e *LostPetEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes JSON bytes into LostPetEvent.
func (e *LostPetEvent) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// FoundPetEvent represents a report of a found pet.
type FoundPetEvent struct {
	PetID    string    `json:"petId"`
	ImageURL string    `json:"imageUrl"`
	FoundAt  time.Time `json:"foundAt"`
	Location string    `json:"location"`
}

// Validate checks that mandatory fields on FoundPetEvent are non-empty.
func (e *FoundPetEvent) Validate() error {
	if strings.TrimSpace(e.PetID) == "" {
		return errors.New("domain: petId is required")
	}
	if strings.TrimSpace(e.ImageURL) == "" {
		return errors.New("domain: imageUrl is required")
	}
	return nil
}

// ToJSON serializes FoundPetEvent to JSON bytes.
func (e *FoundPetEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes JSON bytes into FoundPetEvent.
func (e *FoundPetEvent) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// MatchResult represents the visual scoring outcome between a found pet and a lost pet.
type MatchResult struct {
	FoundPetID   string  `json:"foundPetId"`
	MatchedPetID string  `json:"matchedPetId"`
	Score        float64 `json:"score"`
	IsMatch      bool    `json:"isMatch"`
	Details      string  `json:"details,omitempty"`
}

// Validate checks that mandatory fields on MatchResult are non-empty.
func (m *MatchResult) Validate() error {
	if strings.TrimSpace(m.FoundPetID) == "" || strings.TrimSpace(m.MatchedPetID) == "" {
		return errors.New("domain: both foundPetId and matchedPetId are required")
	}
	if m.Score < 0.0 || m.Score > 1.0 {
		return fmt.Errorf("domain: score must be between 0.0 and 1.0, got %f", m.Score)
	}
	return nil
}

// ToJSON serializes MatchResult to JSON bytes.
func (m *MatchResult) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON deserializes JSON bytes into MatchResult.
func (m *MatchResult) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}
