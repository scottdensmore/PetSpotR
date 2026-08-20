package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreOwnerLostLifecycleIsAtomicAcrossRuntimes(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-owner-lifecycle-%d", time.Now().UnixNano())
	writer, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	broker := pubsub.NewMemoryPubSub()
	if err := broker.Subscribe("petStatusChanged", func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	writerService := lostpet.NewService(writer, broker)
	readerService := lostpet.NewService(reader, broker)
	now := time.Now().UTC()
	petID := fmt.Sprintf("lost-owner-lifecycle-%d", now.UnixNano())
	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "firestore-owner"}
	reported, err := writerService.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: petID, ReporterEmail: "owner@example.com", ReportedAt: now.Add(-time.Hour),
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: &domain.LocationPoint{Latitude: 47.61, Longitude: -122.33}, OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	var lifecycleEventID string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if data, getErr := writer.GetState(cleanupCtx, store.LostPetsCollection, petID); getErr == nil {
			var record domain.LostPetRecord
			if json.Unmarshal(data, &record) == nil && record.OwnerIdentityRef != "" {
				_ = writer.DeleteState(cleanupCtx, store.ReportContactsCollection, record.OwnerIdentityRef)
			}
		}
		_ = writer.DeleteState(cleanupCtx, store.LostPetsCollection, petID)
		_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, reported.EventID)
		if lifecycleEventID != "" {
			_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, lifecycleEventID)
		}
	})

	candidates := store.LostPetCandidateStore(reader)
	query := store.LostPetCandidateQuery{
		Status: string(domain.LostPetStatusLost), GeocodingStatus: string(domain.GeocodingVerified),
		ReportedAfter: now.Add(-24 * time.Hour), ReportedBefore: now.Add(time.Hour),
		MinLatitude: 47, MaxLatitude: 48, MinLongitude: -123, MaxLongitude: -122,
	}
	before, err := candidates.QueryLostPetCandidates(ctx, query)
	if err != nil || before[petID] == nil {
		t.Fatalf("candidate before lifecycle = %#v, %v", before, err)
	}

	result, err := readerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "reunite-firestore-owner", Actor: owner,
	})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleEventID = result.EventID
	retry, err := writerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "reunite-firestore-owner", Actor: owner,
	})
	if err != nil || retry.EventID != result.EventID {
		t.Fatalf("cross-runtime exact retry = %#v, %v", retry, err)
	}
	if _, err := writerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited, OperationID: "different-operation", Actor: owner,
	}); !errors.Is(err, domain.ErrLostPetLifecycleConflict) {
		t.Fatalf("competing terminal operation error = %v", err)
	}
	after, err := candidates.QueryLostPetCandidates(ctx, query)
	if err != nil || after[petID] != nil {
		t.Fatalf("candidate after lifecycle = %#v, %v", after, err)
	}
	recordData, err := writer.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	var record domain.LostPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil || record.Status != domain.LostPetStatusReunited ||
		record.LifecycleAudit == nil || record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("durable lifecycle record = %#v, %v", record, err)
	}
	if durableEvent, err := outbox.GetRecord(ctx, reader, result.EventID); err != nil || durableEvent.Topic != "petStatusChanged" {
		t.Fatalf("durable lifecycle outbox = %#v, %v", durableEvent, err)
	}
}

