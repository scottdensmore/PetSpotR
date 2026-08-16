package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ReportContact is private contact state linked from a report aggregate by a
// stable, report-scoped identity reference. Public DTOs and integration events
// deliberately cannot serialize this type by construction.
type ReportContact struct {
	IdentityRef string `json:"identityRef"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

// NormalizeReportContact canonicalizes private contact before validation and
// persistence.
func NormalizeReportContact(contact ReportContact) ReportContact {
	contact.IdentityRef = strings.TrimSpace(contact.IdentityRef)
	contact.Email = strings.ToLower(strings.TrimSpace(contact.Email))
	contact.Phone = strings.TrimSpace(contact.Phone)
	return contact
}

// Validate checks private contact independently of public report state.
func (c ReportContact) Validate() error {
	if strings.TrimSpace(c.IdentityRef) == "" {
		return errors.New("domain: contact identityRef is required")
	}
	if utf8.RuneCountInString(c.IdentityRef) > 512 {
		return errors.New("domain: contact identityRef exceeds 512 characters")
	}
	if utf8.RuneCountInString(c.Email) > 320 {
		return errors.New("domain: contact email exceeds 320 characters")
	}
	if c.Email != "" && (!strings.Contains(c.Email, "@") || !strings.Contains(c.Email, ".")) {
		return fmt.Errorf("domain: invalid contact email address: %s", c.Email)
	}
	if utf8.RuneCountInString(c.Phone) > 64 {
		return errors.New("domain: contact phone exceeds 64 characters")
	}
	return nil
}

func reportIdentityRef(kind, petID, role string) string {
	return kind + "/" + strings.TrimSpace(petID) + "/" + role
}
