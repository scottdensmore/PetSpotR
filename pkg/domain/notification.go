package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// OwnerNotification represents an alert sent to a pet owner when a potential match is found.
type OwnerNotification struct {
	FromEmail  string  `json:"fromEmail"`
	ToEmail    string  `json:"toEmail"`
	Subject    string  `json:"subject"`
	Body       string  `json:"body"`
	PetName    string  `json:"petName"`
	MatchScore float64 `json:"matchScore"`
}

// Validate checks mandatory notification fields.
func (n *OwnerNotification) Validate() error {
	if strings.TrimSpace(n.ToEmail) == "" {
		return errors.New("recipient email cannot be empty")
	}
	return nil
}

// RenderEmailBody generates an HTML notification body.
func (n *OwnerNotification) RenderEmailBody() string {
	if strings.TrimSpace(n.Body) != "" {
		return n.Body
	}
	return fmt.Sprintf(
		"<h1>🎉 New Match Found for %s!</h1><p>We found a potential match with <strong>%d%%</strong> confidence score.</p>",
		n.PetName, MatchConfidencePercent(n.MatchScore),
	)
}

// MatchConfidencePercent converts a normalized match score to the nearest
// whole percentage for user-facing presentation.
func MatchConfidencePercent(score float64) int {
	return int(math.Round(score * 100))
}

// ToJSON serializes OwnerNotification to JSON.
func (n *OwnerNotification) ToJSON() ([]byte, error) {
	return json.Marshal(n)
}

// FromJSON deserializes JSON bytes into OwnerNotification.
func (n *OwnerNotification) FromJSON(data []byte) error {
	return json.Unmarshal(data, n)
}
