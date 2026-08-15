package notification

import (
	"context"
	"testing"
)

func TestMultiChannelDispatcher(t *testing.T) {
	emailSender := NewMockEmailSender()
	smsSender := NewMockSMSSender()
	pushSender := NewMockWebPushSender()

	dispatcher := NewMultiChannelDispatcher(emailSender, smsSender, pushSender)

	t.Run("dispatches notification across configured channels", func(t *testing.T) {
		msg := &NotificationMessage{
			RecipientID: "user-101",
			Email:       "owner@example.com",
			Phone:       "+12065550199",
			PushToken:   "push-token-xyz",
			Subject:     "Match Found for Buddy",
			Body:        "A high confidence match (95%) was found for your pet Buddy.",
			Channels:    []Channel{ChannelEmail, ChannelSMS, ChannelPush},
		}

		results, err := dispatcher.Dispatch(context.Background(), msg)
		if err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("expected 3 dispatch results, got %d", len(results))
		}

		for _, res := range results {
			if !res.Success {
				t.Errorf("expected channel %s dispatch to succeed", res.Channel)
			}
		}

		if len(emailSender.SentMessages) != 1 {
			t.Errorf("expected 1 email sent, got %d", len(emailSender.SentMessages))
		}
		if len(smsSender.SentMessages) != 1 {
			t.Errorf("expected 1 SMS sent, got %d", len(smsSender.SentMessages))
		}
		if len(pushSender.SentMessages) != 1 {
			t.Errorf("expected 1 Web Push sent, got %d", len(pushSender.SentMessages))
		}
	})

	t.Run("defaults to Email when no channels specified", func(t *testing.T) {
		emailSender.Reset()
		msg := &NotificationMessage{
			RecipientID: "user-102",
			Email:       "owner2@example.com",
			Subject:     "Match Found",
			Body:        "Pet match found",
			Channels:    []Channel{},
		}

		results, err := dispatcher.Dispatch(context.Background(), msg)
		if err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}

		if len(results) != 1 || results[0].Channel != ChannelEmail {
			t.Errorf("expected default Email channel dispatch, got %v", results)
		}
	})

	t.Run("provider retry with the same idempotency key has one side effect", func(t *testing.T) {
		emailSender.Reset()
		msg := &NotificationMessage{
			RecipientID:    "user-idempotent",
			Email:          "owner@example.com",
			Subject:        "Match Found",
			Body:           "Pet match found",
			Channels:       []Channel{ChannelEmail},
			IdempotencyKey: "delivery-stable-key",
		}
		for range 2 {
			results, err := dispatcher.Dispatch(context.Background(), msg)
			if err != nil || len(results) != 1 || !results[0].Success {
				t.Fatalf("Dispatch(idempotent retry) = %#v, %v", results, err)
			}
		}
		if len(emailSender.SentMessages) != 1 {
			t.Fatalf("provider side effects = %d, want 1", len(emailSender.SentMessages))
		}
	})
}
