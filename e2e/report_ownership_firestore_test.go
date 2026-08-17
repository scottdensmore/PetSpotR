package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreReportOwnershipSurvivesIndependentServiceRuntimes(t *testing.T) {
	firestoreHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if firestoreHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projectID := "petspotr-report-ownership"
	first := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	second := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	broker := pubsub.NewMemoryPubSub()
	now := time.Now().UTC()
	suffix := fmt.Sprintf("%d", now.UnixNano())
	owner := &domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-" + suffix,
	}

	lostCommand := lostpet.ReportCommand{
		PetID: "lost-owned-" + suffix, ReporterEmail: "owner@example.com",
		ReportedAt: now, Location: "Seattle, WA", OwnedBy: owner,
	}
	foundCommand := foundpet.ReportCommand{
		PetID: "found-owned-" + suffix, ImageURL: "https://images.invalid/found.jpg",
		FoundAt: now, Location: "Portland, OR", FinderEmail: "finder@example.com", OwnedBy: owner,
	}
	var lostEventID, foundEventID string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, target := range []struct {
			collection string
			petID      string
			contactRef string
			eventID    string
		}{
			{store.LostPetsCollection, lostCommand.PetID, "lost/" + lostCommand.PetID + "/owner", lostEventID},
			{store.FoundPetsCollection, foundCommand.PetID, "found/" + foundCommand.PetID + "/finder", foundEventID},
		} {
			_ = first.Store.DeleteState(cleanupCtx, target.collection, target.petID)
			_ = first.Store.DeleteState(cleanupCtx, store.ReportContactsCollection, target.contactRef)
			if target.eventID != "" {
				_ = first.Store.DeleteState(cleanupCtx, store.OutboxCollection, target.eventID)
			}
		}
	})
	lostFirst, err := lostpet.NewService(first.Store, broker).ReportLostPet(ctx, lostCommand, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatalf("first lost report: %v", err)
	}
	lostEventID = lostFirst.EventID
	lostRetry, err := lostpet.NewService(second.Store, broker).ReportLostPet(ctx, lostCommand, lostpet.ReportMetadata{})
	if err != nil || lostRetry != lostFirst {
		t.Fatalf("cross-runtime lost retry = %#v, %v; want %#v", lostRetry, err, lostFirst)
	}
	assertFirestoreOwnership(t, ctx, second.Store, store.LostPetsCollection, lostCommand.PetID, owner)
	lostClaim := lostCommand
	lostClaim.OwnedBy = &domain.PrincipalRef{Issuer: owner.Issuer, Subject: "attacker-" + suffix}
	if _, err := lostpet.NewService(second.Store, broker).ReportLostPet(ctx, lostClaim, lostpet.ReportMetadata{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("competing lost owner error = %v, want ErrConflict", err)
	}

	foundFirst, err := foundpet.NewReportService(first.Store, broker).ReportFoundPet(ctx, foundCommand, foundpet.ReportMetadata{})
	if err != nil {
		t.Fatalf("first found report: %v", err)
	}
	foundEventID = foundFirst.EventID
	foundRetry, err := foundpet.NewReportService(second.Store, broker).ReportFoundPet(ctx, foundCommand, foundpet.ReportMetadata{})
	if err != nil || foundRetry != foundFirst {
		t.Fatalf("cross-runtime found retry = %#v, %v; want %#v", foundRetry, err, foundFirst)
	}
	assertFirestoreOwnership(t, ctx, second.Store, store.FoundPetsCollection, foundCommand.PetID, owner)
	foundClaim := foundCommand
	foundClaim.OwnedBy = &domain.PrincipalRef{Issuer: owner.Issuer, Subject: "attacker-" + suffix}
	if _, err := foundpet.NewReportService(second.Store, broker).ReportFoundPet(ctx, foundClaim, foundpet.ReportMetadata{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("competing found owner error = %v, want ErrConflict", err)
	}
}

func newOwnershipStateRuntime(
	t *testing.T,
	ctx context.Context,
	projectID string,
	firestoreHost string,
) *runtimeconfig.StateRuntime {
	t.Helper()
	stateRuntime, err := runtimeconfig.NewStateRuntime(ctx, runtimeconfig.StateConfig{
		Mode: runtimeconfig.ModeLocalEmulator, ProjectID: projectID,
		FirestoreEmulatorHost: firestoreHost,
	})
	if err != nil {
		t.Fatalf("create Firestore state runtime: %v", err)
	}
	t.Cleanup(func() { _ = stateRuntime.Close() })
	return stateRuntime
}

func assertFirestoreOwnership(
	t *testing.T,
	ctx context.Context,
	stateStore store.StateStore,
	collection string,
	petID string,
	want *domain.PrincipalRef,
) {
	t.Helper()
	data, err := stateStore.GetState(ctx, collection, petID)
	if err != nil {
		t.Fatalf("load %s/%s: %v", collection, petID, err)
	}
	var persisted struct {
		OwnedBy *domain.PrincipalRef `json:"ownedBy"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode %s/%s: %v", collection, petID, err)
	}
	if persisted.OwnedBy == nil || *persisted.OwnedBy != *want {
		t.Fatalf("%s/%s owner = %#v, want %#v", collection, petID, persisted.OwnedBy, want)
	}
}
