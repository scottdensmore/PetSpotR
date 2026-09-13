package ollama_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
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

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL), ollama.WithMaxRetries(0))

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

	if resp.Provenance.Model != "gemma2" || resp.Provenance.Version != "2b" {
		t.Errorf("unexpected provenance: %+v", resp.Provenance)
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

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL), ollama.WithMaxRetries(0))

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

	if resp.Provenance.Model != "gemma2" || resp.Provenance.Version != "9b" {
		t.Errorf("unexpected provenance: %+v", resp.Provenance)
	}
}

func TestClient_ModelProvenance(t *testing.T) {
	t.Run("canonical Gemma 4 provenance constants", func(t *testing.T) {
		if ollama.Gemma4Model != "gemma4:e2b" {
			t.Errorf("Gemma4Model = %q, want %q", ollama.Gemma4Model, "gemma4:e2b")
		}
		if ollama.Gemma4Version != "e2b" {
			t.Errorf("Gemma4Version = %q, want %q", ollama.Gemma4Version, "e2b")
		}
		if !strings.HasPrefix(ollama.Gemma4Digest, "sha256:") {
			t.Errorf("Gemma4Digest = %q, want sha256 prefix", ollama.Gemma4Digest)
		}

		p := ollama.DefaultGemma4Provenance()
		if p.Model != ollama.Gemma4Model || p.Version != ollama.Gemma4Version || p.Digest != ollama.Gemma4Digest {
			t.Errorf("DefaultGemma4Provenance() = %+v, want valid metadata", p)
		}
		if !strings.Contains(p.String(), ollama.Gemma4Model) {
			t.Errorf("p.String() = %q, want containing model name", p.String())
		}
	})

	t.Run("ResolveModelProvenance derivations", func(t *testing.T) {
		pGemma := ollama.ResolveModelProvenance("gemma4:e2b")
		if pGemma.Model != "gemma4:e2b" || pGemma.Version != "e2b" {
			t.Errorf("unexpected resolved gemma provenance: %+v", pGemma)
		}

		pEmpty := ollama.ResolveModelProvenance("")
		if pEmpty.Model != ollama.Gemma4Model {
			t.Errorf("empty model should default to Gemma4Model, got %+v", pEmpty)
		}

		pCustom := ollama.ResolveModelProvenance("llama3:8b")
		if pCustom.Model != "llama3" || pCustom.Version != "8b" {
			t.Errorf("unexpected custom model provenance: %+v", pCustom)
		}
	})

	t.Run("Generate records Gemma 4 provenance by default", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollama.GenerateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Model != ollama.Gemma4Model {
				t.Errorf("got req.Model %q, want %q", req.Model, ollama.Gemma4Model)
			}
			_ = json.NewEncoder(w).Encode(ollama.GenerateResponse{
				Model:    ollama.Gemma4Model,
				Response: `{"breed":"Beagle"}`,
				Done:     true,
			})
		}))
		defer ts.Close()

		client := ollama.NewClient(ollama.WithBaseURL(ts.URL), ollama.WithMaxRetries(0))
		resp, err := client.Generate(context.Background(), &ollama.GenerateRequest{
			Prompt: "Identify breed",
		})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if resp.Provenance.Model != ollama.Gemma4Model || resp.Provenance.Version != "e2b" {
			t.Errorf("expected Gemma 4 provenance, got %+v", resp.Provenance)
		}
	})

	t.Run("WithModelProvenance custom override", func(t *testing.T) {
		customProv := ollama.ModelProvenance{
			Model:   "custom-vision",
			Version: "v1.2",
			Digest:  "sha256:abc123456",
		}
		client := ollama.NewClient(ollama.WithModelProvenance(customProv))
		if client.ModelProvenance() != customProv {
			t.Errorf("ModelProvenance() = %+v, want %+v", client.ModelProvenance(), customProv)
		}
	})
}

