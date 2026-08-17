package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetReportPersistsOwnershipWithoutPublishingIt(t *testing.T) {
	t.Parallel()

	owner := &domain.PrincipalRef{
		Issuer:  " https://securetoken.google.com/petspotr-test ",
		Subject: " owner-101 ",
	}
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-owned", ReporterEmail: "owner@example.com",
		ReportedAt: time.Date(2026, time.August, 17, 22, 0, 0, 0, time.UTC),
		Location:   "Seattle, WA", OwnedBy: owner,
	})
	owner.Subject = "mutated-after-normalization"
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	record, _ := report.Persisted()
	if record.OwnedBy == nil || record.OwnedBy.Issuer != "https://securetoken.google.com/petspotr-test" ||
		record.OwnedBy.Subject != " owner-101 " {
		t.Fatalf("persisted owner = %#v", record.OwnedBy)
	}

	assertOwnershipPrivate(t, report.Public(), report.ReportedEvent(), record)
}

func TestFoundPetReportPersistsOwnershipWithoutPublishingIt(t *testing.T) {
	t.Parallel()

	finder := &domain.PrincipalRef{
		Issuer: " https://securetoken.google.com/petspotr-test ", Subject: "finder-202",
	}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-owned", ImageURL: "https://images.invalid/found-owned.jpg",
		FoundAt:  time.Date(2026, time.August, 17, 22, 0, 0, 0, time.UTC),
		Location: "Portland, OR", OwnedBy: finder,
	})
	finder.Subject = "mutated-after-normalization"
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	record, _ := report.Persisted()
	if record.OwnedBy == nil || record.OwnedBy.Issuer != "https://securetoken.google.com/petspotr-test" ||
		record.OwnedBy.Subject != "finder-202" {
		t.Fatalf("persisted owner = %#v", record.OwnedBy)
	}

	assertOwnershipPrivate(t, report.Public(), report.ReportedEvent(), record)
}

func TestReportOwnershipValidationAndLegacyCompatibility(t *testing.T) {
	t.Parallel()

	invalid := []*domain.PrincipalRef{
		{Issuer: "https://securetoken.google.com/petspotr-test"},
		{Subject: "user-101"},
		{Issuer: strings.Repeat("i", 513), Subject: "user-101"},
		{Issuer: "issuer", Subject: strings.Repeat("s", 513)},
		{Issuer: string([]byte{0xff}), Subject: "user-101"},
		{Issuer: "issuer", Subject: string([]byte{0xff})},
		{Issuer: "issuer", Subject: string([]byte{0xfe})},
	}
	for _, owner := range invalid {
		lost := domain.NormalizeLostPetReport(domain.LostPetReport{
			PetID: "lost-invalid-owner", ReporterEmail: "owner@example.com",
			ReportedAt: time.Now().UTC(), Location: "Seattle, WA", OwnedBy: owner,
		})
		if err := lost.Validate(); err == nil {
			t.Fatalf("LostPetReport.Validate() accepted owner %#v", owner)
		}
		found := domain.NormalizeFoundPetReport(domain.FoundPetReport{
			PetID: "found-invalid-owner", ImageURL: "https://images.invalid/found.jpg",
			FoundAt: time.Now().UTC(), Location: "Seattle, WA", OwnedBy: owner,
		})
		if err := found.Validate(); err == nil {
			t.Fatalf("FoundPetReport.Validate() accepted owner %#v", owner)
		}
	}

	var lost domain.LostPetRecord
	if err := json.Unmarshal([]byte(`{"petId":"lost-legacy","ownerIdentityRef":"lost/lost-legacy/owner"}`), &lost); err != nil {
		t.Fatal(err)
	}
	if normalized := domain.NormalizeLostPetRecord(lost); normalized.OwnedBy != nil {
		t.Fatalf("legacy lost owner = %#v, want nil", normalized.OwnedBy)
	}
	var found domain.FoundPetRecord
	if err := json.Unmarshal([]byte(`{"petId":"found-legacy","finderIdentityRef":"found/found-legacy/finder"}`), &found); err != nil {
		t.Fatal(err)
	}
	if normalized := domain.NormalizeFoundPetRecord(found); normalized.OwnedBy != nil {
		t.Fatalf("legacy found owner = %#v, want nil", normalized.OwnedBy)
	}
}

func assertOwnershipPrivate(t *testing.T, public, event, record any) {
	t.Helper()
	for name, value := range map[string]any{"public DTO": public, "integration event": event} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "ownedBy") || strings.Contains(string(data), "securetoken.google.com") {
			t.Fatalf("%s exposed ownership: %s", name, data)
		}
	}
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(recordData), `"ownedBy"`) || !strings.Contains(string(recordData), `"subject"`) {
		t.Fatalf("private record omitted ownership: %s", recordData)
	}
}
