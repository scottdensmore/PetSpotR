package microchip

import (
	"fmt"
	"strings"
	"unicode"
)

// Standard represents the recognized microchip RFID transponder format.
type Standard string

const (
	StandardUnknown Standard = "unknown"
	StandardISO15   Standard = "iso_15"
	StandardAvid9   Standard = "avid_9"
	StandardEuro10  Standard = "euro_10"
)

// ValidationResult represents the validation and normalization outcome for a microchip.
type ValidationResult struct {
	Valid        bool     `json:"valid"`
	Standard     Standard `json:"standard"`
	NormalizedID string   `json:"normalizedId,omitempty"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
}

// ValidateAndNormalize validates a raw microchip transponder string and produces
// a canonical normalized identifier conforming to ISO 11784/11785 (15 digits),
// Avid (9 digits), or Euro/Trovan (10 hex characters).
func ValidateAndNormalize(raw string) ValidationResult {
	// Trim whitespace, hyphens, and asterisks
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' || r == '*' {
			return -1
		}
		return r
	}, raw)

	if clean == "" {
		return ValidationResult{
			Valid:        false,
			Standard:     StandardUnknown,
			ErrorMessage: "microchip number is required",
		}
	}

	switch len(clean) {
	case 15:
		if !isAllDigits(clean) {
			return ValidationResult{
				Valid:        false,
				Standard:     StandardUnknown,
				ErrorMessage: "invalid ISO microchip: must be 15 decimal digits",
			}
		}
		return ValidationResult{
			Valid:        true,
			Standard:     StandardISO15,
			NormalizedID: clean,
		}

	case 9:
		if !isAllDigits(clean) {
			return ValidationResult{
				Valid:        false,
				Standard:     StandardUnknown,
				ErrorMessage: "invalid Avid microchip: must be 9 decimal digits",
			}
		}
		return ValidationResult{
			Valid:        true,
			Standard:     StandardAvid9,
			NormalizedID: clean,
		}

	case 10:
		if !isAllHex(clean) {
			return ValidationResult{
				Valid:        false,
				Standard:     StandardUnknown,
				ErrorMessage: "invalid Euro microchip: must be 10 hexadecimal characters",
			}
		}
		return ValidationResult{
			Valid:        true,
			Standard:     StandardEuro10,
			NormalizedID: strings.ToUpper(clean),
		}

	default:
		return ValidationResult{
			Valid:        false,
			Standard:     StandardUnknown,
			ErrorMessage: fmt.Sprintf("invalid microchip length: expected 9, 10, or 15 characters, got %d", len(clean)),
		}
	}
}

// Normalize returns the canonical normalized microchip transponder string if valid,
// or an empty string if invalid or empty.
func Normalize(raw string) string {
	val := ValidateAndNormalize(raw)
	if !val.Valid {
		return ""
	}
	return val.NormalizedID
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isAllHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
