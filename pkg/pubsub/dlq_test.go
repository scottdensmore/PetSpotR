package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordPublisher struct {
	mu        sync.Mutex
	published []struct {
		Topic string
		Data  []byte
	}
	err error
}

func (p *recordPublisher) Publish(_ context.Context, topic string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, struct {
		Topic string
		Data  []byte
	}{
		Topic: topic,
		Data:  bytes.Clone(data),
	})
	return nil
}

func (p *recordPublisher) getPublished() []struct {
	Topic string
	Data  []byte
} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]struct {
		Topic string
		Data  []byte
	}, len(p.published))
	copy(out, p.published)
	return out
}

func TestCanonicalDLQTopics(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"lost pet", LostPetDLQTopic, "lostPet-dlq"},
		{"found pet", FoundPetDLQTopic, "foundPet-dlq"},
		{"match found", MatchFoundDLQTopic, "matchFound-dlq"},
		{"pet status changed", PetStatusChangedDLQTopic, "petStatusChanged-dlq"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("topic = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestWithDeadLetterQueue(t *testing.T) {
	t.Run("nil handler returns nil", func(t *testing.T) {
		wrapped := WithDeadLetterQueue(&recordPublisher{}, LostPetDLQTopic, nil)
		if wrapped != nil {
			t.Fatal("expected nil wrapped handler")
		}
	})

	t.Run("handler success does not publish to dlq", func(t *testing.T) {
		pub := &recordPublisher{}
		handler := func(ctx context.Context, data []byte) error {
			return nil
		}

		wrapped := WithDeadLetterQueue(pub, LostPetDLQTopic, handler)
		err := wrapped(context.Background(), []byte("ok"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pub.getPublished()) != 0 {
			t.Fatalf("expected 0 published messages, got %d", len(pub.getPublished()))
		}
	})

	t.Run("handler failure routes to dlq and acknowledges message", func(t *testing.T) {
		pub := &recordPublisher{}
		origPayload := []byte(`{"id":"msg-123","petId":"p-456","location":"park"}`)
		handlerErr := errors.New("bad payload schema")

		handler := func(ctx context.Context, data []byte) error {
			return handlerErr
		}

		wrapped := WithDeadLetterQueue(pub, LostPetDLQTopic, handler)
		// Failure should return nil so message is acknowledged
		err := wrapped(context.Background(), origPayload)
		if err != nil {
			t.Fatalf("expected nil return to acknowledge message, got: %v", err)
		}

		published := pub.getPublished()
		if len(published) != 1 {
			t.Fatalf("expected 1 published message to DLQ, got %d", len(published))
		}
		if published[0].Topic != LostPetDLQTopic {
			t.Fatalf("published topic = %q, want %q", published[0].Topic, LostPetDLQTopic)
		}

		var env DeadLetterEnvelope
		if err := json.Unmarshal(published[0].Data, &env); err != nil {
			t.Fatalf("unmarshal dead letter envelope: %v", err)
		}

		if !bytes.Equal(env.OriginalPayload, origPayload) {
			t.Fatalf("envelope OriginalPayload = %s, want %s", string(env.OriginalPayload), string(origPayload))
		}
		if env.LastError != handlerErr.Error() {
			t.Fatalf("envelope LastError = %q, want %q", env.LastError, handlerErr.Error())
		}
		if env.Attempts != 1 {
			t.Fatalf("envelope Attempts = %d, want 1", env.Attempts)
		}
		if env.OriginalTopic != "lostPet" {
			t.Fatalf("envelope OriginalTopic = %q, want lostPet", env.OriginalTopic)
		}
		if env.MessageID != "msg-123" {
			t.Fatalf("envelope MessageID = %q, want msg-123", env.MessageID)
		}
		if env.FailedAt.IsZero() {
			t.Fatal("envelope FailedAt should not be zero")
		}
	})

	t.Run("explicit context metadata takes precedence", func(t *testing.T) {
		pub := &recordPublisher{}
		payload := []byte("plain-payload")

		handler := func(ctx context.Context, data []byte) error {
			return errors.New("cannot parse")
		}

		ctx := context.Background()
		ctx = ContextWithTopic(ctx, "custom-topic")
		ctx = ContextWithMessageID(ctx, "custom-msg-id")
		ctx = ContextWithAttempts(ctx, 5)

		wrapped := WithDeadLetterQueue(pub, MatchFoundDLQTopic, handler)
		err := wrapped(ctx, payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		published := pub.getPublished()
		if len(published) != 1 {
			t.Fatalf("expected 1 DLQ delivery, got %d", len(published))
		}

		var env DeadLetterEnvelope
		if err := json.Unmarshal(published[0].Data, &env); err != nil {
			t.Fatalf("unmarshal DLQ envelope: %v", err)
		}
		if env.OriginalTopic != "custom-topic" {
			t.Fatalf("OriginalTopic = %q, want custom-topic", env.OriginalTopic)
		}
		if env.MessageID != "custom-msg-id" {
			t.Fatalf("MessageID = %q, want custom-msg-id", env.MessageID)
		}
		if env.Attempts != 5 {
			t.Fatalf("Attempts = %d, want 5", env.Attempts)
		}
	})

	t.Run("publisher error returns error", func(t *testing.T) {
		pubErr := errors.New("pubsub broker connection refused")
		pub := &recordPublisher{err: pubErr}

		handler := func(ctx context.Context, data []byte) error {
			return errors.New("handler failed")
		}

		wrapped := WithDeadLetterQueue(pub, LostPetDLQTopic, handler)
		err := wrapped(context.Background(), []byte("fail"))
		if err == nil {
			t.Fatal("expected error when publisher fails, got nil")
		}
		if !errors.Is(err, pubErr) {
			t.Fatalf("err = %v, want to contain %v", err, pubErr)
		}
	})

	t.Run("nil publisher returns error when handler fails", func(t *testing.T) {
		handler := func(ctx context.Context, data []byte) error {
			return errors.New("handler failed")
		}

		wrapped := WithDeadLetterQueue(nil, LostPetDLQTopic, handler)
		err := wrapped(context.Background(), []byte("fail"))
		if err == nil {
			t.Fatal("expected error with nil publisher on failure")
		}
	})

	t.Run("composition with WithRetry exhausts retries and forwards to dlq", func(t *testing.T) {
		pub := &recordPublisher{}
		var handlerCalls int
		handlerErr := errors.New("database timeout")

		handler := func(ctx context.Context, data []byte) error {
			handlerCalls++
			return handlerErr
		}

		policy := RetryPolicy{
			MaxAttempts:     3,
			InitialInterval: time.Millisecond,
		}

		resilientHandler := WithDeadLetterQueue(pub, FoundPetDLQTopic, WithRetry(policy, handler))
		payload := []byte(`{"messageId":"msg-found-99","payload":"test"}`)

		err := resilientHandler(context.Background(), payload)
		if err != nil {
			t.Fatalf("expected nil acknowledgement after DLQ routing, got: %v", err)
		}

		if handlerCalls != 3 {
			t.Fatalf("handlerCalls = %d, want 3", handlerCalls)
		}

		published := pub.getPublished()
		if len(published) != 1 {
			t.Fatalf("expected 1 DLQ message, got %d", len(published))
		}
		if published[0].Topic != FoundPetDLQTopic {
			t.Fatalf("topic = %q, want %q", published[0].Topic, FoundPetDLQTopic)
		}

		var env DeadLetterEnvelope
		if err := env.FromJSON(published[0].Data); err != nil {
			t.Fatalf("FromJSON error: %v", err)
		}
		if env.Attempts != 3 {
			t.Fatalf("Attempts = %d, want 3", env.Attempts)
		}
		if env.LastError != handlerErr.Error() {
			t.Fatalf("LastError = %q, want %q", env.LastError, handlerErr.Error())
		}
		if !bytes.Equal(env.OriginalPayload, payload) {
			t.Fatalf("OriginalPayload mismatch")
		}
		if env.OriginalTopic != "foundPet" {
			t.Fatalf("OriginalTopic = %q, want foundPet", env.OriginalTopic)
		}
		if env.MessageID != "msg-found-99" {
			t.Fatalf("MessageID = %q, want msg-found-99", env.MessageID)
		}
	})
}
