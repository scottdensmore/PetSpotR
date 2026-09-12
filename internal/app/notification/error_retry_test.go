package notification

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type failFirstTestEmailProvider struct {
	mu            sync.Mutex
	failRemaining int
	attempts      int
	keys          []string
}

func (p *failFirstTestEmailProvider) SendEmail(_ context.Context, msg EmailMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	p.keys = append(p.keys, msg.IdempotencyKey)
	if p.failRemaining > 0 {
		p.failRemaining--
		return errors.New("sendgrid: transient 503 service unavailable")
	}
	return nil
}

type failFirstTestSMSProvider struct {
	mu            sync.Mutex
	failRemaining int
	attempts      int
	keys          []string
}

func (p *failFirstTestSMSProvider) SendSMS(_ context.Context, msg SMSMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	p.keys = append(p.keys, msg.IdempotencyKey)
	if p.failRemaining > 0 {
		p.failRemaining--
		return errors.New("twilio: transient 500 error")
	}
	return nil
}

func TestWorker_DeliveryErrorHandlingAndRetrySignaling(t *testing.T) {
	stateStore := store.NewMemoryStore()
	emailProvider := &failFirstTestEmailProvider{failRemaining: 1}
	smsProvider := &failFirstTestSMSProvider{failRemaining: 0}
	pushProvider := NewLoggingPushProvider()

	dispatcher := NewMultiChannelDispatcherWithProviders(
		emailProvider,
		smsProvider,
		pushProvider,
		"alerts@petspotr.io",
		"+15005550006",
		NewSMSFormatter(),
	)

	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	now := time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)
	worker.now = func() time.Time { return now }
	worker.deliveryLease = 5 * time.Second

	data := matchFoundEnvelope(t, "found-retry-test", "lost-retry-test")

	// Attempt 1: should fail due to email provider error
	_, err := worker.ProcessMatchFound(context.Background(), data)
	if err == nil {
		t.Fatal("expected error on attempt 1, got nil")
	}
	if emailProvider.attempts != 1 {
		t.Fatalf("expected 1 email attempt, got %d", emailProvider.attempts)
	}

	// Advance time past lease expiration
	now = now.Add(10 * time.Second)

	// Attempt 2: retry should succeed
	notif, err := worker.ProcessMatchFound(context.Background(), data)
	if err != nil {
		t.Fatalf("expected attempt 2 to succeed, got %v", err)
	}
	if notif == nil {
		t.Fatal("expected non-nil notification on success")
	}

	if emailProvider.attempts != 2 {
		t.Fatalf("expected 2 email attempts, got %d", emailProvider.attempts)
	}

	// Verify the provider received the EXACT same idempotency key across retries
	if len(emailProvider.keys) != 2 {
		t.Fatalf("expected 2 keys recorded, got %d", len(emailProvider.keys))
	}
	if emailProvider.keys[0] != emailProvider.keys[1] {
		t.Errorf("idempotency key changed across retries: attempt1=%q, attempt2=%q",
			emailProvider.keys[0], emailProvider.keys[1])
	}

	// Third attempt (replay after completion): should succeed immediately skipping channels
	_, err = worker.ProcessMatchFound(context.Background(), data)
	if err != nil {
		t.Fatalf("expected completed replay to succeed, got %v", err)
	}
	// Email provider shouldn't be called again
	if emailProvider.attempts != 2 {
		t.Errorf("expected email provider not to be called after completion, attempts=%d", emailProvider.attempts)
	}
}

func TestWorker_PartialFailureSkipsCompletedChannelsOnRetry(t *testing.T) {
	stateStore := store.NewMemoryStore()
	// Email succeeds on first attempt, SMS fails on first attempt
	emailProvider := &failFirstTestEmailProvider{failRemaining: 0}
	smsProvider := &failFirstTestSMSProvider{failRemaining: 1}
	pushProvider := NewLoggingPushProvider()

	dispatcher := NewMultiChannelDispatcherWithProviders(
		emailProvider,
		smsProvider,
		pushProvider,
		"alerts@petspotr.io",
		"+15005550006",
		NewSMSFormatter(),
	)

	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	now := time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)
	worker.now = func() time.Time { return now }
	worker.deliveryLease = 5 * time.Second

	data := matchFoundEnvelope(t, "found-partial-test", "lost-partial-test")

	// Attempt 1: fails because SMS fails
	_, err := worker.ProcessMatchFound(context.Background(), data)
	if err == nil {
		t.Fatal("expected attempt 1 to fail due to SMS failure")
	}
	if emailProvider.attempts != 1 {
		t.Errorf("expected 1 email attempt, got %d", emailProvider.attempts)
	}
	if smsProvider.attempts != 1 {
		t.Errorf("expected 1 SMS attempt, got %d", smsProvider.attempts)
	}

	// Advance time past lease
	now = now.Add(10 * time.Second)

	// Attempt 2: redelivery
	_, err = worker.ProcessMatchFound(context.Background(), data)
	if err != nil {
		t.Fatalf("expected attempt 2 to succeed, got: %v", err)
	}

	// Email was already completed, so it must NOT be called again
	if emailProvider.attempts != 1 {
		t.Errorf("expected completed email channel not to be called again, attempts=%d", emailProvider.attempts)
	}
	// SMS failed previously, so it was retried and now has 2 attempts
	if smsProvider.attempts != 2 {
		t.Errorf("expected SMS channel to be retried, attempts=%d", smsProvider.attempts)
	}
	// SMS idempotency keys must match
	if smsProvider.keys[0] != smsProvider.keys[1] {
		t.Errorf("SMS idempotency keys do not match: %q != %q", smsProvider.keys[0], smsProvider.keys[1])
	}
}
