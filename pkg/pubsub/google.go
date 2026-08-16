package pubsub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	gcppubsub "cloud.google.com/go/pubsub/v2"
)

// DetectGoogleProjectID instructs the Pub/Sub client to detect the project
// through Application Default Credentials or the metadata server.
const DetectGoogleProjectID = gcppubsub.DetectProjectID

// GooglePublisher publishes events through managed Pub/Sub or its emulator.
type GooglePublisher struct {
	newTopic    func(string) googleTopic
	closeClient func() error
	mu          sync.Mutex
	topics      map[string]googleTopic
	closeOnce   sync.Once
	closeErr    error
}

type googleTopic interface {
	publish(context.Context, []byte) error
	stop()
}

type googlePubSubTopic struct {
	publisher *gcppubsub.Publisher
}

func (t *googlePubSubTopic) publish(ctx context.Context, data []byte) error {
	_, err := t.publisher.Publish(ctx, &gcppubsub.Message{Data: data}).Get(ctx)
	return err
}

func (t *googlePubSubTopic) stop() {
	t.publisher.Stop()
}

// NewGooglePublisher creates a reusable Pub/Sub client. When emulatorHost is
// nonempty, it must exactly match PUBSUB_EMULATOR_HOST so tests cannot
// accidentally connect to production.
func NewGooglePublisher(ctx context.Context, projectID, emulatorHost string) (*GooglePublisher, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("pubsub: Google project ID is required")
	}
	emulatorHost = strings.TrimSpace(emulatorHost)
	configuredHost := strings.TrimSpace(os.Getenv("PUBSUB_EMULATOR_HOST"))
	if emulatorHost != configuredHost {
		return nil, fmt.Errorf("pubsub: configured emulator host %q does not match environment host %q", emulatorHost, configuredHost)
	}
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub: create Google publisher: %w", err)
	}
	return newGooglePublisher(
		func(topicID string) googleTopic {
			return &googlePubSubTopic{publisher: client.Publisher(topicID)}
		},
		client.Close,
	), nil
}

func newGooglePublisher(newTopic func(string) googleTopic, closeClient func() error) *GooglePublisher {
	return &GooglePublisher{
		newTopic:    newTopic,
		closeClient: closeClient,
		topics:      make(map[string]googleTopic),
	}
}

// Publish waits until Pub/Sub acknowledges the event or the context fails.
func (p *GooglePublisher) Publish(ctx context.Context, topic string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("pubsub: topic is required")
	}
	p.mu.Lock()
	publisher, exists := p.topics[topic]
	if !exists {
		publisher = p.newTopic(topic)
		p.topics[topic] = publisher
	}
	p.mu.Unlock()
	if err := publisher.publish(ctx, append([]byte(nil), data...)); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("pubsub: publish %s: %w", topic, err)
	}
	return nil
}

// Close flushes topic publishers and closes the shared client once.
func (p *GooglePublisher) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		topics := make([]googleTopic, 0, len(p.topics))
		for _, topic := range p.topics {
			topics = append(topics, topic)
		}
		p.mu.Unlock()
		for _, topic := range topics {
			topic.stop()
		}
		p.closeErr = p.closeClient()
	})
	return p.closeErr
}
