package scoring_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/scoring"
)

func TestScoringPipeline_MockOllamaIntegration(t *testing.T) {
	// Mock Ollama server returning structured Gemma 2 vision analysis
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req ollama.GenerateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := ollama.GenerateResponse{
			Model: "gemma2:2b",
			Response: "```json\n{\n" +
				"  \"breed\": \"Golden Retriever\",\n" +
				"  \"primaryColor\": \"Golden\",\n" +
				"  \"secondaryColor\": \"Cream\",\n" +
				"  \"distinctiveMarkings\": [\"White patch on chest\", \"Fluffy tail\"],\n" +
				"  \"eyeColor\": \"Brown\"\n" +
				"}\n```",
			Done: true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL))
	ctx := context.Background()

	// 1. Build prompt
	prompt := scoring.BuildGemmaPrompt("Dog", "Golden Retriever")

	// 2. Query Ollama model
	genResp, err := client.Generate(ctx, &ollama.GenerateRequest{
		Model:  "gemma2:2b",
		Prompt: prompt,
		Images: []string{"base64-image-bytes"},
	})
	if err != nil {
		t.Fatalf("Ollama generation failed: %v", err)
	}

	// 3. Parse Gemma 2 output
	foundTraits, err := scoring.ParseGemmaResponse(genResp.Response)
	if err != nil {
		t.Fatalf("ParseGemmaResponse failed: %v", err)
	}

	// 4. Compare with reported lost pet traits
	lostTraits := &scoring.PetTraits{
		Breed:               "Golden Retriever",
		PrimaryColor:        "Golden",
		SecondaryColor:      "Cream",
		DistinctiveMarkings: []string{"White patch on chest"},
		EyeColor:            "Brown",
	}

	matchResult := scoring.ComparePets("lost-pet-101", "found-pet-202", lostTraits, foundTraits)
	if matchResult == nil {
		t.Fatal("expected non-nil MatchResult")
	}

	if !matchResult.IsMatch {
		t.Errorf("expected IsMatch true, got false (Score: %f)", matchResult.Score)
	}

	if matchResult.Score < 0.80 {
		t.Errorf("expected high match score >= 0.80, got %f", matchResult.Score)
	}
}

func TestScoringPipeline_LiveOllamaIntegration(t *testing.T) {
	if os.Getenv("GO_INTEGRATION_OLLAMA") != "1" {
		t.Skip("Skipping live Ollama integration test (set GO_INTEGRATION_OLLAMA=1 to run)")
	}

	client := ollama.NewClient()
	ctx := context.Background()

	prompt := scoring.BuildGemmaPrompt("Dog", "Labrador")
	genResp, err := client.Generate(ctx, &ollama.GenerateRequest{
		Model:  "gemma2:2b",
		Prompt: prompt,
	})
	if err != nil {
		t.Fatalf("Live Ollama generation failed: %v", err)
	}

	traits, err := scoring.ParseGemmaResponse(genResp.Response)
	if err != nil {
		t.Fatalf("Live ParseGemmaResponse failed: %v", err)
	}

	if traits == nil {
		t.Fatal("expected non-nil traits from live Gemma 2 model")
	}
}
