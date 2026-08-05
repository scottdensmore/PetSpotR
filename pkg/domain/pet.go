package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Pet represents a lost or reported pet in PetSpotR.
type Pet struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Breed      string    `json:"breed"`
	Images     []string  `json:"images"`
	State      string    `json:"state"`
	OwnerEmail string    `json:"ownerEmail"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Validate checks that mandatory pet fields are set.
func (p *Pet) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("pet ID cannot be empty")
	}
	if strings.TrimSpace(p.OwnerEmail) == "" {
		return errors.New("owner email cannot be empty")
	}
	return nil
}

// ToJSON serializes the Pet struct to JSON bytes.
func (p *Pet) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON deserializes JSON bytes into the Pet struct.
func (p *Pet) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}
