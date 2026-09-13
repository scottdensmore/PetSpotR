package ollama

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "http://localhost:11434"
	defaultMaxRetries   = 3
	defaultRetryBackoff = 100 * time.Millisecond
	defaultTimeout      = 60 * time.Second

	// Gemma4Model is the default multimodal inference model for PetSpotR vision tasks.
	Gemma4Model = "gemma4:e2b"
	// Gemma4Version is the default model artifact version.
	Gemma4Version = "e2b"
	// Gemma4Digest is the canonical content digest for the gemma4:e2b model artifact.
	Gemma4Digest = "sha256:7b49479b3922c153724c9c1b7530691efdfa3a60db6e30a5fbbeffbe4bfcb12d"
)

// ModelProvenance tracks the model name, artifact version, and content digest.
type ModelProvenance struct {
	Model   string `json:"model"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// String formats the model provenance as a readable identifier.
func (p ModelProvenance) String() string {
	if p.Digest != "" {
		if p.Version != "" {
			return fmt.Sprintf("%s:%s@%s", p.Model, p.Version, p.Digest)
		}
		return fmt.Sprintf("%s@%s", p.Model, p.Digest)
	}
	if p.Version != "" {
		return fmt.Sprintf("%s:%s", p.Model, p.Version)
	}
	return p.Model
}

// DefaultGemma4Provenance returns the canonical provenance for Gemma 4.
func DefaultGemma4Provenance() ModelProvenance {
	return ModelProvenance{
		Model:   Gemma4Model,
		Version: Gemma4Version,
		Digest:  Gemma4Digest,
	}
}

// ResolveModelProvenance parses or resolves provenance for a given model string.
func ResolveModelProvenance(model string) ModelProvenance {
	clean := strings.TrimSpace(model)
	if clean == "" || clean == Gemma4Model || clean == "gemma4" {
		return DefaultGemma4Provenance()
	}
	parts := strings.SplitN(clean, ":", 2)
	if len(parts) == 2 {
		return ModelProvenance{
			Model:   parts[0],
			Version: parts[1],
			Digest:  fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(clean))),
		}
	}
	return ModelProvenance{
		Model:   clean,
		Version: "latest",
		Digest:  fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(clean))),
	}
}

// InferenceClient defines the inference operations for Ollama.
type InferenceClient interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ModelProvenance() ModelProvenance
}

// Client is an HTTP client for interacting with an Ollama instance.
type Client struct {
	baseURL       string
	httpClient    *http.Client
	maxRetries    int
	retryBackoff  time.Duration
	timeout       time.Duration
	provenance    ModelProvenance
	provenanceMap map[string]ModelProvenance
}

// Option configures a Client instance.
type Option func(*Client)

// WithBaseURL overrides the default Ollama base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		if strings.TrimSpace(url) != "" {
			c.baseURL = strings.TrimSuffix(url, "/")
		}
	}
}

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithMaxRetries configures maximum retry attempts for transient failures.
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		if retries >= 0 {
			c.maxRetries = retries
		}
	}
}

// WithRetryBackoff configures the initial base retry backoff duration.
func WithRetryBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d >= 0 {
			c.retryBackoff = d
		}
	}
}

// WithTimeout configures the per-request timeout context limit.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d >= 0 {
			c.timeout = d
		}
	}
}

// WithModelProvenance overrides the default model provenance on the client.
func WithModelProvenance(p ModelProvenance) Option {
	return func(c *Client) {
		c.provenance = p
	}
}

// NewClient constructs a new Ollama client.
func NewClient(opts ...Option) *Client {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	c := &Client{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		httpClient:   &http.Client{Timeout: defaultTimeout},
		maxRetries:   defaultMaxRetries,
		retryBackoff: defaultRetryBackoff,
		timeout:      defaultTimeout,
		provenance:   DefaultGemma4Provenance(),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// ModelProvenance returns the client's configured default model provenance.
func (c *Client) ModelProvenance() ModelProvenance {
	if c.provenance.Model != "" {
		return c.provenance
	}
	return DefaultGemma4Provenance()
}

// GenerateRequest represents a prompt request sent to /api/generate.
type GenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system,omitempty"`
	Images  []string               `json:"images,omitempty"`
	Stream  bool                   `json:"stream"`
	Format  string                 `json:"format,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// GenerateResponse represents the output from /api/generate.
type GenerateResponse struct {
	Model      string          `json:"model"`
	CreatedAt  string          `json:"created_at"`
	Response   string          `json:"response"`
	Done       bool            `json:"done"`
	Provenance ModelProvenance `json:"provenance,omitempty"`
}

// Message represents a single chat message.
type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// ChatRequest represents a request sent to /api/chat.
type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Stream   bool                   `json:"stream"`
	Format   string                 `json:"format,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// ChatResponse represents a response from /api/chat.
type ChatResponse struct {
	Model      string          `json:"model"`
	CreatedAt  string          `json:"created_at"`
	Message    Message         `json:"message"`
	Done       bool            `json:"done"`
	Provenance ModelProvenance `json:"provenance,omitempty"`
}

// Generate calls the /api/generate endpoint with bounded retries and provenance recording.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, errors.New("ollama: request cannot be nil")
	}
	r := *req
	r.Stream = false
	if strings.TrimSpace(r.Model) == "" {
		r.Model = Gemma4Model
	}
	var resp GenerateResponse
	if err := c.doRequest(ctx, "/api/generate", &r, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Model) == "" {
		resp.Model = r.Model
	}
	if resp.Provenance.Model == "" {
		resp.Provenance = c.resolveProvenance(resp.Model)
	}
	return &resp, nil
}

