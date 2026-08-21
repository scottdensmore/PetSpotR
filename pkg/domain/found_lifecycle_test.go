package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestApplyFinderFoundPetResolutionIsAuditedRedactedAndIdempotent(t *testing.T) {
	finder := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "finder-lifecycle",
	}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-lifecycle", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), Location: "Seattle, WA",
		FinderEmail: "finder@example.com", OwnedBy: &finder,
	})
	record, _ := report.Persisted()
	changedAt := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)

	result, err := domain.ApplyFinderFoundPetResolution(record, finder, "resolve-found-lifecycle", changedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Record.Status != domain.FoundPetStatusResolved || result.EventID == "" ||
		result.Record.LifecycleAudit == nil || result.Record.LifecycleAudit.EventID != result.EventID {
		t.Fatalf("lifecycle result = %#v", result)
	}
	if result.Event.ReportType != "found" || result.Event.PreviousStatus != domain.FoundPetStatusFound ||
		result.Event.Status != domain.FoundPetStatusResolved {
		t.Fatalf("status event = %#v", result.Event)
	}
	eventData, err := json.Marshal(result.Event)
	if err != nil {
		t.Fatal(err)
	}
	auditData, err := json.Marshal(result.Record.LifecycleAudit)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{finder.Issuer, finder.Subject, "finder@example.com"} {
		if strings.Contains(string(eventData), private) || strings.Contains(string(auditData), private) {
			t.Fatalf("lifecycle data exposed private identity %q: %s / %s", private, eventData, auditData)
		}
	}

	retry, err := domain.ApplyFinderFoundPetResolution(
		result.Record, finder, "resolve-found-lifecycle", changedAt.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Changed || retry.EventID != result.EventID || !retry.Event.ChangedAt.Equal(changedAt) {
		t.Fatalf("exact retry = %#v", retry)
	}
	if _, err := domain.ApplyFinderFoundPetResolution(
		result.Record, finder, "different-operation", changedAt,
	); !errors.Is(err, domain.ErrFoundPetLifecycleConflict) {
		t.Fatalf("changed operation error = %v", err)
	}
	stranger := domain.PrincipalRef{Issuer: finder.Issuer, Subject: "stranger"}
	if _, err := domain.ApplyFinderFoundPetResolution(
		record, stranger, "resolve-found-lifecycle", changedAt,
	); !errors.Is(err, domain.ErrFoundPetNotOwned) {
		t.Fatalf("wrong finder error = %v", err)
	}

	ownerless := record
	ownerless.OwnedBy = nil
	if _, err := domain.ApplyFinderFoundPetResolution(
		ownerless, finder, "resolve-ownerless", changedAt,
	); !errors.Is(err, domain.ErrFoundPetNotOwned) {
		t.Fatalf("ownerless record error = %v", err)
	}
}

func TestFoundPetResolutionRejectsCorruptLifecycleState(t *testing.T) {
	finder := domain.PrincipalRef{Issuer: "issuer", Subject: "finder-corrupt"}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-corrupt", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), Location: "Seattle, WA", OwnedBy: &finder,
	})
	record, _ := report.Persisted()
	unauditedResolved := record
	unauditedResolved.Status = domain.FoundPetStatusResolved
	if _, err := domain.ApplyFinderFoundPetResolution(
		unauditedResolved, finder, "resolve-unaudited", time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	); !errors.Is(err, domain.ErrInvalidFoundPetLifecycle) {
		t.Fatalf("unaudited resolved state error = %v", err)
	}

	spliced := record
	spliced.FinderIdentityRef = "contact_spliced"
	if _, err := domain.ApplyFinderFoundPetResolution(
		spliced, finder, "resolve-corrupt", time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	); !errors.Is(err, domain.ErrInvalidFoundPetLifecycle) {
		t.Fatalf("spliced finder reference error = %v", err)
	}

	result, err := domain.ApplyFinderFoundPetResolution(
		record, finder, "resolve-corrupt", time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	result.Record.LifecycleAudit.ActorKey = "role_principal_v1_" + strings.Repeat("0", 64)
	if _, err := domain.ApplyFinderFoundPetResolution(
		result.Record, finder, "resolve-corrupt", time.Now().UTC(),
	); !errors.Is(err, domain.ErrInvalidFoundPetLifecycle) {
		t.Fatalf("corrupt audit error = %v", err)
	}
}

