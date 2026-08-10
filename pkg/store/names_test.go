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
	if store.MatchesCollection != "matches" {
		t.Fatalf("MatchesCollection = %q, want matches", store.MatchesCollection)
	}
	if store.PushSubscriptionsCollection != "pushSubscriptions" {
		t.Fatalf("PushSubscriptionsCollection = %q, want pushSubscriptions", store.PushSubscriptionsCollection)
	}
}
