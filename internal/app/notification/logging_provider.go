package notification

import (
	"context"
	"log"
	"sync"
)

// LoggingNotificationProvider provides fallback logging delivery for Email, SMS, and Push in dev/test.
type LoggingNotificationProvider struct {
	mu         sync.Mutex
	sentEmails []EmailMessage
	sentSMS    []SMSMessage
	sentPush   []PushMessage
	sentKeys   map[string]struct{}
}

// NewLoggingNotificationProvider constructs a thread-safe unified logging provider.
func NewLoggingNotificationProvider() *LoggingNotificationProvider {
	return &LoggingNotificationProvider{
		sentEmails: make([]EmailMessage, 0),
		sentSMS:    make([]SMSMessage, 0),
		sentPush:   make([]PushMessage, 0),
		sentKeys:   make(map[string]struct{}),
	}
}

// NewLoggingEmailProvider constructs an EmailProvider backed by LoggingNotificationProvider.
func NewLoggingEmailProvider() *LoggingNotificationProvider {
	return NewLoggingNotificationProvider()
}

// NewLoggingSMSProvider constructs an SMSProvider backed by LoggingNotificationProvider.
func NewLoggingSMSProvider() *LoggingNotificationProvider {
	return NewLoggingNotificationProvider()
}

// NewLoggingPushProvider constructs a PushProvider backed by LoggingNotificationProvider.
func NewLoggingPushProvider() *LoggingNotificationProvider {
	return NewLoggingNotificationProvider()
}

// SendEmail logs the email dispatch and records the message in memory.
func (p *LoggingNotificationProvider) SendEmail(ctx context.Context, msg EmailMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if msg.IdempotencyKey != "" {
		if _, exists := p.sentKeys[msg.IdempotencyKey]; exists {
			log.Printf("[DEV LOG EMAIL] [IDEMPOTENT REPLAY] To: %s, Subject: %s, Key: %s",
				msg.To, msg.Subject, msg.IdempotencyKey)
			return nil
		}
		p.sentKeys[msg.IdempotencyKey] = struct{}{}
	}

	p.sentEmails = append(p.sentEmails, msg)
	log.Printf("[DEV LOG EMAIL] To: %s, Subject: %s, Key: %s", msg.To, msg.Subject, msg.IdempotencyKey)
	return nil
}

// SendSMS logs the SMS dispatch and records the message in memory.
func (p *LoggingNotificationProvider) SendSMS(ctx context.Context, msg SMSMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if msg.IdempotencyKey != "" {
		if _, exists := p.sentKeys[msg.IdempotencyKey]; exists {
			log.Printf("[DEV LOG SMS] [IDEMPOTENT REPLAY] To: %s, Key: %s", msg.To, msg.IdempotencyKey)
			return nil
		}
		p.sentKeys[msg.IdempotencyKey] = struct{}{}
	}

	p.sentSMS = append(p.sentSMS, msg)
	log.Printf("[DEV LOG SMS] To: %s, Body: %s, Key: %s", msg.To, msg.Body, msg.IdempotencyKey)
	return nil
}

// SendPush logs the Push notification dispatch and records the message in memory.
func (p *LoggingNotificationProvider) SendPush(ctx context.Context, msg PushMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if msg.IdempotencyKey != "" {
		if _, exists := p.sentKeys[msg.IdempotencyKey]; exists {
			log.Printf("[DEV LOG PUSH] [IDEMPOTENT REPLAY] Token: %s, Key: %s", msg.Token, msg.IdempotencyKey)
			return nil
		}
		p.sentKeys[msg.IdempotencyKey] = struct{}{}
	}

	p.sentPush = append(p.sentPush, msg)
	log.Printf("[DEV LOG PUSH] Token: %s, Title: %s, Key: %s", msg.Token, msg.Title, msg.IdempotencyKey)
	return nil
}

// SentEmails returns a snapshot copy of all sent emails.
func (p *LoggingNotificationProvider) SentEmails() []EmailMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	copied := make([]EmailMessage, len(p.sentEmails))
	copy(copied, p.sentEmails)
	return copied
}

// SentSMS returns a snapshot copy of all sent SMS messages.
func (p *LoggingNotificationProvider) SentSMS() []SMSMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	copied := make([]SMSMessage, len(p.sentSMS))
	copy(copied, p.sentSMS)
	return copied
}

// SentPush returns a snapshot copy of all sent push messages.
func (p *LoggingNotificationProvider) SentPush() []PushMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	copied := make([]PushMessage, len(p.sentPush))
	copy(copied, p.sentPush)
	return copied
}

// Reset clears all in-memory recorded messages and deduplication keys.
func (p *LoggingNotificationProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sentEmails = make([]EmailMessage, 0)
	p.sentSMS = make([]SMSMessage, 0)
	p.sentPush = make([]PushMessage, 0)
	p.sentKeys = make(map[string]struct{})
}
