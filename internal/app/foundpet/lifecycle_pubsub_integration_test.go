package foundpet

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

func TestFinderFoundLifecyclePublishesRedactedEventWithPubSubEmulator(t *testing.T) {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	if host == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-found-lifecycle-pubsub-%d", time.Now().UnixNano())
	client, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	topicName := fmt.Sprintf("projects/%s/topics/petStatusChanged", projectID)
	subscriptionName := fmt.Sprintf("projects/%s/subscriptions/found-pet-status-changed-test", projectID)
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
		_ = client.SubscriptionAdminClient.DeleteSubscription(
			context.Background(), &pubsubpb.DeleteSubscriptionRequest{Subscription: subscriptionName},
		)
	}()
	publisher, err := petpubsub.NewGooglePublisher(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()

	finder := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "pubsub-finder",
	}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-pubsub-lifecycle", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Now().UTC(), Location: "Seattle, WA", FinderEmail: "finder@example.com", OwnedBy: &finder,
	})
	record, _ := report.Persisted()
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	state := store.NewMemoryStore()
	if err := state.SaveState(ctx, store.FoundPetsCollection, record.PetID, recordData); err != nil {
		t.Fatal(err)
	}
	result, err := NewReportService(state, publisher).ResolveFoundPet(ctx, LifecycleCommand{
		PetID: record.PetID, Status: domain.FoundPetStatusResolved,
		OperationID: "resolve-pubsub-finder", Actor: finder,
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
	event, envelope, err := domain.DecodePetStatusChanged(messageData)
	if err != nil || envelope == nil || envelope.ID != result.EventID ||
		envelope.PayloadVersion != domain.PetStatusChangedFoundPayloadVersion ||
		event.ReportType != "found" || event.Status != "resolved" {
		t.Fatalf("published lifecycle event = %#v / %#v, %v", envelope, event, err)
	}
	for _, private := range []string{finder.Issuer, finder.Subject, "finder@example.com"} {
		if bytes.Contains(messageData, []byte(private)) {
			t.Fatalf("published lifecycle event exposed %q: %s", private, messageData)
		}
	}
}
