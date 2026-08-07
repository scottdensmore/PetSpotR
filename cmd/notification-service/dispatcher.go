package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
)

// Channel defines supported notification delivery channels.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

// NotificationMessage encapsulates multi-channel notification details.
type NotificationMessage struct {
	RecipientID string    `json:"recipientId"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	PushToken   string    `json:"pushToken"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	Channels    []Channel `json:"channels"`
}

// DispatchResult tracks delivery outcome for a specific channel.
type DispatchResult struct {
	Channel Channel `json:"channel"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
}

// ChannelSender defines the interface for channel-specific senders.
type ChannelSender interface {
	Send(ctx context.Context, msg *NotificationMessage) error
	Channel() Channel
}

// MockEmailSender handles email delivery in dev/test environments.
type MockEmailSender struct {
	mu           sync.Mutex
	SentMessages []*NotificationMessage
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{SentMessages: make([]*NotificationMessage, 0)}
}

func (e *MockEmailSender) Channel() Channel { return ChannelEmail }

func (e *MockEmailSender) Send(ctx context.Context, msg *NotificationMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(msg.Email) == "" {
		return fmt.Errorf("email address missing")
	}
	e.SentMessages = append(e.SentMessages, msg)
	log.Printf("[EMAIL SENDER] Sent to %s: %s", msg.Email, msg.Subject)
	return nil
}

func (e *MockEmailSender) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.SentMessages = make([]*NotificationMessage, 0)
}

// MockSMSSender handles SMS text message delivery.
type MockSMSSender struct {
	mu           sync.Mutex
	SentMessages []*NotificationMessage
}

func NewMockSMSSender() *MockSMSSender {
	return &MockSMSSender{SentMessages: make([]*NotificationMessage, 0)}
}

func (s *MockSMSSender) Channel() Channel { return ChannelSMS }

func (s *MockSMSSender) Send(ctx context.Context, msg *NotificationMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(msg.Phone) == "" {
		return fmt.Errorf("phone number missing for SMS delivery")
	}
	s.SentMessages = append(s.SentMessages, msg)
	log.Printf("[SMS SENDER] Text sent to %s: %s", msg.Phone, msg.Body)
	return nil
}

func (s *MockSMSSender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SentMessages = make([]*NotificationMessage, 0)
}

// MockWebPushSender handles Web Push notifications.
type MockWebPushSender struct {
	mu           sync.Mutex
	SentMessages []*NotificationMessage
}

func NewMockWebPushSender() *MockWebPushSender {
	return &MockWebPushSender{SentMessages: make([]*NotificationMessage, 0)}
}

func (p *MockWebPushSender) Channel() Channel { return ChannelPush }

func (p *MockWebPushSender) Send(ctx context.Context, msg *NotificationMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SentMessages = append(p.SentMessages, msg)
	log.Printf("[WEB PUSH SENDER] Push alert sent to token %s: %s", msg.PushToken, msg.Subject)
	return nil
}

func (p *MockWebPushSender) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SentMessages = make([]*NotificationMessage, 0)
}

// MultiChannelDispatcher coordinates delivery across multiple notification channels.
type MultiChannelDispatcher struct {
	senders map[Channel]ChannelSender
}

// NewMultiChannelDispatcher constructs a MultiChannelDispatcher with given senders.
func NewMultiChannelDispatcher(senders ...ChannelSender) *MultiChannelDispatcher {
	m := make(map[Channel]ChannelSender)
	for _, s := range senders {
		m[s.Channel()] = s
	}
	return &MultiChannelDispatcher{senders: m}
}

// Dispatch routes the notification message across all requested channels.
func (d *MultiChannelDispatcher) Dispatch(ctx context.Context, msg *NotificationMessage) ([]DispatchResult, error) {
	channels := msg.Channels
	if len(channels) == 0 {
		channels = []Channel{ChannelEmail}
	}

	results := make([]DispatchResult, 0, len(channels))
	for _, ch := range channels {
		sender, ok := d.senders[ch]
		if !ok {
			results = append(results, DispatchResult{
				Channel: ch,
				Success: false,
				Error:   fmt.Sprintf("no sender registered for channel %s", ch),
			})
			continue
		}

		err := sender.Send(ctx, msg)
		if err != nil {
			results = append(results, DispatchResult{
				Channel: ch,
				Success: false,
				Error:   err.Error(),
			})
		} else {
			results = append(results, DispatchResult{
				Channel: ch,
				Success: true,
			})
		}
	}

	return results, nil
}
