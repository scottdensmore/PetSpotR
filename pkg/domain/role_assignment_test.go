package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestRoleAssignmentKeysAreStableOpaqueAndUnambiguous(t *testing.T) {
	t.Parallel()

	principal := domain.PrincipalRef{
		Issuer:  " https://securetoken.google.com/demo-petspotr-auth ",
		Subject: "operator-subject-123",
	}
	key, err := domain.RolePrincipalKey(principal)
	if err != nil {
		t.Fatalf("RolePrincipalKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "role_principal_v1_") ||
		strings.Contains(key, "securetoken") || strings.Contains(key, principal.Subject) {
		t.Fatalf("RolePrincipalKey() = %q, want opaque versioned digest", key)
	}
	canonical, err := domain.RolePrincipalKey(domain.PrincipalRef{
		Issuer:  strings.TrimSpace(principal.Issuer),
		Subject: principal.Subject,
	})
	if err != nil || canonical != key {
		t.Fatalf("canonical RolePrincipalKey() = %q, %v; want %q", canonical, err, key)
	}

	left, err := domain.RolePrincipalKey(domain.PrincipalRef{Issuer: "issuer", Subject: "a\x00b"})
	if err != nil {
		t.Fatal(err)
	}
	right, err := domain.RolePrincipalKey(domain.PrincipalRef{Issuer: "issuer\x00a", Subject: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("length-prefixed principal identities produced the same key")
	}
	changedSubject, err := domain.RolePrincipalKey(domain.PrincipalRef{
		Issuer:  strings.TrimSpace(principal.Issuer),
		Subject: " operator-subject-123 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changedSubject == key {
		t.Fatal("opaque subject whitespace was normalized")
	}
}

func TestRoleAssignmentGrantRevokeAndRegrant(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 16, 0, 0, 0, time.FixedZone("PDT", -7*60*60))
	change := validRoleAssignmentChange(grantedAt)

	granted, grantAudit, err := domain.GrantRoleAssignment(nil, change)
	if err != nil {
		t.Fatalf("GrantRoleAssignment() error = %v", err)
	}
	if granted.Version != domain.RoleAssignmentVersion || granted.Status != domain.RoleAssignmentStatusActive ||
		granted.Revision != 1 || granted.GrantedAt != grantedAt.UTC() || granted.RevokedAt != nil {
		t.Fatalf("granted assignment = %#v", granted)
	}
	if grantAudit.Action != domain.RoleAssignmentActionGrant || grantAudit.Result != granted ||
		grantAudit.OccurredAt != grantedAt.UTC() {
		t.Fatalf("grant audit = %#v", grantAudit)
	}
	if strings.Contains(granted.PrincipalKey, change.Target.Subject) ||
		strings.Contains(granted.GrantedByKey, change.Actor.Subject) {
		t.Fatalf("assignment leaked principal data: %#v", granted)
	}
	if err := granted.Validate(); err != nil {
		t.Fatalf("granted Validate() error = %v", err)
	}
	if err := grantAudit.Validate(); err != nil {
		t.Fatalf("grant audit Validate() error = %v", err)
	}

	if _, _, err := domain.GrantRoleAssignment(&granted, change); !errors.Is(err, domain.ErrRoleAlreadyActive) {
		t.Fatalf("duplicate active grant error = %v, want ErrRoleAlreadyActive", err)
	}

	revokedAt := grantedAt.Add(time.Hour)
	revokeChange := change
	revokeChange.OperationID = "role-op-revoke-1"
	revokeChange.OccurredAt = revokedAt
	revoked, revokeAudit, err := domain.RevokeRoleAssignment(&granted, revokeChange)
	if err != nil {
		t.Fatalf("RevokeRoleAssignment() error = %v", err)
	}
	if revoked.Status != domain.RoleAssignmentStatusRevoked || revoked.Revision != 2 ||
		revoked.RevokedAt == nil || *revoked.RevokedAt != revokedAt.UTC() ||
		revoked.RevokedByKey == "" {
		t.Fatalf("revoked assignment = %#v", revoked)
	}
	if revokeAudit.Action != domain.RoleAssignmentActionRevoke || revokeAudit.Result != revoked {
		t.Fatalf("revoke audit = %#v", revokeAudit)
	}
	outOfOrderRegrant := change
	outOfOrderRegrant.OperationID = "role-op-out-of-order-regrant"
	outOfOrderRegrant.OccurredAt = revokedAt.Add(-time.Minute)
	if _, _, err := domain.GrantRoleAssignment(&revoked, outOfOrderRegrant); err == nil {
		t.Fatal("out-of-order regrant error = nil")
	}

	regrantChange := change
	regrantChange.OperationID = "role-op-regrant-1"
	regrantChange.OccurredAt = revokedAt.Add(time.Hour)
	regranted, _, err := domain.GrantRoleAssignment(&revoked, regrantChange)
	if err != nil {
		t.Fatalf("regrant error = %v", err)
	}
	if regranted.Status != domain.RoleAssignmentStatusActive || regranted.Revision != 3 ||
		regranted.RevokedAt != nil || regranted.RevokedByKey != "" ||
		regranted.GrantedAt != regrantChange.OccurredAt.UTC() {
		t.Fatalf("regranted assignment = %#v", regranted)
	}
}

func TestRoleAssignmentRejectsInvalidScopesAndRecords(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 20, 23, 0, 0, 0, time.UTC)
	change := validRoleAssignmentChange(now)
	tests := []struct {
		name   string
		mutate func(*domain.RoleAssignmentChange)
	}{
		{name: "unknown role", mutate: func(c *domain.RoleAssignmentChange) { c.Role = "administrator" }},
		{name: "unknown scope", mutate: func(c *domain.RoleAssignmentChange) { c.Scope.Kind = "region" }},
		{name: "global scope with ID", mutate: func(c *domain.RoleAssignmentChange) {
			c.Scope = domain.RoleScope{Kind: domain.RoleScopeGlobal, ShelterID: "shelter-1"}
		}},
		{name: "shelter scope without ID", mutate: func(c *domain.RoleAssignmentChange) {
			c.Scope = domain.RoleScope{Kind: domain.RoleScopeShelter}
		}},
		{name: "noncanonical shelter ID", mutate: func(c *domain.RoleAssignmentChange) {
			c.Scope = domain.RoleScope{Kind: domain.RoleScopeShelter, ShelterID: " shelter-1 "}
		}},
		{name: "missing operation", mutate: func(c *domain.RoleAssignmentChange) { c.OperationID = "" }},
		{name: "missing time", mutate: func(c *domain.RoleAssignmentChange) { c.OccurredAt = time.Time{} }},
		{name: "invalid actor", mutate: func(c *domain.RoleAssignmentChange) { c.Actor.Subject = "" }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			invalid := change
			test.mutate(&invalid)
			if _, _, err := domain.GrantRoleAssignment(nil, invalid); err == nil {
				t.Fatal("GrantRoleAssignment() error = nil, want invalid change")
			}
		})
	}

	if _, _, err := domain.RevokeRoleAssignment(nil, change); !errors.Is(err, domain.ErrRoleAssignmentMissing) {
		t.Fatalf("RevokeRoleAssignment(nil) error = %v, want ErrRoleAssignmentMissing", err)
	}

	assignment, audit, err := domain.GrantRoleAssignment(nil, change)
	if err != nil {
		t.Fatal(err)
	}
	tampered := assignment
	tampered.AssignmentID = "role_assignment_v1_" + strings.Repeat("0", 64)
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered assignment Validate() error = nil")
	}
	tamperedAudit := audit
	tamperedAudit.Result.Status = domain.RoleAssignmentStatusRevoked
	if err := tamperedAudit.Validate(); err == nil {
		t.Fatal("tampered audit Validate() error = nil")
	}
	revoked, _, err := domain.RevokeRoleAssignment(&assignment, domain.RoleAssignmentChange{
		Target: change.Target, Role: change.Role, Scope: change.Scope, Actor: change.Actor,
		OperationID: "role-op-revoke-tamper", OccurredAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	revoked.Revision = 1
	if err := revoked.Validate(); err == nil {
		t.Fatal("revoked revision-one assignment Validate() error = nil")
	}
}

func TestRoleAssignmentAuditChainRejectsCorruptedTransitions(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 23, 0, 0, 0, time.UTC)
	grantChange := validRoleAssignmentChange(grantedAt)
	granted, grantAudit, err := domain.GrantRoleAssignment(nil, grantChange)
	if err != nil {
		t.Fatal(err)
	}
	revokeChange := grantChange
	revokeChange.OperationID = "role-op-chain-revoke"
	revokeChange.OccurredAt = grantedAt.Add(time.Hour)
	revoked, revokeAudit, err := domain.RevokeRoleAssignment(&granted, revokeChange)
	if err != nil {
		t.Fatal(err)
	}
	regrantChange := grantChange
	regrantChange.OperationID = "role-op-chain-regrant"
	regrantChange.OccurredAt = grantedAt.Add(2 * time.Hour)
	_, regrantAudit, err := domain.GrantRoleAssignment(&revoked, regrantChange)
	if err != nil {
		t.Fatal(err)
	}
	valid := []domain.RoleAssignmentAudit{grantAudit, revokeAudit, regrantAudit}
	if err := domain.ValidateRoleAssignmentAuditChain(valid); err != nil {
		t.Fatalf("ValidateRoleAssignmentAuditChain(valid) error = %v", err)
	}

	flippedTombstone := revoked
	flippedTombstone.Status = domain.RoleAssignmentStatusActive
	flippedTombstone.RevokedByKey = ""
	flippedTombstone.RevokedAt = nil
	if err := flippedTombstone.Validate(); err == nil {
		t.Fatal("flipped revision-two tombstone Validate() error = nil")
	}

	otherActorKey, err := domain.RolePrincipalKey(domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/demo-petspotr-auth", Subject: "rewritten-actor",
	})
	if err != nil {
		t.Fatal(err)
	}
	rewrittenGrant := append([]domain.RoleAssignmentAudit(nil), valid...)
	rewrittenGrant[1].Result.GrantedByKey = otherActorKey
	rewrittenGrant[1].Result.GrantedAt = grantedAt.Add(-time.Hour)
	if err := rewrittenGrant[1].Validate(); err != nil {
		t.Fatalf("individually valid rewritten revoke audit error = %v", err)
	}
	if err := domain.ValidateRoleAssignmentAuditChain(rewrittenGrant); err == nil {
		t.Fatal("rewritten grant metadata chain error = nil")
	}

	outOfOrderRegrant := append([]domain.RoleAssignmentAudit(nil), valid...)
	outOfOrderTime := revokeChange.OccurredAt.Add(-time.Minute)
	outOfOrderRegrant[2].OccurredAt = outOfOrderTime
	outOfOrderRegrant[2].Result.GrantedAt = outOfOrderTime
	if err := outOfOrderRegrant[2].Validate(); err != nil {
		t.Fatalf("individually valid out-of-order regrant audit error = %v", err)
	}
	if err := domain.ValidateRoleAssignmentAuditChain(outOfOrderRegrant); err == nil {
		t.Fatal("out-of-order regrant chain error = nil")
	}
}

func validRoleAssignmentChange(occurredAt time.Time) domain.RoleAssignmentChange {
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
		OperationID: "role-op-grant-1",
		OccurredAt:  occurredAt,
	}
}
