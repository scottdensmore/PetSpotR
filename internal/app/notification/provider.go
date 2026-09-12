package notification

import (
	"context"
	"fmt"
	"strings"
)

// EmailMessage defines standard email dispatch parameters.
type EmailMessage struct {
	To             string            `json:"to"`
	From           string            `json:"from,omitempty"`
	Subject        string            `json:"subject"`
	TextBody       string            `json:"textBody"`
	HTMLBody       string            `json:"htmlBody"`
	IdempotencyKey string            `json:"idempotencyKey,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// SMSMessage defines standard SMS text dispatch parameters.
type SMSMessage struct {
	To             string `json:"to"`
	From           string `json:"from,omitempty"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// PushMessage defines standard push notification dispatch parameters.
type PushMessage struct {
	Token          string            `json:"token"`
	Title          string            `json:"title"`
	Body           string            `json:"body"`
	Data           map[string]string `json:"data,omitempty"`
	IdempotencyKey string            `json:"idempotencyKey,omitempty"`
}

// EmailProvider delivers email messages via a backend provider.
type EmailProvider interface {
	SendEmail(ctx context.Context, msg EmailMessage) error
}

// SMSProvider delivers SMS messages via a backend provider.
type SMSProvider interface {
	SendSMS(ctx context.Context, msg SMSMessage) error
}

// PushProvider delivers push notifications via a backend provider.
type PushProvider interface {
	SendPush(ctx context.Context, msg PushMessage) error
}

// ProviderEmailSender adapts an EmailProvider to ChannelSender.
type ProviderEmailSender struct {
	provider  EmailProvider
	fromEmail string
}

// NewProviderEmailSender constructs an email channel sender backed by an EmailProvider.
func NewProviderEmailSender(provider EmailProvider, fromEmail string) *ProviderEmailSender {
	if fromEmail == "" {
		fromEmail = "alerts@petspotr.io"
	}
	return &ProviderEmailSender{
		provider:  provider,
		fromEmail: fromEmail,
	}
}

// Channel returns ChannelEmail.
func (s *ProviderEmailSender) Channel() Channel { return ChannelEmail }

// Send delivers the notification as an email through the underlying EmailProvider.
func (s *ProviderEmailSender) Send(ctx context.Context, msg *NotificationMessage) error {
	if strings.TrimSpace(msg.Email) == "" {
		return fmt.Errorf("email address missing")
	}
	htmlBody := msg.HTMLBody
	if htmlBody == "" {
		htmlBody = msg.Body
	}
	textBody := msg.TextBody
	if textBody == "" {
		textBody = msg.Body
	}
	return s.provider.SendEmail(ctx, EmailMessage{
		To:             msg.Email,
		From:           s.fromEmail,
		Subject:        msg.Subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		IdempotencyKey: msg.IdempotencyKey,
	})
}

// ProviderSMSSender adapts an SMSProvider to ChannelSender with phone number formatting.
type ProviderSMSSender struct {
	provider  SMSProvider
	formatter *SMSFormatter
	fromPhone string
}

// NewProviderSMSSender constructs an SMS channel sender backed by an SMSProvider.
func NewProviderSMSSender(provider SMSProvider, formatter *SMSFormatter, fromPhone string) *ProviderSMSSender {
	if formatter == nil {
		formatter = NewSMSFormatter()
	}
	return &ProviderSMSSender{
		provider:  provider,
		formatter: formatter,
		fromPhone: fromPhone,
	}
}

// Channel returns ChannelSMS.
func (s *ProviderSMSSender) Channel() Channel { return ChannelSMS }

// Send delivers the notification as an SMS through the underlying SMSProvider.
func (s *ProviderSMSSender) Send(ctx context.Context, msg *NotificationMessage) error {
	if strings.TrimSpace(msg.Phone) == "" {
		return fmt.Errorf("phone number missing for SMS delivery")
	}
	formattedPhone, err := s.formatter.FormatPhoneNumber(msg.Phone)
	if err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}
	body := msg.SMSBody
	if body == "" {
		body = msg.Body
	}
	body = s.formatter.ConstrainLength(body)
	return s.provider.SendSMS(ctx, SMSMessage{
		To:             formattedPhone,
		From:           s.fromPhone,
		Body:           body,
		IdempotencyKey: msg.IdempotencyKey,
	})
}

// ProviderPushSender adapts a PushProvider to ChannelSender.
type ProviderPushSender struct {
	provider PushProvider
}

// NewProviderPushSender constructs a push channel sender backed by a PushProvider.
func NewProviderPushSender(provider PushProvider) *ProviderPushSender {
	return &ProviderPushSender{provider: provider}
}

// Channel returns ChannelPush.
func (s *ProviderPushSender) Channel() Channel { return ChannelPush }

// Send delivers the notification as a push notification through the underlying PushProvider.
func (s *ProviderPushSender) Send(ctx context.Context, msg *NotificationMessage) error {
	return s.provider.SendPush(ctx, PushMessage{
		Token:          msg.PushToken,
		Title:          msg.Subject,
		Body:           msg.Body,
		IdempotencyKey: msg.IdempotencyKey,
	})
}
