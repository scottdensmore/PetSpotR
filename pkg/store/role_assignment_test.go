package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryRoleAssignmentsAreAtomicIdempotentAndAudited(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	roles := store.NewMemoryStore()
	change := validStoreRoleChange(time.Date(2026, time.August, 20, 23, 0, 0, 0, time.UTC))
	if err := roles.SaveState(ctx, "operatorRoleAssignments", "injected", []byte(`{"status":"active"}`)); err == nil {
		t.Fatal("generic SaveState() accepted the private role collection")
	}

	if _, err := roles.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetRoleAssignment(missing) error = %v, want ErrNotFound", err)
	}

	granted, changed, err := roles.GrantRoleAssignment(ctx, change)
	if err != nil || !changed {
		t.Fatalf("GrantRoleAssignment() = %#v, %t, %v; want changed", granted, changed, err)
	}
	retry, changed, err := roles.GrantRoleAssignment(ctx, change)
	if err != nil || changed || retry != granted {
		t.Fatalf("GrantRoleAssignment(retry) = %#v, %t, %v; want original no-op", retry, changed, err)
	}

	collision := change
	collision.Actor.Subject = "different-actor"
	if _, _, err := roles.GrantRoleAssignment(ctx, collision); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("GrantRoleAssignment(operation collision) error = %v, want ErrConflict", err)
	}
	secondGrant := change
	secondGrant.OperationID = "grant-operation-2"
	secondGrant.OccurredAt = change.OccurredAt.Add(time.Minute)
	if _, _, err := roles.GrantRoleAssignment(ctx, secondGrant); !errors.Is(err, domain.ErrRoleAlreadyActive) {
		t.Fatalf("GrantRoleAssignment(active) error = %v, want ErrRoleAlreadyActive", err)
	}

	revoke := change
	revoke.OperationID = "revoke-operation-1"
	revoke.OccurredAt = change.OccurredAt.Add(time.Hour)
	revoked, changed, err := roles.RevokeRoleAssignment(ctx, revoke)
	if err != nil || !changed || revoked.Status != domain.RoleAssignmentStatusRevoked {
		t.Fatalf("RevokeRoleAssignment() = %#v, %t, %v; want revoked", revoked, changed, err)
	}
	retry, changed, err = roles.RevokeRoleAssignment(ctx, revoke)
	if err != nil || changed || !reflect.DeepEqual(retry, revoked) {
		t.Fatalf("RevokeRoleAssignment(retry) = %#v, %t, %v; want original no-op", retry, changed, err)
	}

	stored, err := roles.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope)
	if err != nil || !reflect.DeepEqual(stored, revoked) {
		t.Fatalf("GetRoleAssignment() = %#v, %v; want revoked state", stored, err)
	}

	audit, err := roles.ListRoleAssignmentAudit(ctx, granted.AssignmentID)
	if err != nil {
		t.Fatalf("ListRoleAssignmentAudit() error = %v", err)
	}
	if len(audit) != 2 || audit[0].Result.Revision != 1 || audit[1].Result.Revision != 2 ||
		audit[0].Action != domain.RoleAssignmentActionGrant || audit[1].Action != domain.RoleAssignmentActionRevoke {
		t.Fatalf("audit = %#v, want ordered grant and revoke", audit)
	}

	originalAudit := audit[0]
	audit[0].OperationID = "mutated"
	again, err := roles.ListRoleAssignmentAudit(ctx, granted.AssignmentID)
	if err != nil || again[0] != originalAudit {
		t.Fatalf("stored audit mutated through returned value: %#v, %v", again, err)
	}

	regrant := change
	regrant.OperationID = "regrant-operation-1"
	regrant.OccurredAt = change.OccurredAt.Add(2 * time.Hour)
	regranted, changed, err := roles.GrantRoleAssignment(ctx, regrant)
	if err != nil || !changed || regranted.Status != domain.RoleAssignmentStatusActive || regranted.Revision != 3 {
		t.Fatalf("regrant = %#v, %t, %v", regranted, changed, err)
	}

	persistedJSON, err := json.Marshal(regranted)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{change.Target.Issuer, change.Target.Subject, change.Actor.Subject, "@"} {
		if bytes.Contains(persistedJSON, []byte(secret)) {
			t.Fatalf("persisted assignment contains private identity %q: %s", secret, persistedJSON)
		}
	}
}

func TestMemoryRoleAssignmentConcurrentGrantHasOneWinner(t *testing.T) {
	t.Parallel()

	roles := store.NewMemoryStore()
	change := validStoreRoleChange(time.Date(2026, time.August, 20, 23, 0, 0, 0, time.UTC))
	type result struct {
		changed bool
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			attempt := change
			attempt.OperationID += string(rune('a' + index))
			attempt.OccurredAt = attempt.OccurredAt.Add(time.Duration(index) * time.Second)
			_, changed, err := roles.GrantRoleAssignment(context.Background(), attempt)
			results <- result{changed: changed, err: err}
		}(i)
	}
	wg.Wait()
	close(results)

	winners := 0
	alreadyActive := 0
	for got := range results {
		if got.err == nil && got.changed {
			winners++
		}
		if errors.Is(got.err, domain.ErrRoleAlreadyActive) {
			alreadyActive++
		}
	}
	if winners != 1 || alreadyActive != 1 {
		t.Fatalf("concurrent grants = %d winners, %d already active; want 1/1", winners, alreadyActive)
	}
}

func TestMemoryRoleAssignmentsRespectContextCancellation(t *testing.T) {
	t.Parallel()

	roles := store.NewMemoryStore()
	change := validStoreRoleChange(time.Now().UTC())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := roles.GrantRoleAssignment(ctx, change); !errors.Is(err, context.Canceled) {
		t.Fatalf("GrantRoleAssignment(canceled) error = %v", err)
	}
	if _, err := roles.GetRoleAssignment(ctx, change.Target, change.Role, change.Scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetRoleAssignment(canceled) error = %v", err)
	}
	if _, err := roles.ListRoleAssignmentAudit(ctx, "role_assignment_v1_missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRoleAssignmentAudit(canceled) error = %v", err)
	}
}

func validStoreRoleChange(occurredAt time.Time) domain.RoleAssignmentChange {
	return domain.RoleAssignmentChange{
		Target: domain.PrincipalRef{
			Issuer:  "https://securetoken.google.com/demo-petspotr-auth",
			Subject: "operator-subject-123",
		},
		Role:  domain.RoleOperator,
		Scope: domain.RoleScope{Kind: domain.RoleScopeShelter, ShelterID: "shelter-seattle-1"},
		Actor: domain.PrincipalRef{
			Issuer:  "https://securetoken.google.com/demo-petspotr-auth",
			Subject: "granting-admin-456",
		},
		OperationID: "grant-operation-1",
		OccurredAt:  occurredAt,
	}
}
