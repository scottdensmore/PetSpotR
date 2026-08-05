package pubsub

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

// Handler handles an incoming pub/sub event message payload.
type Handler func(ctx context.Context, data []byte) error

// Broker defines publish/subscribe messaging operations.
type Broker interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(topic string, handler Handler) error
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