// Chat calls the /api/chat endpoint with bounded retries and provenance recording.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, errors.New("ollama: request cannot be nil")
	}
	r := *req
	r.Stream = false
	if strings.TrimSpace(r.Model) == "" {
		r.Model = Gemma4Model
	}
	var resp ChatResponse
	if err := c.doRequest(ctx, "/api/chat", &r, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Model) == "" {
		resp.Model = r.Model
	}
	if resp.Provenance.Model == "" {
		resp.Provenance = c.resolveProvenance(resp.Model)
	}
	return &resp, nil
}

func (c *Client) resolveProvenance(model string) ModelProvenance {
	if c.provenanceMap != nil {
		if p, ok := c.provenanceMap[model]; ok {
			return p
		}
	}
	clean := strings.TrimSpace(model)
	if clean == "" || clean == Gemma4Model || clean == "gemma4" {
		if c.provenance.Model != "" {
			return c.provenance
		}
		return DefaultGemma4Provenance()
	}
	return ResolveModelProvenance(clean)
}

func isRetryableStatusCode(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusInternalServerError ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

func (c *Client) doRequest(ctx context.Context, path string, reqBody interface{}, respBody interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	maxRetries := c.maxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.retryBackoff * time.Duration(1<<(attempt-1))
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		reqCtx := ctx
		var cancel context.CancelFunc
		if c.timeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}

		httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return fmt.Errorf("ollama: failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, err := c.httpClient.Do(httpReq)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("ollama: request failed: %w", err)
			continue
		}

		bodyBytes, readErr := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		if cancel != nil {
			cancel()
		}
		if readErr != nil {
			lastErr = fmt.Errorf("ollama: failed to read response body: %w", readErr)
			continue
		}

		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			var errResp struct {
				Error string `json:"error"`
			}
			msg := ""
			if err := json.Unmarshal(bodyBytes, &errResp); err == nil && strings.TrimSpace(errResp.Error) != "" {
				msg = errResp.Error
			}
			var statusErr error
			if msg != "" {
				statusErr = fmt.Errorf("ollama: API returned status code %d: %s", httpResp.StatusCode, msg)
			} else {
				statusErr = fmt.Errorf("ollama: API returned status code %d", httpResp.StatusCode)
			}

			if isRetryableStatusCode(httpResp.StatusCode) {
				lastErr = statusErr
				continue
			}
			return statusErr
		}

		if err := json.Unmarshal(bodyBytes, respBody); err != nil {
			return fmt.Errorf("ollama: failed to decode response: %w", err)
		}

		return nil
	}

	return fmt.Errorf("ollama: request failed after %d retries: %w", maxRetries, lastErr)
}

