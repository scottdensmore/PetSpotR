package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreRoleAuthorizedMatchUpdateFencesConcurrentRevocation(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const projectID = "petspotr-role-authorized-match"
	config := runtimeconfig.StateConfig{
		Mode: runtimeconfig.ModeLocalEmulator, ProjectID: projectID, FirestoreEmulatorHost: host,
	}
	writer, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	roleMatchStore, ok := reader.Store.(store.RoleAuthorizedMatchStateStore)
	if !ok {
		t.Fatalf("reader store %T does not support role-authorized match updates", reader.Store)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	lostPetID := "lost-role-race-" + suffix
	foundPetID := "found-role-race-" + suffix
	matchID, err := domain.StableMatchID("event-role-race-"+suffix, foundPetID, lostPetID)
	if err != nil {
		t.Fatal(err)
	}
	regrantLostPetID := "lost-role-regrant-" + suffix
	regrantFoundPetID := "found-role-regrant-" + suffix
	regrantMatchID, err := domain.StableMatchID(
		"event-role-regrant-"+suffix, regrantFoundPetID, regrantLostPetID,
	)
	if err != nil {
		t.Fatal(err)
	}
	principal := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "operator-" + suffix,
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: "bootstrap-" + suffix}
	scope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	grant := domain.RoleAssignmentChange{
		Target: principal, Role: domain.RoleOperator, Scope: scope, Actor: actor,
		OperationID: "grant-" + suffix, OccurredAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	assignmentID, err := domain.RoleAssignmentID(principal, domain.RoleOperator, scope)
	if err != nil {
		t.Fatal(err)
	}
	grantAuditID, err := domain.RoleAssignmentAuditID(assignmentID, grant.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	revoke := grant
	revoke.OperationID = "revoke-" + suffix
	revoke.OccurredAt = grant.OccurredAt.Add(time.Second)
	revokeAuditID, err := domain.RoleAssignmentAuditID(assignmentID, revoke.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	regrant := grant
	regrant.OperationID = "regrant-" + suffix
	regrant.OccurredAt = revoke.OccurredAt.Add(time.Second)
	regrantAuditID, err := domain.RoleAssignmentAuditID(assignmentID, regrant.OperationID)
	if err != nil {
		t.Fatal(err)
	}

	seedFirestoreReunionMatch(
		t, ctx, writer.Store, matchID, "event-role-race-"+suffix, lostPetID, foundPetID, grant.OccurredAt,
	)
	if _, changed, err := writer.RoleAssignments.GrantRoleAssignment(ctx, grant); err != nil || !changed {
		t.Fatalf("grant = changed %t, error %v", changed, err)
	}
	rawClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawClient.Close() })
	assignmentDoc := rawClient.Collection("operatorRoleAssignments").Doc(firestoreDocumentIDForTest(assignmentID))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = writer.Store.DeleteState(cleanupCtx, store.MatchesCollection, matchID)
		_ = writer.Store.DeleteState(cleanupCtx, store.MatchParticipantsCollection, matchID)
		_ = writer.Store.DeleteState(cleanupCtx, store.MatchesCollection, regrantMatchID)
		_ = writer.Store.DeleteState(cleanupCtx, store.MatchParticipantsCollection, regrantMatchID)
		for _, auditID := range []string{grantAuditID, revokeAuditID, regrantAuditID} {
			_, _ = assignmentDoc.Collection("audit").Doc(firestoreDocumentIDForTest(auditID)).Delete(cleanupCtx)
		}
		_, _ = assignmentDoc.Delete(cleanupCtx)
	})

	start := make(chan struct{})
	updateResult := make(chan error, 1)
	go func() {
		<-start
		updateResult <- roleMatchStore.UpdateMatchAndParticipantsAsRole(
			ctx, principal, domain.RoleOperator, scope, matchID,
			firestoreReunionUpdater(
				principal, "resolve-race-"+suffix, grant.OccurredAt.Add(500*time.Millisecond),
			),
		)
	}()
	type revokeResult struct {
		changed bool
		err     error
	}
	revokeResults := make(chan revokeResult, 1)
	go func() {
		<-start
		_, changed, revokeErr := writer.RoleAssignments.RevokeRoleAssignment(ctx, revoke)
		revokeResults <- revokeResult{changed: changed, err: revokeErr}
	}()
	close(start)
	var updateErr error
	select {
	case updateErr = <-updateResult:
	case <-ctx.Done():
		t.Fatal("role-authorized transaction did not complete")
	}
	var revoked revokeResult
	select {
	case revoked = <-revokeResults:
	case <-ctx.Done():
		t.Fatal("role revocation transaction did not complete")
	}
	if revoked.err != nil || !revoked.changed {
		t.Fatalf("concurrent revoke = changed %t, error %v", revoked.changed, revoked.err)
	}
	if updateErr != nil && !errors.Is(updateErr, store.ErrRoleDenied) {
		t.Fatalf("concurrent update error = %v, want nil or ErrRoleDenied", updateErr)
	}
	match, participants := loadFirestoreReunionMatch(t, ctx, writer.Store, matchID)
	if updateErr == nil {
		if match.Status != domain.MatchStatusReunited || participants.ReunionAudit == nil ||
			participants.ReunionAudit.AssignmentID != assignmentID || participants.ReunionAudit.AssignmentRevision != 1 {
			t.Fatalf("authorized-before-revoke state = %#v / %#v", match, participants.ReunionAudit)
		}
	} else if match.Status != domain.MatchStatusConfirmed || participants.ReunionAudit != nil {
		t.Fatalf("revoked-before-update state = %#v / %#v", match, participants.ReunionAudit)
	}

	if _, changed, err := reader.RoleAssignments.GrantRoleAssignment(ctx, regrant); err != nil || !changed {
		t.Fatalf("regrant = changed %t, error %v", changed, err)
	}
	seedFirestoreReunionMatch(
		t, ctx, writer.Store, regrantMatchID, "event-role-regrant-"+suffix,
		regrantLostPetID, regrantFoundPetID, regrant.OccurredAt,
	)
	err = roleMatchStore.UpdateMatchAndParticipantsAsRole(
		ctx, principal, domain.RoleOperator, scope, regrantMatchID,
		firestoreReunionUpdater(
			principal, "resolve-regrant-"+suffix, regrant.OccurredAt.Add(time.Second),
		),
	)
	if err != nil {
		t.Fatalf("regranted update error = %v", err)
	}
	match, participants = loadFirestoreReunionMatch(t, ctx, writer.Store, regrantMatchID)
	if match.Status != domain.MatchStatusReunited || participants.ReunionAudit == nil ||
		participants.ReunionAudit.AssignmentID != assignmentID || participants.ReunionAudit.AssignmentRevision != 3 {
		t.Fatalf("match after regranted update = %#v / %#v", match, participants.ReunionAudit)
	}
}

