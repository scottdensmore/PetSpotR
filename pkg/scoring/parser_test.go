package scoring_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/scoring"
)

func TestBuildGemmaPrompt(t *testing.T) {
	t.Run("with type and breed hint", func(t *testing.T) {
		prompt := scoring.BuildGemmaPrompt(" Dog ", " Golden Retriever ")
		if !strings.Contains(prompt, "(Dog, hinted breed: Golden Retriever)") {
			t.Errorf("prompt expected trimmed strings: %s", prompt)
		}
	})

	t.Run("empty inputs fallback to defaults", func(t *testing.T) {
		prompt := scoring.BuildGemmaPrompt("", "")
		if !strings.Contains(prompt, "(pet, hinted breed: unknown)") {
			t.Errorf("prompt expected default fallbacks: %s", prompt)
		}
	})
}

func TestParseGemmaResponse(t *testing.T) {
	t.Run("valid JSON response", func(t *testing.T) {
		raw := `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"White","distinctiveMarkings":["White patch on chest"],"eyeColor":"Brown"}`
		traits, err := scoring.ParseGemmaResponse(raw)
		if err != nil {
			t.Fatalf("expected valid traits, got error: %v", err)
		}

		if traits.Breed != "Golden Retriever" || traits.PrimaryColor != "Golden" {
			t.Errorf("got traits %+v, mismatch", traits)
		}
	})

	t.Run("markdown fenced JSON response", func(t *testing.T) {
		raw := "```json\n{\"breed\":\"Siamese\",\"primaryColor\":\"Cream\",\"secondaryColor\":\"Dark Brown\",\"distinctiveMarkings\":[\"Point markings on ears and paws\"],\"eyeColor\":\"Blue\"}\n```"
		traits, err := scoring.ParseGemmaResponse(raw)
		if err != nil {
			t.Fatalf("expected valid traits from markdown fenced JSON, got: %v", err)
		}

		if traits.Breed != "Siamese" || traits.EyeColor != "Blue" {
			t.Errorf("got traits %+v, mismatch", traits)
		}
	})

	t.Run("extraneous braces surrounding JSON block", func(t *testing.T) {
		raw := `Context: {id: 123}` + "\n" + `{"breed":"St. Bernard","primaryColor":"Brown"}` + "\n" + `Note: {done}`
		traits, err := scoring.ParseGemmaResponse(raw)
		if err != nil {
			t.Fatalf("expected extraction despite surrounding braces: %v", err)
		}

		if traits.Breed != "St. Bernard" {
			t.Errorf("got breed %s, want St. Bernard", traits.Breed)
		}
	})

	t.Run("whitespace-only fields evaluate as empty", func(t *testing.T) {
		raw := `{"breed":"   ","primaryColor":"   "}`
		_, err := scoring.ParseGemmaResponse(raw)
		if err == nil {
			t.Fatal("expected error for whitespace-only fields, got nil")
		}
	})

	t.Run("multiline bullet points and period abbreviations", func(t *testing.T) {
		raw := "Breed: St. Bernard\nPrimary Color: Brown\nMarkings:\n- White patch on chest\n- Black spots on ear\n"
		traits, err := scoring.ParseGemmaResponse(raw)
		if err != nil {
			t.Fatalf("expected multiline bullet parsing, got: %v", err)
		}

		if traits.Breed != "St. Bernard" {
			t.Errorf("got breed %s, want St. Bernard", traits.Breed)
		}
		if len(traits.DistinctiveMarkings) != 2 {
			t.Errorf("expected 2 markings, got %v", traits.DistinctiveMarkings)
		}
	})

	t.Run("empty response returns error", func(t *testing.T) {
		_, err := scoring.ParseGemmaResponse("")
		if err == nil {
			t.Fatal("expected error for empty response, got nil")
		}
	})
}
