package store

// Canonical state collection names shared by every PetSpotR service.
const (
	LostPetsCollection          = "lostPets"
	FoundPetsCollection         = "foundPets"
	MatchesCollection           = "matches"
	PushSubscriptionsCollection = "pushSubscriptions"
	OutboxCollection            = "eventOutbox"
)
