package lostpet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/scottdensmore/petspotr/pkg/domain"
	petpubsub "github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestOwnerLostLifecyclePublishesRedactedEventWithPubSubEmulator(t *testing.T) {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	if host == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-lifecycle-pubsub-%d", time.Now().UnixNano())
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	topicName := fmt.Sprintf("projects/%s/topics/petStatusChanged", projectID)
	subscriptionName := fmt.Sprintf("projects/%s/subscriptions/pet-status-changed-test", projectID)
	if _, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topicName}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = client.TopicAdminClient.DeleteTopic(context.Background(), &pubsubpb.DeleteTopicRequest{Topic: topicName})
	}()
	if _, err := client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name: subscriptionName, Topic: topicName,
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = client.SubscriptionAdminClient.DeleteSubscription(context.Background(), &pubsubpb.DeleteSubscriptionRequest{Subscription: subscriptionName})
	}()
	publisher, err := petpubsub.NewGooglePublisher(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()

	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "pubsub-owner"}
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-pubsub-lifecycle", ReporterEmail: "owner@example.com", ReportedAt: time.Now().UTC(), OwnedBy: &owner,
	})
	record, _ := report.Persisted()
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	state := store.NewMemoryStore()
	if err := state.SaveState(ctx, store.LostPetsCollection, record.PetID, recordData); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(state, publisher).ReuniteLostPet(ctx, LifecycleCommand{
		PetID: record.PetID, Status: domain.LostPetStatusReunited,
		OperationID: "reunite-pubsub-owner", Actor: owner,
	})
	if err != nil {
		t.Fatal(err)
	}
	subscriber := client.Subscriber(subscriptionName)
	subscriber.ReceiveSettings.NumGoroutines = 1
	subscriber.ReceiveSettings.MaxOutstandingMessages = 1
	receiveCtx, stopReceive := context.WithCancel(ctx)
	var messageData []byte
	err = subscriber.Receive(receiveCtx, func(_ context.Context, message *gcppubsub.Message) {
		messageData = append([]byte(nil), message.Data...)
		message.Ack()
		stopReceive()
	})
	stopReceive()
	if err != nil && receiveCtx.Err() == nil {
		t.Fatalf("receive lifecycle event: %v", err)
	}
	if len(messageData) == 0 {
		t.Fatal("received lifecycle event is empty")
	}
	var event domain.PetStatusChangedV1
	envelope, err := domain.DecodeEventPayload(messageData, domain.EventTypePetStatusChanged, &event)
	if err != nil || envelope == nil || envelope.ID != result.EventID || event.Status != domain.LostPetStatusReunited {
		t.Fatalf("published lifecycle event = %#v / %#v, %v", envelope, event, err)
	}
	for _, private := range []string{owner.Issuer, owner.Subject, "owner@example.com"} {
		if bytes.Contains(messageData, []byte(private)) {
			t.Fatalf("published lifecycle event exposed %q: %s", private, messageData)
		}
	}
}
