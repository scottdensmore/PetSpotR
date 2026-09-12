package notification

import (
	"strings"
	"testing"
)

func TestSMSFormatter_FormatPhoneNumber(t *testing.T) {
	formatter := NewSMSFormatter()

	tests := []struct {
		name      string
		input     string
		want      string
		expectErr bool
	}{
		{
			name:  "already in standard E.164",
			input: "+12065550199",
			want:  "+12065550199",
		},
		{
			name:  "US local with dashes and parentheses",
			input: "(206) 555-0199",
			want:  "+12065550199",
		},
		{
			name:  "US local with dots",
			input: "206.555.0199",
			want:  "+12065550199",
		},
		{
			name:  "US 11 digits starting with 1",
			input: "12065550199",
			want:  "+12065550199",
		},
		{
			name:  "international with spaces",
			input: "+44 7911 123456",
			want:  "+447911123456",
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			expectErr: true,
		},
		{
			name:      "non-digit characters only",
			input:     "abc-def-ghij",
			expectErr: true,
		},
		{
			name:      "too short",
			input:     "12345",
			expectErr: true,
		},
		{
			name:      "too long",
			input:     "+12345678901234567",
			expectErr: true,
		},
		{
			name:      "multiple plus signs",
			input:     "++12065550199",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := formatter.FormatPhoneNumber(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("FormatPhoneNumber(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSMSFormatter_FormatMatchAlert(t *testing.T) {
	formatter := NewSMSFormatter()

	baseData := MatchAlertEmailData{
		MatchID:           "match_98765",
		PetID:             "lost-101",
		PetName:           "Buddy",
		PhotoURL:          "https://storage.petspotr.io/buddy.jpg",
		Score:             0.95,
		ConfidencePercent: 95,
		Explanation:       "Visual score: 0.96, Spatial score: 0.94",
	}

	t.Run("formats standard match alert within 160 characters", func(t *testing.T) {
		msg, err := formatter.FormatMatchAlert(baseData)
		if err != nil {
			t.Fatalf("FormatMatchAlert failed: %v", err)
		}

		if len(msg) > DefaultSMSMaxLength {
			t.Errorf("message length %d exceeds max length %d: %q", len(msg), DefaultSMSMaxLength, msg)
		}

		if !strings.Contains(msg, "Buddy") {
			t.Errorf("message does not contain pet name: %q", msg)
		}
		if !strings.Contains(msg, "95%") {
			t.Errorf("message does not contain confidence percent: %q", msg)
		}
		if !strings.Contains(msg, "match_98765") {
			t.Errorf("message does not contain match ID: %q", msg)
		}
	})

	t.Run("strictly enforces 160 character limit on very long explanations", func(t *testing.T) {
		longData := baseData
		longData.Explanation = strings.Repeat("Visual score is extremely high based on distinctive golden retriever fur patterns and brown collar found at Green Lake Park in Seattle WA. ", 5)

		msg, err := formatter.FormatMatchAlert(longData)
		if err != nil {
			t.Fatalf("FormatMatchAlert failed: %v", err)
		}

		if len(msg) > DefaultSMSMaxLength {
			t.Errorf("message length %d exceeds max %d for long text: %q", len(msg), DefaultSMSMaxLength, msg)
		}

		if !strings.HasSuffix(msg, "...") {
			t.Errorf("expected truncated message to end with '...': %q", msg)
		}

		// Ensure essential info is preserved even with truncation
		if !strings.Contains(msg, "Buddy") || !strings.Contains(msg, "match_98765") {
			t.Errorf("essential information was lost in truncation: %q", msg)
		}
	})

	t.Run("supports custom configured max length", func(t *testing.T) {
		customMax := 80
		customFormatter := NewSMSFormatter(WithSMSMaxLength(customMax))

		msg, err := customFormatter.FormatMatchAlert(baseData)
		if err != nil {
			t.Fatalf("FormatMatchAlert failed: %v", err)
		}

		if len(msg) > customMax {
			t.Errorf("message length %d exceeds custom max length %d: %q", len(msg), customMax, msg)
		}
	})

	t.Run("requires match ID", func(t *testing.T) {
		invalidData := baseData
		invalidData.MatchID = ""

		_, err := formatter.FormatMatchAlert(invalidData)
		if err == nil {
			t.Fatal("expected error when match ID is empty, got nil")
		}
	})
}
