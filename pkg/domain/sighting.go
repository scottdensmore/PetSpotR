package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SightingStatus represents the moderation or lifecycle state of a community sighting.
type SightingStatus string

const (
	// SightingStatusActive indicates the sighting is active and eligible for trajectory calculations.
	SightingStatusActive SightingStatus = "active"
	// SightingStatusFlagged indicates the sighting has been flagged for moderation.
	SightingStatusFlagged SightingStatus = "flagged"
	// SightingStatusDismissed indicates the sighting has been dismissed or rejected.
	SightingStatusDismissed SightingStatus = "dismissed"
)

// PetSightingRecord captures a community witness report of a lost pet.
type PetSightingRecord struct {
	SightingID          string         `json:"sightingId"`
	LostPetID           string         `json:"lostPetId"`
	ReportedAt          time.Time      `json:"reportedAt"`
	SightedAt           time.Time      `json:"sightedAt"`
	LocationDescription string         `json:"locationDescription"`
	Coordinates         *LocationPoint `json:"coordinates"`
	MovementDirection   string         `json:"movementDirection,omitempty"` // e.g. "North", "Stationary"
	ImageURL            string         `json:"imageUrl,omitempty"`
	ImageObject         string         `json:"imageObject,omitempty"`
	Notes               string         `json:"notes,omitempty"`
	Status              SightingStatus `json:"status"`
}

// Validate checks that the PetSightingRecord contains required fields and valid coordinates.
func (s PetSightingRecord) Validate() error {
	if strings.TrimSpace(s.SightingID) == "" {
		return errors.New("domain: sighting ID is required")
	}
	if strings.TrimSpace(s.LostPetID) == "" {
		return errors.New("domain: lost pet ID is required")
	}
	if s.SightedAt.IsZero() {
		return errors.New("domain: sightedAt timestamp is required")
	}
	if s.Coordinates == nil {
		return errors.New("domain: coordinates are required")
	}
	if err := s.Coordinates.Validate(); err != nil {
		return err
	}
	switch s.Status {
	case SightingStatusActive, SightingStatusFlagged, SightingStatusDismissed:
	default:
		return fmt.Errorf("domain: invalid sighting status: %s", s.Status)
	}
	return nil
}

// NotificationItem represents an alert generated for pet events, such as a sighting report.
type NotificationItem struct {
	ID        string    `json:"id"`
	PetID     string    `json:"petId"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
}

// SightingEventPayload represents the real-time event envelope payload broadcast to reunion rooms.
type SightingEventPayload struct {
	Type                string         `json:"type"`
	PetID               string         `json:"petId"`
	SightingID          string         `json:"sightingId"`
	SightedAt           time.Time      `json:"sightedAt"`
	LocationDescription string         `json:"locationDescription"`
	Coordinates         *LocationPoint `json:"coordinates"`
	MovementDirection   string         `json:"movementDirection,omitempty"`
}
