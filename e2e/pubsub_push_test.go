package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	petpubsub "github.com/scottdensmore/petspotr/pkg/pubsub"
)

func TestPubSubEmulatorRedeliversWrappedMessageUntilHandlerSucceeds(t *testing.T) {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	if host == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := "petspotr-push-contract"
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("create emulator client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topicID := "found-pet-" + suffix
	subscriptionID := "found-pet-matcher-" + suffix
	expectedSubscription := fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID)
	received := make(chan []byte, 1)
	var attempts atomic.Int32
	handler := petpubsub.NewPushHandler(
		petpubsub.NewStaticPushAuthorizer("emulator-secret"),
		expectedSubscription,
		func(_ context.Context, data []byte) error {
			if attempts.Add(1) == 1 {
				return errors.New("transient matcher failure")
			}
			received <- append([]byte(nil), data...)
			return nil
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	topic, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: fmt.Sprintf("projects/%s/topics/%s", projectID, topicID),
	})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	t.Cleanup(func() {
		_ = client.TopicAdminClient.DeleteTopic(context.Background(), &pubsubpb.DeleteTopicRequest{Topic: topic.GetName()})
	})
	subscription, err := client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID),
		Topic: topic.GetName(),
		PushConfig: &pubsubpb.PushConfig{
			PushEndpoint: server.URL + "/pubsub/found-pet?token=emulator-secret",
		},
	})
	if err != nil {
		t.Fatalf("create push subscription: %v", err)
	}
	t.Cleanup(func() {
		_ = client.SubscriptionAdminClient.DeleteSubscription(context.Background(), &pubsubpb.DeleteSubscriptionRequest{
			Subscription: subscription.GetName(),
		})
	})

	publisher, err := petpubsub.NewGooglePublisher(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewGooglePublisher() error = %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })
	payload := []byte(`{"envelopeVersion":1,"id":"evt-emulator"}`)
	if err := publisher.Publish(ctx, topicID, payload); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case got := <-received:
		if string(got) != string(payload) {
			t.Fatalf("push payload = %s, want %s", got, payload)
		}
		if attempts.Load() < 2 {
			t.Fatalf("push attempts = %d, want redelivery after failure", attempts.Load())
		}
	case <-ctx.Done():
		t.Fatalf("wait for emulator push: %v", ctx.Err())
	}
}
