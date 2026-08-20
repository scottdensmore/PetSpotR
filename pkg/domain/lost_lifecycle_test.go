package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestApplyOwnerLostPetReunionIsAuditedRedactedAndIdempotent(t *testing.T) {
	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-lifecycle"}
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-lifecycle", ReporterEmail: "owner@example.com", ReportedAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC),
		Location: "Seattle, WA", OwnedBy: &owner,
	})
	record, _ := report.Persisted()
	changedAt := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)

	result, err := domain.ApplyOwnerLostPetReunion(record, owner, "reunite-lifecycle", changedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Record.Status != domain.LostPetStatusReunited || result.EventID == "" ||
		result.Record.LifecycleAudit == nil || result.Record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("lifecycle result = %#v", result)
	}
	eventData, err := json.Marshal(result.Event)
	if err != nil {
		t.Fatal(err)
	}
	auditData, err := json.Marshal(result.Record.LifecycleAudit)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{owner.Issuer, owner.Subject, "owner@example.com"} {
		if strings.Contains(string(eventData), private) || strings.Contains(string(auditData), private) {
			t.Fatalf("lifecycle data exposed private identity %q: %s / %s", private, eventData, auditData)
		}
	}

	retry, err := domain.ApplyOwnerLostPetReunion(result.Record, owner, "reunite-lifecycle", changedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if retry.Changed || retry.EventID != result.EventID || !retry.Event.ChangedAt.Equal(changedAt) {
		t.Fatalf("exact retry = %#v", retry)
	}
	if _, err := domain.ApplyOwnerLostPetReunion(result.Record, owner, "different-operation", changedAt); !errors.Is(err, domain.ErrLostPetLifecycleConflict) {
		t.Fatalf("changed operation error = %v", err)
	}
	stranger := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "stranger"}
	if _, err := domain.ApplyOwnerLostPetReunion(record, stranger, "reunite-lifecycle", changedAt); !errors.Is(err, domain.ErrLostPetNotOwned) {
		t.Fatalf("wrong owner error = %v", err)
	}
}

func TestApplyOwnerLostPetReunionRejectsCorruptLifecycleAudit(t *testing.T) {
	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-corrupt"}
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-corrupt", ReporterEmail: "owner@example.com", ReportedAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC),
		OwnedBy: &owner,
	})
	record, _ := report.Persisted()
	splicedOwnerRef := record
	splicedOwnerRef.OwnerIdentityRef = "contact_spliced"
	if _, err := domain.ApplyOwnerLostPetReunion(
		splicedOwnerRef, owner, "reunite-corrupt", time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	); err == nil {
		t.Fatal("spliced owner reference error = nil")
	}
	result, err := domain.ApplyOwnerLostPetReunion(record, owner, "reunite-corrupt", time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	result.Record.LifecycleAudit.ActorKey = "principal-v1-" + strings.Repeat("0", 64)
	if _, err := domain.ApplyOwnerLostPetReunion(result.Record, owner, "reunite-corrupt", time.Now().UTC()); err == nil {
		t.Fatal("corrupt audit error = nil")
	}
}