func seedFirestoreReunionMatch(
	t *testing.T,
	ctx context.Context,
	state store.StateStore,
	matchID, sourceEventID, lostPetID, foundPetID string,
	matchedAt time.Time,
) {
	t.Helper()
	reporter := &domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101"}
	finder := &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "finder-202"}
	decidedAt := matchedAt.Add(time.Minute)
	match := domain.MatchRecord{
		MatchID: matchID, MatchedPetID: lostPetID, FoundPetID: foundPetID,
		Score: 0.91, Status: domain.MatchStatusConfirmed, MatchedAt: matchedAt,
		Scores: domain.MatchScoreBreakdown{
			Visual: 0.95, Color: 1, Spatial: 0.82, DistanceMiles: 2.7, Threshold: 0.7,
		},
		LostPet:       domain.MatchPetDetail{PetID: lostPetID, Breed: "Golden Retriever"},
		FoundPet:      domain.MatchPetDetail{PetID: foundPetID, Breed: "Golden Retriever"},
		SourceEventID: sourceEventID, Model: "gemma4:e2b", ThresholdVersion: "visual-spatial-v1",
		Explanation: "High visual and geographic similarity",
	}
	participants := domain.MatchParticipantRecord{
		MatchID: matchID, LostPetID: lostPetID, FoundPetID: foundPetID,
		Reporter: reporter, Finder: finder,
		ReporterDecision: domain.MatchDecisionConfirm,
		FinderDecision:   domain.MatchDecisionConfirm,
		DecisionAudit: []domain.MatchDecisionAudit{
			{Role: domain.MatchParticipantRoleReporter, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
			{Role: domain.MatchParticipantRoleFinder, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
		},
	}
	matchData, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	participantsData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.MatchesCollection, matchID, matchData); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.MatchParticipantsCollection, matchID, participantsData); err != nil {
		t.Fatal(err)
	}
}

func firestoreReunionUpdater(
	actor domain.PrincipalRef,
	operationID string,
	resolvedAt time.Time,
) store.RoleAuthorizedMatchStateUpdater {
	return func(assignment domain.RoleAssignment, matchData, participantData []byte) ([]byte, []byte, error) {
		var match domain.MatchRecord
		if err := json.Unmarshal(matchData, &match); err != nil {
			return nil, nil, err
		}
		var participants domain.MatchParticipantRecord
		if err := json.Unmarshal(participantData, &participants); err != nil {
			return nil, nil, err
		}
		nextMatch, nextParticipants, _, err := domain.ApplyGlobalOperatorReunion(
			match, participants, actor, assignment, operationID, 5, "Reunited safely", resolvedAt,
		)
		if err != nil {
			return nil, nil, err
		}
		nextMatchData, err := json.Marshal(nextMatch)
		if err != nil {
			return nil, nil, err
		}
		nextParticipantsData, err := json.Marshal(nextParticipants)
		return nextMatchData, nextParticipantsData, err
	}
}

func loadFirestoreReunionMatch(
	t *testing.T,
	ctx context.Context,
	state store.StateStore,
	matchID string,
) (domain.MatchRecord, domain.MatchParticipantRecord) {
	t.Helper()
	matchData, err := state.GetState(ctx, store.MatchesCollection, matchID)
	if err != nil {
		t.Fatal(err)
	}
	participantsData, err := state.GetState(ctx, store.MatchParticipantsCollection, matchID)
	if err != nil {
		t.Fatal(err)
	}
	var match domain.MatchRecord
	if err := json.Unmarshal(matchData, &match); err != nil {
		t.Fatal(err)
	}
	var participants domain.MatchParticipantRecord
	if err := json.Unmarshal(participantsData, &participants); err != nil {
		t.Fatal(err)
	}
	return match, participants
}

var _ store.RoleAuthorizedMatchStateStore = (*store.FirestoreStore)(nil)
