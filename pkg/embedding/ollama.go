package embedding

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

var _ Embedder = (*OllamaEmbedder)(nil)

// OllamaEmbedder calls Ollama embedding API for private GPU Cloud Run deployments.
type OllamaEmbedder struct {
	baseURL     string
	model       string
	visionModel string
	httpClient  *http.Client
	dimension   int
}

// NewOllamaEmbedder creates an Ollama embedder instance.
// If visionModel is provided and non-empty, it is used for image embeddings;
// otherwise it defaults to model or fallback ("nomic-embed-text").
func NewOllamaEmbedder(baseURL, model string, visionModel ...string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	vm := model
	if len(visionModel) > 0 && strings.TrimSpace(visionModel[0]) != "" {
		vm = strings.TrimSpace(visionModel[0])
	}
	return &OllamaEmbedder{
		baseURL:     baseURL,
		model:       model,
		visionModel: vm,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		dimension:   768,
	}
}

// SetVisionModel sets the vision model used for image embeddings.
func (o *OllamaEmbedder) SetVisionModel(model string) {
	if strings.TrimSpace(model) != "" {
		o.visionModel = strings.TrimSpace(model)
	}
}

// VisionModel returns the configured vision model.
func (o *OllamaEmbedder) VisionModel() string {
	if o.visionModel != "" {
		return o.visionModel
	}
	if o.model != "" {
		return o.model
	}
	return "nomic-embed-text"
}

// Model returns the configured text model.
func (o *OllamaEmbedder) Model() string {
	if o.model != "" {
		return o.model
	}
	return "nomic-embed-text"
}

// SetHTTPClient overrides the HTTP client (primarily for testing).
func (o *OllamaEmbedder) SetHTTPClient(client *http.Client) {
	o.httpClient = client
}

// Dimension returns the vector dimensionality (768).
func (o *OllamaEmbedder) Dimension() int {
	if o.dimension <= 0 {
		return 768
	}
	return o.dimension
}

// EmbedImage computes an embedding vector from raw image bytes and MIME type.
// If image bytes are present, they are base64 encoded and sent to the Ollama API
// with the configured vision model.
func (o *OllamaEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(imageBytes) == 0 {
		return nil, errors.New("embedding: image bytes cannot be empty")
	}
	return o.EmbedMultimodal(ctx, imageBytes, mimeType, "")
}

// EmbedText computes an embedding vector from descriptive text.
func (o *OllamaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if text == "" {
		return nil, errors.New("embedding: text description required for ollama embeddings")
	}
	model := o.model
	if model == "" {
		model = "nomic-embed-text"
	}
	return o.embed(ctx, model, text, nil)
}

// EmbedMultimodal combines image bytes and contextual text into a single normalized vector.
// If image bytes are present, both base64-encoded image and text prompt are sent.
// If only text is present, EmbedText is used.
func (o *OllamaEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(imageBytes) == 0 && text == "" {
		return nil, errors.New("embedding: text description or image required for ollama embeddings")
	}
	if len(imageBytes) == 0 {
		return o.EmbedText(ctx, text)
	}

	model := o.visionModel
	if model == "" {
		model = o.model
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	b64Image := base64.StdEncoding.EncodeToString(imageBytes)
	return o.embed(ctx, model, text, []string{b64Image})
}

func (o *OllamaEmbedder) embed(ctx context.Context, model, prompt string, images []string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/embeddings", o.baseURL)
	payload := map[string]any{
		"model":  model,
		"prompt": prompt,
	}
	if len(images) > 0 {
		payload["images"] = images
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
		return nil, fmt.Errorf("ollama embedding: failed to decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, errors.New("ollama embedding: empty embedding returned")
	}

	dim := o.Dimension()
	if len(result.Embedding) != dim {
		return nil, fmt.Errorf("ollama embedding: unexpected dimension %d (expected %d)", len(result.Embedding), dim)
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
