package store

// Canonical state collection names shared by every PetSpotR service.
const (
	LostPetsCollection               = "lostPets"
	FoundPetsCollection              = "foundPets"
	ReportContactsCollection         = "reportContacts"
	MatchesCollection                = "matches"
	PushSubscriptionsCollection      = "pushSubscriptions"
	OutboxCollection                 = "eventOutbox"
	NotificationDeliveriesCollection = "notificationDeliveries"
	MatcherResultsCollection         = "matcherResults"
)
