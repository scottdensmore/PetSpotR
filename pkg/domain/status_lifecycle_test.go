package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestReportCreationRequiresActiveStatus(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	for _, status := range []domain.LostPetStatus{
		domain.LostPetStatusReunited,
		domain.LostPetStatusExpired,
		domain.LostPetStatusClosed,
	} {
		report := domain.NormalizeLostPetReport(domain.LostPetReport{
			PetID: "lost-terminal", ReporterEmail: "owner@example.com",
			ReportedAt: now, Location: "Seattle, WA", Status: status,
		})
		if err := report.Validate(); err == nil {
			t.Errorf("new lost report accepted terminal status %s", status)
		}
	}
	for _, status := range []domain.FoundPetStatus{
		domain.FoundPetStatusResolved,
		domain.FoundPetStatusExpired,
	} {
		report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
			PetID: "found-terminal", ImageURL: "https://images.invalid/found.jpg",
			FoundAt: now, Location: "Seattle, WA", Status: status,
		})
		if err := report.Validate(); err == nil {
			t.Errorf("new found report accepted terminal status %s", status)
		}
	}
}

func TestLostPetStatusLifecycle(t *testing.T) {
	terminal := []domain.LostPetStatus{
		domain.LostPetStatusReunited,
		domain.LostPetStatusExpired,
		domain.LostPetStatusClosed,
	}
	if !domain.LostPetStatusLost.IsActive() || domain.LostPetStatusLost.IsTerminal() {
		t.Fatalf("lost status active/terminal = %t/%t, want true/false",
			domain.LostPetStatusLost.IsActive(), domain.LostPetStatusLost.IsTerminal())
	}
	for _, status := range terminal {
		if status.IsActive() || !status.IsTerminal() {
			t.Errorf("%s active/terminal = %t/%t, want false/true", status, status.IsActive(), status.IsTerminal())
		}
		if !domain.LostPetStatusLost.CanTransitionTo(status) {
			t.Errorf("lost -> %s transition rejected", status)
		}
		if status.CanTransitionTo(domain.LostPetStatusLost) {
			t.Errorf("terminal %s -> lost transition accepted", status)
		}
		for _, next := range terminal {
			if status.CanTransitionTo(next) {
				t.Errorf("terminal %s -> %s transition accepted", status, next)
			}
		}
	}
	for _, status := range []domain.LostPetStatus{"", "unknown", domain.LostPetStatusLost} {
		if domain.LostPetStatusLost.CanTransitionTo(status) {
			t.Errorf("lost -> %q transition accepted", status)
		}
	}
}

func TestFoundPetStatusLifecycle(t *testing.T) {
	terminal := []domain.FoundPetStatus{
		domain.FoundPetStatusResolved,
		domain.FoundPetStatusExpired,
	}
	if !domain.FoundPetStatusFound.IsActive() || domain.FoundPetStatusFound.IsTerminal() {
		t.Fatalf("found status active/terminal = %t/%t, want true/false",
			domain.FoundPetStatusFound.IsActive(), domain.FoundPetStatusFound.IsTerminal())
	}
	for _, status := range terminal {
		if status.IsActive() || !status.IsTerminal() {
			t.Errorf("%s active/terminal = %t/%t, want false/true", status, status.IsActive(), status.IsTerminal())
		}
		if !domain.FoundPetStatusFound.CanTransitionTo(status) {
			t.Errorf("found -> %s transition rejected", status)
		}
		if status.CanTransitionTo(domain.FoundPetStatusFound) {
			t.Errorf("terminal %s -> found transition accepted", status)
		}
		for _, next := range terminal {
			if status.CanTransitionTo(next) {
				t.Errorf("terminal %s -> %s transition accepted", status, next)
			}
		}
	}
	for _, status := range []domain.FoundPetStatus{"", "unknown", domain.FoundPetStatusFound} {
		if domain.FoundPetStatusFound.CanTransitionTo(status) {
			t.Errorf("found -> %q transition accepted", status)
		}
	}
}
