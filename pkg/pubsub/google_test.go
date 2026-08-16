package pubsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestGooglePublisherWaitsForAcknowledgementAndReusesTopic(t *testing.T) {
	t.Parallel()

	topic := &stubGoogleTopic{
		publishStarted: make(chan struct{}, 2),
		acknowledged:   make(chan struct{}),
	}
	var factoryCalls int
	var factoryTopicID string
	publisher := newGooglePublisher(
		func(topicID string) googleTopic {
			factoryCalls++
			factoryTopicID = topicID
			return topic
		},
		func() error { return nil },
	)

	payload := []byte("lost-pet-event")
	done := make(chan error, 1)
	go func() { done <- publisher.Publish(context.Background(), "lostPet", payload) }()

	select {
	case <-topic.publishStarted:
	case <-time.After(time.Second):
		t.Fatal("Publish() did not start")
	}
	payload[0] = 'X'
	select {
	case err := <-done:
		t.Fatalf("Publish() returned before acknowledgement with %v", err)
	default:
	}
	close(topic.acknowledged)
	if err := <-done; err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := publisher.Publish(context.Background(), "lostPet", []byte("second-event")); err != nil {
		t.Fatalf("second Publish() error = %v", err)
	}

	if factoryCalls != 1 {
		t.Fatalf("publisher factory calls = %d, want 1", factoryCalls)
	}
	if factoryTopicID != "lostPet" {
		t.Fatalf("topic ID = %q, want lostPet", factoryTopicID)
	}
	got := topic.payloadsSnapshot()
	if len(got) != 2 || string(got[0]) != "lost-pet-event" || string(got[1]) != "second-event" {
		t.Fatalf("published payloads = %q, want original payload followed by second event", got)
	}
}

func TestGooglePublisherClosesPublishersAndClientOnce(t *testing.T) {
	t.Parallel()

	first := &stubGoogleTopic{acknowledged: closedChannel()}
	second := &stubGoogleTopic{acknowledged: closedChannel()}
	topics := map[string]*stubGoogleTopic{"lostPet": first, "foundPet": second}
	clientErr := errors.New("close client")
	clientCloses := 0
	publisher := newGooglePublisher(
		func(topicID string) googleTopic { return topics[topicID] },
		func() error {
			clientCloses++
			return clientErr
		},
	)

	for topicID := range topics {
		if err := publisher.Publish(context.Background(), topicID, []byte("event")); err != nil {
			t.Fatalf("Publish(%q) error = %v", topicID, err)
		}
	}
	if err := publisher.Close(); !errors.Is(err, clientErr) {
		t.Fatalf("Close() error = %v, want %v", err, clientErr)
	}
	if err := publisher.Close(); !errors.Is(err, clientErr) {
		t.Fatalf("second Close() error = %v, want %v", err, clientErr)
	}
	if clientCloses != 1 {
		t.Fatalf("client close calls = %d, want 1", clientCloses)
	}
	if first.stopCalls != 1 || second.stopCalls != 1 {
		t.Fatalf("publisher stop calls = %d, %d; want 1, 1", first.stopCalls, second.stopCalls)
	}
}

func TestNewGooglePublisherRejectsMismatchedEmulatorHost(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "127.0.0.1:8086")

	_, err := NewGooglePublisher(context.Background(), "petspotr-local", "127.0.0.1:9999")
	if err == nil {
		t.Fatal("NewGooglePublisher() error = nil, want emulator isolation error")
	}
}

type stubGoogleTopic struct {
	mu             sync.Mutex
	payloads       [][]byte
	publishStarted chan struct{}
	acknowledged   chan struct{}
	stopCalls      int
}

func (t *stubGoogleTopic) publish(ctx context.Context, data []byte) error {
	t.mu.Lock()
	t.payloads = append(t.payloads, data)
	t.mu.Unlock()
	if t.publishStarted != nil {
		t.publishStarted <- struct{}{}
	}
	select {
	case <-t.acknowledged:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *stubGoogleTopic) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopCalls++
}

func (t *stubGoogleTopic) payloadsSnapshot() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	payloads := make([][]byte, len(t.payloads))
	for i := range t.payloads {
		payloads[i] = append([]byte(nil), t.payloads[i]...)
	}
	return payloads
}

func closedChannel() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
