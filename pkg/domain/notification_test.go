package domain_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestOwnerNotification_Validation(t *testing.T) {
	t.Run("valid notification", func(t *testing.T) {
		n := domain.OwnerNotification{
			FromEmail:  "petspotr@petspotr.io",
			ToEmail:    "owner@example.com",
			Subject:    "🎉 New Match for Buddy!",
			Body:       "We found a potential match for Buddy.",
			PetName:    "Buddy",
			MatchScore: 0.92,
		}

		if err := n.Validate(); err != nil {
			t.Fatalf("expected valid notification, got error: %v", err)
		}
	})

	t.Run("missing recipient email", func(t *testing.T) {
		n := domain.OwnerNotification{
			FromEmail: "petspotr@petspotr.io",
			PetName:   "Buddy",
		}

		if err := n.Validate(); err == nil {
			t.Fatal("expected error for missing recipient email, got nil")
		}
	})
}

func TestOwnerNotification_RenderBody(t *testing.T) {
	t.Run("default template rendering", func(t *testing.T) {
		n := domain.OwnerNotification{
			FromEmail:  "petspotr@petspotr.io",
			ToEmail:    "owner@example.com",
			Subject:    "🎉 New Match for Max!",
			PetName:    "Max",
			MatchScore: 0.88,
		}

		body := n.RenderEmailBody()
		if !strings.Contains(body, "Max") {
			t.Errorf("rendered body expected to contain PetName 'Max', got: %s", body)
		}
		if !strings.Contains(body, "88%") {
			t.Errorf("rendered body expected to contain percentage '88%%', got: %s", body)
		}
	})

	t.Run("pre-set custom body", func(t *testing.T) {
		customBody := "<p>Custom notification content</p>"
		n := domain.OwnerNotification{
			ToEmail: "owner@example.com",
			Body:    customBody,
		}

		if got := n.RenderEmailBody(); got != customBody {
			t.Errorf("RenderEmailBody() = %q, want custom body %q", got, customBody)
		}
	})
}

func TestOwnerNotification_JSONSerialization(t *testing.T) {
	n := domain.OwnerNotification{
		FromEmail:  "petspotr@petspotr.io",
		ToEmail:    "owner@example.com",
		Subject:    "Match Alert",
		PetName:    "Charlie",
		MatchScore: 0.95,
	}

	data, err := n.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	var deserialized domain.OwnerNotification
	if err := deserialized.FromJSON(data); err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	if deserialized.ToEmail != n.ToEmail || deserialized.PetName != n.PetName {
		t.Errorf("deserialized mismatch: got %+v, want %+v", deserialized, n)
	}
}
