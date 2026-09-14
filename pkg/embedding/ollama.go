package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// OllamaEmbedder calls Ollama embedding API for private GPU Cloud Run deployments.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

// NewOllamaEmbedder creates an Ollama embedder instance.
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		dimension:  768,
	}
}

// SetHTTPClient overrides the HTTP client (primarily for testing).
func (o *OllamaEmbedder) SetHTTPClient(client *http.Client) {
	o.httpClient = client
}

// Dimension returns the vector dimensionality (768).
func (o *OllamaEmbedder) Dimension() int {
	return o.dimension
}

// EmbedImage computes an embedding vector from raw image bytes and MIME type.
// Note: Ollama text embedding models do not support direct image inputs.
func (o *OllamaEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	return o.EmbedMultimodal(ctx, imageBytes, mimeType, "")
}

// EmbedText computes an embedding vector from descriptive text.
func (o *OllamaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return o.EmbedMultimodal(ctx, nil, "", text)
}

// EmbedMultimodal combines image bytes and contextual text into a single normalized vector.
func (o *OllamaEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(imageBytes) > 0 && text == "" {
		return nil, errors.New("embedding: ollama model does not support direct image embedding; text description required")
	}
	if text == "" {
		return nil, errors.New("embedding: text description required for ollama embeddings")
	}

	url := fmt.Sprintf("%s/api/embeddings", o.baseURL)
	payload := map[string]any{
		"model":  o.model,
		"prompt": text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := o.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedding error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Embedding) != o.dimension {
		return nil, fmt.Errorf("ollama embedding: unexpected dimension %d (expected %d)", len(result.Embedding), o.dimension)
	}

	var sumSq float64
	for _, x := range result.Embedding {
		sumSq += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sumSq))
	if norm <= 0 {
		return nil, errors.New("ollama embedding: vector norm is zero")
	}
	for i := range result.Embedding {
		result.Embedding[i] /= norm
	}

	return result.Embedding, nil
}
