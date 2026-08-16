package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetPersistenceSeparatesOwnerContact(t *testing.T) {
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         " lost-101 ",
		PetName:       " Buddy ",
		ReporterEmail: " OWNER@EXAMPLE.COM ",
		Phone:         " (555) 019-2834 ",
		ReportedAt:    time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC),
		Location:      " Seattle, WA ",
	})

	record, contact := report.Persisted()
	if record.OwnerIdentityRef == "" || contact.IdentityRef != record.OwnerIdentityRef {
		t.Fatalf("record/contact identity refs = %q/%q", record.OwnerIdentityRef, contact.IdentityRef)
	}
	if contact.Email != "owner@example.com" || contact.Phone != "(555) 019-2834" {
		t.Fatalf("private contact = %#v", contact)
	}
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recordData), "owner@example.com") || strings.Contains(string(recordData), "reporterEmail") ||
		strings.Contains(string(recordData), "(555) 019-2834") || strings.Contains(string(recordData), "phone") {
		t.Fatalf("persisted report exposed private contact: %s", recordData)
	}
	publicData, err := json.Marshal(record.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "ownerIdentityRef") || strings.Contains(string(publicData), record.OwnerIdentityRef) {
		t.Fatalf("public report exposed private identity reference: %s", publicData)
	}
}

func TestFoundPetPersistenceSeparatesFinderContact(t *testing.T) {
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:       " found-101 ",
		ImageURL:    "https://storage.petspotr.io/found-101.jpg",
		FoundAt:     time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC),
		Location:    " Seattle, WA ",
		FinderEmail: " FINDER@EXAMPLE.COM ",
	})

	record, contact := report.Persisted()
	if record.FinderIdentityRef == "" || contact.IdentityRef != record.FinderIdentityRef {
		t.Fatalf("record/contact identity refs = %q/%q", record.FinderIdentityRef, contact.IdentityRef)
	}
	if contact.Email != "finder@example.com" || contact.Phone != "" {
		t.Fatalf("private contact = %#v", contact)
	}
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recordData), "finder@example.com") || strings.Contains(string(recordData), "finderEmail") {
		t.Fatalf("persisted report exposed private contact: %s", recordData)
	}
	publicData, err := json.Marshal(record.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "finderIdentityRef") || strings.Contains(string(publicData), record.FinderIdentityRef) {
		t.Fatalf("public report exposed private identity reference: %s", publicData)
	}
}
