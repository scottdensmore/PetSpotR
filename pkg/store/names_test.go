package store_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestCanonicalCollectionNames(t *testing.T) {
	t.Parallel()

	if store.LostPetsCollection != "lostPets" {
		t.Fatalf("LostPetsCollection = %q, want lostPets", store.LostPetsCollection)
	}
	if store.FoundPetsCollection != "foundPets" {
		t.Fatalf("FoundPetsCollection = %q, want foundPets", store.FoundPetsCollection)
	}
	if store.ReportContactsCollection != "reportContacts" {
		t.Fatalf("ReportContactsCollection = %q, want reportContacts", store.ReportContactsCollection)
	}
	if store.MatchesCollection != "matches" {
		t.Fatalf("MatchesCollection = %q, want matches", store.MatchesCollection)
	}
	if store.MatchParticipantsCollection != "matchParticipants" {
		t.Fatalf("MatchParticipantsCollection = %q, want matchParticipants", store.MatchParticipantsCollection)
	}
	if store.PushSubscriptionsCollection != "pushSubscriptions" {
		t.Fatalf("PushSubscriptionsCollection = %q, want pushSubscriptions", store.PushSubscriptionsCollection)
	}
	if store.NotificationDeliveriesCollection != "notificationDeliveries" {
		t.Fatalf("NotificationDeliveriesCollection = %q, want notificationDeliveries", store.NotificationDeliveriesCollection)
	}
	if store.MatcherResultsCollection != "matcherResults" {
		t.Fatalf("MatcherResultsCollection = %q, want matcherResults", store.MatcherResultsCollection)
	}
	if store.SightingsCollection != "sightings" {
		t.Fatalf("SightingsCollection = %q, want sightings", store.SightingsCollection)
	}
	if store.NotificationsCollection != "notifications" {
		t.Fatalf("NotificationsCollection = %q, want notifications", store.NotificationsCollection)
	}
	if store.SearchPartiesCollection != "searchParties" {
		t.Fatalf("SearchPartiesCollection = %q, want searchParties", store.SearchPartiesCollection)
	}
	if store.SectorAssignmentsCollection != "sectorAssignments" {
		t.Fatalf("SectorAssignmentsCollection = %q, want sectorAssignments", store.SectorAssignmentsCollection)
	}
	if store.WebhooksCollection != "webhooks" {
		t.Fatalf("WebhooksCollection = %q, want webhooks", store.WebhooksCollection)
	}
	if store.WebhookDeliveriesCollection != "webhookDeliveries" {
		t.Fatalf("WebhookDeliveriesCollection = %q, want webhookDeliveries", store.WebhookDeliveriesCollection)
	}
	if store.BreadcrumbsCollection != "volunteerBreadcrumbs" {
		t.Fatalf("BreadcrumbsCollection = %q, want volunteerBreadcrumbs", store.BreadcrumbsCollection)
	}
	if store.SMSOptOutCollection != "smsOptOuts" {
		t.Fatalf("SMSOptOutCollection = %q, want smsOptOuts", store.SMSOptOutCollection)
	}
	if store.SMSDeliveriesCollection != "smsDeliveries" {
		t.Fatalf("SMSDeliveriesCollection = %q, want smsDeliveries", store.SMSDeliveriesCollection)
	}
	if store.TrajectoriesCollection != "trajectories" {
		t.Fatalf("TrajectoriesCollection = %q, want trajectories", store.TrajectoriesCollection)
	}
	if store.EvacuationHubsCollection != "evacuation_hubs" {
		t.Fatalf("EvacuationHubsCollection = %q, want evacuation_hubs", store.EvacuationHubsCollection)
	}
	if store.TransferManifestsCollection != "transfer_manifests" {
		t.Fatalf("TransferManifestsCollection = %q, want transfer_manifests", store.TransferManifestsCollection)
	}
	if store.CrisisIntakesCollection != "crisis_intakes" {
		t.Fatalf("CrisisIntakesCollection = %q, want crisis_intakes", store.CrisisIntakesCollection)
	}
	if store.CrisisReunificationsCollection != "crisis_reunifications" {
		t.Fatalf("CrisisReunificationsCollection = %q, want crisis_reunifications", store.CrisisReunificationsCollection)
	}
}
