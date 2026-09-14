package embedding

import (
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// Config configures the multimodal embedder.
type Config struct {
	Provider    string // "mock", "vertex", "ollama"
	Dimension   int
	ProjectID   string
	Location    string
	OllamaURL   string
	OllamaModel string
	VisionModel string
	TokenSource oauth2.TokenSource // optional custom token source for vertex provider
}

// NewEmbedder constructs an Embedder according to configuration.
func NewEmbedder(cfg Config) (Embedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "mock", "":
		return NewMockEmbedder(), nil
	case "vertex":
		if cfg.ProjectID == "" {
			return nil, fmt.Errorf("embedding: projectID is required for vertex provider")
		}
		embedder := NewVertexAIEmbedder(cfg.ProjectID, cfg.Location)
		if cfg.TokenSource != nil {
			embedder.SetTokenSource(cfg.TokenSource)
		}
		return embedder, nil
	case "ollama":
		return NewOllamaEmbedder(cfg.OllamaURL, cfg.OllamaModel, cfg.VisionModel), nil
	default:
		return nil, fmt.Errorf("embedding: unknown provider %q", cfg.Provider)
	}
}
