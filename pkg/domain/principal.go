package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// PrincipalRef is the provider-neutral, immutable identity key that owns a
// private resource. Subject is opaque provider data and is never normalized.
type PrincipalRef struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// Validate requires a complete bounded ownership key.
func (p PrincipalRef) Validate() error {
	if !utf8.ValidString(p.Issuer) {
		return errors.New("domain: principal issuer must be valid UTF-8")
	}
	if !utf8.ValidString(p.Subject) {
		return errors.New("domain: principal subject must be valid UTF-8")
	}
	if strings.TrimSpace(p.Issuer) == "" {
		return errors.New("domain: principal issuer is required")
	}
	if p.Subject == "" {
		return errors.New("domain: principal subject is required")
	}
	if utf8.RuneCountInString(p.Issuer) > 512 {
		return errors.New("domain: principal issuer exceeds 512 characters")
	}
	if utf8.RuneCountInString(p.Subject) > 512 {
		return errors.New("domain: principal subject exceeds 512 characters")
	}
	return nil
}

func normalizePrincipalRef(principal *PrincipalRef) *PrincipalRef {
	if principal == nil {
		return nil
	}
	return &PrincipalRef{
		Issuer:  strings.TrimSpace(principal.Issuer),
		Subject: principal.Subject,
	}
}