func TestPetStatusChangedDecoderAcceptsLostV1AndFoundV2(t *testing.T) {
	lostEvent := domain.PetStatusChangedV1{
		PetID: "lost-v1", ReportType: "lost", PreviousStatus: domain.LostPetStatusLost,
		Status: domain.LostPetStatusReunited, ChangedAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC),
	}
	lostEnvelope, err := domain.NewPetStatusChangedEnvelope(lostEvent)
	if err != nil {
		t.Fatal(err)
	}
	lostData, err := json.Marshal(lostEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	decodedLost, decodedLostEnvelope, err := domain.DecodePetStatusChanged(lostData)
	if err != nil {
		t.Fatal(err)
	}
	if decodedLostEnvelope.PayloadVersion != domain.PetStatusChangedPayloadVersion ||
		decodedLost.ReportType != "lost" || decodedLost.PreviousStatus != "lost" || decodedLost.Status != "reunited" {
		t.Fatalf("decoded lost v1 = %#v / %#v", decodedLost, decodedLostEnvelope)
	}

	foundEvent := domain.PetStatusChangedV2{
		PetID: "found-v2", ReportType: "found", PreviousStatus: domain.FoundPetStatusFound,
		Status: domain.FoundPetStatusResolved, ChangedAt: time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	}
	foundEnvelope, err := domain.NewFoundPetStatusChangedEnvelope(foundEvent)
	if err != nil {
		t.Fatal(err)
	}
	if foundEnvelope.PayloadVersion != domain.PetStatusChangedFoundPayloadVersion {
		t.Fatalf("found payload version = %d", foundEnvelope.PayloadVersion)
	}
	foundData, err := json.Marshal(foundEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	decodedFound, decodedFoundEnvelope, err := domain.DecodePetStatusChanged(foundData)
	if err != nil {
		t.Fatal(err)
	}
	if decodedFoundEnvelope.PayloadVersion != domain.PetStatusChangedFoundPayloadVersion ||
		decodedFound.ReportType != "found" || decodedFound.PreviousStatus != "found" || decodedFound.Status != "resolved" {
		t.Fatalf("decoded found v2 = %#v / %#v", decodedFound, decodedFoundEnvelope)
	}
}

func TestPetStatusChangedDecoderRejectsUnknownAndMismatchedEvents(t *testing.T) {
	event := domain.PetStatusChangedV2{
		PetID: "found-invalid", ReportType: "found", PreviousStatus: domain.FoundPetStatusFound,
		Status: domain.FoundPetStatusResolved, ChangedAt: time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name             string
		aggregateID      string
		payloadVersion   int
		aggregateVersion int64
	}{
		{name: "unknown payload", aggregateID: event.PetID, payloadVersion: 3, aggregateVersion: 2},
		{name: "mismatched aggregate", aggregateID: "different", payloadVersion: 2, aggregateVersion: 2},
		{name: "wrong aggregate version", aggregateID: event.PetID, payloadVersion: 2, aggregateVersion: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
				Type: domain.EventTypePetStatusChanged, OccurredAt: event.ChangedAt,
				AggregateID: tt.aggregateID, AggregateVersion: tt.aggregateVersion,
				PayloadVersion: tt.payloadVersion, Payload: payload,
			})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := domain.DecodePetStatusChanged(data); err == nil {
				t.Fatal("DecodePetStatusChanged() error = nil")
			}
		})
	}
}
