package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestPet_Validation(t *testing.T) {
	t.Run("valid pet struct", func(t *testing.T) {
		p := domain.Pet{
			ID:         "pet-123",
			Name:       "Buddy",
			Type:       "Dog",
			Breed:      "Golden Retriever",
			Images:     []string{"image1.jpg", "image2.jpg"},
			State:      "Lost",
			OwnerEmail: "owner@example.com",
			CreatedAt:  time.Now(),
		}

		if err := p.Validate(); err != nil {
			t.Fatalf("expected valid pet, got error: %v", err)
		}
	})

	t.Run("missing pet ID", func(t *testing.T) {
		p := domain.Pet{
			Name:       "Buddy",
			Type:       "Dog",
			Breed:      "Golden Retriever",
			OwnerEmail: "owner@example.com",
		}

		if err := p.Validate(); err == nil {
			t.Fatal("expected error for missing ID, got nil")
		}
	})

	t.Run("missing owner email", func(t *testing.T) {
		p := domain.Pet{
			ID:    "pet-123",
			Name:  "Buddy",
			Type:  "Dog",
			Breed: "Golden Retriever",
		}

		if err := p.Validate(); err == nil {
			t.Fatal("expected error for missing owner email, got nil")
		}
	})
}

func TestPet_JSONSerialization(t *testing.T) {
	p := domain.Pet{
		ID:         "pet-456",
		Name:       "Milo",
		Type:       "Cat",
		Breed:      "Siamese",
		Images:     []string{"milo.jpg"},
		State:      "Lost",
		OwnerEmail: "milo_owner@example.com",
		CreatedAt:  time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
	}

	data, err := p.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize pet: %v", err)
	}

	var deserialized domain.Pet
	if err := deserialized.FromJSON(data); err != nil {
		t.Fatalf("failed to deserialize pet: %v", err)
	}

	if deserialized.ID != p.ID || deserialized.Name != p.Name || deserialized.OwnerEmail != p.OwnerEmail {
		t.Errorf("deserialized pet mismatch: got %+v, want %+v", deserialized, p)
	}
}
