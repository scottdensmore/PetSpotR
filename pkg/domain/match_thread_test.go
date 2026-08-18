package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMatchParticipantRecordAppendsIdempotentMediatedMessages(t *testing.T) {
	reporter := domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101"}
	finder := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "finder-202"}
	sentAt := time.Date(2026, time.August, 18, 3, 0, 0, 0, time.UTC)
	record := domain.MatchParticipantRecord{
		MatchID: "match-101", LostPetID: "lost-101", FoundPetID: "found-202",
		Reporter: &reporter, Finder: &finder,
	}

	withMessage, message, created, err := record.AppendMediatedMessage(
		reporter, "request-101", "  Can we compare identifying marks?  ", sentAt,
	)
	if err != nil {
		t.Fatalf("AppendMediatedMessage() error = %v", err)
	}
	if !created || message.MessageID == "" || message.SenderRole != domain.MatchParticipantRoleReporter ||
		message.Message != "Can we compare identifying marks?" || !message.SentAt.Equal(sentAt) {
		t.Fatalf("created message = %#v, %t", message, created)
	}
	if len(withMessage.Messages) != 1 || !reflect.DeepEqual(withMessage.Messages[0], message) {
		t.Fatalf("stored messages = %#v", withMessage.Messages)
	}

	retry, retryMessage, retryCreated, err := withMessage.AppendMediatedMessage(
		reporter, "request-101", "Can we compare identifying marks?", sentAt.Add(time.Hour),
	)
	if err != nil || retryCreated || !reflect.DeepEqual(retry, withMessage) || !reflect.DeepEqual(retryMessage, message) {
		t.Fatalf("exact retry = %#v / %#v / %t / %v", retry, retryMessage, retryCreated, err)
	}
	if _, _, _, err := withMessage.AppendMediatedMessage(reporter, "request-101", "Different body", sentAt); !errors.Is(err, domain.ErrMatchMessageConflict) {
		t.Fatalf("changed retry error = %v, want ErrMatchMessageConflict", err)
	}

	fromFinder, finderMessage, created, err := withMessage.AppendMediatedMessage(
		finder, "request-202", "Yes, there is a white spot on the left paw.", sentAt.Add(time.Minute),
	)
	if err != nil || !created || finderMessage.SenderRole != domain.MatchParticipantRoleFinder ||
		len(fromFinder.Messages) != 2 {
		t.Fatalf("finder message = %#v / %#v / %t / %v", fromFinder, finderMessage, created, err)
	}
	if err := fromFinder.Validate(); err != nil {
		t.Fatalf("thread Validate() error = %v", err)
	}

	stranger := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "stranger-303"}
	if _, _, _, err := record.AppendMediatedMessage(stranger, "request-303", "hello", sentAt); !errors.Is(err, domain.ErrNotMatchParticipant) {
		t.Fatalf("stranger error = %v, want ErrNotMatchParticipant", err)
	}
	incomplete := record
	incomplete.Finder = nil
	if _, _, _, err := incomplete.AppendMediatedMessage(reporter, "request-404", "hello", sentAt); !errors.Is(err, domain.ErrIncompleteMatchParticipants) {
		t.Fatalf("incomplete error = %v, want ErrIncompleteMatchParticipants", err)
	}
	sharedOwner := record
	sharedOwner.Finder = &reporter
	if _, _, _, err := sharedOwner.AppendMediatedMessage(reporter, "request-505", "hello", sentAt); !errors.Is(err, domain.ErrNoMediatedMessageRecipient) {
		t.Fatalf("shared-owner error = %v, want ErrNoMediatedMessageRecipient", err)
	}
	if _, _, _, err := record.AppendMediatedMessage(reporter, "", "hello", sentAt); err == nil {
		t.Fatal("empty idempotency key error = nil")
	}
	if _, _, _, err := record.AppendMediatedMessage(
		reporter, "request-606", strings.Repeat("x", domain.MaxMediatedMessageRunes+1), sentAt,
	); err == nil {
		t.Fatal("oversized message error = nil")
	}
	if _, _, _, err := record.AppendMediatedMessage(reporter, "request-607", "hidden\x00control", sentAt); err == nil {
		t.Fatal("control-character message error = nil")
	}
	full := record
	for i := 0; i < domain.MaxMediatedMatchMessages; i++ {
		var err error
		full, _, _, err = full.AppendMediatedMessage(
			reporter, "capacity-"+strings.Repeat("x", i), "message", sentAt.Add(time.Duration(i)*time.Second),
		)
		if err != nil {
			t.Fatalf("fill message %d: %v", i, err)
		}
	}
	if _, _, _, err := full.AppendMediatedMessage(reporter, "over-capacity", "message", sentAt); !errors.Is(err, domain.ErrMatchThreadFull) {
		t.Fatalf("full thread error = %v, want ErrMatchThreadFull", err)
	}
}
