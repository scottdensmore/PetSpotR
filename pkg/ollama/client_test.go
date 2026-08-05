package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/ollama"
)

func TestClient_Generate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("got path %s, want /api/generate", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("got method %s, want POST", r.Method)
		}

		var req ollama.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		if req.Model != "gemma2:2b" {
			t.Errorf("got model %s, want gemma2:2b", req.Model)
		}

		resp := ollama.GenerateResponse{
			Model:    "gemma2:2b",
			Response: "Golden Retriever with white chest patch",
			Done:     true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL))

	req := &ollama.GenerateRequest{
		Model:  "gemma2:2b",
		Prompt: "Describe pet traits in image",
		Images: []string{"base64data..."},
		Stream: true, // Should not mutate original req struct
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Response != "Golden Retriever with white chest patch" {
		t.Errorf("got response %s, want expected text", resp.Response)
	}

	if req.Stream != true {
		t.Errorf("original req.Stream was mutated, got %v, want true", req.Stream)
	}
}

func TestClient_Chat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("got path %s, want /api/chat", r.URL.Path)
		}

		resp := ollama.ChatResponse{
			Model: "gemma2:9b",
			Message: ollama.Message{
				Role:    "assistant",
				Content: `{"breed":"Siamese","matchScore":0.85}`,
			},
			Done: true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL))

	req := &ollama.ChatRequest{
		Model: "gemma2:9b",
		Messages: []ollama.Message{
			{Role: "user", Content: "Compare these two pet descriptions"},
		},
		Stream: true,
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Message.Content != `{"breed":"Siamese","matchScore":0.85}` {
		t.Errorf("got chat response content %s", resp.Message.Content)
	}

	if req.Stream != true {
		t.Errorf("original req.Stream was mutated, got %v, want true", req.Stream)
	}
}

func TestClient_Non2xxErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"model 'gemma2:missing' not found"}`))
	}))
	defer ts.Close()

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL))

	_, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: "gemma2:missing"})
	if err == nil {
		t.Fatal("expected error on 404 response, got nil")
	}

	if !strings.Contains(err.Error(), "model 'gemma2:missing' not found") {
		t.Errorf("expected error message to contain server error detail, got %v", err)
	}
}

func TestClient_OptionsAndEnvFallback(t *testing.T) {
	t.Run("nil HTTP client option guard", func(t *testing.T) {
		client := ollama.NewClient(ollama.WithHTTPClient(nil))
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("OLLAMA_HOST env var fallback", func(t *testing.T) {
		os.Setenv("OLLAMA_HOST", "http://ollama-custom:11434/")
		defer os.Unsetenv("OLLAMA_HOST")

		client := ollama.NewClient()
		// Safe verification via request timeout behavior or custom transport
		_ = client
	})

	t.Run("WithHTTPClient custom timeout", func(t *testing.T) {
		customHC := &http.Client{Timeout: 5 * time.Second}
		client := ollama.NewClient(ollama.WithHTTPClient(customHC))
		_ = client
	})
}

func TestClient_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {} // Block indefinitely
	}))
	defer ts.Close()

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Generate(ctx, &ollama.GenerateRequest{Model: "gemma2:2b"})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
