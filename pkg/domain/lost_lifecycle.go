package domain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	EventTypePetStatusChanged             = "petspotr.pet.status-changed"
	PetStatusChangedPayloadVersion        = 1
	PetStatusChangedAggregateVersion      = 2
	MaxLostPetLifecycleOperationRunes     = 128
	LostPetLifecycleAuthorizationOwner    = "owner"
	LostPetLifecycleAuthorizationOperator = "operator"
)

var (
	ErrInvalidLostPetLifecycle  = errors.New("domain: invalid lost-pet lifecycle transition")
	ErrLostPetNotOwned          = errors.New("domain: principal does not own lost-pet report")
	ErrLostPetLifecycleConflict = errors.New("domain: lost-pet lifecycle transition conflicts with persisted state")
)

// LostPetLifecycleAudit is the private immutable receipt for an authorized
// terminal transition. ActorKey is opaque and never exposes provider identity
// or contact. Operator receipts pin the assignment used for authorization.
type LostPetLifecycleAudit struct {
	OperationID        string        `json:"operationId"`
	ActorKey           string        `json:"actorKey"`
	AuthorizedAs       string        `json:"authorizedAs"`
	AssignmentID       string        `json:"assignmentId,omitempty"`
	AssignmentRevision int64         `json:"assignmentRevision,omitempty"`
	PreviousStatus     LostPetStatus `json:"previousStatus"`
	Status             LostPetStatus `json:"status"`
	ChangedAt          time.Time     `json:"changedAt"`
	EventID            string        `json:"eventId"`
}

// PetStatusChangedV1 is the contact- and identity-free lifecycle event.
type PetStatusChangedV1 struct {
	PetID          string        `json:"petId"`
	ReportType     string        `json:"reportType"`
	PreviousStatus LostPetStatus `json:"previousStatus"`
	Status         LostPetStatus `json:"status"`
	ChangedAt      time.Time     `json:"changedAt"`
}

// LostPetLifecycleResult carries the canonical persisted and event state.
type LostPetLifecycleResult struct {
	Record  LostPetRecord
	Event   PetStatusChangedV1
	EventID string
	Changed bool
}

// ApplyOwnerLostPetReunion applies one owner-authorized lost -> reunited
// transition. Exact actor/key retries are stable no-ops.
func ApplyOwnerLostPetReunion(
	record LostPetRecord,
	actor PrincipalRef,
	operationID string,
	changedAt time.Time,
) (LostPetLifecycleResult, error) {
	if err := validateLostPetLifecycleRecord(record, true); err != nil {
		return LostPetLifecycleResult{}, err
	}
	actor.Issuer = strings.TrimSpace(actor.Issuer)
	if err := actor.Validate(); err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	if err := validateCanonicalRoleText("lifecycle operation ID", operationID, MaxLostPetLifecycleOperationRunes); err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	if !principalRefsEqual(record.OwnedBy, actor) {
		return LostPetLifecycleResult{}, ErrLostPetNotOwned
	}
	actorKey, err := RolePrincipalKey(actor)
	if err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	if record.LifecycleAudit != nil {
		if record.Status == LostPetStatusReunited &&
			record.LifecycleAudit.AuthorizedAs == LostPetLifecycleAuthorizationOwner &&
			record.LifecycleAudit.OperationID == operationID &&
			record.LifecycleAudit.ActorKey == actorKey {
			return lifecycleResultFromAudit(record, false)
		}
		return LostPetLifecycleResult{}, ErrLostPetLifecycleConflict
	}
	if record.Status != LostPetStatusLost {
		return LostPetLifecycleResult{}, ErrLostPetLifecycleConflict
	}
	if changedAt.IsZero() {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: transition time is required", ErrInvalidLostPetLifecycle)
	}
	changedAt = changedAt.UTC()
	event := PetStatusChangedV1{
		PetID: record.PetID, ReportType: "lost", PreviousStatus: LostPetStatusLost,
		Status: LostPetStatusReunited, ChangedAt: changedAt,
	}
	eventID, err := petStatusChangedEventID(event)
	if err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	next := record
	next.Status = LostPetStatusReunited
	next.LifecycleAudit = &LostPetLifecycleAudit{
		OperationID: operationID, ActorKey: actorKey, AuthorizedAs: LostPetLifecycleAuthorizationOwner,
		PreviousStatus: LostPetStatusLost, Status: LostPetStatusReunited,
		ChangedAt: changedAt, EventID: eventID,
	}
	if err := validateLostPetLifecycleRecord(next, true); err != nil {
		return LostPetLifecycleResult{}, err
	}
	return LostPetLifecycleResult{Record: next, Event: event, EventID: eventID, Changed: true}, nil
}

