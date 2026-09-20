package domain

import "time"

type ReunionEventType string

const (
	ReunionEventMessageCreated ReunionEventType = "message"
	ReunionEventPresence       ReunionEventType = "presence"
	ReunionEventResolved       ReunionEventType = "reunion_resolved"
	ReunionEventSighting       ReunionEventType = "sighting"
)

type ReunionStreamEvent struct {
	EventID   string           `json:"eventId"`
	Type      ReunionEventType `json:"type"`
	MatchID   string           `json:"matchId"`
	Timestamp time.Time        `json:"timestamp"`
	Payload   any              `json:"payload"`
}

type ReunionPresencePayload struct {
	SenderRole MatchParticipantRole `json:"senderRole"`
	Status     string               `json:"status"`
}
