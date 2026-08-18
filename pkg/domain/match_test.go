package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMatchRecordUsesStableIdentityAndValidatedProvenance(t *testing.T) {
	first, err := domain.StableMatchID("evt-found-1", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	retry, err := domain.StableMatchID("evt-found-1", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := domain.StableMatchID("evt-found-2", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || retry != first || distinct == first {
		t.Fatalf("stable match IDs = %q, %q, %q", first, retry, distinct)
	}

	record := domain.MatchRecord{
		MatchID:      first,
		FoundPetID:   "found-1",
		MatchedPetID: "lost-1",
		Score:        0.91,
		Status:       domain.MatchStatusPendingReview,
		MatchedAt:    time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC),
		Scores: domain.MatchScoreBreakdown{
			Visual:        0.95,
			Color:         1,
			Spatial:       0.82,
			DistanceMiles: 2.7,
			Threshold:     0.7,
		},
		LostPet:          domain.MatchPetDetail{PetID: "lost-1", Breed: "Golden Retriever"},
		FoundPet:         domain.MatchPetDetail{PetID: "found-1", Breed: "Golden Retriever"},
		SourceEventID:    "evt-found-1",
		Model:            "gemma4:e2b",
		ThresholdVersion: "visual-spatial-v1",
		Explanation:      "High visual and geographic similarity",
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	record.FoundPet.PetID = "found-other"
	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want mismatched snapshot rejection")
	}
	record.FoundPet.PetID = record.FoundPetID
	record.Scores.Threshold = 0
	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing threshold rejection")
	}
}

func TestMatchParticipantRecordValidatesBilateralOwnership(t *testing.T) {
	record := domain.MatchParticipantRecord{
		MatchID:    "match-101",
		LostPetID:  "lost-101",
		FoundPetID: "found-202",
		Reporter: &domain.PrincipalRef{
			Issuer:  "https://securetoken.google.com/petspotr-test",
			Subject: "reporter-101",
		},
		Finder: &domain.PrincipalRef{
			Issuer:  "https://securetoken.google.com/petspotr-test",
			Subject: "finder-202",
		},
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	legacyPartial := record
	legacyPartial.Finder = nil
	if err := legacyPartial.Validate(); err != nil {
		t.Fatalf("Validate() partial legacy ownership error = %v", err)
	}

	invalid := record
	invalid.Reporter = &domain.PrincipalRef{Issuer: record.Reporter.Issuer}
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid reporter rejection")
	}

	missingOwners := record
	missingOwners.Reporter = nil
	missingOwners.Finder = nil
	if err := missingOwners.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want ownerless participant rejection")
	}
}

func TestMatchParticipantRecordAppliesImmutableBilateralDecisions(t *testing.T) {
	reporter := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101"}
	finder := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "finder-202"}
	decidedAt := time.Date(2026, time.August, 17, 12, 30, 0, 0, time.UTC)
	record := domain.MatchParticipantRecord{
		MatchID: "match-101", LostPetID: "lost-101", FoundPetID: "found-202",
		Reporter: &reporter, Finder: &finder,
	}

	afterReporter, status, changed, err := record.ApplyDecision(reporter, domain.MatchDecisionConfirm, decidedAt)
	if err != nil {
		t.Fatalf("reporter confirmation: %v", err)
	}
	if !changed || status != domain.MatchStatusPendingReview ||
		afterReporter.ReporterDecision != domain.MatchDecisionConfirm || afterReporter.FinderDecision != "" {
		t.Fatalf("reporter result = %#v, %q, %t", afterReporter, status, changed)
	}
	if len(afterReporter.DecisionAudit) != 1 || afterReporter.DecisionAudit[0].Role != domain.MatchParticipantRoleReporter ||
		afterReporter.DecisionAudit[0].Decision != domain.MatchDecisionConfirm ||
		!afterReporter.DecisionAudit[0].DecidedAt.Equal(decidedAt) {
		t.Fatalf("reporter audit = %#v", afterReporter.DecisionAudit)
	}

	retry, retryStatus, retryChanged, err := afterReporter.ApplyDecision(
		reporter, domain.MatchDecisionConfirm, decidedAt.Add(time.Hour),
	)
	if err != nil || retryChanged || retryStatus != domain.MatchStatusPendingReview || !reflect.DeepEqual(retry, afterReporter) {
		t.Fatalf("exact retry = %#v, %q, %t, %v", retry, retryStatus, retryChanged, err)
	}
	if _, _, _, err := afterReporter.ApplyDecision(reporter, domain.MatchDecisionReject, decidedAt); !errors.Is(err, domain.ErrMatchDecisionConflict) {
		t.Fatalf("changed reporter decision error = %v, want ErrMatchDecisionConflict", err)
	}

	completed, status, changed, err := afterReporter.ApplyDecision(finder, domain.MatchDecisionConfirm, decidedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("finder confirmation: %v", err)
	}
	if !changed || status != domain.MatchStatusConfirmed || completed.FinderDecision != domain.MatchDecisionConfirm ||
		len(completed.DecisionAudit) != 2 {
		t.Fatalf("completed result = %#v, %q, %t", completed, status, changed)
	}
	if err := completed.Validate(); err != nil {
		t.Fatalf("completed Validate() error = %v", err)
	}
	tamperedAudit := completed
	tamperedAudit.DecisionAudit = append([]domain.MatchDecisionAudit(nil), completed.DecisionAudit...)
	tamperedAudit.DecisionAudit[0].Decision = domain.MatchDecisionReject
	if err := tamperedAudit.Validate(); err == nil {
		t.Fatal("tampered decision audit Validate() error = nil")
	}

	rejected, status, changed, err := record.ApplyDecision(finder, domain.MatchDecisionReject, decidedAt)
	if err != nil || !changed || status != domain.MatchStatusRejected || rejected.FinderDecision != domain.MatchDecisionReject {
		t.Fatalf("rejection result = %#v, %q, %t, %v", rejected, status, changed, err)
	}

	sharedOwner := record
	sharedOwner.Finder = &reporter
	shared, status, changed, err := sharedOwner.ApplyDecision(reporter, domain.MatchDecisionConfirm, decidedAt)
	if err != nil || !changed || status != domain.MatchStatusConfirmed ||
		shared.ReporterDecision != domain.MatchDecisionConfirm || shared.FinderDecision != domain.MatchDecisionConfirm ||
		len(shared.DecisionAudit) != 2 {
		t.Fatalf("shared-owner result = %#v, %q, %t, %v", shared, status, changed, err)
	}

	stranger := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "stranger-303"}
	if _, _, _, err := record.ApplyDecision(stranger, domain.MatchDecisionConfirm, decidedAt); !errors.Is(err, domain.ErrNotMatchParticipant) {
		t.Fatalf("stranger decision error = %v, want ErrNotMatchParticipant", err)
	}
	incomplete := record
	incomplete.Finder = nil
	if _, _, _, err := incomplete.ApplyDecision(reporter, domain.MatchDecisionConfirm, decidedAt); !errors.Is(err, domain.ErrIncompleteMatchParticipants) {
		t.Fatalf("incomplete decision error = %v, want ErrIncompleteMatchParticipants", err)
	}
}
