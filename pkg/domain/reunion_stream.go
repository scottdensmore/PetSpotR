package domain

// ReunionEventBeaconPing is the event type for real-time beacon ping broadcasts.
const ReunionEventBeaconPing ReunionEventType = "beacon_ping"

// BeaconPingEventPayload represents the SSE broadcast payload for a BLE collar beacon observation.
type BeaconPingEventPayload struct {
	Type          string `json:"type"` // "beacon_ping"
	PetID         string `json:"petId"`
	Ping          any    `json:"ping"`
	Triangulation any    `json:"triangulation"`
}
