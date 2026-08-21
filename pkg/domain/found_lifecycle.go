package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// PetStatusChangedFoundPayloadVersion introduces found-report lifecycle
	// values without widening the published lost-only payload-v1 contract.
	PetStatusChangedFoundPayloadVersion  = 2
	MaxFoundPetLifecycleOperationRunes   = 128
	FoundPetLifecycleAuthorizationFinder = "finder"
)

var (
	ErrInvalidFoundPetLifecycle  = errors.New("domain: invalid found-pet lifecycle transition")
	ErrFoundPetNotOwned          = errors.New("domain: principal does not own found-pet report")
	ErrFoundPetLifecycleConflict = errors.New("domain: found-pet lifecycle transition conflicts with persisted state")
)

// FoundPetLifecycleAudit is the private immutable receipt for a finder-owned
// terminal transition. ActorKey is opaque and never exposes provider identity
// or contact.
type FoundPetLifecycleAudit struct {
	OperationID    string         `json:"operationId"`
	ActorKey       string         `json:"actorKey"`
	AuthorizedAs   string         `json:"authorizedAs"`
	PreviousStatus FoundPetStatus `json:"previousStatus"`
	Status         FoundPetStatus `json:"status"`
	ChangedAt      time.Time      `json:"changedAt"`
	EventID        string         `json:"eventId"`
}

// PetStatusChangedV2 is the contact- and identity-free found lifecycle event.
// Payload v1 remains the lost -> reunited contract represented by
// PetStatusChangedV1.
type PetStatusChangedV2 struct {
	PetID          string         `json:"petId"`
	ReportType     string         `json:"reportType"`
	PreviousStatus FoundPetStatus `json:"previousStatus"`
	Status         FoundPetStatus `json:"status"`
	ChangedAt      time.Time      `json:"changedAt"`
}

// DecodedPetStatusChanged is the normalized reader representation shared by
// the lost payload-v1 and found payload-v2 contracts.
type DecodedPetStatusChanged struct {
	PetID          string
	ReportType     string
	PreviousStatus string
	Status         string
	ChangedAt      time.Time
}

// FoundPetLifecycleResult carries the canonical persisted and event state.
type FoundPetLifecycleResult struct {
	Record  FoundPetRecord
	Event   PetStatusChangedV2
	EventID string
	Changed bool
}

// ApplyFinderFoundPetResolution applies one finder-owned found -> resolved
// transition. Exact actor/key retries are stable no-ops.
func ApplyFinderFoundPetResolution(
	record FoundPetRecord,
	actor PrincipalRef,
	operationID string,
	changedAt time.Time,
) (FoundPetLifecycleResult, error) {
	if err := validateFoundPetLifecycleRecord(record); err != nil {
		return FoundPetLifecycleResult{}, err
	}
	actor.Issuer = strings.TrimSpace(actor.Issuer)
	if err := actor.Validate(); err != nil {
		return FoundPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidFoundPetLifecycle, err)
	}
	if err := validateCanonicalRoleText(
		"lifecycle operation ID", operationID, MaxFoundPetLifecycleOperationRunes,
	); err != nil {
		return FoundPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidFoundPetLifecycle, err)
	}
	if !principalRefsEqual(record.OwnedBy, actor) {
		return FoundPetLifecycleResult{}, ErrFoundPetNotOwned
	}
	actorKey, err := RolePrincipalKey(actor)
	if err != nil {
		return FoundPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidFoundPetLifecycle, err)
	}
	if record.LifecycleAudit != nil {
		if record.Status == FoundPetStatusResolved &&
			record.LifecycleAudit.AuthorizedAs == FoundPetLifecycleAuthorizationFinder &&
			record.LifecycleAudit.OperationID == operationID &&
			record.LifecycleAudit.ActorKey == actorKey {
			return foundLifecycleResultFromAudit(record, false)
		}
		return FoundPetLifecycleResult{}, ErrFoundPetLifecycleConflict
	}
	if record.Status != FoundPetStatusFound {
		return FoundPetLifecycleResult{}, ErrFoundPetLifecycleConflict
	}
	if changedAt.IsZero() {
		return FoundPetLifecycleResult{}, fmt.Errorf("%w: transition time is required", ErrInvalidFoundPetLifecycle)
	}
	changedAt = changedAt.UTC()
	event := PetStatusChangedV2{
		PetID: record.PetID, ReportType: "found", PreviousStatus: FoundPetStatusFound,
		Status: FoundPetStatusResolved, ChangedAt: changedAt,
	}
	eventID, err := foundPetStatusChangedEventID(event)
	if err != nil {
		return FoundPetLifecycleResult{}, fmt.Errorf("%w: %w", ErrInvalidFoundPetLifecycle, err)
	}
	next := record
	next.Status = FoundPetStatusResolved
	next.LifecycleAudit = &FoundPetLifecycleAudit{
		OperationID: operationID, ActorKey: actorKey, AuthorizedAs: FoundPetLifecycleAuthorizationFinder,
		PreviousStatus: FoundPetStatusFound, Status: FoundPetStatusResolved,
		ChangedAt: changedAt, EventID: eventID,
	}
	if err := validateFoundPetLifecycleRecord(next); err != nil {
		return FoundPetLifecycleResult{}, err
	}
	return FoundPetLifecycleResult{Record: next, Event: event, EventID: eventID, Changed: true}, nil
}

