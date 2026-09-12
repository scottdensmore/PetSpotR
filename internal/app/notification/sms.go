package notification

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

const (
	DefaultSMSMaxLength = 160
	DefaultCountryCode  = "+1"
)

// SMSFormatterOption configures an SMSFormatter instance.
type SMSFormatterOption func(*SMSFormatter)

// WithSMSMaxLength sets a custom maximum character length for SMS messages.
func WithSMSMaxLength(maxLength int) SMSFormatterOption {
	return func(f *SMSFormatter) {
		if maxLength > 0 {
			f.maxLength = maxLength
		}
	}
}

// WithDefaultCountryCode sets the default country code used when normalizing local phone numbers.
func WithDefaultCountryCode(code string) SMSFormatterOption {
	return func(f *SMSFormatter) {
		if strings.TrimSpace(code) != "" {
			if !strings.HasPrefix(code, "+") {
				code = "+" + code
			}
			f.defaultCountryCode = code
		}
	}
}

// SMSFormatter formats phone numbers and SMS alert messages.
type SMSFormatter struct {
	maxLength          int
	defaultCountryCode string
}

// NewSMSFormatter constructs an SMSFormatter with optional configurations.
func NewSMSFormatter(opts ...SMSFormatterOption) *SMSFormatter {
	f := &SMSFormatter{
		maxLength:          DefaultSMSMaxLength,
		defaultCountryCode: DefaultCountryCode,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}
	return f
}

var nonDigitOrPlus = regexp.MustCompile(`[^0-9+]`)

// FormatPhoneNumber normalizes a raw phone number into standard E.164 format.
func (f *SMSFormatter) FormatPhoneNumber(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("phone number cannot be empty")
	}

	cleaned := nonDigitOrPlus.ReplaceAllString(trimmed, "")
	if cleaned == "" {
		return "", errors.New("phone number contains no valid digits")
	}

	if strings.HasPrefix(cleaned, "+") {
		digits := cleaned[1:]
		if strings.Contains(digits, "+") {
			return "", errors.New("phone number has invalid misplaced '+' sign")
		}
		if len(digits) < 10 || len(digits) > 15 {
			return "", fmt.Errorf("E.164 phone number length invalid: expected 10-15 digits, got %d", len(digits))
		}
		for _, r := range digits {
			if !unicode.IsDigit(r) {
				return "", fmt.Errorf("phone number contains invalid character %q", r)
			}
		}
		return cleaned, nil
	}

	for _, r := range cleaned {
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("phone number contains invalid character %q", r)
		}
	}

	// If 11 digits starting with 1 and country code is +1
	if len(cleaned) == 11 && cleaned[0] == '1' && f.defaultCountryCode == "+1" {
		return "+" + cleaned, nil
	}

	// If standard 10-digit North American number
	if len(cleaned) == 10 {
		return f.defaultCountryCode + cleaned, nil
	}

	// If between 10 and 15 digits, prefix default country code if not already matching
	if len(cleaned) >= 10 && len(cleaned) <= 15 {
		return "+" + cleaned, nil
	}

	return "", fmt.Errorf("unrecognized phone number format with %d digits", len(cleaned))
}

// FormatMatchAlert formats a concise match alert SMS adhering to maximum length constraints.
func (f *SMSFormatter) FormatMatchAlert(data MatchAlertEmailData) (string, error) {
	petName := strings.TrimSpace(data.PetName)
	if petName == "" {
		petName = strings.TrimSpace(data.PetID)
	}
	if petName == "" {
		petName = "Your Pet"
	}

	confidence := data.ConfidencePercent
	if confidence == 0 && data.Score > 0 {
		confidence = int(math.Round(data.Score * 100))
	}

	matchID := strings.TrimSpace(data.MatchID)
	if matchID == "" {
		return "", errors.New("match ID is required for SMS formatting")
	}

	// Structure:
	// PetSpotR Alert: Match found for <pet> (<confidence>% confidence). Match: <matchID>. Details: <explanation>
	prefix := fmt.Sprintf("PetSpotR: Match found for %s (%d%% match). Match: %s", petName, confidence, matchID)

	explanation := strings.TrimSpace(data.Explanation)
	if explanation == "" {
		return f.ConstrainLength(prefix), nil
	}

	full := fmt.Sprintf("%s. Details: %s", prefix, explanation)
	if len(full) <= f.maxLength {
		return full, nil
	}

	// If prefix itself exceeds or is close to maxLength, constrain prefix
	availableForExplanation := f.maxLength - len(prefix) - len(". Details: ")
	if availableForExplanation <= 3 {
		return f.ConstrainLength(prefix), nil
	}

	truncatedExplanation := explanation[:availableForExplanation-3] + "..."
	return fmt.Sprintf("%s. Details: %s", prefix, truncatedExplanation), nil
}

// ConstrainLength ensures any text fits within the configured maximum SMS length.
func (f *SMSFormatter) ConstrainLength(text string) string {
	if len(text) <= f.maxLength {
		return text
	}
	if f.maxLength <= 3 {
		return text[:f.maxLength]
	}
	return text[:f.maxLength-3] + "..."
}
