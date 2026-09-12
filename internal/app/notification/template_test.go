package notification

import (
	"strings"
	"testing"
)

func TestEmailTemplateRenderer_RenderMatchAlert(t *testing.T) {
	renderer := NewEmailTemplateRenderer()

	validData := MatchAlertEmailData{
		MatchID:           "match_abc123",
		PetID:             "lost-fluffy-42",
		PetName:           "Fluffy",
		PhotoURL:          "https://storage.petspotr.io/photos/found-fluffy.jpg",
		Score:             0.94,
		ConfidencePercent: 94,
		Explanation:       "Visual similarity: 96%, Color match: 92%, Distance: 0.8 mi",
		DetailsURL:        "https://petspotr.io/matches/match_abc123",
	}

	t.Run("renders HTML and plain text with all required fields", func(t *testing.T) {
		content, err := renderer.RenderMatchAlert(validData)
		if err != nil {
			t.Fatalf("RenderMatchAlert failed: %v", err)
		}

		// Verify subject
		if !strings.Contains(content.Subject, "Fluffy") {
			t.Errorf("Subject %q does not contain pet name", content.Subject)
		}

		// Verify HTML required fields
		html := content.HTMLBody
		if !strings.Contains(html, validData.MatchID) {
			t.Errorf("HTML does not contain match ID: %s", validData.MatchID)
		}
		if !strings.Contains(html, validData.PhotoURL) {
			t.Errorf("HTML does not contain pet photo link: %s", validData.PhotoURL)
		}
		if !strings.Contains(html, validData.Explanation) {
			t.Errorf("HTML does not contain score explanation: %s", validData.Explanation)
		}
		if !strings.Contains(html, "94%") {
			t.Errorf("HTML does not contain confidence percentage: 94%%")
		}
		if !strings.Contains(html, validData.DetailsURL) {
			t.Errorf("HTML does not contain details link: %s", validData.DetailsURL)
		}
		if !strings.Contains(html, "<img src=") {
			t.Errorf("HTML does not render an img tag for photo link")
		}

		// Verify Plain Text required fields
		text := content.TextBody
		if !strings.Contains(text, validData.MatchID) {
			t.Errorf("Text does not contain match ID: %s", validData.MatchID)
		}
		if !strings.Contains(text, validData.PhotoURL) {
			t.Errorf("Text does not contain pet photo link: %s", validData.PhotoURL)
		}
		if !strings.Contains(text, validData.Explanation) {
			t.Errorf("Text does not contain score explanation: %s", validData.Explanation)
		}
		if !strings.Contains(text, "94%") {
			t.Errorf("Text does not contain confidence percentage: 94%%")
		}
	})

	t.Run("validates required fields", func(t *testing.T) {
		tests := []struct {
			name        string
			data        MatchAlertEmailData
			wantErrPart string
		}{
			{
				name: "missing match ID",
				data: MatchAlertEmailData{
					PhotoURL:    "https://storage.petspotr.io/photo.jpg",
					Explanation: "High confidence match",
				},
				wantErrPart: "matchId is required",
			},
			{
				name: "missing photo link",
				data: MatchAlertEmailData{
					MatchID:     "match_123",
					Explanation: "High confidence match",
				},
				wantErrPart: "pet photo link is required",
			},
			{
				name: "missing explanation",
				data: MatchAlertEmailData{
					MatchID:  "match_123",
					PhotoURL: "https://storage.petspotr.io/photo.jpg",
				},
				wantErrPart: "score explanation is required",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := renderer.RenderMatchAlert(tc.data)
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Errorf("expected error containing %q, got %q", tc.wantErrPart, err.Error())
				}
			})
		}
	})

	t.Run("escapes HTML in user-supplied values to prevent XSS", func(t *testing.T) {
		maliciousData := validData
		maliciousData.PetName = "<script>alert('xss')</script>"
		maliciousData.Explanation = "<b>Dangerous bold</b> & 'quotes'"

		content, err := renderer.RenderMatchAlert(maliciousData)
		if err != nil {
			t.Fatalf("RenderMatchAlert failed: %v", err)
		}

		if strings.Contains(content.HTMLBody, "<script>") {
			t.Errorf("HTML contains unescaped script tag: %s", content.HTMLBody)
		}
		if !strings.Contains(content.HTMLBody, "&lt;script&gt;") {
			t.Errorf("HTML does not properly escape script tag")
		}
	})
}
