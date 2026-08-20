package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryRoleAuthorizedStateAndOutboxUpdateFailsClosedAfterRevocation(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	principal := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "state-operator-101",
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: "bootstrap-admin"}
	scope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	if err := state.SaveState(ctx, store.LostPetsCollection, "lost-role-state", []byte(`{"status":"lost"}`)); err != nil {
		t.Fatal(err)
	}
	assignment, changed, err := state.GrantRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: principal, Role: domain.RoleOperator, Scope: scope, Actor: actor,
		OperationID: "grant-state-101", OccurredAt: time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("grant = changed %t, error %v", changed, err)
	}

	updates := 0
	err = state.UpdateStateAndCreateOutboxAsRole(
		ctx, principal, domain.RoleOperator, scope, store.LostPetsCollection, "lost-role-state",
		func(got domain.RoleAssignment, _ []byte) (store.StateWrite, store.StateWrite, error) {
			updates++
			if got.AssignmentID != assignment.AssignmentID || got.Revision != assignment.Revision {
				t.Fatalf("authorization = %#v, want %#v", got, assignment)
			}
			return store.StateWrite{
					StoreName: store.LostPetsCollection, Key: "lost-role-state", Data: []byte(`{"status":"reunited"}`),
				}, store.StateWrite{
					StoreName: store.OutboxCollection, Key: "evt-role-state", Data: []byte(`{"id":"evt-role-state"}`),
				}, nil
		},
	)
	if err != nil || updates != 1 {
		t.Fatalf("authorized update = calls %d, error %v", updates, err)
	}
	if got, err := state.GetState(ctx, store.LostPetsCollection, "lost-role-state"); err != nil || string(got) != `{"status":"reunited"}` {
		t.Fatalf("authorized state = %s, %v", got, err)
	}
	if got, err := state.GetState(ctx, store.OutboxCollection, "evt-role-state"); err != nil || string(got) != `{"id":"evt-role-state"}` {
		t.Fatalf("authorized outbox = %s, %v", got, err)
	}

	if err := state.SaveState(ctx, store.LostPetsCollection, "lost-role-denied", []byte(`{"status":"lost"}`)); err != nil {
		t.Fatal(err)
	}
	_, changed, err = state.RevokeRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: principal, Role: domain.RoleOperator, Scope: scope, Actor: actor,
		OperationID: "revoke-state-101", OccurredAt: time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("revoke = changed %t, error %v", changed, err)
	}
	err = state.UpdateStateAndCreateOutboxAsRole(
		ctx, principal, domain.RoleOperator, scope, store.LostPetsCollection, "lost-role-denied",
		func(_ domain.RoleAssignment, _ []byte) (store.StateWrite, store.StateWrite, error) {
			updates++
			return store.StateWrite{}, store.StateWrite{}, nil
		},
	)
	if !errors.Is(err, store.ErrRoleDenied) || updates != 1 {
		t.Fatalf("revoked update = calls %d, error %v; want 1/ErrRoleDenied", updates, err)
	}
	if got, err := state.GetState(ctx, store.LostPetsCollection, "lost-role-denied"); err != nil || string(got) != `{"status":"lost"}` {
		t.Fatalf("state after denied update = %s, %v", got, err)
	}
	if _, err := outbox.GetRecord(ctx, state, "evt-role-denied"); !errors.Is(err, store.ErrNotFound) &&
		!errors.Is(err, store.ErrStoreNotFound) {
		t.Fatalf("denied outbox error = %v, want not found", err)
	}
}
