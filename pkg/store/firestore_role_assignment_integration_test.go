package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type rawRoleRecord struct {
	Key  string `firestore:"key"`
	Data []byte `firestore:"data"`
}

func TestFirestoreRoleAssignmentsCrossRuntimeAndFailClosed(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const projectID = "petspotr-role-assignment-contract"
	config := runtimeconfig.StateConfig{
		Mode:                  runtimeconfig.ModeLocalEmulator,
		ProjectID:             projectID,
		FirestoreEmulatorHost: host,
	}
	writer, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewStateRuntime(writer) error = %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewStateRuntime(reader) error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	rawClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("firestore.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = rawClient.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	change := validStoreRoleChange(time.Now().UTC().Truncate(time.Microsecond))
	change.Target.Subject += "-" + suffix
	change.Actor.Subject += "-" + suffix
	change.OperationID += "-" + suffix
	assignmentID, err := domain.RoleAssignmentID(change.Target, change.Role, change.Scope)
	if err != nil {
		t.Fatal(err)
	}
	grantAuditID, err := domain.RoleAssignmentAuditID(assignmentID, change.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	assignmentDoc := rawClient.Collection("operatorRoleAssignments").Doc(firestoreDocumentIDForTest(assignmentID))
	grantAuditDoc := assignmentDoc.Collection("audit").Doc(firestoreDocumentIDForTest(grantAuditID))

	revoke := change
	revoke.OperationID = "revoke-operation-" + suffix
	revoke.OccurredAt = change.OccurredAt.Add(time.Minute)
	revokeAuditID, err := domain.RoleAssignmentAuditID(assignmentID, revoke.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	revokeAuditDoc := assignmentDoc.Collection("audit").Doc(firestoreDocumentIDForTest(revokeAuditID))

	malformedTarget := change.Target
	malformedTarget.Subject += "-malformed"
	malformedID, err := domain.RoleAssignmentID(malformedTarget, change.Role, change.Scope)
	if err != nil {
		t.Fatal(err)
	}
	malformedDoc := rawClient.Collection("operatorRoleAssignments").Doc(firestoreDocumentIDForTest(malformedID))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = grantAuditDoc.Delete(cleanupCtx)
		_, _ = revokeAuditDoc.Delete(cleanupCtx)
		_, _ = assignmentDoc.Delete(cleanupCtx)
		_, _ = malformedDoc.Delete(cleanupCtx)
	})

	granted, changed, err := writer.RoleAssignments.GrantRoleAssignment(ctx, change)
	if err != nil || !changed {
		t.Fatalf("writer GrantRoleAssignment() = %#v, %t, %v", granted, changed, err)
	}
	fromReader, err := reader.RoleAssignments.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope)
	if err != nil || !reflect.DeepEqual(fromReader, granted) {
		t.Fatalf("reader GetRoleAssignment() = %#v, %v; want %#v", fromReader, err, granted)
	}
	retry, changed, err := reader.RoleAssignments.GrantRoleAssignment(ctx, change)
	if err != nil || changed || !reflect.DeepEqual(retry, granted) {
		t.Fatalf("cross-runtime exact retry = %#v, %t, %v; want original no-op", retry, changed, err)
	}

	assignmentSnapshot, err := assignmentDoc.Get(ctx)
	if err != nil {
		t.Fatalf("read raw assignment: %v", err)
	}
	var rawAssignment rawRoleRecord
	if err := assignmentSnapshot.DataTo(&rawAssignment); err != nil {
		t.Fatalf("decode raw assignment: %v", err)
	}
	if rawAssignment.Key != assignmentID {
		t.Fatalf("raw assignment key = %q, want %q", rawAssignment.Key, assignmentID)
	}
	for _, privateValue := range []string{change.Target.Issuer, change.Target.Subject, change.Actor.Subject, "@"} {
		if bytes.Contains(rawAssignment.Data, []byte(privateValue)) || bytes.Contains([]byte(rawAssignment.Key), []byte(privateValue)) {
			t.Fatalf("raw assignment leaked private identity %q: key=%s data=%s", privateValue, rawAssignment.Key, rawAssignment.Data)
		}
	}
	if _, err := grantAuditDoc.Get(ctx); err != nil {
		t.Fatalf("grant audit was not committed atomically: %v", err)
	}
	grantAuditSnapshot, err := grantAuditDoc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var rawAudit rawRoleRecord
	if err := grantAuditSnapshot.DataTo(&rawAudit); err != nil {
		t.Fatalf("decode raw audit: %v", err)
	}
	for _, privateValue := range []string{change.Target.Issuer, change.Target.Subject, change.Actor.Subject, "@"} {
		if bytes.Contains(rawAudit.Data, []byte(privateValue)) || bytes.Contains([]byte(rawAudit.Key), []byte(privateValue)) {
			t.Fatalf("raw audit leaked private identity %q: key=%s data=%s", privateValue, rawAudit.Key, rawAudit.Data)
		}
	}
	if err := reader.Store.SaveState(ctx, "operatorRoleAssignments", assignmentID, []byte(`{"status":"injected"}`)); err == nil {
		t.Fatal("generic StateStore accepted the private role collection")
	}

	revoked, changed, err := reader.RoleAssignments.RevokeRoleAssignment(ctx, revoke)
	if err != nil || !changed || revoked.Status != domain.RoleAssignmentStatusRevoked {
		t.Fatalf("reader RevokeRoleAssignment() = %#v, %t, %v", revoked, changed, err)
	}
	fromWriter, err := writer.RoleAssignments.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope)
	if err != nil || !reflect.DeepEqual(fromWriter, revoked) {
		t.Fatalf("writer observed after revoke = %#v, %v; want %#v", fromWriter, err, revoked)
	}
	restarted, err := runtimeconfig.NewStateRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewStateRuntime(restarted) error = %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	fromRestarted, err := restarted.RoleAssignments.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope)
	if err != nil || !reflect.DeepEqual(fromRestarted, revoked) {
		t.Fatalf("restarted runtime observed = %#v, %v; want %#v", fromRestarted, err, revoked)
	}
	audit, err := writer.RoleAssignments.ListRoleAssignmentAudit(ctx, assignmentID)
	if err != nil || len(audit) != 2 || audit[0].Result.Revision != 1 || audit[1].Result.Revision != 2 {
		t.Fatalf("cross-runtime audit = %#v, %v", audit, err)
	}

	malformedData, err := json.Marshal(map[string]any{"version": 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := malformedDoc.Set(ctx, rawRoleRecord{Key: malformedID, Data: malformedData}); err != nil {
		t.Fatalf("seed malformed assignment: %v", err)
	}
	if _, err := reader.RoleAssignments.GetRoleAssignment(ctx, malformedTarget, change.Role, change.Scope); err == nil {
		t.Fatal("GetRoleAssignment(malformed) error = nil, want fail closed")
	}

	validData, err := json.Marshal(granted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := malformedDoc.Set(ctx, rawRoleRecord{Key: assignmentID, Data: validData}); err != nil {
		t.Fatalf("seed key-mismatched assignment: %v", err)
	}
	if _, err := reader.RoleAssignments.GetRoleAssignment(ctx, malformedTarget, change.Role, change.Scope); err == nil {
		t.Fatal("GetRoleAssignment(key mismatch) error = nil, want fail closed")
	}

	concurrent := change
	concurrent.Target.Subject += "-concurrent"
	concurrent.OperationID = "concurrent-grant-a-" + suffix
	concurrentID, err := domain.RoleAssignmentID(concurrent.Target, concurrent.Role, concurrent.Scope)
	if err != nil {
		t.Fatal(err)
	}
	concurrentDoc := rawClient.Collection("operatorRoleAssignments").Doc(firestoreDocumentIDForTest(concurrentID))
	concurrentB := concurrent
	concurrentB.OperationID = "concurrent-grant-b-" + suffix
	concurrentB.OccurredAt = concurrent.OccurredAt.Add(time.Second)
	concurrentAuditA, err := domain.RoleAssignmentAuditID(concurrentID, concurrent.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	concurrentAuditB, err := domain.RoleAssignmentAuditID(concurrentID, concurrentB.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = concurrentDoc.Collection("audit").Doc(firestoreDocumentIDForTest(concurrentAuditA)).Delete(cleanupCtx)
		_, _ = concurrentDoc.Collection("audit").Doc(firestoreDocumentIDForTest(concurrentAuditB)).Delete(cleanupCtx)
		_, _ = concurrentDoc.Delete(cleanupCtx)
	})
	type grantResult struct {
		changed bool
		err     error
	}
	grantResults := make(chan grantResult, 2)
	var grants sync.WaitGroup
	for index, attempt := range []domain.RoleAssignmentChange{concurrent, concurrentB} {
		index, attempt := index, attempt
		grants.Add(1)
		go func() {
			defer grants.Done()
			roleStore := writer.RoleAssignments
			if index == 1 {
				roleStore = reader.RoleAssignments
			}
			_, changed, grantErr := roleStore.GrantRoleAssignment(ctx, attempt)
			grantResults <- grantResult{changed: changed, err: grantErr}
		}()
	}
	grants.Wait()
	close(grantResults)
	winners := 0
	alreadyActive := 0
	for result := range grantResults {
		if result.err == nil && result.changed {
			winners++
		}
		if errors.Is(result.err, domain.ErrRoleAlreadyActive) {
			alreadyActive++
		}
	}
	if winners != 1 || alreadyActive != 1 {
		t.Fatalf("concurrent Firestore grants = %d winners, %d already active; want 1/1", winners, alreadyActive)
	}
	concurrentAudit, err := reader.RoleAssignments.ListRoleAssignmentAudit(ctx, concurrentID)
	if err != nil || len(concurrentAudit) != 1 {
		t.Fatalf("concurrent grant audit = %#v, %v; want one committed receipt", concurrentAudit, err)
	}

	flippedTombstone := revoked
	flippedTombstone.Status = domain.RoleAssignmentStatusActive
	flippedTombstone.RevokedByKey = ""
	flippedTombstone.RevokedAt = nil
	flippedData, err := json.Marshal(flippedTombstone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignmentDoc.Set(ctx, rawRoleRecord{Key: assignmentID, Data: flippedData}); err != nil {
		t.Fatalf("seed flipped tombstone: %v", err)
	}
	if _, err := reader.RoleAssignments.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope); err == nil {
		t.Fatal("GetRoleAssignment(flipped tombstone) error = nil, want fail closed")
	}

	rewrittenRevoke := audit[1]
	rewrittenActorKey, err := domain.RolePrincipalKey(domain.PrincipalRef{
		Issuer: change.Actor.Issuer, Subject: "rewritten-audit-actor-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	rewrittenRevoke.Result.GrantedByKey = rewrittenActorKey
	rewrittenRevoke.Result.GrantedAt = change.OccurredAt.Add(-time.Hour)
	rewrittenData, err := json.Marshal(rewrittenRevoke)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := revokeAuditDoc.Set(ctx, rawRoleRecord{Key: revokeAuditID, Data: rewrittenData}); err != nil {
		t.Fatalf("seed rewritten audit: %v", err)
	}
	if _, err := reader.RoleAssignments.ListRoleAssignmentAudit(ctx, assignmentID); err == nil {
		t.Fatal("ListRoleAssignmentAudit(rewritten chain) error = nil, want fail closed")
	}

	canceledCtx, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	if _, err := reader.RoleAssignments.GetRoleAssignment(canceledCtx, change.Target, change.Role, change.Scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetRoleAssignment(canceled) error = %v, want context.Canceled", err)
	}
}

var _ store.RoleAssignmentStore = (*store.FirestoreStore)(nil)