func foundLifecycleResultFromAudit(record FoundPetRecord, changed bool) (FoundPetLifecycleResult, error) {
	audit := record.LifecycleAudit
	if audit == nil {
		return FoundPetLifecycleResult{}, ErrInvalidFoundPetLifecycle
	}
	event := PetStatusChangedV2{
		PetID: record.PetID, ReportType: "found", PreviousStatus: audit.PreviousStatus,
		Status: audit.Status, ChangedAt: audit.ChangedAt,
	}
	return FoundPetLifecycleResult{Record: record, Event: event, EventID: audit.EventID, Changed: changed}, nil
}

func validateFoundPetLifecycleRecord(record FoundPetRecord) error {
	if strings.TrimSpace(record.PetID) == "" || strings.TrimSpace(record.PetID) != record.PetID ||
		record.FoundAt.IsZero() || record.FoundAt.Location() != time.UTC ||
		record.FinderIdentityRef != reportIdentityRef("found", record.PetID, "finder") {
		return fmt.Errorf("%w: invalid persisted found-pet lifecycle record", ErrInvalidFoundPetLifecycle)
	}
	if record.OwnedBy == nil {
		return ErrFoundPetNotOwned
	}
	if err := record.OwnedBy.Validate(); err != nil || strings.TrimSpace(record.OwnedBy.Issuer) != record.OwnedBy.Issuer {
		return fmt.Errorf("%w: invalid persisted found-pet owner", ErrInvalidFoundPetLifecycle)
	}
	canonical, err := json.Marshal(NormalizeFoundPetRecord(record))
	if err != nil {
		return fmt.Errorf("%w: normalize persisted found-pet lifecycle record: %w", ErrInvalidFoundPetLifecycle, err)
	}
	persisted, err := json.Marshal(record)
	if err != nil || !bytes.Equal(persisted, canonical) {
		return fmt.Errorf("%w: persisted found-pet lifecycle record is not canonical", ErrInvalidFoundPetLifecycle)
	}
	canonicalFields := FoundPetReport{
		PetID: record.PetID, ImageURL: record.ImageURL, ImageObject: record.ImageObject,
		FoundAt: record.FoundAt, Location: record.Location, GeocodingStatus: record.GeocodingStatus,
		Coordinates: record.Coordinates, Species: record.Species, Breed: record.Breed,
		PrimaryColor: record.PrimaryColor, SecondaryColor: record.SecondaryColor,
		DistinctiveMarkings: record.DistinctiveMarkings, CustodyStatus: record.CustodyStatus,
		Status: FoundPetStatusFound, OwnedBy: record.OwnedBy,
	}
	if err := canonicalFields.Validate(); err != nil {
		return fmt.Errorf("%w: invalid persisted found-pet lifecycle fields: %w", ErrInvalidFoundPetLifecycle, err)
	}
	switch record.Status {
	case FoundPetStatusFound, FoundPetStatusResolved:
	default:
		return fmt.Errorf("%w: unsupported persisted found-pet lifecycle status %q", ErrInvalidFoundPetLifecycle, record.Status)
	}
	if record.LifecycleAudit == nil {
		if record.Status != FoundPetStatusFound {
			return fmt.Errorf("%w: terminal found-pet lifecycle state has no audit", ErrInvalidFoundPetLifecycle)
		}
		return nil
	}
	audit := record.LifecycleAudit
	if audit.AuthorizedAs != FoundPetLifecycleAuthorizationFinder ||
		audit.PreviousStatus != FoundPetStatusFound || audit.Status != FoundPetStatusResolved ||
		audit.ChangedAt.IsZero() || audit.ChangedAt.Location() != time.UTC || !validEventID(audit.EventID) {
		return fmt.Errorf("%w: invalid found-pet lifecycle audit", ErrInvalidFoundPetLifecycle)
	}
	if err := validateCanonicalRoleText(
		"lifecycle operation ID", audit.OperationID, MaxFoundPetLifecycleOperationRunes,
	); err != nil {
		return fmt.Errorf("%w: invalid persisted lifecycle operation: %w", ErrInvalidFoundPetLifecycle, err)
	}
	ownerKey, err := RolePrincipalKey(*record.OwnedBy)
	if err != nil || audit.ActorKey != ownerKey {
		return fmt.Errorf("%w: found-pet lifecycle audit does not match its finder", ErrInvalidFoundPetLifecycle)
	}
	result, err := foundLifecycleResultFromAudit(record, false)
	if err != nil {
		return err
	}
	wantEventID, err := foundPetStatusChangedEventID(result.Event)
	if err != nil || audit.EventID != wantEventID || record.Status != audit.Status {
		return fmt.Errorf("%w: found-pet lifecycle audit does not match its event and status", ErrInvalidFoundPetLifecycle)
	}
	return nil
}

