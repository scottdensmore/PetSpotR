package scoring

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var jsonBlockRegex = regexp.MustCompile(`(?s)\{.*?\}`)

// ParseGemmaResponse parses raw model output from Gemma 2 into a PetTraits struct.
// It handles raw JSON, markdown-fenced JSON, regex extraction, and unformatted line fallbacks.
func ParseGemmaResponse(rawResponse string) (*PetTraits, error) {
	trimmed := strings.TrimSpace(rawResponse)
	if trimmed == "" {
		return nil, errors.New("scoring: empty response from model")
	}

	// 1. Clean markdown fences
	cleanText := stripMarkdownFences(trimmed)

	// 2. Direct JSON unmarshaling into fresh struct
	var traits PetTraits
	if err := json.Unmarshal([]byte(cleanText), &traits); err == nil {
		if hasAnyTrait(&traits) {
			return sanitizeTraits(&traits), nil
		}
		return nil, errors.New("scoring: json contains no valid trait values")
	}

	// 3. Extract JSON object via non-greedy regex match
	matches := jsonBlockRegex.FindAllString(trimmed, -1)
	for _, match := range matches {
		var regexTraits PetTraits
		if err := json.Unmarshal([]byte(match), &regexTraits); err == nil && hasAnyTrait(&regexTraits) {
			return sanitizeTraits(&regexTraits), nil
		}
	}

	// If input appears to be structured JSON but parsing traits failed, return error
	if strings.HasPrefix(cleanText, "{") && strings.HasSuffix(cleanText, "}") {
		return nil, errors.New("scoring: invalid or empty json trait object")
	}

	// 4. Fallback parser for unformatted text
	return parseUnformattedText(trimmed)
}

func stripMarkdownFences(input string) string {
	lines := strings.Split(input, "\n")
	var out []string
	inFence := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmedLine, "```") {
			out = append(out, line)
		}
	}

	result := strings.TrimSpace(strings.Join(out, "\n"))
	if result == "" {
		return input
	}
	return result
}

func hasAnyTrait(t *PetTraits) bool {
	if t == nil {
		return false
	}
	if strings.TrimSpace(t.Breed) != "" ||
		strings.TrimSpace(t.PrimaryColor) != "" ||
		strings.TrimSpace(t.SecondaryColor) != "" ||
		strings.TrimSpace(t.EyeColor) != "" {
		return true
	}
	for _, m := range t.DistinctiveMarkings {
		if strings.TrimSpace(m) != "" {
			return true
		}
	}
	return false
}

func sanitizeTraits(t *PetTraits) *PetTraits {
	if t == nil {
		return nil
	}
	t.Breed = strings.TrimSpace(t.Breed)
	t.PrimaryColor = strings.TrimSpace(t.PrimaryColor)
	t.SecondaryColor = strings.TrimSpace(t.SecondaryColor)
	t.EyeColor = strings.TrimSpace(t.EyeColor)

	var cleanMarkings []string
	for _, m := range t.DistinctiveMarkings {
		mClean := strings.TrimSpace(m)
		if mClean != "" {
			cleanMarkings = append(cleanMarkings, mClean)
		}
	}
	t.DistinctiveMarkings = cleanMarkings
	return t
}

func parseUnformattedText(text string) (*PetTraits, error) {
	traits := &PetTraits{}
	lines := strings.Split(text, "\n")

	var currentCategory string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// Handle bullet point items under current Category (check BEFORE split)
		if (strings.HasPrefix(trimmedLine, "-") || strings.HasPrefix(trimmedLine, "*")) && currentCategory == "marking" {
			item := strings.Trim(trimmedLine, "-* ")
			if item != "" {
				traits.DistinctiveMarkings = append(traits.DistinctiveMarkings, item)
			}
			continue
		}

		parts := strings.SplitN(trimmedLine, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.Trim(strings.TrimSpace(parts[1]), " .,*\"'")

		switch {
		case strings.Contains(key, "breed"):
			traits.Breed = val
			currentCategory = "breed"
		case strings.Contains(key, "primary"):
			traits.PrimaryColor = val
			currentCategory = "primary"
		case strings.Contains(key, "secondary"):
			traits.SecondaryColor = val
			currentCategory = "secondary"
		case key == "color":
			if traits.PrimaryColor == "" {
				traits.PrimaryColor = val
			}
			currentCategory = "color"
		case strings.Contains(key, "marking"):
			if val != "" {
				traits.DistinctiveMarkings = append(traits.DistinctiveMarkings, val)
			}
			currentCategory = "marking"
		case strings.Contains(key, "eye"):
			traits.EyeColor = val
			currentCategory = "eye"
		}
	}

	if !hasAnyTrait(traits) {
		return nil, errors.New("scoring: unable to parse traits from model response")
	}

	return sanitizeTraits(traits), nil
}
