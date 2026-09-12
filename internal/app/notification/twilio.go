package notification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultTwilioBaseURL = "https://api.twilio.com"

// TwilioOption configures a TwilioSMSProvider.
type TwilioOption func(*TwilioSMSProvider)

// WithTwilioBaseURL overrides the default Twilio API base URL.
func WithTwilioBaseURL(baseURL string) TwilioOption {
	return func(p *TwilioSMSProvider) {
		if strings.TrimSpace(baseURL) != "" {
			p.baseURL = strings.TrimSuffix(baseURL, "/")
		}
	}
}

// WithTwilioHTTPClient overrides the default HTTP client.
func WithTwilioHTTPClient(client *http.Client) TwilioOption {
	return func(p *TwilioSMSProvider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// WithTwilioFromNumber overrides the default Twilio sender phone number.
func WithTwilioFromNumber(fromNumber string) TwilioOption {
	return func(p *TwilioSMSProvider) {
		if strings.TrimSpace(fromNumber) != "" {
			p.fromNumber = fromNumber
		}
	}
}

// TwilioSMSProvider delivers SMS messages via the Twilio REST API.
type TwilioSMSProvider struct {
	accountSID string
	authToken  string
	fromNumber string
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	sentKeys   map[string]struct{}
}

// NewTwilioSMSProvider constructs a Twilio SMS provider.
func NewTwilioSMSProvider(accountSID, authToken string, opts ...TwilioOption) (*TwilioSMSProvider, error) {
	if strings.TrimSpace(accountSID) == "" {
		return nil, errors.New("twilio: account SID cannot be empty")
	}
	if strings.TrimSpace(authToken) == "" {
		return nil, errors.New("twilio: auth token cannot be empty")
	}
	p := &TwilioSMSProvider{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: "+15005550006", // Twilio magic number default
		baseURL:    defaultTwilioBaseURL,
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

// SendSMS delivers an SMS text message using the Twilio Messages API.
func (p *TwilioSMSProvider) SendSMS(ctx context.Context, msg SMSMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(msg.To) == "" {
		return errors.New("twilio: recipient phone number is required")
	}
	if strings.TrimSpace(msg.Body) == "" {
		return errors.New("twilio: message body is required")
	}

	p.mu.Lock()
	if msg.IdempotencyKey != "" {
		if _, exists := p.sentKeys[msg.IdempotencyKey]; exists {
			p.mu.Unlock()
			return nil
		}
	}
	p.mu.Unlock()

	from := msg.From
	if strings.TrimSpace(from) == "" {
		from = p.fromNumber
	}

	formData := url.Values{}
	formData.Set("To", msg.To)
	formData.Set("From", from)
	formData.Set("Body", msg.Body)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", p.baseURL, p.accountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.accountSID, p.authToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio: API returned status %d: %s", resp.StatusCode, string(body))
	}

	if msg.IdempotencyKey != "" {
		p.mu.Lock()
		p.sentKeys[msg.IdempotencyKey] = struct{}{}
		p.mu.Unlock()
	}

	return nil
}
