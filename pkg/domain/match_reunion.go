package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxReunionFeedbackRunes = 1000

var (
	ErrInvalidMatchReunion  = errors.New("domain: invalid match reunion resolution")
	ErrMatchReunionConflict = errors.New("domain: match reunion resolution conflicts with persisted state")
)

// MatchReunionAudit is the private immutable receipt for one operator terminal
// transition. ActorKey is an opaque application digest rather than provider
// identity or contact data.
type MatchReunionAudit struct {
	OperationID        string    `json:"operationId"`
	ActorKey           string    `json:"actorKey"`
	Role               Role      `json:"role"`
	Scope              RoleScope `json:"scope"`
	AssignmentID       string    `json:"assignmentId"`
	AssignmentRevision int64     `json:"assignmentRevision"`
	ResolvedAt         time.Time `json:"resolvedAt"`
	Rating             int       `json:"rating"`
	Feedback           string    `json:"feedback,omitempty"`
}

// Validate requires the only role/scope combination currently authorized to
// resolve a match and rejects malformed persisted audit state.
func (a MatchReunionAudit) Validate() error {
	if err := validateCanonicalRoleText("reunion operation ID", a.OperationID, maxRoleOperationRunes); err != nil {
		return err
	}
	if !validRoleDigest(a.ActorKey, "role_principal_v1_") {
		return errors.New("domain: invalid reunion audit actor key")
	}
	if a.Role != RoleOperator || a.Scope != (RoleScope{Kind: RoleScopeGlobal}) {
		return errors.New("domain: reunion audit requires a global operator")
	}
	if a.AssignmentID != roleAssignmentIDFromKey(a.ActorKey, a.Role, a.Scope) {
		return errors.New("domain: reunion audit assignment does not match its actor, role, and scope")
	}
	if a.AssignmentRevision < 1 || a.AssignmentRevision%2 == 0 {
		return errors.New("domain: reunion audit requires an active odd assignment revision")
	}
	if a.ResolvedAt.IsZero() || a.ResolvedAt.Location() != time.UTC {
		return errors.New("domain: reunion audit time must be UTC")
	}
	if a.Rating < 1 || a.Rating > 5 {
		return errors.New("domain: reunion rating must be between 1 and 5")
	}
	if !utf8.ValidString(a.Feedback) || strings.TrimSpace(a.Feedback) != a.Feedback ||
		utf8.RuneCountInString(a.Feedback) > maxReunionFeedbackRunes {
		return errors.New("domain: reunion feedback must be canonical bounded valid UTF-8")
	}
	for _, char := range a.Feedback {
		if unicode.IsControl(char) {
			return errors.New("domain: reunion feedback must not contain control characters")
		}
	}
	return nil
}

// ApplyGlobalOperatorReunion applies one audited terminal transition. An exact
// retry is a no-op; every changed retry or non-confirmed source state conflicts.
func ApplyGlobalOperatorReunion(
	match MatchRecord,
	participants MatchParticipantRecord,
	actor PrincipalRef,
	authorization RoleAssignment,
	operationID string,
	rating int,
	feedback string,
	resolvedAt time.Time,
) (MatchRecord, MatchParticipantRecord, bool, error) {
	if err := match.Validate(); err != nil {
		return MatchRecord{}, MatchParticipantRecord{}, false, fmt.Errorf("domain: invalid persisted reunion match: %w", err)
	}
	if participants.MatchID != match.MatchID || participants.LostPetID != match.MatchedPetID ||
		participants.FoundPetID != match.FoundPetID {
		return MatchRecord{}, MatchParticipantRecord{}, false, errors.New("domain: match participants do not match reunion target")
	}
	if err := participants.Validate(); err != nil {
		return MatchRecord{}, MatchParticipantRecord{}, false, err
	}
	actorKey, err := RolePrincipalKey(actor)
	if err != nil {
		return MatchRecord{}, MatchParticipantRecord{}, false, err
	}
	if err := authorization.Validate(); err != nil || authorization.Status != RoleAssignmentStatusActive ||
		authorization.PrincipalKey != actorKey || authorization.Role != RoleOperator ||
		authorization.Scope != (RoleScope{Kind: RoleScopeGlobal}) {
		return MatchRecord{}, MatchParticipantRecord{}, false, errors.New("domain: active global operator assignment is required")
	}
	audit := MatchReunionAudit{
		OperationID:        operationID,
		ActorKey:           actorKey,
		Role:               RoleOperator,
		Scope:              RoleScope{Kind: RoleScopeGlobal},
		AssignmentID:       authorization.AssignmentID,
		AssignmentRevision: authorization.Revision,
		ResolvedAt:         resolvedAt.UTC(),
		Rating:             rating,
		Feedback:           strings.TrimSpace(feedback),
	}
	if err := audit.Validate(); err != nil {
		return MatchRecord{}, MatchParticipantRecord{}, false, fmt.Errorf("%w: %w", ErrInvalidMatchReunion, err)
	}
	if participants.ReunionAudit != nil {
		if match.Status == MatchStatusReunited && participants.ReunionAudit.matchesRequest(audit) {
			return match, participants, false, nil
		}
		return MatchRecord{}, MatchParticipantRecord{}, false, ErrMatchReunionConflict
	}
	if match.Status != MatchStatusConfirmed {
		return MatchRecord{}, MatchParticipantRecord{}, false, ErrMatchReunionConflict
	}
	nextMatch := match
	nextMatch.Status = MatchStatusReunited
	nextParticipants := participants
	nextParticipants.ReunionAudit = &audit
	return nextMatch, nextParticipants, true, nil
}

func (a MatchReunionAudit) matchesRequest(other MatchReunionAudit) bool {
	return a.OperationID == other.OperationID && a.ActorKey == other.ActorKey && a.Role == other.Role &&
		a.Scope == other.Scope && a.AssignmentID == other.AssignmentID &&
		a.Rating == other.Rating && a.Feedback == other.Feedback
}