func TestApplyGlobalOperatorLostPetReunionPinsAssignmentAndRequiresActiveRole(t *testing.T) {
	owner := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-operator-domain"}
	operator := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "operator-domain"}
	grantor := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "bootstrap-admin"}
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-operator-domain", ReporterEmail: "owner@example.com",
		ReportedAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), OwnedBy: &owner,
	})
	record, _ := report.Persisted()
	assignment := validGlobalOperatorAssignment(t, operator, grantor, 1, domain.RoleAssignmentStatusActive)
	changedAt := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)

	result, err := domain.ApplyGlobalOperatorLostPetReunion(
		record, operator, assignment, "operator-reunite", changedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	audit := result.Record.LifecycleAudit
	if !result.Changed || audit == nil || audit.AuthorizedAs != domain.LostPetLifecycleAuthorizationOperator ||
		audit.AssignmentID != assignment.AssignmentID || audit.AssignmentRevision != assignment.Revision {
		t.Fatalf("operator lifecycle result = %#v", result)
	}
	auditData, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{operator.Issuer, operator.Subject} {
		if strings.Contains(string(auditData), private) {
			t.Fatalf("operator audit exposed private identity %q: %s", private, auditData)
		}
	}

	regrant := validGlobalOperatorAssignment(t, operator, grantor, 3, domain.RoleAssignmentStatusActive)
	retry, err := domain.ApplyGlobalOperatorLostPetReunion(
		result.Record, operator, regrant, "operator-reunite", changedAt.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Changed || retry.Record.LifecycleAudit.AssignmentRevision != assignment.Revision ||
		!retry.Event.ChangedAt.Equal(changedAt) {
		t.Fatalf("operator exact retry after regrant = %#v", retry)
	}

	revoked := validGlobalOperatorAssignment(t, operator, grantor, 2, domain.RoleAssignmentStatusRevoked)
	if _, err := domain.ApplyGlobalOperatorLostPetReunion(
		result.Record, operator, revoked, "operator-reunite", changedAt,
	); !errors.Is(err, domain.ErrLostPetNotOwned) {
		t.Fatalf("revoked operator retry error = %v, want ErrLostPetNotOwned", err)
	}
	result.Record.LifecycleAudit.AssignmentRevision = 2
	if _, err := domain.ApplyOwnerLostPetReunion(
		result.Record, owner, "owner-retry", changedAt,
	); !errors.Is(err, domain.ErrInvalidLostPetLifecycle) {
		t.Fatalf("corrupt operator audit error = %v, want ErrInvalidLostPetLifecycle", err)
	}

	legacyReport := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-ownerless-operator-domain", ReporterEmail: "legacy@example.com",
		ReportedAt: time.Date(2026, 8, 20, 17, 30, 0, 0, time.UTC), Location: "Tacoma, WA",
	})
	legacyRecord, _ := legacyReport.Persisted()
	legacyResult, err := domain.ApplyGlobalOperatorLostPetReunion(
		legacyRecord, operator, assignment, "operator-ownerless-reunite", changedAt,
	)
	if err != nil || legacyResult.Record.LifecycleAudit == nil ||
		legacyResult.Record.LifecycleAudit.AuthorizedAs != domain.LostPetLifecycleAuthorizationOperator {
		t.Fatalf("ownerless operator lifecycle = %#v, %v", legacyResult, err)
	}
	if _, err := domain.ApplyOwnerLostPetReunion(
		legacyRecord, owner, "owner-ownerless-reunite", changedAt,
	); !errors.Is(err, domain.ErrLostPetNotOwned) {
		t.Fatalf("ownerless owner lifecycle error = %v, want ErrLostPetNotOwned", err)
	}
}

func validGlobalOperatorAssignment(
	t *testing.T,
	operator domain.PrincipalRef,
	grantor domain.PrincipalRef,
	revision int64,
	status domain.RoleAssignmentStatus,
) domain.RoleAssignment {
	t.Helper()
	principalKey, err := domain.RolePrincipalKey(operator)
	if err != nil {
		t.Fatal(err)
	}
	grantorKey, err := domain.RolePrincipalKey(grantor)
	if err != nil {
		t.Fatal(err)
	}
	scope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	assignmentID, err := domain.RoleAssignmentID(operator, domain.RoleOperator, scope)
	if err != nil {
		t.Fatal(err)
	}
	assignment := domain.RoleAssignment{
		Version: domain.RoleAssignmentVersion, AssignmentID: assignmentID, PrincipalKey: principalKey,
		Role: domain.RoleOperator, Scope: scope, Status: status, Revision: revision,
		GrantedByKey: grantorKey, GrantedAt: time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC),
	}
	if status == domain.RoleAssignmentStatusRevoked {
		revokedAt := assignment.GrantedAt.Add(time.Hour)
		assignment.RevokedByKey = grantorKey
		assignment.RevokedAt = &revokedAt
	}
	if err := assignment.Validate(); err != nil {
		t.Fatal(err)
	}
	return assignment
}
