package domain_test

import (
	"encoding/json"
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

func TestAppendMediatedMessageWithImages(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	reporter := domain.PrincipalRef{Issuer: "https://accounts.google.com", Subject: "owner-1"}
	finder := domain.PrincipalRef{Issuer: "https://accounts.google.com", Subject: "finder-1"}

	record := domain.MatchParticipantRecord{
		MatchID:    "match-123",
		LostPetID:  "lost-456",
		FoundPetID: "found-789",
		Reporter:   &reporter,
		Finder:     &finder,
	}

	t.Run("appends valid images", func(t *testing.T) {
		images := []string{"images/reunions/match-123/collar.jpg"}
		next, msg, created, err := record.AppendMediatedMessageWithImages(
			reporter, "idem-1", "Here is the collar photo", images, now,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatal("expected message to be created")
		}
		if len(msg.Images) != 1 || msg.Images[0] != images[0] {
			t.Fatalf("expected 1 image %q, got %v", images[0], msg.Images)
		}
		if len(next.Messages) != 1 {
			t.Fatalf("expected 1 message in participant record, got %d", len(next.Messages))
		}
	})

	t.Run("rejects more than 3 images", func(t *testing.T) {
		images := []string{"img1.jpg", "img2.jpg", "img3.jpg", "img4.jpg"}
		_, _, _, err := record.AppendMediatedMessageWithImages(
			reporter, "idem-2", "Too many photos", images, now,
		)
		if !errors.Is(err, domain.ErrInvalidMediatedMessage) {
			t.Fatalf("expected ErrInvalidMediatedMessage, got %v", err)
		}
	})

	t.Run("rejects invalid image paths", func(t *testing.T) {
		invalidCases := [][]string{
			{""},
			{"   "},
			{" leading/space.jpg"},
			{"trailing/space.jpg "},
			{strings.Repeat("a", 1025)},
		}
		for _, invalid := range invalidCases {
			_, _, _, err := record.AppendMediatedMessageWithImages(
				reporter, "idem-invalid", "photo", invalid, now,
			)
			if !errors.Is(err, domain.ErrInvalidMediatedMessage) {
				t.Fatalf("expected ErrInvalidMediatedMessage for %v, got %v", invalid, err)
			}
		}
	})

	t.Run("idempotent retries with images", func(t *testing.T) {
		images := []string{"images/reunions/match-123/collar.jpg"}
		rec, msg, created, err := record.AppendMediatedMessageWithImages(
			reporter, "idem-retry", "photo", images, now,
		)
		if err != nil || !created {
			t.Fatalf("first call failed: %v, created=%v", err, created)
		}

		// Exact retry
		retryRec, retryMsg, retryCreated, err := rec.AppendMediatedMessageWithImages(
			reporter, "idem-retry", "photo", images, now.Add(time.Minute),
		)
		if err != nil {
			t.Fatalf("exact retry error: %v", err)
		}
		if retryCreated {
			t.Fatal("expected retry to not create a new message")
		}
		if !reflect.DeepEqual(msg, retryMsg) {
			t.Fatalf("expected %v, got %v", msg, retryMsg)
		}
		if len(retryRec.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(retryRec.Messages))
		}

		// Changed retry (different image)
		_, _, _, err = rec.AppendMediatedMessageWithImages(
			reporter, "idem-retry", "photo", []string{"images/reunions/match-123/other.jpg"}, now,
		)
		if !errors.Is(err, domain.ErrMatchMessageConflict) {
			t.Fatalf("expected ErrMatchMessageConflict on different images, got %v", err)
		}

		// Changed retry (no images)
		_, _, _, err = rec.AppendMediatedMessageWithImages(
			reporter, "idem-retry", "photo", nil, now,
		)
		if !errors.Is(err, domain.ErrMatchMessageConflict) {
			t.Fatalf("expected ErrMatchMessageConflict on removed images, got %v", err)
		}
	})

	t.Run("delegation from AppendMediatedMessage", func(t *testing.T) {
		rec, msg, created, err := record.AppendMediatedMessage(
			reporter, "idem-delegate", "text only", now,
		)
		if err != nil || !created {
			t.Fatalf("AppendMediatedMessage failed: %v, created=%v", err, created)
		}
		if len(msg.Images) != 0 {
			t.Fatalf("expected nil/empty images, got %v", msg.Images)
		}
		if err := rec.Validate(); err != nil {
			t.Fatalf("record validation failed: %v", err)
		}
	})
}

func TestReunionStreamEvent_JSON(t *testing.T) {
	t.Parallel()
	timestamp := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	t.Run("marshals message created event", func(t *testing.T) {
		msg := domain.MediatedMatchMessage{
			MessageID:  "msg-123",
			SenderRole: domain.MatchParticipantRoleReporter,
			Message:    "Found collar",
			Images:     []string{"images/reunions/match-1/collar.jpg"},
			SentAt:     timestamp,
		}
		evt := domain.ReunionStreamEvent{
			EventID:   "evt-101",
			Type:      domain.ReunionEventMessageCreated,
			MatchID:   "match-1",
			Timestamp: timestamp,
			Payload:   msg,
		}

		data, err := json.Marshal(evt)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		raw := string(data)
		if !strings.Contains(raw, `"eventId":"evt-101"`) ||
			!strings.Contains(raw, `"type":"message"`) ||
			!strings.Contains(raw, `"matchId":"match-1"`) ||
			!strings.Contains(raw, `"images":["images/reunions/match-1/collar.jpg"]`) {
			t.Fatalf("unexpected JSON: %s", raw)
		}
	})

	t.Run("marshals presence event", func(t *testing.T) {
		presence := domain.ReunionPresencePayload{
			SenderRole: domain.MatchParticipantRoleFinder,
			Status:     "typing",
		}
		evt := domain.ReunionStreamEvent{
			EventID:   "evt-102",
			Type:      domain.ReunionEventPresence,
			MatchID:   "match-1",
			Timestamp: timestamp,
			Payload:   presence,
		}

		data, err := json.Marshal(evt)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		raw := string(data)
		if !strings.Contains(raw, `"type":"presence"`) ||
			!strings.Contains(raw, `"senderRole":"finder"`) ||
			!strings.Contains(raw, `"status":"typing"`) {
			t.Fatalf("unexpected JSON: %s", raw)
		}
	})

	t.Run("omits empty images in mediated message JSON", func(t *testing.T) {
		msg := domain.MediatedMatchMessage{
			MessageID:  "msg-no-img",
			SenderRole: domain.MatchParticipantRoleReporter,
			Message:    "Text only",
			SentAt:     timestamp,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"images"`) {
			t.Fatalf("expected images to be omitted, got: %s", string(data))
		}
	})
}



