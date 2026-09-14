package embedding

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VertexAIEmbedder calls Google Cloud Vertex AI Multimodal Embedding API.
type VertexAIEmbedder struct {
	projectID   string
	location    string
	httpClient  *http.Client
	dimension   int
	endpointURL string
}

// NewVertexAIEmbedder creates a Vertex AI embedder instance.
func NewVertexAIEmbedder(projectID, location string) *VertexAIEmbedder {
	if location == "" {
		location = "us-central1"
	}
	return &VertexAIEmbedder{
		projectID:  projectID,
		location:   location,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		dimension:  768,
	}
}

// SetEndpointURL overrides the API endpoint URL (primarily for testing).
func (v *VertexAIEmbedder) SetEndpointURL(url string) {
	v.endpointURL = url
}

// SetHTTPClient overrides the HTTP client (primarily for testing).
func (v *VertexAIEmbedder) SetHTTPClient(client *http.Client) {
	v.httpClient = client
}

// Dimension returns the vector dimensionality (768).
func (v *VertexAIEmbedder) Dimension() int {
	return v.dimension
}

// EmbedImage computes an embedding vector from raw image bytes and MIME type.
func (v *VertexAIEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	return v.EmbedMultimodal(ctx, imageBytes, mimeType, "")
}

// EmbedText computes an embedding vector from descriptive text.
func (v *VertexAIEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return v.EmbedMultimodal(ctx, nil, "", text)
}

// EmbedMultimodal combines image bytes and contextual text into a single normalized vector.
func (v *VertexAIEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	url := v.endpointURL
	if url == "" {
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/multimodalembedding@001:predict",
			v.location, v.projectID, v.location)
	}

	instance := map[string]any{}
	if text != "" {
		instance["text"] = text
	}
	if len(imageBytes) > 0 {
		instance["image"] = map[string]string{
			"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imageBytes),
		}
	}
	body, err := json.Marshal(map[string]any{"instances": []any{instance}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := v.httpClient
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
		return nil, fmt.Errorf("vertex embedding api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Predictions []struct {
			ImageEmbedding []float32 `json:"imageEmbedding"`
			TextEmbedding  []float32 `json:"textEmbedding"`
		} `json:"predictions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Predictions) == 0 {
		return nil, fmt.Errorf("vertex embedding api returned no predictions")
	}
	if len(result.Predictions[0].ImageEmbedding) > 0 {
		return result.Predictions[0].ImageEmbedding, nil
	}
	if len(result.Predictions[0].TextEmbedding) > 0 {
		return result.Predictions[0].TextEmbedding, nil
	}
	return nil, fmt.Errorf("vertex embedding api returned empty embeddings")
}
