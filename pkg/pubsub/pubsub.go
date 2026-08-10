package pubsub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrNoSubscribers prevents an in-memory broker from falsely acknowledging an
// event that no local consumer received.
var ErrNoSubscribers = errors.New("pubsub: topic has no subscribers")

// Handler handles an incoming pub/sub event message payload.
type Handler func(ctx context.Context, data []byte) error

// Broker defines publish/subscribe messaging operations.
type Broker interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(topic string, handler Handler) error
}

// TopicAvailability is implemented by brokers that can determine locally
// whether publishing a topic can currently reach any consumer.
type TopicAvailability interface {
	HasSubscribers(topic string) bool
}

// PubSubBroker is an alias for Broker.
type PubSubBroker = Broker

// MemoryPubSub implements Broker in memory for testing and local dev.
type MemoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
}

// NewMemoryPubSub constructs a thread-safe MemoryPubSub instance.
func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		subscribers: make(map[string][]Handler),
	}
}

// Publish publishes a cloned message payload to all subscribers of a topic, collecting errors.
func (m *MemoryPubSub) Publish(ctx context.Context, topic string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.RLock()
	handlers := append([]Handler(nil), m.subscribers[topic]...)
	m.mu.RUnlock()
	if len(handlers) == 0 {
		return fmt.Errorf("%w: %s", ErrNoSubscribers, topic)
	}

	var errs []error
	for _, h := range handlers {
		payload := bytes.Clone(data)
		if err := h(ctx, payload); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Subscribe registers an event handler for a topic.
func (m *MemoryPubSub) Subscribe(topic string, handler Handler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribers[topic] = append(m.subscribers[topic], handler)
	return nil
}

// HasSubscribers reports whether this process has a handler for topic.
func (m *MemoryPubSub) HasSubscribers(topic string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.subscribers[topic]) > 0
}