func TestFirestoreGlobalOperatorLostLifecycleFencesConcurrentRevocation(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := fmt.Sprintf("petspotr-operator-lifecycle-%d", time.Now().UnixNano())
	writer, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	broker := pubsub.NewMemoryPubSub()
	if err := broker.Subscribe("petStatusChanged", func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	writerService := lostpet.NewService(writer, broker)
	readerService := lostpet.NewService(reader, broker)
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", now.UnixNano())
	petID := "lost-operator-lifecycle-" + suffix
	owner := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "firestore-owner-" + suffix,
	}
	operator := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "firestore-operator-" + suffix}
	admin := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "bootstrap-" + suffix}
	reported, err := writerService.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: petID, ReporterEmail: "owner@example.com", ReportedAt: now.Add(-time.Hour),
		Location: "Seattle, WA", OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	scope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	grant := domain.RoleAssignmentChange{
		Target: operator, Role: domain.RoleOperator, Scope: scope, Actor: admin,
		OperationID: "grant-operator-lifecycle-" + suffix, OccurredAt: now,
	}
	assignment, changed, err := writer.GrantRoleAssignment(ctx, grant)
	if err != nil || !changed {
		t.Fatalf("grant = %#v, changed %t, error %v", assignment, changed, err)
	}
	var lifecycleEventID string
	ownerlessPetID := "lost-ownerless-operator-lifecycle-" + suffix
	var ownerlessEventID string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if data, getErr := writer.GetState(cleanupCtx, store.LostPetsCollection, petID); getErr == nil {
			var record domain.LostPetRecord
			if json.Unmarshal(data, &record) == nil && record.OwnerIdentityRef != "" {
				_ = writer.DeleteState(cleanupCtx, store.ReportContactsCollection, record.OwnerIdentityRef)
			}
		}
		_ = writer.DeleteState(cleanupCtx, store.LostPetsCollection, petID)
		_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, reported.EventID)
		if lifecycleEventID != "" {
			_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, lifecycleEventID)
		}
		_ = writer.DeleteState(cleanupCtx, store.LostPetsCollection, ownerlessPetID)
		if ownerlessEventID != "" {
			_ = writer.DeleteState(cleanupCtx, store.OutboxCollection, ownerlessEventID)
		}
	})

	revoke := grant
	revoke.OperationID = "revoke-operator-lifecycle-" + suffix
	revoke.OccurredAt = now.Add(time.Second)
	start := make(chan struct{})
	updateResult := make(chan struct {
		result lostpet.LifecycleResult
		err    error
	}, 1)
	go func() {
		<-start
		result, updateErr := readerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
			PetID: petID, Status: domain.LostPetStatusReunited,
			OperationID: "operator-lifecycle-" + suffix, Actor: operator,
		})
		updateResult <- struct {
			result lostpet.LifecycleResult
			err    error
		}{result: result, err: updateErr}
	}()
	revokeResult := make(chan struct {
		changed bool
		err     error
	}, 1)
	go func() {
		<-start
		_, revoked, revokeErr := writer.RevokeRoleAssignment(ctx, revoke)
		revokeResult <- struct {
			changed bool
			err     error
		}{changed: revoked, err: revokeErr}
	}()
	close(start)
	updated := <-updateResult
	revoked := <-revokeResult
	if revoked.err != nil || !revoked.changed {
		t.Fatalf("concurrent revoke = changed %t, error %v", revoked.changed, revoked.err)
	}
	if updated.err != nil && !errors.Is(updated.err, lostpet.ErrLifecycleHidden) {
		t.Fatalf("concurrent lifecycle error = %v, want nil or hidden", updated.err)
	}

	recordData, err := writer.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	var record domain.LostPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil {
		t.Fatal(err)
	}
	wantRevision := int64(3)
	if updated.err == nil {
		lifecycleEventID = updated.result.EventID
		wantRevision = assignment.Revision
		if record.Status != domain.LostPetStatusReunited || record.LifecycleAudit == nil ||
			record.LifecycleAudit.AuthorizedAs != domain.LostPetLifecycleAuthorizationOperator ||
			record.LifecycleAudit.AssignmentID != assignment.AssignmentID ||
			record.LifecycleAudit.AssignmentRevision != assignment.Revision {
			t.Fatalf("authorized-before-revoke lifecycle = %#v", record)
		}
	} else if record.Status != domain.LostPetStatusLost || record.LifecycleAudit != nil {
		t.Fatalf("revoked-before-update lifecycle = %#v", record)
	}

	regrant := grant
	regrant.OperationID = "regrant-operator-lifecycle-" + suffix
	regrant.OccurredAt = now.Add(2 * time.Second)
	if _, changed, err := reader.GrantRoleAssignment(ctx, regrant); err != nil || !changed {
		t.Fatalf("regrant = changed %t, error %v", changed, err)
	}
	result, err := writerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: petID, Status: domain.LostPetStatusReunited,
		OperationID: "operator-lifecycle-" + suffix, Actor: operator,
	})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleEventID = result.EventID
	recordData, err = reader.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(recordData, &record); err != nil || record.LifecycleAudit == nil ||
		record.LifecycleAudit.AssignmentID != assignment.AssignmentID ||
		record.LifecycleAudit.AssignmentRevision != wantRevision {
		t.Fatalf("lifecycle after regrant = %#v, %v; want assignment revision %d", record, err, wantRevision)
	}
	if durableEvent, err := outbox.GetRecord(ctx, reader, result.EventID); err != nil || durableEvent.Topic != "petStatusChanged" {
		t.Fatalf("durable operator lifecycle outbox = %#v, %v", durableEvent, err)
	}

	ownerlessReport := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: ownerlessPetID, ReporterEmail: "legacy@example.com",
		ReportedAt: now.Add(-30 * time.Minute), Location: "Tacoma, WA",
	})
	ownerlessRecord, _ := ownerlessReport.Persisted()
	ownerlessData, err := json.Marshal(ownerlessRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.SaveState(ctx, store.LostPetsCollection, ownerlessPetID, ownerlessData); err != nil {
		t.Fatal(err)
	}
	ownerlessResult, err := readerService.ReuniteLostPet(ctx, lostpet.LifecycleCommand{
		PetID: ownerlessPetID, Status: domain.LostPetStatusReunited,
		OperationID: "operator-ownerless-lifecycle-" + suffix, Actor: operator,
	})
	if err != nil {
		t.Fatal(err)
	}
	ownerlessEventID = ownerlessResult.EventID
	ownerlessData, err = writer.GetState(ctx, store.LostPetsCollection, ownerlessPetID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(ownerlessData, &ownerlessRecord); err != nil || ownerlessRecord.LifecycleAudit == nil ||
		ownerlessRecord.LifecycleAudit.AuthorizedAs != domain.LostPetLifecycleAuthorizationOperator ||
		ownerlessRecord.LifecycleAudit.AssignmentID != assignment.AssignmentID ||
		ownerlessRecord.LifecycleAudit.AssignmentRevision != 3 {
		t.Fatalf("durable ownerless operator lifecycle = %#v, %v", ownerlessRecord, err)
	}
	if durableEvent, err := outbox.GetRecord(ctx, writer, ownerlessEventID); err != nil ||
		durableEvent.Topic != "petStatusChanged" {
		t.Fatalf("durable ownerless lifecycle outbox = %#v, %v", durableEvent, err)
	}
}
