package notification

import (
	"context"
	"testing"
)

func TestLoggingNotificationProvider(t *testing.T) {
	provider := NewLoggingNotificationProvider()
	ctx := context.Background()

	t.Run("records and logs email delivery", func(t *testing.T) {
		provider.Reset()
		email := EmailMessage{
			To:             "owner@example.com",
			Subject:        "Match Found for Buddy",
			TextBody:       "Match alert text body",
			HTMLBody:       "<p>Match alert html body</p>",
			IdempotencyKey: "key-email-1",
		}

		if err := provider.SendEmail(ctx, email); err != nil {
			t.Fatalf("SendEmail failed: %v", err)
		}

		sent := provider.SentEmails()
		if len(sent) != 1 {
			t.Fatalf("expected 1 sent email, got %d", len(sent))
		}
		if sent[0].To != email.To || sent[0].Subject != email.Subject {
			t.Errorf("unexpected recorded email: %+v", sent[0])
		}
	})

	t.Run("records and logs SMS delivery", func(t *testing.T) {
		provider.Reset()
		sms := SMSMessage{
			To:             "+12065550199",
			Body:           "PetSpotR: Match found for Buddy",
			IdempotencyKey: "key-sms-1",
		}

		if err := provider.SendSMS(ctx, sms); err != nil {
			t.Fatalf("SendSMS failed: %v", err)
		}

		sent := provider.SentSMS()
		if len(sent) != 1 {
			t.Fatalf("expected 1 sent SMS, got %d", len(sent))
		}
		if sent[0].To != sms.To || sent[0].Body != sms.Body {
			t.Errorf("unexpected recorded SMS: %+v", sent[0])
		}
	})

	t.Run("records and logs Push delivery", func(t *testing.T) {
		provider.Reset()
		push := PushMessage{
			Token:          "push-token-123",
			Title:          "Match Alert",
			Body:           "Match found for pet",
			IdempotencyKey: "key-push-1",
		}

		if err := provider.SendPush(ctx, push); err != nil {
			t.Fatalf("SendPush failed: %v", err)
		}

		sent := provider.SentPush()
		if len(sent) != 1 {
			t.Fatalf("expected 1 sent push, got %d", len(sent))
		}
		if sent[0].Token != push.Token || sent[0].Title != push.Title {
			t.Errorf("unexpected recorded push: %+v", sent[0])
		}
	})

	t.Run("enforces idempotency: duplicate key is no-op without duplicate side effects", func(t *testing.T) {
		provider.Reset()
		email := EmailMessage{
			To:             "owner@example.com",
			Subject:        "Match Found",
			TextBody:       "Text",
			IdempotencyKey: "stable-idempotency-key",
		}

		for range 3 {
			if err := provider.SendEmail(ctx, email); err != nil {
				t.Fatalf("idempotent send failed: %v", err)
			}
		}

		sent := provider.SentEmails()
		if len(sent) != 1 {
			t.Fatalf("expected exactly 1 recorded email across 3 replays, got %d", len(sent))
		}
	})
}

func TestNewProvidersFromConfig_DevFallback(t *testing.T) {
	t.Run("falls back to logging providers when credentials are unset", func(t *testing.T) {
		emptyLookup := func(string) string { return "" }
		cfg := LoadProviderConfig(emptyLookup)

		email, sms, push := NewProvidersFromConfig(cfg)

		if email == nil || sms == nil || push == nil {
			t.Fatal("expected non-nil providers for all channels")
		}

		// Ensure that the email provider is indeed a LoggingNotificationProvider
		loggingEmail, ok := email.(*LoggingNotificationProvider)
		if !ok {
			t.Errorf("expected email provider to be *LoggingNotificationProvider, got %T", email)
		} else {
			ctx := context.Background()
			_ = loggingEmail.SendEmail(ctx, EmailMessage{
				To:             "dev@example.com",
				Subject:        "Dev Test",
				IdempotencyKey: "dev-key",
			})
			if len(loggingEmail.SentEmails()) != 1 {
				t.Errorf("expected 1 sent email in dev mode, got %d", len(loggingEmail.SentEmails()))
			}
		}

		loggingSMS, ok := sms.(*LoggingNotificationProvider)
		if !ok {
			t.Errorf("expected SMS provider to be *LoggingNotificationProvider, got %T", sms)
		} else {
			ctx := context.Background()
			_ = loggingSMS.SendSMS(ctx, SMSMessage{
				To:             "+12065550199",
				Body:           "Dev SMS",
				IdempotencyKey: "dev-key",
			})
			if len(loggingSMS.SentSMS()) != 1 {
				t.Errorf("expected 1 sent SMS in dev mode, got %d", len(loggingSMS.SentSMS()))
			}
		}
	})
}
