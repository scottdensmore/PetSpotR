package embedding_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/embedding"
	"golang.org/x/oauth2"
)

func TestMockEmbedder_Properties(t *testing.T) {
	ctx := context.Background()
	embedder := embedding.NewMockEmbedder()

	if embedder.Dimension() != 768 {
		t.Fatalf("expected dimension 768, got %d", embedder.Dimension())
	}

	t.Run("deterministic embeddings for identical text", func(t *testing.T) {
		v1, err := embedder.EmbedText(ctx, "Golden Retriever with white chest patch")
		if err != nil {
			t.Fatal(err)
		}
		v2, err := embedder.EmbedText(ctx, "Golden Retriever with white chest patch")
		if err != nil {
			t.Fatal(err)
		}
		if len(v1) != 768 || len(v2) != 768 {
			t.Fatalf("unexpected vector length: %d, %d", len(v1), len(v2))
		}
		for i := range v1 {
			if v1[i] != v2[i] {
				t.Fatalf("expected deterministic values at index %d: %f != %f", i, v1[i], v2[i])
			}
		}
	})

	t.Run("unit normalization", func(t *testing.T) {
		v, err := embedder.EmbedText(ctx, "Tabby cat green eyes")
		if err != nil {
			t.Fatal(err)
		}
		var sumSq float64
		for _, x := range v {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("distinct inputs produce distinct vectors", func(t *testing.T) {
		dog, err := embedder.EmbedText(ctx, "Golden retriever dog")
		if err != nil {
			t.Fatal(err)
		}
		cat, err := embedder.EmbedText(ctx, "Black domestic short hair cat")
		if err != nil {
			t.Fatal(err)
		}
		var dot float64
		for i := range dog {
			dot += float64(dog[i]) * float64(cat[i])
		}
		if dot >= 0.99 {
			t.Fatalf("expected distinct vectors to have lower cosine similarity, got %f", dot)
		}
	})

	t.Run("multimodal embedding combines image and text", func(t *testing.T) {
		dummyImage := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
		vec, err := embedder.EmbedMultimodal(ctx, dummyImage, "image/jpeg", "Beagle puppy")
		if err != nil {
			t.Fatal(err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768-dim vector, got %d", len(vec))
		}

		var sumSq float64
		for _, x := range vec {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("embed image only", func(t *testing.T) {
		dummyImage := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		vec1, err := embedder.EmbedImage(ctx, dummyImage, "image/png")
		if err != nil {
			t.Fatal(err)
		}
		vec2, err := embedder.EmbedImage(ctx, dummyImage, "image/png")
		if err != nil {
			t.Fatal(err)
		}
		if len(vec1) != 768 || len(vec2) != 768 {
			t.Fatalf("unexpected vector length: %d, %d", len(vec1), len(vec2))
		}
		for i := range vec1 {
			if vec1[i] != vec2[i] {
				t.Fatalf("expected deterministic values at index %d: %f != %f", i, vec1[i], vec2[i])
			}
		}
	})

	t.Run("context cancellation honored", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, err := embedder.EmbedText(canceledCtx, "test"); err == nil {
			t.Fatal("expected error on canceled context for EmbedText")
		}
		if _, err := embedder.EmbedImage(canceledCtx, []byte{1, 2, 3}, "image/jpeg"); err == nil {
			t.Fatal("expected error on canceled context for EmbedImage")
		}
		if _, err := embedder.EmbedMultimodal(canceledCtx, []byte{1, 2, 3}, "image/jpeg", "test"); err == nil {
			t.Fatal("expected error on canceled context for EmbedMultimodal")
		}
	})
}

func TestEmbedderFactory(t *testing.T) {
	t.Run("mock provider default", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider: "mock",
		})
		if err != nil {
			t.Fatalf("NewEmbedder error = %v", err)
		}
		if embedder.Dimension() != 768 {
			t.Fatalf("expected 768 dimension, got %d", embedder.Dimension())
		}
	})

	t.Run("empty provider defaults to mock", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{})
		if err != nil {
			t.Fatalf("NewEmbedder error = %v", err)
		}
		if embedder.Dimension() != 768 {
			t.Fatalf("expected 768 dimension, got %d", embedder.Dimension())
		}
	})

	t.Run("vertex provider requires projectID", func(t *testing.T) {
		_, err := embedding.NewEmbedder(embedding.Config{
			Provider: "vertex",
		})
		if err == nil {
			t.Fatal("expected error when projectID is missing for vertex provider")
		}
	})

	t.Run("vertex provider success with custom token source", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider:    "vertex",
			ProjectID:   "test-project",
			Location:    "us-central1",
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "mock-token"}),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if embedder.Dimension() != 768 {
			t.Fatalf("expected 768 dimension, got %d", embedder.Dimension())
		}
	})

	t.Run("ollama provider success", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider:    "ollama",
			OllamaURL:   "http://localhost:11434",
			OllamaModel: "nomic-embed-text",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if embedder.Dimension() != 768 {
			t.Fatalf("expected 768 dimension, got %d", embedder.Dimension())
		}
	})

	t.Run("ollama provider with vision model configured", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider:    "ollama",
			OllamaURL:   "http://localhost:11434",
			OllamaModel: "nomic-embed-text",
			VisionModel: "gemma4:e2b",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ollamaEmb, ok := embedder.(*embedding.OllamaEmbedder)
		if !ok {
			t.Fatalf("expected *OllamaEmbedder, got %T", embedder)
		}
		if ollamaEmb.VisionModel() != "gemma4:e2b" {
			t.Fatalf("expected vision model gemma4:e2b, got %s", ollamaEmb.VisionModel())
		}
		if ollamaEmb.Model() != "nomic-embed-text" {
			t.Fatalf("expected model nomic-embed-text, got %s", ollamaEmb.Model())
		}
	})

	t.Run("ollama provider without vision model defaults to model", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider:    "ollama",
			OllamaURL:   "http://localhost:11434",
			OllamaModel: "custom-text-model",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ollamaEmb, ok := embedder.(*embedding.OllamaEmbedder)
		if !ok {
			t.Fatalf("expected *OllamaEmbedder, got %T", embedder)
		}
		if ollamaEmb.VisionModel() != "custom-text-model" {
			t.Fatalf("expected vision model custom-text-model, got %s", ollamaEmb.VisionModel())
		}
	})

	t.Run("ollama provider with empty models defaults to fallback", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider: "ollama",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ollamaEmb, ok := embedder.(*embedding.OllamaEmbedder)
		if !ok {
			t.Fatalf("expected *OllamaEmbedder, got %T", embedder)
		}
		if ollamaEmb.VisionModel() != "nomic-embed-text" {
			t.Fatalf("expected fallback vision model nomic-embed-text, got %s", ollamaEmb.VisionModel())
		}
		if ollamaEmb.Model() != "nomic-embed-text" {
			t.Fatalf("expected fallback model nomic-embed-text, got %s", ollamaEmb.Model())
		}
	})

	t.Run("unknown provider returns error", func(t *testing.T) {
		_, err := embedding.NewEmbedder(embedding.Config{
			Provider: "unsupported-provider",
		})
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}

func TestVertexAIEmbedder(t *testing.T) {
	ctx := context.Background()
	testToken := "vertex-adc-test-token"
	mockTokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testToken})

	t.Run("successful text embedding asserting payload and auth header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
			}
			if r.Header.Get("Authorization") != "Bearer "+testToken {
				t.Errorf("expected Authorization Bearer %s, got %s", testToken, r.Header.Get("Authorization"))
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Instances []struct {
					Text  string `json:"text"`
					Image *struct {
						BytesBase64Encoded string `json:"bytesBase64Encoded"`
					} `json:"image"`
				} `json:"instances"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if len(reqPayload.Instances) != 1 {
				t.Fatalf("expected 1 instance, got %d", len(reqPayload.Instances))
			}
			if reqPayload.Instances[0].Text != "lost golden retriever" {
				t.Errorf("expected text 'lost golden retriever', got %s", reqPayload.Instances[0].Text)
			}
			if reqPayload.Instances[0].Image != nil {
				t.Errorf("expected nil image for text-only call")
			}

			resp := map[string]any{
				"predictions": []map[string]any{
					{
						"textEmbedding": make([]float32, 768),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		vec, err := v.EmbedText(ctx, "lost golden retriever")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}
	})

	t.Run("successful image embedding asserting payload and auth header", func(t *testing.T) {
		dummyBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		expectedB64 := base64.StdEncoding.EncodeToString(dummyBytes)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+testToken {
				t.Errorf("expected Authorization Bearer %s, got %s", testToken, r.Header.Get("Authorization"))
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Instances []struct {
					Text  string `json:"text"`
					Image *struct {
						BytesBase64Encoded string `json:"bytesBase64Encoded"`
					} `json:"image"`
				} `json:"instances"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if len(reqPayload.Instances) != 1 {
				t.Fatalf("expected 1 instance, got %d", len(reqPayload.Instances))
			}
			if reqPayload.Instances[0].Text != "" {
				t.Errorf("expected empty text for image-only call, got %s", reqPayload.Instances[0].Text)
			}
			if reqPayload.Instances[0].Image == nil || reqPayload.Instances[0].Image.BytesBase64Encoded != expectedB64 {
				t.Errorf("expected base64 encoded image %s, got %v", expectedB64, reqPayload.Instances[0].Image)
			}

			resp := map[string]any{
				"predictions": []map[string]any{
					{
						"imageEmbedding": make([]float32, 768),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		vec, err := v.EmbedImage(ctx, dummyBytes, "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}
	})

	t.Run("multimodal embedding fuses both image and text embeddings into unit vector", func(t *testing.T) {
		dummyBytes := []byte{1, 2, 3, 4}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			imgEmb := make([]float32, 768)
			txtEmb := make([]float32, 768)
			for i := 0; i < 768; i++ {
				imgEmb[i] = 1.0
				txtEmb[i] = 2.0
			}
			resp := map[string]any{
				"predictions": []map[string]any{
					{
						"imageEmbedding": imgEmb,
						"textEmbedding":  txtEmb,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		vec, err := v.EmbedMultimodal(ctx, dummyBytes, "image/png", "husky puppy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}

		// Verify combined vector is unit-normalized
		var sumSq float64
		for _, x := range vec {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("dimension mismatch returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"predictions": []map[string]any{
					{
						"textEmbedding": make([]float32, 512),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		_, err := v.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on dimension mismatch")
		}
	})

	t.Run("server error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		_, err := v.EmbedText(ctx, "lost cat")
		if err == nil {
			t.Fatal("expected error on 500 status")
		}
	})

	t.Run("empty predictions returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"predictions": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		v.SetEndpointURL(server.URL)
		v.SetHTTPClient(server.Client())
		v.SetTokenSource(mockTokenSource)

		_, err := v.EmbedText(ctx, "lost cat")
		if err == nil {
			t.Fatal("expected error on empty predictions")
		}
	})

	t.Run("context cancellation honored", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		if _, err := v.EmbedText(canceledCtx, "lost cat"); err == nil {
			t.Fatal("expected error on canceled context")
		}
	})

	t.Run("empty inputs returns error", func(t *testing.T) {
		v := embedding.NewVertexAIEmbedder("test-proj", "us-central1")
		if _, err := v.EmbedMultimodal(ctx, nil, "", ""); err == nil {
			t.Fatal("expected error when both image and text are empty")
		}
	})
}

func TestOllamaEmbedder(t *testing.T) {
	ctx := context.Background()

	t.Run("successful text embedding asserting payload and unit normalization", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/embeddings" {
				t.Errorf("expected /api/embeddings, got %s", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Model  string `json:"model"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if reqPayload.Model != "nomic-embed-text" {
				t.Errorf("expected model nomic-embed-text, got %s", reqPayload.Model)
			}
			if reqPayload.Prompt != "calico cat" {
				t.Errorf("expected prompt 'calico cat', got %s", reqPayload.Prompt)
			}

			// Return unnormalized raw vector (all components = 2.0)
			raw := make([]float32, 768)
			for i := range raw {
				raw[i] = 2.0
			}
			resp := map[string]any{
				"embedding": raw,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		if o.Dimension() != 768 {
			t.Fatalf("expected 768 dim, got %d", o.Dimension())
		}

		vec, err := o.EmbedText(ctx, "calico cat")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}

		// Verify output was L2 unit-normalized
		var sumSq float64
		for _, x := range vec {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("successful image embedding with configured vision model asserts payload and normalization", func(t *testing.T) {
		dummyImage := []byte{0x89, 0x50, 0x4E, 0x47, 0x01, 0x02, 0x03, 0x04}
		expectedB64 := base64.StdEncoding.EncodeToString(dummyImage)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/embeddings" {
				t.Errorf("expected /api/embeddings, got %s", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Model  string   `json:"model"`
				Prompt string   `json:"prompt"`
				Images []string `json:"images"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if reqPayload.Model != "gemma4:e2b" {
				t.Errorf("expected model gemma4:e2b, got %s", reqPayload.Model)
			}
			if len(reqPayload.Images) != 1 || reqPayload.Images[0] != expectedB64 {
				t.Errorf("expected base64 image %s, got %v", expectedB64, reqPayload.Images)
			}

			raw := make([]float32, 768)
			for i := range raw {
				raw[i] = 3.0
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": raw})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text", "gemma4:e2b")
		o.SetHTTPClient(server.Client())

		vec, err := o.EmbedImage(ctx, dummyImage, "image/png")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}

		var sumSq float64
		for _, x := range vec {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("successful multimodal embedding with image and text sends both and normalizes", func(t *testing.T) {
		dummyImage := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x05, 0x06, 0x07, 0x08}
		expectedB64 := base64.StdEncoding.EncodeToString(dummyImage)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Model  string   `json:"model"`
				Prompt string   `json:"prompt"`
				Images []string `json:"images"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if reqPayload.Model != "gemma4:e2b" {
				t.Errorf("expected vision model gemma4:e2b, got %s", reqPayload.Model)
			}
			if reqPayload.Prompt != "golden retriever puppy" {
				t.Errorf("expected prompt 'golden retriever puppy', got %s", reqPayload.Prompt)
			}
			if len(reqPayload.Images) != 1 || reqPayload.Images[0] != expectedB64 {
				t.Errorf("expected base64 image %s, got %v", expectedB64, reqPayload.Images)
			}

			raw := make([]float32, 768)
			for i := range raw {
				raw[i] = 1.5
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": raw})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text", "gemma4:e2b")
		o.SetHTTPClient(server.Client())

		vec, err := o.EmbedMultimodal(ctx, dummyImage, "image/jpeg", "golden retriever puppy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}

		var sumSq float64
		for _, x := range vec {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("multimodal with only text delegates to EmbedText", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			var reqPayload struct {
				Model  string   `json:"model"`
				Prompt string   `json:"prompt"`
				Images []string `json:"images"`
			}
			if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
				t.Fatalf("unmarshaling request: %v", err)
			}
			if reqPayload.Model != "nomic-embed-text" {
				t.Errorf("expected text model nomic-embed-text, got %s", reqPayload.Model)
			}
			if reqPayload.Prompt != "husky dog" {
				t.Errorf("expected prompt 'husky dog', got %s", reqPayload.Prompt)
			}
			if len(reqPayload.Images) != 0 {
				t.Errorf("expected no images in text-only delegation, got %v", reqPayload.Images)
			}

			raw := make([]float32, 768)
			for i := range raw {
				raw[i] = 1.0
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": raw})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text", "gemma4:e2b")
		o.SetHTTPClient(server.Client())

		vec, err := o.EmbedMultimodal(ctx, nil, "", "husky dog")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}
	})

	t.Run("image embedding without configured vision model defaults to model fallback", func(t *testing.T) {
		dummyImage := []byte{1, 2, 3, 4}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqPayload struct {
				Model  string   `json:"model"`
				Images []string `json:"images"`
			}
			_ = json.NewDecoder(r.Body).Decode(&reqPayload)
			if reqPayload.Model != "custom-fallback" {
				t.Errorf("expected model custom-fallback, got %s", reqPayload.Model)
			}
			if len(reqPayload.Images) != 1 {
				t.Errorf("expected 1 image, got %d", len(reqPayload.Images))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": make([]float32, 768)})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "custom-fallback")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedImage(ctx, dummyImage, "image/png")
		if err == nil || !strings.Contains(err.Error(), "vector norm is zero") {
			t.Fatalf("expected vector norm is zero error, got %v", err)
		}
	})

	t.Run("empty image bytes returns error", func(t *testing.T) {
		o := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
		if _, err := o.EmbedImage(ctx, nil, "image/jpeg"); err == nil {
			t.Fatal("expected error on nil image bytes")
		}
		if _, err := o.EmbedImage(ctx, []byte{}, "image/jpeg"); err == nil {
			t.Fatal("expected error on empty image bytes")
		}
	})

	t.Run("empty inputs in EmbedMultimodal returns error", func(t *testing.T) {
		o := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
		if _, err := o.EmbedMultimodal(ctx, nil, "", ""); err == nil {
			t.Fatal("expected error when both image and text are empty")
		}
		if _, err := o.EmbedMultimodal(ctx, []byte{}, "", ""); err == nil {
			t.Fatal("expected error when empty image bytes and empty text are provided")
		}
	})

	t.Run("empty embedding array in response returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{}})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on empty embedding in response")
		}
	})

	t.Run("empty json response body returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on response without embedding field")
		}
	})

	t.Run("empty response body returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on empty response body")
		}
	})

	t.Run("zero vector norm returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": make([]float32, 768)})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on zero vector norm")
		}
		if !strings.Contains(err.Error(), "vector norm is zero") {
			t.Fatalf("expected 'vector norm is zero' error, got: %v", err)
		}
	})

	t.Run("SetVisionModel and getters", func(t *testing.T) {
		o := embedding.NewOllamaEmbedder("http://localhost:11434", "text-model", "initial-vision")
		if o.Model() != "text-model" {
			t.Fatalf("expected text-model, got %s", o.Model())
		}
		if o.VisionModel() != "initial-vision" {
			t.Fatalf("expected initial-vision, got %s", o.VisionModel())
		}
		o.SetVisionModel("updated-vision")
		if o.VisionModel() != "updated-vision" {
			t.Fatalf("expected updated-vision, got %s", o.VisionModel())
		}
	})

	t.Run("empty text returns error", func(t *testing.T) {
		o := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
		_, err := o.EmbedText(ctx, "")
		if err == nil {
			t.Fatal("expected error on empty text")
		}
	})

	t.Run("dimension mismatch returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": make([]float32, 512)})
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "nomic-embed-text")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on dimension mismatch")
		}
	})

	t.Run("server error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "model not found", http.StatusNotFound)
		}))
		defer server.Close()

		o := embedding.NewOllamaEmbedder(server.URL, "")
		o.SetHTTPClient(server.Client())

		_, err := o.EmbedText(ctx, "test")
		if err == nil {
			t.Fatal("expected error on 404 status")
		}
	})

	t.Run("default parameters", func(t *testing.T) {
		o := embedding.NewOllamaEmbedder("", "")
		if o.Dimension() != 768 {
			t.Fatalf("expected 768 dim, got %d", o.Dimension())
		}
	})

	t.Run("context cancellation honored", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		o := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text", "gemma4:e2b")
		if _, err := o.EmbedText(canceledCtx, "test"); err == nil {
			t.Fatal("expected error on canceled context for EmbedText")
		}
		if _, err := o.EmbedImage(canceledCtx, []byte{1, 2, 3}, "image/jpeg"); err == nil {
			t.Fatal("expected error on canceled context for EmbedImage")
		}
		if _, err := o.EmbedMultimodal(canceledCtx, []byte{1, 2, 3}, "image/jpeg", "test"); err == nil {
			t.Fatal("expected error on canceled context for EmbedMultimodal")
		}
	})
}