// ApplyGlobalOperatorLostPetReunion applies one global-operator-authorized
// lost -> reunited transition. Exact retries require a currently active global
// assignment but preserve the original pinned assignment revision.
func ApplyGlobalOperatorLostPetReunion(
	record LostPetRecord,
	actor PrincipalRef,
	authorization RoleAssignment,
	operationID string,
	changedAt time.Time,
) (LostPetLifecycleResult, error) {
	if err := validateLostPetLifecycleRecord(record, false); err != nil {
		return LostPetLifecycleResult{}, err
	}
	actor.Issuer = strings.TrimSpace(actor.Issuer)
	if err := actor.Validate(); err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	if err := validateCanonicalRoleText("lifecycle operation ID", operationID, MaxLostPetLifecycleOperationRunes); err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	actorKey, err := RolePrincipalKey(actor)
	if err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	globalScope := RoleScope{Kind: RoleScopeGlobal}
	if err := authorization.Validate(); err != nil || authorization.Status != RoleAssignmentStatusActive ||
		authorization.PrincipalKey != actorKey || authorization.Role != RoleOperator || authorization.Scope != globalScope {
		return LostPetLifecycleResult{}, ErrLostPetNotOwned
	}
	if record.LifecycleAudit != nil {
		if record.Status == LostPetStatusReunited &&
			record.LifecycleAudit.AuthorizedAs == LostPetLifecycleAuthorizationOperator &&
			record.LifecycleAudit.OperationID == operationID && record.LifecycleAudit.ActorKey == actorKey {
			return lifecycleResultFromAudit(record, false)
		}
		return LostPetLifecycleResult{}, ErrLostPetLifecycleConflict
	}
	if record.Status != LostPetStatusLost {
		return LostPetLifecycleResult{}, ErrLostPetLifecycleConflict
	}
	if changedAt.IsZero() {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: transition time is required", ErrInvalidLostPetLifecycle)
	}
	changedAt = changedAt.UTC()
	event := PetStatusChangedV1{
		PetID: record.PetID, ReportType: "lost", PreviousStatus: LostPetStatusLost,
		Status: LostPetStatusReunited, ChangedAt: changedAt,
	}
	eventID, err := petStatusChangedEventID(event)
	if err != nil {
		return LostPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidLostPetLifecycle, err)
	}
	next := record
	next.Status = LostPetStatusReunited
	next.LifecycleAudit = &LostPetLifecycleAudit{
		OperationID: operationID, ActorKey: actorKey, AuthorizedAs: LostPetLifecycleAuthorizationOperator,
		AssignmentID: authorization.AssignmentID, AssignmentRevision: authorization.Revision,
		PreviousStatus: LostPetStatusLost, Status: LostPetStatusReunited,
		ChangedAt: changedAt, EventID: eventID,
	}
	if err := validateLostPetLifecycleRecord(next, false); err != nil {
		return LostPetLifecycleResult{}, err
	}
	return LostPetLifecycleResult{Record: next, Event: event, EventID: eventID, Changed: true}, nil
}

func lifecycleResultFromAudit(record LostPetRecord, changed bool) (LostPetLifecycleResult, error) {
	audit := record.LifecycleAudit
	event := PetStatusChangedV1{
		PetID: record.PetID, ReportType: "lost", PreviousStatus: audit.PreviousStatus,
		Status: audit.Status, ChangedAt: audit.ChangedAt,
	}
	return LostPetLifecycleResult{Record: record, Event: event, EventID: audit.EventID, Changed: changed}, nil
}

