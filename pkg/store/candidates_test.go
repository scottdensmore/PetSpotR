package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryStoreQueriesBoundedLostPetCandidates(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	records := []domain.LostPetRecord{
		{
			PetID: "eligible-dog", Species: "Dog", OwnerIdentityRef: "identity-dog",
			ReportedAt: now.Add(-time.Hour), Location: "Seattle", Status: domain.LostPetStatusLost,
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.62, Longitude: -122.32},
		},
		{
			PetID: "eligible-unknown-species", OwnerIdentityRef: "identity-unknown",
			ReportedAt: now.Add(-time.Hour), Location: "Seattle", Status: domain.LostPetStatusLost,
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.61, Longitude: -122.31},
		},
		{
			PetID: "wrong-species", Species: "Cat", OwnerIdentityRef: "identity-cat",
			ReportedAt: now.Add(-time.Hour), Location: "Seattle", Status: domain.LostPetStatusLost,
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.62, Longitude: -122.32},
		},
		{
			PetID: "outside-time", Species: "Dog", OwnerIdentityRef: "identity-time",
			ReportedAt: now.Add(-31 * 24 * time.Hour), Location: "Seattle", Status: domain.LostPetStatusLost,
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.62, Longitude: -122.32},
		},
		{
			PetID: "outside-box", Species: "Dog", OwnerIdentityRef: "identity-box",
			ReportedAt: now.Add(-time.Hour), Location: "Tacoma", Status: domain.LostPetStatusLost,
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.25, Longitude: -122.44},
		},
	}
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := stateStore.SaveState(ctx, store.LostPetsCollection, record.PetID, data); err != nil {
			t.Fatal(err)
		}
	}

	got, err := stateStore.QueryLostPetCandidates(ctx, store.LostPetCandidateQuery{
		Status: "lost", GeocodingStatus: "verified", Species: "dog",
		ReportedAfter: now.Add(-30 * 24 * time.Hour), ReportedBefore: now,
		MinLatitude: 47.4, MaxLatitude: 47.8, MinLongitude: -122.6, MaxLongitude: -122.1,
	})
	if err != nil {
		t.Fatalf("QueryLostPetCandidates() error = %v", err)
	}
	if len(got) != 2 || got["eligible-dog"] == nil || got["eligible-unknown-species"] == nil {
		t.Fatalf("QueryLostPetCandidates() = %#v, want dog and unknown-species candidates", got)
	}
}

func TestMemoryStoreRejectsInvalidLostPetCandidateQuery(t *testing.T) {
	_, err := store.NewMemoryStore().QueryLostPetCandidates(context.Background(), store.LostPetCandidateQuery{})
	if err == nil {
		t.Fatal("QueryLostPetCandidates() error = nil, want invalid-bounds error")
	}
}
