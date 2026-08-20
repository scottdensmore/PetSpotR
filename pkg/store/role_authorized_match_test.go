package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryRoleAuthorizedMatchUpdateFailsClosedAfterRevocation(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	principal := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "operator-101",
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: "bootstrap-admin"}
	scope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	if err := state.SaveState(ctx, store.MatchesCollection, "match-101", []byte(`{"status":"CONFIRMED"}`)); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.MatchParticipantsCollection, "match-101", []byte(`{"matchId":"match-101"}`)); err != nil {
		t.Fatal(err)
	}
	_, changed, err := state.GrantRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: principal, Role: domain.RoleOperator, Scope: scope, Actor: actor,
		OperationID: "grant-101", OccurredAt: time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("grant = changed %t, error %v", changed, err)
	}

	updates := 0
	err = state.UpdateMatchAndParticipantsAsRole(
		ctx, principal, domain.RoleOperator, scope, "match-101",
		func(_ domain.RoleAssignment, _, _ []byte) ([]byte, []byte, error) {
			updates++
			return []byte(`{"status":"REUNITED"}`), []byte(`{"matchId":"match-101","audited":true}`), nil
		},
	)
	if err != nil || updates != 1 {
		t.Fatalf("authorized update = calls %d, error %v", updates, err)
	}

	_, changed, err = state.RevokeRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: principal, Role: domain.RoleOperator, Scope: scope, Actor: actor,
		OperationID: "revoke-101", OccurredAt: time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("revoke = changed %t, error %v", changed, err)
	}
	err = state.UpdateMatchAndParticipantsAsRole(
		ctx, principal, domain.RoleOperator, scope, "match-101",
		func(_ domain.RoleAssignment, _, _ []byte) ([]byte, []byte, error) {
			updates++
			return []byte(`{"status":"CORRUPTED"}`), []byte(`{"audited":false}`), nil
		},
	)
	if !errors.Is(err, store.ErrRoleDenied) || updates != 1 {
		t.Fatalf("revoked update = calls %d, error %v; want 1/ErrRoleDenied", updates, err)
	}
	match, err := state.GetState(ctx, store.MatchesCollection, "match-101")
	if err != nil || string(match) != `{"status":"REUNITED"}` {
		t.Fatalf("match after denied update = %s, %v", match, err)
	}
}
