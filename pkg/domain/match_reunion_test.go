package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestApplyGlobalOperatorReunionIsAuditedAndIdempotent(t *testing.T) {
	match, participants := confirmedReunionFixture(t)
	actor := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "operator-101",
	}
	resolvedAt := time.Date(2026, time.August, 20, 14, 0, 0, 0, time.UTC)
	authorization := activeGlobalOperatorAssignment(t, actor, resolvedAt.Add(-time.Hour))

	nextMatch, nextParticipants, changed, err := domain.ApplyGlobalOperatorReunion(
		match, participants, actor, authorization, "resolve-101", 5, "Reunited safely", resolvedAt,
	)
	if err != nil || !changed {
		t.Fatalf("ApplyGlobalOperatorReunion() = changed %t, error %v", changed, err)
	}
	if nextMatch.Status != domain.MatchStatusReunited || nextParticipants.ReunionAudit == nil {
		t.Fatalf("reunion result = %#v / %#v", nextMatch, nextParticipants.ReunionAudit)
	}
	if err := nextParticipants.ReunionAudit.Validate(); err != nil {
		t.Fatalf("reunion audit validation: %v", err)
	}
	if strings.Contains(nextParticipants.ReunionAudit.ActorKey, actor.Issuer) ||
		strings.Contains(nextParticipants.ReunionAudit.ActorKey, actor.Subject) {
		t.Fatalf("reunion audit actor key exposed raw principal: %q", nextParticipants.ReunionAudit.ActorKey)
	}
	if nextParticipants.ReunionAudit.AssignmentID != authorization.AssignmentID ||
		nextParticipants.ReunionAudit.AssignmentRevision != authorization.Revision {
		t.Fatalf("reunion audit assignment = %q revision %d, want %q revision %d",
			nextParticipants.ReunionAudit.AssignmentID,
			nextParticipants.ReunionAudit.AssignmentRevision,
			authorization.AssignmentID,
			authorization.Revision,
		)
	}

	retriedMatch, retriedParticipants, changed, err := domain.ApplyGlobalOperatorReunion(
		nextMatch, nextParticipants, actor, authorization, "resolve-101", 5, "Reunited safely", resolvedAt.Add(time.Hour),
	)
	if err != nil || changed {
		t.Fatalf("exact retry = changed %t, error %v", changed, err)
	}
	if !retriedParticipants.ReunionAudit.ResolvedAt.Equal(resolvedAt) || retriedMatch.Status != domain.MatchStatusReunited {
		t.Fatalf("exact retry replaced original result: %#v / %#v", retriedMatch, retriedParticipants.ReunionAudit)
	}

	roleChange := domain.RoleAssignmentChange{
		Target: actor, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor: domain.PrincipalRef{Issuer: actor.Issuer, Subject: "bootstrap-admin"},
	}
	roleChange.OperationID = "revoke-operator"
	roleChange.OccurredAt = resolvedAt.Add(time.Minute)
	revoked, _, err := domain.RevokeRoleAssignment(&authorization, roleChange)
	if err != nil {
		t.Fatal(err)
	}
	roleChange.OperationID = "regrant-operator"
	roleChange.OccurredAt = resolvedAt.Add(2 * time.Minute)
	regranted, _, err := domain.GrantRoleAssignment(&revoked, roleChange)
	if err != nil {
		t.Fatal(err)
	}
	_, afterRegrant, changed, err := domain.ApplyGlobalOperatorReunion(
		nextMatch, nextParticipants, actor, regranted, "resolve-101", 5, "Reunited safely", resolvedAt.Add(3*time.Minute),
	)
	if err != nil || changed || afterRegrant.ReunionAudit.AssignmentRevision != authorization.Revision {
		t.Fatalf("exact retry after regrant = changed %t, error %v, audit %#v", changed, err, afterRegrant.ReunionAudit)
	}

	otherAssignmentID, err := domain.RoleAssignmentID(
		domain.PrincipalRef{Issuer: actor.Issuer, Subject: "other-operator"},
		domain.RoleOperator,
		domain.RoleScope{Kind: domain.RoleScopeGlobal},
	)
	if err != nil {
		t.Fatal(err)
	}
	for name, corruptAudit := range map[string]domain.MatchReunionAudit{
		"identity-spliced assignment": func() domain.MatchReunionAudit {
			audit := *nextParticipants.ReunionAudit
			audit.AssignmentID = otherAssignmentID
			return audit
		}(),
		"revoked assignment revision": func() domain.MatchReunionAudit {
			audit := *nextParticipants.ReunionAudit
			audit.AssignmentRevision = 2
			return audit
		}(),
	} {
		corrupted := nextParticipants
		corrupted.ReunionAudit = &corruptAudit
		if err := corrupted.Validate(); err == nil {
			t.Fatalf("%s validation error = nil, want fail-closed rejection", name)
		}
	}

	for name, tt := range map[string]struct {
		operationID string
		rating      int
		feedback    string
	}{
		"changed operation": {"resolve-102", 5, "Reunited safely"},
		"changed rating":    {"resolve-101", 4, "Reunited safely"},
		"changed feedback":  {"resolve-101", 5, "Different"},
	} {
		_, _, _, err := domain.ApplyGlobalOperatorReunion(
			nextMatch, nextParticipants, actor, authorization, tt.operationID, tt.rating, tt.feedback, resolvedAt.Add(time.Hour),
		)
		if !errors.Is(err, domain.ErrMatchReunionConflict) {
			t.Fatalf("%s error = %v, want ErrMatchReunionConflict", name, err)
		}
	}
}

