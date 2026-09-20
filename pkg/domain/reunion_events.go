package domain

import "time"

type ReunionEventType string

const (
	ReunionEventMessageCreated     ReunionEventType = "message"
	ReunionEventPresence           ReunionEventType = "presence"
	ReunionEventResolved           ReunionEventType = "reunion_resolved"
	ReunionEventSighting           ReunionEventType = "sighting"
	ReunionEventSearchPartyUpdated ReunionEventType = "search_party_updated"
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

type SearchPartyEventPayload struct {
	Type                  string  `json:"type"`
	PartyID               string  `json:"partyId"`
	SectorID              string  `json:"sectorId,omitempty"`
	Status                string  `json:"status,omitempty"`
	CoveragePercentage    float64 `json:"coveragePercentage"`
	ActiveVolunteersCount int     `json:"activeVolunteersCount"`
}
