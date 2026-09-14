package embedding_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/embedding"
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

	t.Run("vertex provider success", func(t *testing.T) {
		embedder, err := embedding.NewEmbedder(embedding.Config{
			Provider:  "vertex",
			ProjectID: "test-project",
			Location:  "us-central1",
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

	t.Run("successful text embedding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
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

		vec, err := v.EmbedText(ctx, "lost golden retriever")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
		}
	})

	t.Run("successful image embedding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		vec, err := v.EmbedImage(ctx, []byte{1, 2, 3}, "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vec))
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
}

func TestOllamaEmbedder(t *testing.T) {
	ctx := context.Background()

	t.Run("successful text embedding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/embeddings" {
				t.Errorf("expected /api/embeddings, got %s", r.URL.Path)
			}
			resp := map[string]any{
				"embedding": make([]float32, 768),
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

		vecImg, err := o.EmbedImage(ctx, []byte{1, 2, 3}, "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vecImg) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vecImg))
		}

		vecMulti, err := o.EmbedMultimodal(ctx, []byte{1, 2, 3}, "image/jpeg", "calico cat")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vecMulti) != 768 {
			t.Fatalf("expected 768 dim vector, got %d", len(vecMulti))
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

		o := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
		if _, err := o.EmbedText(canceledCtx, "test"); err == nil {
			t.Fatal("expected error on canceled context")
		}
	})
}
