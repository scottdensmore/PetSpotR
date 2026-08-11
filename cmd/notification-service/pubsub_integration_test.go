package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub"
	"github.com/scottdensmore/petspotr/pkg/domain"
	petpubsub "github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestNotificationPubSubEmulatorDeliversRetriesAndRetainsPoison(t *testing.T) {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	if host == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	projectID := "petspotr-notification-push-contract"
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("create emulator client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topicID := "match-found-" + suffix
	subscriptionID := "match-found-notification-" + suffix
	expectedSubscription := fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID)
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 1)
	push := newIdempotentTestSender(ChannelPush, 0)
	worker := NewWorkerWithStoreAndDispatcher(
		store.NewMemoryStore(),
		nil,
		NewMultiChannelDispatcher(email, sms, push),
	)
	handler := newNotificationHTTPHandler(
		worker,
		petpubsub.NewStaticPushAuthorizer("emulator-secret"),
		expectedSubscription,
		"projects/"+projectID+"/subscriptions/lost-pet-notification-unused",
	)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	topic, err := client.CreateTopic(ctx, topicID)
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	t.Cleanup(func() { _ = topic.Delete(context.Background()) })
	subscription, err := client.CreateSubscription(ctx, subscriptionID, gcppubsub.SubscriptionConfig{
		Topic: topic,
		PushConfig: gcppubsub.PushConfig{
			Endpoint: server.URL + "/pubsub/match-found?token=emulator-secret",
		},
	})
	if err != nil {
		t.Fatalf("create push subscription: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Delete(context.Background()) })
	publisher, err := petpubsub.NewGooglePublisher(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewGooglePublisher() error = %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(ctx, topicID, matchFoundEnvelope(t, "found-emulator", "lost-emulator")); err != nil {
		t.Fatalf("publish matchFound: %v", err)
	}
	waitForNotificationEffect(t, ctx, push, 1)
	if requests.Load() < 2 {
		t.Fatalf("push requests = %d, want redelivery after provider failure", requests.Load())
	}
	if calls, effects := email.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("email calls/effects = %d/%d, want 1/1", calls, effects)
	}
	if calls, effects := sms.snapshot(); calls != 2 || effects != 1 {
		t.Fatalf("sms calls/effects = %d/%d, want 2/1", calls, effects)
	}

	beforePoison := requests.Load()
	if err := publisher.Publish(ctx, topicID, []byte(`{"not":"a match"}`)); err != nil {
		t.Fatalf("publish poison message: %v", err)
	}
	for requests.Load() < beforePoison+2 {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for poison-message redelivery: %v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestNotificationLostPetPubSubEmulatorDeliversRetriesAndRetainsPoison(t *testing.T) {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	if host == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	projectID := "petspotr-lost-pet-push-contract"
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("create emulator client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topicID := "lost-pet-" + suffix
	subscriptionID := "lost-pet-notification-" + suffix
	expectedSubscription := fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID)
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 1)
	push := newIdempotentTestSender(ChannelPush, 0)
	dispatcher := NewMultiChannelDispatcher(email, sms, push)
	worker := NewWorkerWithStoreAndDispatcher(store.NewMemoryStore(), nil, dispatcher)
	worker.geoEngine = NewGeoBroadcastEngine([]CommunitySubscriber{
		{
			ID:          "subscriber-emulator",
			Email:       "neighbor@example.com",
			Phone:       "+12065550123",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5,
			Channels:    []Channel{ChannelEmail, ChannelSMS, ChannelPush},
		},
	}, dispatcher)
	handler := newNotificationHTTPHandler(
		worker,
		petpubsub.NewStaticPushAuthorizer("emulator-secret"),
		"projects/"+projectID+"/subscriptions/match-found-notification-unused",
		expectedSubscription,
	)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	topic, err := client.CreateTopic(ctx, topicID)
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	t.Cleanup(func() { _ = topic.Delete(context.Background()) })
	subscription, err := client.CreateSubscription(ctx, subscriptionID, gcppubsub.SubscriptionConfig{
		Topic: topic,
		PushConfig: gcppubsub.PushConfig{
			Endpoint: server.URL + "/pubsub/lost-pet?token=emulator-secret",
		},
	})
	if err != nil {
		t.Fatalf("create push subscription: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Delete(context.Background()) })
	publisher, err := petpubsub.NewGooglePublisher(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewGooglePublisher() error = %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(ctx, topicID, lostPetEnvelope(t, "lost-emulator")); err != nil {
		t.Fatalf("publish lostPet: %v", err)
	}
	waitForNotificationEffect(t, ctx, push, 1)
	if requests.Load() < 2 {
		t.Fatalf("push requests = %d, want redelivery after provider failure", requests.Load())
	}
	if calls, effects := email.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("email calls/effects = %d/%d, want 1/1", calls, effects)
	}
	if calls, effects := sms.snapshot(); calls != 2 || effects != 1 {
		t.Fatalf("sms calls/effects = %d/%d, want 2/1", calls, effects)
	}

	beforePoison := requests.Load()
	if err := publisher.Publish(ctx, topicID, []byte(`{"not":"a lost pet"}`)); err != nil {
		t.Fatalf("publish poison message: %v", err)
	}
	for requests.Load() < beforePoison+2 {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for poison-message redelivery: %v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func waitForNotificationEffect(
	t *testing.T,
	ctx context.Context,
	sender *idempotentTestSender,
	want int,
) {
	t.Helper()
	for {
		_, effects := sender.snapshot()
		if effects >= want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for notification effect: %v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
