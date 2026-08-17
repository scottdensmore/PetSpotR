package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreQueriesAndBackfillsBoundedLostPetCandidates(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-candidate-index-%d", time.Now().UnixNano())
	stateStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })
	rawClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("firestore.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = rawClient.Close() })

	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	legacy := candidateRecord("lost-legacy-index", "Dog", now.Add(-2*time.Hour), 47.61, -122.31)
	legacyData, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyDoc := rawClient.Collection(store.LostPetsCollection).Doc(firestoreDocumentIDForTest(legacy.PetID))
	if _, err := legacyDoc.Set(ctx, map[string]any{"key": legacy.PetID, "data": legacyData}); err != nil {
		t.Fatalf("write legacy lost-pet record: %v", err)
	}

	current := candidateRecord("lost-current-index", "Dog", now.Add(-time.Hour), 47.62, -122.32)
	currentData, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.LostPetsCollection, current.PetID, currentData); err != nil {
		t.Fatalf("SaveState(current) error = %v", err)
	}
	unknownSpecies := candidateRecord("lost-unknown-species-index", "", now.Add(-time.Hour), 47.63, -122.33)
	unknownSpeciesData, err := json.Marshal(unknownSpecies)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.LostPetsCollection, unknownSpecies.PetID, unknownSpeciesData); err != nil {
		t.Fatalf("SaveState(unknown species) error = %v", err)
	}
	query := store.LostPetCandidateQuery{
		Status: "lost", GeocodingStatus: "verified", Species: "dog",
		ReportedAfter: now.Add(-30 * 24 * time.Hour), ReportedBefore: now,
		MinLatitude: 47.4, MaxLatitude: 47.8, MinLongitude: -122.6, MaxLongitude: -122.1,
	}
	before, err := stateStore.QueryLostPetCandidates(ctx, query)
	if err != nil {
		t.Fatalf("QueryLostPetCandidates() before backfill error = %v", err)
	}
	if len(before) != 2 || before[current.PetID] == nil || before[unknownSpecies.PetID] == nil {
		t.Fatalf("before backfill = %#v, want current indexed records", before)
	}

	migrated, complete, err := stateStore.BackfillLostPetCandidateIndexes(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillLostPetCandidateIndexes() error = %v", err)
	}
	if migrated != 1 || !complete {
		t.Fatalf("BackfillLostPetCandidateIndexes() = %d, %t, want 1, true", migrated, complete)
	}
	after, err := stateStore.QueryLostPetCandidates(ctx, query)
	if err != nil {
		t.Fatalf("QueryLostPetCandidates() after backfill error = %v", err)
	}
	if len(after) != 3 || after[current.PetID] == nil || after[unknownSpecies.PetID] == nil || after[legacy.PetID] == nil {
		t.Fatalf("after backfill = %#v, want current, unknown-species, and legacy records", after)
	}
	migrated, complete, err = stateStore.BackfillLostPetCandidateIndexes(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillLostPetCandidateIndexes() completed retry error = %v", err)
	}
	if migrated != 0 || !complete {
		t.Fatalf("completed backfill retry = %d, %t, want 0, true", migrated, complete)
	}
	query.Species = ""
	withoutSpecies, err := stateStore.QueryLostPetCandidates(ctx, query)
	if err != nil {
		t.Fatalf("QueryLostPetCandidates() without species error = %v", err)
	}
	if len(withoutSpecies) != 3 {
		t.Fatalf("query without species = %#v, want all 3 bounded records", withoutSpecies)
	}
}

func candidateRecord(id, species string, reportedAt time.Time, latitude, longitude float64) domain.LostPetRecord {
	return domain.LostPetRecord{
		PetID: id, Species: species, OwnerIdentityRef: "identity-" + id,
		ReportedAt: reportedAt, Location: "Seattle", Status: domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: latitude, Longitude: longitude},
	}
}
