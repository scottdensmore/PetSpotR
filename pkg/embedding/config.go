package embedding

import (
	"fmt"
	"strings"
)

// Config configures the multimodal embedder.
type Config struct {
	Provider    string // "mock", "vertex", "ollama"
	Dimension   int
	ProjectID   string
	Location    string
	OllamaURL   string
	OllamaModel string
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
		return NewVertexAIEmbedder(cfg.ProjectID, cfg.Location), nil
	case "ollama":
		return NewOllamaEmbedder(cfg.OllamaURL, cfg.OllamaModel), nil
	default:
		return nil, fmt.Errorf("embedding: unknown provider %q", cfg.Provider)
	}
}
