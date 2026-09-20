package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// InboundMessage represents an incoming SMS webhook payload.
type InboundMessage struct {
	FromNumber string    `json:"from"`
	ToNumber   string    `json:"to"`
	Body       string    `json:"body"`
	Timestamp  time.Time `json:"timestamp"`
}

// OutboundMessage represents an outgoing SMS message.
type OutboundMessage struct {
	ToNumber   string `json:"to"`
	FromNumber string `json:"from,omitempty"`
	Body       string `json:"body"`
}

// CommandType defines supported SMS command directives.
type CommandType string

const (
	CommandClaim       CommandType = "CLAIM"
	CommandSighted     CommandType = "SIGHTED"
	CommandStatus      CommandType = "STATUS"
	CommandOptOut      CommandType = "STOP"
	CommandUnsubscribe CommandType = "UNSUBSCRIBE"
	CommandOptIn       CommandType = "START"
	CommandUnknown     CommandType = "UNKNOWN"
)

// Provider abstracts SMS dispatch operations.
type Provider interface {
	SendSMS(ctx context.Context, to string, body string) error
}

var nonDigitOrPlus = regexp.MustCompile(`[^0-9+]`)

// NormalizeE164 normalizes a raw phone number into standard E.164 format.
// Defaults to North American (+1) when 10 digits are provided without a country code.
func NormalizeE164(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("phone number cannot be empty")
	}

	cleaned := nonDigitOrPlus.ReplaceAllString(trimmed, "")
	if cleaned == "" {
		return "", errors.New("phone number contains no valid digits")
	}

	if strings.HasPrefix(cleaned, "+") {
		digits := cleaned[1:]
		if strings.Contains(digits, "+") {
			return "", errors.New("phone number has invalid misplaced '+' sign")
		}
		if len(digits) < 10 || len(digits) > 15 {
			return "", fmt.Errorf("E.164 phone number length invalid: expected 10-15 digits, got %d", len(digits))
		}
		for _, r := range digits {
			if !unicode.IsDigit(r) {
				return "", fmt.Errorf("phone number contains invalid character %q", r)
			}
		}
		return cleaned, nil
	}

	for _, r := range cleaned {
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("phone number contains invalid character %q", r)
		}
	}

	if len(cleaned) == 11 && cleaned[0] == '1' {
		return "+" + cleaned, nil
	}

	if len(cleaned) == 10 {
		return "+1" + cleaned, nil
	}

	if len(cleaned) >= 10 && len(cleaned) <= 15 {
		return "+" + cleaned, nil
	}

	return "", fmt.Errorf("unrecognized phone number format with %d digits", len(cleaned))
}

// DefaultSMSSalt returns the HMAC salt used for phone tokenization,
// reading from PETSPOTR_SMS_SALT or falling back to a safe default.
func DefaultSMSSalt() []byte {
	if salt := os.Getenv("PETSPOTR_SMS_SALT"); salt != "" {
		return []byte(salt)
	}
	return []byte("petspotr-default-sms-salt-2026")
}

// TokenizeNumber hashes a phone number with HMAC-SHA256 and returns a 64-char hex string,
// ensuring zero plaintext PII is stored.
func TokenizeNumber(phone string, secretKey []byte) string {
	if len(secretKey) == 0 {
		secretKey = DefaultSMSSalt()
	}
	normalized, err := NormalizeE164(phone)
	if err != nil {
		normalized = strings.TrimSpace(phone)
	}
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// ParseCommand parses the first word of an SMS body into a CommandType and returns the remainder as arguments.
func ParseCommand(body string) (CommandType, string) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return CommandUnknown, ""
	}

	parts := strings.SplitN(trimmed, " ", 2)
	keyword := strings.ToUpper(strings.TrimSpace(parts[0]))
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch keyword {
	case "CLAIM":
		return CommandClaim, arg
	case "SIGHTED":
		return CommandSighted, arg
	case "STATUS":
		return CommandStatus, arg
	case "STOP", "QUIT", "CANCEL", "END":
		return CommandOptOut, arg
	case "UNSUBSCRIBE":
		return CommandUnsubscribe, arg
	case "START", "UNSTOP", "YES":
		return CommandOptIn, arg
	default:
		return CommandUnknown, trimmed
	}
}

// MockMessage records a sent SMS message for testing assertions.
type MockMessage struct {
	To        string    `json:"to"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
}

// MockProvider is an in-memory thread-safe implementation of Provider for testing.
type MockProvider struct {
	mu      sync.Mutex
	sent    []MockMessage
	sendErr error
}

// NewMockProvider creates an initialized MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		sent: make([]MockMessage, 0),
	}
}

// SendSMS records the outgoing message and returns any injected error.
func (m *MockProvider) SendSMS(ctx context.Context, to string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}

	m.sent = append(m.sent, MockMessage{
		To:        to,
		Body:      body,
		Timestamp: time.Now().UTC(),
	})
	return nil
}

// SentMessages returns a copy of all recorded sent messages.
func (m *MockProvider) SentMessages() []MockMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]MockMessage, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// LastMessage returns the most recently sent message.
func (m *MockProvider) LastMessage() (MockMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sent) == 0 {
		return MockMessage{}, false
	}
	return m.sent[len(m.sent)-1], true
}

// SetError injects an error to be returned by SendSMS.
func (m *MockProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

// Reset clears all sent messages and errors.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = make([]MockMessage, 0)
	m.sendErr = nil
}