func validateLostPetLifecycleRecord(record LostPetRecord, requireOwner bool) error {
	if strings.TrimSpace(record.PetID) == "" || strings.TrimSpace(record.PetID) != record.PetID ||
		record.ReportedAt.IsZero() || record.ReportedAt.Location() != time.UTC ||
		record.OwnerIdentityRef != reportIdentityRef("lost", record.PetID, "owner") {
		return fmt.Errorf("%w: invalid persisted lost-pet lifecycle record", ErrInvalidLostPetLifecycle)
	}
	if record.OwnedBy == nil {
		if requireOwner {
			return ErrLostPetNotOwned
		}
	} else if err := record.OwnedBy.Validate(); err != nil || strings.TrimSpace(record.OwnedBy.Issuer) != record.OwnedBy.Issuer {
		return fmt.Errorf("%w: invalid persisted lost-pet owner", ErrInvalidLostPetLifecycle)
	}
	canonical, err := json.Marshal(NormalizeLostPetRecord(record))
	if err != nil {
		return fmt.Errorf("%w: normalize persisted lost-pet lifecycle record: %w", ErrInvalidLostPetLifecycle, err)
	}
	persisted, err := json.Marshal(record)
	if err != nil || !bytes.Equal(persisted, canonical) {
		return fmt.Errorf("%w: persisted lost-pet lifecycle record is not canonical", ErrInvalidLostPetLifecycle)
	}
	canonicalFields := LostPetReport{
		PetID: record.PetID, PetName: record.PetName, Species: record.Species, Breed: record.Breed,
		PrimaryColor: record.PrimaryColor, Description: record.Description, ImageObject: record.ImageObject,
		ReportedAt: record.ReportedAt, Location: record.Location, GeocodingStatus: record.GeocodingStatus,
		Coordinates: record.Coordinates, Status: LostPetStatusLost, OwnedBy: record.OwnedBy,
	}
	if err := validateLostPetCanonicalFields(canonicalFields); err != nil {
		return fmt.Errorf("%w: invalid persisted lost-pet lifecycle fields: %w", ErrInvalidLostPetLifecycle, err)
	}
	switch record.Status {
	case LostPetStatusLost, LostPetStatusReunited:
	default:
		return fmt.Errorf("%w: unsupported persisted lost-pet lifecycle status %q", ErrInvalidLostPetLifecycle, record.Status)
	}
	if record.LifecycleAudit == nil {
		return nil
	}
	audit := record.LifecycleAudit
	if audit.PreviousStatus != LostPetStatusLost || audit.Status != LostPetStatusReunited ||
		audit.ChangedAt.IsZero() || audit.ChangedAt.Location() != time.UTC ||
		!validEventID(audit.EventID) {
		return fmt.Errorf("%w: invalid lost-pet lifecycle audit", ErrInvalidLostPetLifecycle)
	}
	if err := validateCanonicalRoleText("lifecycle operation ID", audit.OperationID, MaxLostPetLifecycleOperationRunes); err != nil {
		return fmt.Errorf("%w: invalid persisted lifecycle operation: %w", ErrInvalidLostPetLifecycle, err)
	}
	switch audit.AuthorizedAs {
	case LostPetLifecycleAuthorizationOwner:
		if record.OwnedBy == nil {
			return fmt.Errorf("%w: owner lifecycle audit has no owner", ErrInvalidLostPetLifecycle)
		}
		ownerKey, err := RolePrincipalKey(*record.OwnedBy)
		if err != nil || audit.ActorKey != ownerKey || audit.AssignmentID != "" || audit.AssignmentRevision != 0 {
			return fmt.Errorf("%w: lost-pet lifecycle audit does not match its owner", ErrInvalidLostPetLifecycle)
		}
	case LostPetLifecycleAuthorizationOperator:
		globalScope := RoleScope{Kind: RoleScopeGlobal}
		if !validRoleDigest(audit.ActorKey, "role_principal_v1_") ||
			audit.AssignmentID != roleAssignmentIDFromKey(audit.ActorKey, RoleOperator, globalScope) ||
			audit.AssignmentRevision < 1 || audit.AssignmentRevision%2 == 0 {
			return fmt.Errorf("%w: lost-pet lifecycle audit does not match a global operator assignment", ErrInvalidLostPetLifecycle)
		}
	default:
		return fmt.Errorf("%w: invalid lost-pet lifecycle authorization source", ErrInvalidLostPetLifecycle)
	}
	result, err := lifecycleResultFromAudit(record, false)
	if err != nil {
		return err
	}
	wantEventID, err := petStatusChangedEventID(result.Event)
	if err != nil || audit.EventID != wantEventID || record.Status != audit.Status {
		return fmt.Errorf("%w: lost-pet lifecycle audit does not match its event and status", ErrInvalidLostPetLifecycle)
	}
	return nil
}

func petStatusChangedEventID(event PetStatusChangedV1) (string, error) {
	envelope, err := NewPetStatusChangedEnvelope(event)
	if err != nil {
		return "", err
	}
	return envelope.ID, nil
}

// NewPetStatusChangedEnvelope creates the canonical redacted lifecycle event.
func NewPetStatusChangedEnvelope(event PetStatusChangedV1) (EventEnvelope, error) {
	if strings.TrimSpace(event.PetID) == "" || event.ReportType != "lost" ||
		event.PreviousStatus != LostPetStatusLost || event.Status != LostPetStatusReunited ||
		event.ChangedAt.IsZero() || event.ChangedAt.Location() != time.UTC {
		return EventEnvelope{}, errors.New("domain: invalid pet status changed event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return EventEnvelope{}, err
	}
	envelope, err := NewEventEnvelope(EventEnvelopeInput{
		Type: EventTypePetStatusChanged, OccurredAt: event.ChangedAt,
		AggregateID: event.PetID, AggregateVersion: PetStatusChangedAggregateVersion,
		PayloadVersion: PetStatusChangedPayloadVersion, Payload: payload,
	})
	if err != nil {
		return EventEnvelope{}, err
	}
	return envelope, nil
}

func validEventID(value string) bool {
	if !strings.HasPrefix(value, "evt_") || len(value) != len("evt_")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "evt_"))
	return err == nil
}
