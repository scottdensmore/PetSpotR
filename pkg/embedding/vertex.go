package embedding

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexAIEmbedder calls Google Cloud Vertex AI Multimodal Embedding API.
type VertexAIEmbedder struct {
	projectID   string
	location    string
	httpClient  *http.Client
	tokenSource oauth2.TokenSource
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

// SetTokenSource overrides the OAuth2 token source (primarily for testing or custom credentials).
func (v *VertexAIEmbedder) SetTokenSource(ts oauth2.TokenSource) {
	v.tokenSource = ts
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
	if len(imageBytes) == 0 && text == "" {
		return nil, fmt.Errorf("vertex embedding: image or text is required")
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

	// Authenticate request using TokenSource or Application Default Credentials (ADC)
	var authToken string
	if v.tokenSource != nil {
		tok, tokErr := v.tokenSource.Token()
		if tokErr != nil {
			return nil, fmt.Errorf("vertex embedding: failed to obtain token from token source: %w", tokErr)
		}
		authToken = tok.AccessToken
	} else {
		ts, adcErr := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if adcErr == nil && ts != nil {
			tok, tokErr := ts.Token()
			if tokErr != nil {
				return nil, fmt.Errorf("vertex embedding: failed to obtain ADC token: %w", tokErr)
			}
			authToken = tok.AccessToken
		} else if v.endpointURL == "" {
			return nil, fmt.Errorf("vertex embedding: application default credentials not found: %w", adcErr)
		}
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

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

	pred := result.Predictions[0]
	hasImg := len(pred.ImageEmbedding) > 0
	hasTxt := len(pred.TextEmbedding) > 0

	// When both modalities are returned, fuse them via element-wise average and L2 unit-normalize
	if hasImg && hasTxt {
		if len(pred.ImageEmbedding) != v.dimension || len(pred.TextEmbedding) != v.dimension {
			return nil, fmt.Errorf("vertex embedding: dimension mismatch (img=%d, txt=%d, expected=%d)",
				len(pred.ImageEmbedding), len(pred.TextEmbedding), v.dimension)
		}
		combined := make([]float32, v.dimension)
		var sumSq float64
		for i := 0; i < v.dimension; i++ {
			val := (pred.ImageEmbedding[i] + pred.TextEmbedding[i]) / 2.0
			combined[i] = val
			sumSq += float64(val) * float64(val)
		}
		norm := float32(math.Sqrt(sumSq))
		if norm > 0 {
			for i := 0; i < v.dimension; i++ {
				combined[i] /= norm
			}
		}
		return combined, nil
	}
	if hasImg {
		if len(pred.ImageEmbedding) != v.dimension {
			return nil, fmt.Errorf("vertex embedding: unexpected image embedding dimension %d (expected %d)",
				len(pred.ImageEmbedding), v.dimension)
		}
		return pred.ImageEmbedding, nil
	}
	if hasTxt {
		if len(pred.TextEmbedding) != v.dimension {
			return nil, fmt.Errorf("vertex embedding: unexpected text embedding dimension %d (expected %d)",
				len(pred.TextEmbedding), v.dimension)
		}
		return pred.TextEmbedding, nil
	}
	return nil, fmt.Errorf("vertex embedding api returned empty embeddings")
}
