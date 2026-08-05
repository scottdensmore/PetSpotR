package scoring

import (
	"fmt"
	"strings"
)

// PetTraits defines the structured visual traits extracted from a pet image.
type PetTraits struct {
	Breed               string   `json:"breed"`
	PrimaryColor        string   `json:"primaryColor"`
	SecondaryColor      string   `json:"secondaryColor"`
	DistinctiveMarkings []string `json:"distinctiveMarkings"`
	EyeColor            string   `json:"eyeColor"`
}

// BuildGemmaPrompt builds a structured prompt instructing Gemma 2 to extract pet visual traits.
func BuildGemmaPrompt(petType, breedHint string) string {
	petType = strings.TrimSpace(petType)
	if petType == "" {
		petType = "pet"
	}
	breedHint = strings.TrimSpace(breedHint)
	if breedHint == "" {
		breedHint = "unknown"
	}

	return fmt.Sprintf(
		"Analyze this pet image (%s, hinted breed: %s). Extract visual characteristics and return ONLY a valid JSON object matching this schema:\n"+
			"{\n"+
			"  \"breed\": \"string\",\n"+
			"  \"primaryColor\": \"string\",\n"+
			"  \"secondaryColor\": \"string\",\n"+
			"  \"distinctiveMarkings\": [\"string\"],\n"+
			"  \"eyeColor\": \"string\"\n"+
			"}\n"+
			"Do not include any extra explanatory text.",
		petType, breedHint,
	)
}