// FakeClient is a deterministic mock/fake client for routine unit/integration testing.
type FakeClient struct {
	GenerateFunc  func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	ChatFunc      func(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	Provenance    ModelProvenance
	GenerateCalls []*GenerateRequest
	ChatCalls     []*ChatRequest
}

// NewFakeClient constructs a deterministic FakeClient with a custom GenerateFunc.
func NewFakeClient(generateFunc func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)) *FakeClient {
	return &FakeClient{
		GenerateFunc: generateFunc,
		Provenance:   DefaultGemma4Provenance(),
	}
}

// ModelProvenance returns the configured provenance metadata on the fake client.
func (f *FakeClient) ModelProvenance() ModelProvenance {
	if f.Provenance.Model != "" {
		return f.Provenance
	}
	return DefaultGemma4Provenance()
}

// Generate implements InferenceClient.Generate on FakeClient.
func (f *FakeClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	f.GenerateCalls = append(f.GenerateCalls, req)
	if f.GenerateFunc != nil {
		return f.GenerateFunc(ctx, req)
	}
	model := Gemma4Model
	if req != nil && strings.TrimSpace(req.Model) != "" {
		model = req.Model
	}
	return &GenerateResponse{
		Model:      model,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"White","distinctiveMarkings":["white patch"],"eyeColor":"Brown"}`,
		Done:       true,
		Provenance: f.ModelProvenance(),
	}, nil
}

// Chat implements InferenceClient.Chat on FakeClient.
func (f *FakeClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	f.ChatCalls = append(f.ChatCalls, req)
	if f.ChatFunc != nil {
		return f.ChatFunc(ctx, req)
	}
	model := Gemma4Model
	if req != nil && strings.TrimSpace(req.Model) != "" {
		model = req.Model
	}
	return &ChatResponse{
		Model: model,
		Message: Message{
			Role:    "assistant",
			Content: `{"breed":"Golden Retriever","matchScore":0.95}`,
		},
		Done:       true,
		Provenance: f.ModelProvenance(),
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewDeterministicClient returns an *ollama.Client backed by a mock HTTP transport
// that produces deterministic responses without network calls.
func NewDeterministicClient(generateResp *GenerateResponse, chatResp *ChatResponse) *Client {
	if generateResp == nil {
		generateResp = &GenerateResponse{
			Model:      Gemma4Model,
			Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"White","distinctiveMarkings":["white patch"],"eyeColor":"Brown"}`,
			Done:       true,
			Provenance: DefaultGemma4Provenance(),
		}
	}
	if chatResp == nil {
		chatResp = &ChatResponse{
			Model:      Gemma4Model,
			Message:    Message{Role: "assistant", Content: `{"breed":"Golden Retriever","matchScore":0.95}`},
			Done:       true,
			Provenance: DefaultGemma4Provenance(),
		}
	}
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		var err error
		switch req.URL.Path {
		case "/api/generate":
			body, err = json.Marshal(generateResp)
		case "/api/chat":
			body, err = json.Marshal(chatResp)
		case "/api/version":
			body = []byte(`{"version":"0.32.5"}`)
		default:
			body = []byte(`{"error":"not found"}`)
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}
		if err != nil {
			return nil, err
		}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})
	return NewClient(
		WithBaseURL("http://mock-ollama:11434"),
		WithHTTPClient(&http.Client{Transport: rt}),
		WithMaxRetries(0),
		WithRetryBackoff(0),
	)
}
