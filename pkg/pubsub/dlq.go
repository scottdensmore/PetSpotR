package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Canonical dead-letter queue topics for PetSpotR event pipelines.
const (
	LostPetDLQTopic          = "lostPet-dlq"
	FoundPetDLQTopic         = "foundPet-dlq"
	MatchFoundDLQTopic       = "matchFound-dlq"
	PetStatusChangedDLQTopic = "petStatusChanged-dlq"
)

// DeadLetterEnvelope wraps a poison-pill message with delivery failure context.
type DeadLetterEnvelope struct {
	OriginalPayload []byte    `json:"originalPayload"`
	OriginalTopic   string    `json:"originalTopic"`
	FailedAt        time.Time `json:"failedAt"`
	Attempts        int       `json:"attempts"`
	LastError       string    `json:"lastError"`
	MessageID       string    `json:"messageId"`
}

// ToJSON serializes DeadLetterEnvelope to JSON bytes.
func (e *DeadLetterEnvelope) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes JSON bytes into DeadLetterEnvelope.
func (e *DeadLetterEnvelope) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

type contextKey string

const (
	attemptsTrackerKey = contextKey("pubsub.attemptsTracker")
	attemptsKey        = contextKey("pubsub.attempts")
	topicKey           = contextKey("pubsub.topic")
	messageIDKey       = contextKey("pubsub.messageID")
)

type attemptsTracker struct {
	count int
}

// ContextWithTopic returns a context decorated with the original event topic.
func ContextWithTopic(ctx context.Context, topic string) context.Context {
	return context.WithValue(ctx, topicKey, topic)
}

// ContextWithMessageID returns a context decorated with the message ID.
func ContextWithMessageID(ctx context.Context, messageID string) context.Context {
	return context.WithValue(ctx, messageIDKey, messageID)
}

// ContextWithAttempts returns a context decorated with an explicit attempt count.
func ContextWithAttempts(ctx context.Context, attempts int) context.Context {
	return context.WithValue(ctx, attemptsKey, attempts)
}

// WithDeadLetterQueue executes handler. If handler fails, wraps error and payload
// into DeadLetterEnvelope, marshals to JSON, publishes to dlqTopic, and returns nil
// so the poison-pill message is acknowledged and does not block the pipeline.
func WithDeadLetterQueue(publisher Publisher, dlqTopic string, handler Handler) Handler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, data []byte) error {
		tracker := &attemptsTracker{count: 1}
		ctxWithTracker := context.WithValue(ctx, attemptsTrackerKey, tracker)

		err := handler(ctxWithTracker, data)
		if err == nil {
			return nil
		}

		if publisher == nil {
			return fmt.Errorf("pubsub: publisher is nil, cannot route to DLQ %s: %w", dlqTopic, err)
		}

		attempts := tracker.count
		if explicitAttempts, ok := ctx.Value(attemptsKey).(int); ok && explicitAttempts > 0 {
			attempts = explicitAttempts
		}

		origTopic := ""
		if t, ok := ctx.Value(topicKey).(string); ok && t != "" {
			origTopic = t
		} else if strings.HasSuffix(dlqTopic, "-dlq") {
			origTopic = strings.TrimSuffix(dlqTopic, "-dlq")
		}

		messageID := ""
		if m, ok := ctx.Value(messageIDKey).(string); ok && m != "" {
			messageID = m
		} else {
			var probe struct {
				ID        string `json:"id"`
				MessageID string `json:"messageId"`
			}
			if jsonErr := json.Unmarshal(data, &probe); jsonErr == nil {
				if probe.MessageID != "" {
					messageID = probe.MessageID
				} else if probe.ID != "" {
					messageID = probe.ID
				}
			}
		}

		envelope := DeadLetterEnvelope{
			OriginalPayload: bytes.Clone(data),
			OriginalTopic:   origTopic,
			FailedAt:        time.Now().UTC(),
			Attempts:        attempts,
			LastError:       err.Error(),
			MessageID:       messageID,
		}

		dlqBytes, marshalErr := json.Marshal(envelope)
		if marshalErr != nil {
			return fmt.Errorf("pubsub: marshal dead letter envelope: %w", marshalErr)
		}

		if pubErr := publisher.Publish(ctx, dlqTopic, dlqBytes); pubErr != nil {
			return fmt.Errorf("pubsub: publish dead letter to %s: %w", dlqTopic, pubErr)
		}

		return nil
	}
}
