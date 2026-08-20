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
