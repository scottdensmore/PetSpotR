package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultSendGridBaseURL = "https://api.sendgrid.com"

// SendGridOption configures a SendGridEmailProvider.
type SendGridOption func(*SendGridEmailProvider)

// WithSendGridBaseURL overrides the default SendGrid API base URL.
func WithSendGridBaseURL(url string) SendGridOption {
	return func(p *SendGridEmailProvider) {
		if strings.TrimSpace(url) != "" {
			p.baseURL = strings.TrimSuffix(url, "/")
		}
	}
}

// WithSendGridHTTPClient overrides the default HTTP client.
func WithSendGridHTTPClient(client *http.Client) SendGridOption {
	return func(p *SendGridEmailProvider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// WithSendGridFromEmail overrides the default sender email.
func WithSendGridFromEmail(from string) SendGridOption {
	return func(p *SendGridEmailProvider) {
		if strings.TrimSpace(from) != "" {
			p.fromEmail = from
		}
	}
}

// SendGridEmailProvider delivers emails via the SendGrid v3 Mail Send API.
type SendGridEmailProvider struct {
	apiKey     string
	fromEmail  string
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	sentKeys   map[string]struct{}
}

// NewSendGridEmailProvider constructs a SendGrid email provider.
func NewSendGridEmailProvider(apiKey string, opts ...SendGridOption) (*SendGridEmailProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("sendgrid: api key cannot be empty")
	}
	p := &SendGridEmailProvider{
		apiKey:    apiKey,
		fromEmail: "alerts@petspotr.io",
		baseURL:   defaultSendGridBaseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		sentKeys: make(map[string]struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p, nil
}

type sendGridEmail struct {
	Email string `json:"email"`
}

type sendGridPersonalization struct {
	To []sendGridEmail `json:"to"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendGridPayload struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

// SendEmail delivers an email message using the SendGrid Mail Send API.
func (p *SendGridEmailProvider) SendEmail(ctx context.Context, msg EmailMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(msg.To) == "" {
		return errors.New("sendgrid: recipient email is required")
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return errors.New("sendgrid: subject is required")
	}

	p.mu.Lock()
	if msg.IdempotencyKey != "" {
		if _, exists := p.sentKeys[msg.IdempotencyKey]; exists {
			p.mu.Unlock()
			return nil
		}
	}
	p.mu.Unlock()

	fromEmail := msg.From
	if strings.TrimSpace(fromEmail) == "" {
		fromEmail = p.fromEmail
	}

	contents := make([]sendGridContent, 0, 2)
	if strings.TrimSpace(msg.TextBody) != "" {
		contents = append(contents, sendGridContent{Type: "text/plain", Value: msg.TextBody})
	}
	if strings.TrimSpace(msg.HTMLBody) != "" {
		contents = append(contents, sendGridContent{Type: "text/html", Value: msg.HTMLBody})
	}
	if len(contents) == 0 {
		contents = append(contents, sendGridContent{Type: "text/plain", Value: "Notification from PetSpotR"})
	}

	payload := sendGridPayload{
		Personalizations: []sendGridPersonalization{
			{To: []sendGridEmail{{Email: msg.To}}},
		},
		From:    sendGridEmail{Email: fromEmail},
		Subject: msg.Subject,
		Content: contents,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sendgrid: marshal payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v3/mail/send", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sendgrid: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if msg.IdempotencyKey != "" {
		req.Header.Set("X-Entity-Ref-ID", msg.IdempotencyKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid: API returned status %d: %s", resp.StatusCode, string(body))
	}

	if msg.IdempotencyKey != "" {
		p.mu.Lock()
		p.sentKeys[msg.IdempotencyKey] = struct{}{}
		p.mu.Unlock()
	}

	return nil
}
