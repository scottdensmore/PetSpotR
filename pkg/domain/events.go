package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// LostPetEvent is published when a lost pet report is registered.
type LostPetEvent struct {
	PetID         string    `json:"petId"`
	ReporterEmail string    `json:"reporterEmail"`
	ReportedAt    time.Time `json:"reportedAt"`
	Location      string    `json:"location"`
}

// Validate checks that required fields are present in LostPetEvent.
func (e *LostPetEvent) Validate() error {
	if strings.TrimSpace(e.PetID) == "" {
		return errors.New("pet ID cannot be empty")
	}
	return nil
}

// ToJSON serializes LostPetEvent to JSON.
func (e *LostPetEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes JSON bytes into LostPetEvent.
func (e *LostPetEvent) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// FoundPetEvent is published when a found pet image is uploaded for prediction.
type FoundPetEvent struct {
	PetID    string    `json:"petId"`
	ImageURL string    `json:"imageUrl"`
	FoundAt  time.Time `json:"foundAt"`
	Location string    `json:"location"`
}

// Validate checks that required fields are present in FoundPetEvent.
func (e *FoundPetEvent) Validate() error {
	if strings.TrimSpace(e.ImageURL) == "" {
		return errors.New("image URL cannot be empty")
	}
	return nil
}

// ToJSON serializes FoundPetEvent to JSON.
func (e *FoundPetEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes JSON bytes into FoundPetEvent.
func (e *FoundPetEvent) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// MatchResult stores the prediction outcome between a found pet and a lost pet.
type MatchResult struct {
	FoundPetID   string  `json:"foundPetId"`
	MatchedPetID string  `json:"matchedPetId"`
	Score        float64 `json:"score"`
	IsMatch      bool    `json:"isMatch"`
	Details      string  `json:"details"`
}

// Validate checks that MatchResult contains both found and matched pet identifiers.
func (m *MatchResult) Validate() error {
	if strings.TrimSpace(m.FoundPetID) == "" || strings.TrimSpace(m.MatchedPetID) == "" {
		return errors.New("match result requires both foundPetId and matchedPetId")
	}
	return nil
}

// ToJSON serializes MatchResult to JSON.
func (m *MatchResult) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON deserializes JSON bytes into MatchResult.
func (m *MatchResult) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}
