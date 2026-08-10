package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CurrentEnvelopeVersion is the version of the event transport envelope.
const CurrentEnvelopeVersion = 1

// Stable event type names. Payload versions evolve independently from the
// transport envelope version.
const (
	EventTypeLostPetReported  = "petspotr.lost-pet.reported"
	EventTypeFoundPetReported = "petspotr.found-pet.reported"
)

// EventEnvelope carries stable event identity and ordering metadata around a
// versioned domain payload.
type EventEnvelope struct {
	EnvelopeVersion  int             `json:"envelopeVersion"`
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	OccurredAt       time.Time       `json:"occurredAt"`
	CorrelationID    string          `json:"correlationId"`
	TraceID          string          `json:"traceId"`
	AggregateID      string          `json:"aggregateId"`
	AggregateVersion int64           `json:"aggregateVersion"`
	PayloadVersion   int             `json:"payloadVersion"`
	Payload          json.RawMessage `json:"payload"`
}

// EventEnvelopeInput contains the metadata needed to construct an envelope.
type EventEnvelopeInput struct {
	Type             string
	OccurredAt       time.Time
	CorrelationID    string
	TraceID          string
	AggregateID      string
	AggregateVersion int64
	PayloadVersion   int
	Payload          json.RawMessage
}

// NewEventEnvelope validates input and derives a stable event ID from the
// event type, aggregate identity/version, payload version, and payload bytes.
func NewEventEnvelope(input EventEnvelopeInput) (EventEnvelope, error) {
	eventType := strings.TrimSpace(input.Type)
	if eventType == "" {
		return EventEnvelope{}, errors.New("domain: event type is required")
	}
	if input.OccurredAt.IsZero() {
		return EventEnvelope{}, errors.New("domain: event occurrence time is required")
	}
	aggregateID := strings.TrimSpace(input.AggregateID)
	if aggregateID == "" {
		return EventEnvelope{}, errors.New("domain: aggregate ID is required")
	}
	if input.AggregateVersion < 1 {
		return EventEnvelope{}, errors.New("domain: aggregate version must be positive")
	}
	if input.PayloadVersion < 1 {
		return EventEnvelope{}, errors.New("domain: payload version must be positive")
	}
	if len(input.Payload) == 0 || !json.Valid(input.Payload) {
		return EventEnvelope{}, errors.New("domain: event payload must be valid JSON")
	}

	id := stableEventID(eventType, aggregateID, input.AggregateVersion, input.PayloadVersion, input.Payload)
	correlationID := strings.TrimSpace(input.CorrelationID)
	if correlationID == "" {
		correlationID = id
	}
	traceID := strings.TrimSpace(input.TraceID)
	if traceID == "" {
		traceID = correlationID
	}

	return EventEnvelope{
		EnvelopeVersion:  CurrentEnvelopeVersion,
		ID:               id,
		Type:             eventType,
		OccurredAt:       input.OccurredAt.UTC(),
		CorrelationID:    correlationID,
		TraceID:          traceID,
		AggregateID:      aggregateID,
		AggregateVersion: input.AggregateVersion,
		PayloadVersion:   input.PayloadVersion,
		Payload:          append(json.RawMessage(nil), input.Payload...),
	}, nil
}

// DecodeEventPayload reads either the current envelope format or the legacy
// raw payload. Supporting both keeps in-flight messages backward compatible.
func DecodeEventPayload(data []byte, expectedType string, destination any) (*EventEnvelope, error) {
	var probe struct {
		EnvelopeVersion int `json:"envelopeVersion"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	if probe.EnvelopeVersion == 0 {
		return nil, json.Unmarshal(data, destination)
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if envelope.EnvelopeVersion != CurrentEnvelopeVersion {
		return nil, fmt.Errorf("domain: unsupported event envelope version %d", envelope.EnvelopeVersion)
	}
	if strings.TrimSpace(expectedType) != "" && envelope.Type != expectedType {
		return nil, fmt.Errorf("domain: event type %q does not match %q", envelope.Type, expectedType)
	}
	if envelope.ID == "" || strings.TrimSpace(envelope.Type) == "" || envelope.OccurredAt.IsZero() ||
		strings.TrimSpace(envelope.CorrelationID) == "" || strings.TrimSpace(envelope.TraceID) == "" ||
		strings.TrimSpace(envelope.AggregateID) == "" || envelope.AggregateVersion < 1 || envelope.PayloadVersion < 1 ||
		len(envelope.Payload) == 0 || !json.Valid(envelope.Payload) {
		return nil, errors.New("domain: incomplete event envelope metadata")
	}
	wantID := stableEventID(
		strings.TrimSpace(envelope.Type),
		strings.TrimSpace(envelope.AggregateID),
		envelope.AggregateVersion,
		envelope.PayloadVersion,
		envelope.Payload,
	)
	if envelope.ID != wantID {
		return nil, errors.New("domain: event envelope identity does not match payload")
	}
	if err := json.Unmarshal(envelope.Payload, destination); err != nil {
		return nil, fmt.Errorf("domain: decode event payload: %w", err)
	}
	return &envelope, nil
}

func stableEventID(eventType, aggregateID string, aggregateVersion int64, payloadVersion int, payload []byte) string {
	digest := sha256.New()
	_, _ = fmt.Fprintf(digest, "%s\x00%s\x00%d\x00%d\x00", eventType, aggregateID, aggregateVersion, payloadVersion)
	_, _ = digest.Write(payload)
	return "evt_" + hex.EncodeToString(digest.Sum(nil))
}