func foundPetStatusChangedEventID(event PetStatusChangedV2) (string, error) {
	envelope, err := NewFoundPetStatusChangedEnvelope(event)
	if err != nil {
		return "", err
	}
	return envelope.ID, nil
}

// NewFoundPetStatusChangedEnvelope creates the canonical redacted found
// lifecycle payload-v2 event.
func NewFoundPetStatusChangedEnvelope(event PetStatusChangedV2) (EventEnvelope, error) {
	if strings.TrimSpace(event.PetID) == "" || strings.TrimSpace(event.PetID) != event.PetID ||
		event.ReportType != "found" || event.PreviousStatus != FoundPetStatusFound ||
		event.Status != FoundPetStatusResolved || event.ChangedAt.IsZero() ||
		event.ChangedAt.Location() != time.UTC {
		return EventEnvelope{}, errors.New("domain: invalid found pet status changed event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return EventEnvelope{}, err
	}
	return NewEventEnvelope(EventEnvelopeInput{
		Type: EventTypePetStatusChanged, OccurredAt: event.ChangedAt,
		AggregateID: event.PetID, AggregateVersion: PetStatusChangedAggregateVersion,
		PayloadVersion: PetStatusChangedFoundPayloadVersion, Payload: payload,
	})
}

// DecodePetStatusChanged reads the lost payload-v1 and found payload-v2 event
// shapes without widening payload v1's status semantics.
func DecodePetStatusChanged(data []byte) (DecodedPetStatusChanged, *EventEnvelope, error) {
	var payload struct {
		PetID          string    `json:"petId"`
		ReportType     string    `json:"reportType"`
		PreviousStatus string    `json:"previousStatus"`
		Status         string    `json:"status"`
		ChangedAt      time.Time `json:"changedAt"`
	}
	envelope, err := DecodeEventPayload(data, EventTypePetStatusChanged, &payload)
	if err != nil {
		return DecodedPetStatusChanged{}, nil, err
	}
	version := PetStatusChangedPayloadVersion
	if envelope != nil {
		version = envelope.PayloadVersion
		if envelope.AggregateVersion != PetStatusChangedAggregateVersion ||
			envelope.AggregateID != payload.PetID || !envelope.OccurredAt.Equal(payload.ChangedAt) {
			return DecodedPetStatusChanged{}, nil, errors.New("domain: pet status changed envelope does not match payload")
		}
	}
	if strings.TrimSpace(payload.PetID) == "" || strings.TrimSpace(payload.PetID) != payload.PetID ||
		payload.ChangedAt.IsZero() || payload.ChangedAt.Location() != time.UTC {
		return DecodedPetStatusChanged{}, nil, errors.New("domain: invalid pet status changed payload")
	}
	switch version {
	case PetStatusChangedPayloadVersion:
		if payload.ReportType != "lost" || payload.PreviousStatus != string(LostPetStatusLost) ||
			payload.Status != string(LostPetStatusReunited) {
			return DecodedPetStatusChanged{}, nil, errors.New("domain: invalid pet status changed payload v1")
		}
	case PetStatusChangedFoundPayloadVersion:
		if payload.ReportType != "found" || payload.PreviousStatus != string(FoundPetStatusFound) ||
			payload.Status != string(FoundPetStatusResolved) {
			return DecodedPetStatusChanged{}, nil, errors.New("domain: invalid pet status changed payload v2")
		}
	default:
		return DecodedPetStatusChanged{}, nil, fmt.Errorf("domain: unsupported pet status changed payload version %d", version)
	}
	return DecodedPetStatusChanged{
		PetID: payload.PetID, ReportType: payload.ReportType,
		PreviousStatus: payload.PreviousStatus, Status: payload.Status, ChangedAt: payload.ChangedAt,
	}, envelope, nil
}