func TestClient_BoundedRetriesAndErrorHandling(t *testing.T) {
	t.Run("transient 503 succeeds on subsequent attempt", func(t *testing.T) {
		var attempts int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			curr := atomic.AddInt32(&attempts, 1)
			if curr < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"model temporarily loading"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ollama.GenerateResponse{
				Model:    ollama.Gemma4Model,
				Response: "Success after retries",
				Done:     true,
			})
		}))
		defer ts.Close()

		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(3),
			ollama.WithRetryBackoff(5*time.Millisecond),
		)

		resp, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: ollama.Gemma4Model})
		if err != nil {
			t.Fatalf("expected success after retries, got error: %v", err)
		}
		if resp.Response != "Success after retries" {
			t.Errorf("got %q, want 'Success after retries'", resp.Response)
		}
		if attempts != 3 {
			t.Errorf("got %d attempts, want 3", attempts)
		}
	})

	t.Run("persistent 500 exhausts bounded retries", func(t *testing.T) {
		var attempts int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"cuda core panic"}`))
		}))
		defer ts.Close()

		maxRetries := 2
		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(maxRetries),
			ollama.WithRetryBackoff(5*time.Millisecond),
		)

		_, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: ollama.Gemma4Model})
		if err == nil {
			t.Fatal("expected error on persistent 500, got nil")
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("after %d retries", maxRetries)) {
			t.Errorf("expected error to mention retry exhaustion, got: %v", err)
		}
		if int(attempts) != maxRetries+1 {
			t.Errorf("got %d attempts, want %d", attempts, maxRetries+1)
		}
	})

	t.Run("non-retryable 404 does not retry", func(t *testing.T) {
		var attempts int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"model 'gemma4:missing' not found"}`))
		}))
		defer ts.Close()

		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(3),
			ollama.WithRetryBackoff(5*time.Millisecond),
		)

		_, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: "gemma4:missing"})
		if err == nil {
			t.Fatal("expected error on 404 response, got nil")
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt for non-retryable 404, got %d", attempts)
		}
	})

	t.Run("non-retryable 400 does not retry", func(t *testing.T) {
		var attempts int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad base64 format"}`))
		}))
		defer ts.Close()

		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(3),
			ollama.WithRetryBackoff(5*time.Millisecond),
		)

		_, err := client.Generate(context.Background(), &ollama.GenerateRequest{Prompt: "bad"})
		if err == nil {
			t.Fatal("expected error on 400 response, got nil")
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt for non-retryable 400, got %d", attempts)
		}
	})

	t.Run("context cancellation during retry backoff halts immediately", func(t *testing.T) {
		var attempts int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(context.Background())
		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(5),
			ollama.WithRetryBackoff(500*time.Millisecond),
		)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := client.Generate(ctx, &ollama.GenerateRequest{Model: ollama.Gemma4Model})
		if err == nil {
			t.Fatal("expected error on canceled context, got nil")
		}
		if attempts > 2 {
			t.Errorf("expected cancellation to abort quickly, got %d attempts", attempts)
		}
	})

	t.Run("per-request timeout handling aborts slow requests", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response":"too late"}`))
		}))
		defer ts.Close()

		client := ollama.NewClient(
			ollama.WithBaseURL(ts.URL),
			ollama.WithMaxRetries(0),
			ollama.WithTimeout(20*time.Millisecond),
		)

		_, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: ollama.Gemma4Model})
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
	})
}

func TestClient_DeterministicMockAndFakeClient(t *testing.T) {
	t.Run("NewDeterministicClient produces responses without live network", func(t *testing.T) {
		customGen := &ollama.GenerateResponse{
			Model:      ollama.Gemma4Model,
			Response:   `{"breed":"Siberian Husky","eyeColor":"Blue"}`,
			Done:       true,
			Provenance: ollama.DefaultGemma4Provenance(),
		}
		customChat := &ollama.ChatResponse{
			Model:      ollama.Gemma4Model,
			Message:    ollama.Message{Role: "assistant", Content: `{"match":true}`},
			Done:       true,
			Provenance: ollama.DefaultGemma4Provenance(),
		}

		client := ollama.NewDeterministicClient(customGen, customChat)

		genResp, err := client.Generate(context.Background(), &ollama.GenerateRequest{Model: ollama.Gemma4Model})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if genResp.Response != `{"breed":"Siberian Husky","eyeColor":"Blue"}` {
			t.Errorf("unexpected response %s", genResp.Response)
		}
		if genResp.Provenance.Model != ollama.Gemma4Model {
			t.Errorf("unexpected provenance %+v", genResp.Provenance)
		}

		chatResp, err := client.Chat(context.Background(), &ollama.ChatRequest{Model: ollama.Gemma4Model})
		if err != nil {
			t.Fatalf("Chat failed: %v", err)
		}
		if chatResp.Message.Content != `{"match":true}` {
			t.Errorf("unexpected chat message %s", chatResp.Message.Content)
		}
	})

	t.Run("FakeClient satisfies InferenceClient and records calls", func(t *testing.T) {
		fake := ollama.NewFakeClient(func(ctx context.Context, req *ollama.GenerateRequest) (*ollama.GenerateResponse, error) {
			return &ollama.GenerateResponse{
				Model:      ollama.Gemma4Model,
				Response:   `{"breed":"Corgi"}`,
				Done:       true,
				Provenance: ollama.DefaultGemma4Provenance(),
			}, nil
		})

		var infClient ollama.InferenceClient = fake

		resp, err := infClient.Generate(context.Background(), &ollama.GenerateRequest{
			Prompt: "Extract traits",
		})
		if err != nil {
			t.Fatalf("fake Generate failed: %v", err)
		}
		if resp.Response != `{"breed":"Corgi"}` {
			t.Errorf("got %s, want Corgi", resp.Response)
		}
		if len(fake.GenerateCalls) != 1 {
			t.Errorf("got %d calls, want 1", len(fake.GenerateCalls))
		}
		if infClient.ModelProvenance().Model != ollama.Gemma4Model {
			t.Errorf("ModelProvenance() = %+v", infClient.ModelProvenance())
		}
	})
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

	client := ollama.NewClient(ollama.WithBaseURL(ts.URL), ollama.WithMaxRetries(0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Generate(ctx, &ollama.GenerateRequest{Model: "gemma2:2b"})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
