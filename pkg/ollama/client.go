package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBaseURL = "http://localhost:11434"

// Client is an HTTP client for interacting with an Ollama instance.
type Client struct {
	baseURL    string
	httpClient *http.Client
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

// NewClient constructs a new Ollama client.
func NewClient(opts ...Option) *Client {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	c := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
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
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
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
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

// Generate calls the /api/generate endpoint.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, errors.New("ollama: request cannot be nil")
	}
	r := *req
	r.Stream = false
	var resp GenerateResponse
	if err := c.doRequest(ctx, "/api/generate", &r, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Chat calls the /api/chat endpoint.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, errors.New("ollama: request cannot be nil")
	}
	r := *req
	r.Stream = false
	var resp ChatResponse
	if err := c.doRequest(ctx, "/api/chat", &r, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ollama: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && strings.TrimSpace(errResp.Error) != "" {
			return fmt.Errorf("ollama: API returned status code %d: %s", httpResp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("ollama: API returned status code %d", httpResp.StatusCode)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("ollama: failed to decode response: %w", err)
	}

	return nil
}
