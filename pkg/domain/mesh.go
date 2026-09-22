package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// MeshRole represents the tactical role of a volunteer in the wilderness mesh.
type MeshRole string

const (
	MeshRoleSearcher          MeshRole = "SEARCHER"
	MeshRoleIncidentCommander MeshRole = "INCIDENT_COMMANDER"
	MeshRoleK9Handler         MeshRole = "K9_HANDLER"
	MeshRoleMedic             MeshRole = "MEDIC"
)

// MeshNode represents a peer device participating in the off-grid mesh.
type MeshNode struct {
	NodeID         string    `json:"nodeId"`
	VolunteerID    string    `json:"volunteerId"`
	VolunteerName  string    `json:"volunteerName"`
	Role           MeshRole  `json:"role"`
	SearchPartyID  string    `json:"searchPartyId"`
	BatteryLevel   int       `json:"batteryLevel"` // 0-100 percentage
	ConnectedPeers []string  `json:"connectedPeers"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
}

// MeshMessageType defines message types transferred over WebRTC DataChannels.
type MeshMessageType string

const (
	MeshMsgJoin         MeshMessageType = "JOIN"
	MeshMsgLeave        MeshMessageType = "LEAVE"
	MeshMsgHeartbeat    MeshMessageType = "HEARTBEAT"
	MeshMsgGossipDigest MeshMessageType = "GOSSIP_DIGEST"
	MeshMsgDeltaPayload MeshMessageType = "DELTA_PAYLOAD"
	MeshMsgSOSAlert     MeshMessageType = "SOS_ALERT"
)

// MeshMessage is the standard wire envelope for peer-to-peer data frames.
type MeshMessage struct {
	MessageID     string          `json:"messageId"`
	SearchPartyID string          `json:"searchPartyId"`
	SenderNodeID  string          `json:"senderNodeId"`
	Type          MeshMessageType `json:"type"`
	LamportClock  uint64          `json:"lamportClock"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
}

// SectorRank defines the monotonic progression order of search sectors.
type SectorRank int

const (
	SectorRankUnclaimed SectorRank = 0
	SectorRankClaimed   SectorRank = 1
	SectorRankSearching SectorRank = 2
	SectorRankCleared   SectorRank = 3
)

const (
	SectorStateUnclaimed = "UNCLAIMED"
	SectorStateClaimed   = "CLAIMED"
	SectorStateSearching = "SEARCHING"
	SectorStateCleared   = "CLEARED"
)

// State returns the string representation corresponding to a SectorRank.
func (r SectorRank) State() string {
	switch r {
	case SectorRankClaimed:
		return SectorStateClaimed
	case SectorRankSearching:
		return SectorStateSearching
	case SectorRankCleared:
		return SectorStateCleared
	default:
		return SectorStateUnclaimed
	}
}

// RankFromState resolves a state string into its corresponding monotonic SectorRank.
func RankFromState(state string) SectorRank {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case SectorStateClaimed:
		return SectorRankClaimed
	case SectorStateSearching:
		return SectorRankSearching
	case SectorStateCleared:
		return SectorRankCleared
	default:
		return SectorRankUnclaimed
	}
}

// MeshSectorDelta represents a conflict-free sector state mutation.
type MeshSectorDelta struct {
	PetID                string     `json:"petId"`
	SectorID             string     `json:"sectorId"`
	State                string     `json:"state"` // UNCLAIMED, CLAIMED, SEARCHING, CLEARED
	Rank                 SectorRank `json:"rank"`
	ClaimedByVolunteerID string     `json:"claimedByVolunteerId"`
	ClaimedByName        string     `json:"claimedByName"`
	NodeID               string     `json:"nodeId,omitempty"`
	LamportClock         uint64     `json:"lamportClock"`
	Timestamp            time.Time  `json:"timestamp"`
}

// MeshBreadcrumbDelta represents a volunteer GPS point.
type MeshBreadcrumbDelta struct {
	PetID         string    `json:"petId"`
	VolunteerID   string    `json:"volunteerId"`
	VolunteerName string    `json:"volunteerName"`
	Seq           uint64    `json:"seq"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Accuracy      float64   `json:"accuracy"`
	Timestamp     time.Time `json:"timestamp"`
}

// MeshSightingDelta represents a field sighting logged offline.
type MeshSightingDelta struct {
	SightingID       string    `json:"sightingId"`
	PetID            string    `json:"petId"`
	VolunteerID      string    `json:"volunteerId"`
	VolunteerName    string    `json:"volunteerName"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	Notes            string    `json:"notes"`
	MediaChunkID     string    `json:"mediaChunkId,omitempty"`
	PhotoThumbBase64 string    `json:"photoThumbBase64,omitempty"`
	LamportClock     uint64    `json:"lamportClock"`
	Timestamp        time.Time `json:"timestamp"`
}

// MeshEvacuationDelta represents a field update to animal disaster rosters.
type MeshEvacuationDelta struct {
	IntakeID     string    `json:"intakeId"`
	PetID        string    `json:"petId"`
	FacilityID   string    `json:"facilityId"`
	Status       string    `json:"status"`
	LamportClock uint64    `json:"lamportClock"`
	Timestamp    time.Time `json:"timestamp"`
}

// MeshSOSAlert represents a distress beacon broadcast across the mesh.
type MeshSOSAlert struct {
	AlertID       string    `json:"alertId"`
	VolunteerID   string    `json:"volunteerId"`
	VolunteerName string    `json:"volunteerName"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
}

// MeshBatchDelta is the synchronization payload exchanged during gossip or cloud uplink.
type MeshBatchDelta struct {
	SearchPartyID string                `json:"searchPartyId"`
	SenderNodeID  string                `json:"senderNodeId"`
	Sectors       []MeshSectorDelta     `json:"sectors,omitempty"`
	Breadcrumbs   []MeshBreadcrumbDelta `json:"breadcrumbs,omitempty"`
	Sightings     []MeshSightingDelta   `json:"sightings,omitempty"`
	Evacuations   []MeshEvacuationDelta `json:"evacuations,omitempty"`
	SOSAlerts     []MeshSOSAlert        `json:"sosAlerts,omitempty"`
}

// MeshUplinkSyncResponse is returned by the cloud backend upon WAN reconciliation.
type MeshUplinkSyncResponse struct {
	Success               bool            `json:"success"`
	ReconciledSectors     int             `json:"reconciledSectors"`
	ReconciledBreadcrumbs int             `json:"reconciledBreadcrumbs"`
	ReconciledSightings   int             `json:"reconciledSightings"`
	ReconciledEvacuations int             `json:"reconciledEvacuations"`
	ServerLatestDeltas    *MeshBatchDelta `json:"serverLatestDeltas,omitempty"`
	SyncedAt              time.Time       `json:"syncedAt"`
}