func TestApplyGlobalOperatorReunionRejectsInvalidOrNonConfirmedState(t *testing.T) {
	match, participants := confirmedReunionFixture(t)
	actor := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "operator-101",
	}
	resolvedAt := time.Date(2026, time.August, 20, 14, 0, 0, 0, time.UTC)
	authorization := activeGlobalOperatorAssignment(t, actor, resolvedAt.Add(-time.Hour))
	corruptMatch := match
	corruptMatch.SourceEventID = "event-corrupted"
	if _, _, _, err := domain.ApplyGlobalOperatorReunion(
		corruptMatch, participants, actor, authorization, "resolve-corrupt", 5, "Reunited safely", resolvedAt,
	); err == nil {
		t.Fatal("corrupt persisted match error = nil, want fail-closed rejection")
	}

	match.Status = domain.MatchStatusPendingReview
	if _, _, _, err := domain.ApplyGlobalOperatorReunion(
		match, participants, actor, authorization, "resolve-101", 5, "Reunited safely", resolvedAt,
	); !errors.Is(err, domain.ErrMatchReunionConflict) {
		t.Fatalf("pending match error = %v, want ErrMatchReunionConflict", err)
	}

	match.Status = domain.MatchStatusConfirmed
	for name, tt := range map[string]struct {
		operationID string
		rating      int
		feedback    string
	}{
		"missing operation": {"", 5, "Reunited safely"},
		"invalid rating":    {"resolve-101", 0, "Reunited safely"},
		"control feedback":  {"resolve-101", 5, "unsafe\nfeedback"},
	} {
		if _, _, _, err := domain.ApplyGlobalOperatorReunion(
			match, participants, actor, authorization, tt.operationID, tt.rating, tt.feedback, resolvedAt,
		); !errors.Is(err, domain.ErrInvalidMatchReunion) {
			t.Fatalf("%s error = %v, want ErrInvalidMatchReunion", name, err)
		}
	}
}

func activeGlobalOperatorAssignment(t *testing.T, target domain.PrincipalRef, grantedAt time.Time) domain.RoleAssignment {
	t.Helper()
	assignment, _, err := domain.GrantRoleAssignment(nil, domain.RoleAssignmentChange{
		Target: target, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor:       domain.PrincipalRef{Issuer: target.Issuer, Subject: "bootstrap-admin"},
		OperationID: "grant-operator", OccurredAt: grantedAt,
	})
	if err != nil {
		t.Fatalf("create active operator assignment: %v", err)
	}
	return assignment
}

func confirmedReunionFixture(t *testing.T) (domain.MatchRecord, domain.MatchParticipantRecord) {
	t.Helper()
	reporter := &domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101",
	}
	finder := &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "finder-202"}
	decidedAt := time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC)
	matchID, err := domain.StableMatchID("event-found-202", "found-202", "lost-101")
	if err != nil {
		t.Fatal(err)
	}
	return domain.MatchRecord{
			MatchID: matchID, MatchedPetID: "lost-101", FoundPetID: "found-202",
			Score: 0.91, Status: domain.MatchStatusConfirmed, MatchedAt: decidedAt.Add(-time.Hour),
			Scores: domain.MatchScoreBreakdown{
				Visual: 0.95, Color: 1, Spatial: 0.82, DistanceMiles: 2.7, Threshold: 0.7,
			},
			LostPet:       domain.MatchPetDetail{PetID: "lost-101", Breed: "Golden Retriever"},
			FoundPet:      domain.MatchPetDetail{PetID: "found-202", Breed: "Golden Retriever"},
			SourceEventID: "event-found-202", Model: "gemma4:e2b", ThresholdVersion: "visual-spatial-v1",
			Explanation: "High visual and geographic similarity",
		}, domain.MatchParticipantRecord{
			MatchID: matchID, LostPetID: "lost-101", FoundPetID: "found-202",
			Reporter: reporter, Finder: finder,
			ReporterDecision: domain.MatchDecisionConfirm,
			FinderDecision:   domain.MatchDecisionConfirm,
			DecisionAudit: []domain.MatchDecisionAudit{
				{Role: domain.MatchParticipantRoleReporter, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
				{Role: domain.MatchParticipantRoleFinder, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
			},
		}
}
