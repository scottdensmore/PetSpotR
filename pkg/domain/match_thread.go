package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxMediatedMatchMessages = 100
	MaxMediatedMessageRunes  = 1000
	maxIdempotencyKeyRunes   = 128
)

var (
	ErrMatchMessageConflict       = errors.New("domain: idempotency key conflicts with accepted match message")
	ErrMatchThreadFull            = errors.New("domain: mediated match thread is full")
	ErrNoMediatedMessageRecipient = errors.New("domain: match has no distinct mediated-message recipient")
	ErrInvalidMediatedMessage     = errors.New("domain: invalid mediated match message input")
)

// MediatedMatchMessage is a private, role-labeled message between match
// participants. It contains no provider subject or report contact details.
type MediatedMatchMessage struct {
	MessageID  string               `json:"messageId"`
	SenderRole MatchParticipantRole `json:"senderRole"`
	Message    string               `json:"message"`
	SentAt     time.Time            `json:"sentAt"`
}

// AppendMediatedMessage returns a copied participant record with one bounded
// private message appended. The caller-supplied idempotency key makes exact
// retries no-ops and changed retries conflicts.
func (r MatchParticipantRecord) AppendMediatedMessage(
	actor PrincipalRef,
	idempotencyKey string,
	messageBody string,
	sentAt time.Time,
) (MatchParticipantRecord, MediatedMatchMessage, bool, error) {
	if err := r.Validate(); err != nil {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, err
	}
	if r.Reporter == nil || r.Finder == nil {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, ErrIncompleteMatchParticipants
	}
	if err := actor.Validate(); err != nil {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, fmt.Errorf("domain: invalid mediated-message actor: %w", err)
	}
	reporter := principalRefsEqual(r.Reporter, actor)
	finder := principalRefsEqual(r.Finder, actor)
	if !reporter && !finder {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, ErrNotMatchParticipant
	}
	if reporter && finder {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, ErrNoMediatedMessageRecipient
	}

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	messageBody = strings.TrimSpace(messageBody)
	if !utf8.ValidString(idempotencyKey) || idempotencyKey == "" ||
		utf8.RuneCountInString(idempotencyKey) > maxIdempotencyKeyRunes {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false,
			fmt.Errorf("%w: a bounded valid UTF-8 idempotency key is required", ErrInvalidMediatedMessage)
	}
	if !validMediatedMessageText(messageBody) || sentAt.IsZero() {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false,
			fmt.Errorf("%w: a bounded valid UTF-8 message and sentAt are required", ErrInvalidMediatedMessage)
	}

	role := MatchParticipantRoleReporter
	if finder {
		role = MatchParticipantRoleFinder
	}
	messageID := stableMediatedMessageID(r.MatchID, actor, idempotencyKey)
	for _, existing := range r.Messages {
		if existing.MessageID != messageID {
			continue
		}
		if existing.SenderRole == role && existing.Message == messageBody {
			return r, existing, false, nil
		}
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, ErrMatchMessageConflict
	}
	if len(r.Messages) >= MaxMediatedMatchMessages {
		return MatchParticipantRecord{}, MediatedMatchMessage{}, false, ErrMatchThreadFull
	}

	message := MediatedMatchMessage{
		MessageID: messageID, SenderRole: role, Message: messageBody, SentAt: sentAt,
	}
	next := r
	next.Messages = append(append([]MediatedMatchMessage(nil), r.Messages...), message)
	return next, message, true, nil
}

func validateMediatedMatchMessages(
	reporter *PrincipalRef,
	finder *PrincipalRef,
	messages []MediatedMatchMessage,
) error {
	if len(messages) > MaxMediatedMatchMessages {
		return ErrMatchThreadFull
	}
	seen := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		if !utf8.ValidString(message.MessageID) || strings.TrimSpace(message.MessageID) == "" || len(message.MessageID) > 80 {
			return errors.New("domain: invalid mediated match message ID")
		}
		if _, exists := seen[message.MessageID]; exists {
			return errors.New("domain: duplicate mediated match message ID")
		}
		seen[message.MessageID] = struct{}{}
		if message.SenderRole != MatchParticipantRoleReporter && message.SenderRole != MatchParticipantRoleFinder {
			return errors.New("domain: invalid mediated match sender role")
		}
		if (message.SenderRole == MatchParticipantRoleReporter && reporter == nil) ||
			(message.SenderRole == MatchParticipantRoleFinder && finder == nil) {
			return errors.New("domain: mediated match sender role has no participant")
		}
		if strings.TrimSpace(message.Message) != message.Message ||
			!validMediatedMessageText(message.Message) || message.SentAt.IsZero() {
			return errors.New("domain: invalid mediated match message")
		}
	}
	return nil
}

func validMediatedMessageText(message string) bool {
	if !utf8.ValidString(message) || message == "" || utf8.RuneCountInString(message) > MaxMediatedMessageRunes {
		return false
	}
	for _, char := range message {
		if (char < 0x20 && char != '\n' && char != '\r' && char != '\t') || char == 0x7f {
			return false
		}
	}
	return true
}

func stableMediatedMessageID(matchID string, actor PrincipalRef, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(
		matchID + "\x00" + actor.Issuer + "\x00" + actor.Subject + "\x00" + idempotencyKey,
	))
	return "message_" + hex.EncodeToString(digest[:])
}
