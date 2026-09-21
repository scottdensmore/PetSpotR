package domain

import "time"

// EvacuationHubType represents the operational facility category of an evacuation site.
type EvacuationHubType string

const (
	HubTypePermanentShelter  EvacuationHubType = "PERMANENT_SHELTER"
	HubTypePopUpCrisisCenter EvacuationHubType = "POP_UP_CRISIS_CENTER"
	HubTypeFairgroundStaging EvacuationHubType = "FAIRGROUND_STAGING"
	HubTypeMobileTriage      EvacuationHubType = "MOBILE_TRIAGE"
)

// EvacuationHubStatus indicates the current operational readiness or capacity level.
type EvacuationHubStatus string

const (
	HubStatusActive  EvacuationHubStatus = "ACTIVE"
	HubStatusFull    EvacuationHubStatus = "FULL"
	HubStatusStandby EvacuationHubStatus = "STANDBY"
	HubStatusClosed  EvacuationHubStatus = "CLOSED"
)

// EvacuationHub represents an emergency staging, triage, or holding shelter for displaced animals.
type EvacuationHub struct {
	HubID            string              `json:"hubId"`
	Name             string              `json:"name"`
	Type             EvacuationHubType   `json:"type"`
	Status           EvacuationHubStatus `json:"status"`
	Address          string              `json:"address"`
	Coordinates      LocationPoint       `json:"coordinates"`
	TotalCapacity    int                 `json:"totalCapacity"`
	CurrentOccupancy int                 `json:"currentOccupancy"`
	DogCapacity      int                 `json:"dogCapacity"`
	DogOccupancy     int                 `json:"dogOccupancy"`
	CatCapacity      int                 `json:"catCapacity"`
	CatOccupancy     int                 `json:"catOccupancy"`
	ContactName      string              `json:"contactName"`
	ContactPhone     string              `json:"contactPhone"`
	ContactEmail     string              `json:"contactEmail"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

// TransferStatus represents the chain-of-custody stage of an inter-agency convoy.
type TransferStatus string

const (
	TransferStatusStaged     TransferStatus = "STAGED"
	TransferStatusInTransit  TransferStatus = "IN_TRANSIT"
	TransferStatusReceived   TransferStatus = "RECEIVED"
	TransferStatusReconciled TransferStatus = "RECONCILED"
)

// TransferManifest represents an immutable custody ledger for animals transferred between facilities.
type TransferManifest struct {
	TransferID       string         `json:"transferId"`
	OriginHubID      string         `json:"originHubId"`
	OriginHubName    string         `json:"originHubName"`
	DestHubID        string         `json:"destHubId"`
	DestHubName      string         `json:"destHubName"`
	Status           TransferStatus `json:"status"`
	AnimalIDs        []string       `json:"animalIds"`
	TotalAnimals     int            `json:"totalAnimals"`
	TransporterName  string         `json:"transporterName"`
	TransporterPhone string         `json:"transporterPhone"`
	VehicleNotes     string         `json:"vehicleNotes"`
	DepartureTime    *time.Time     `json:"departureTime,omitempty"`
	ArrivalTime      *time.Time     `json:"arrivalTime,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// CrisisReunificationPriority categorizes the confidence level of a disaster pet match.
type CrisisReunificationPriority string

const (
	CrisisPriorityMicrochipMatch CrisisReunificationPriority = "MICROCHIP_EXACT"
	CrisisPriorityHighSimilarity CrisisReunificationPriority = "HIGH_SIMILARITY"
	CrisisPriorityProximityAlert CrisisReunificationPriority = "PROXIMITY_ALERT"
)

// CrisisReunificationItem represents an urgent match candidate awaiting emergency owner notification.
type CrisisReunificationItem struct {
	MatchID         string                      `json:"matchId"`
	FoundPetID      string                      `json:"foundPetId"`
	LostPetID       string                      `json:"lostPetId"`
	PetName         string                      `json:"petName"`
	Species         string                      `json:"species"`
	Breed           string                      `json:"breed"`
	CurrentHubID    string                      `json:"currentHubId"`
	CurrentHubName  string                      `json:"currentHubName"`
	OwnerName       string                      `json:"ownerName"`
	OwnerContact    string                      `json:"ownerContact"`
	MicrochipID     string                      `json:"microchipId,omitempty"`
	Priority        CrisisReunificationPriority `json:"priority"`
	SimilarityScore float64                     `json:"similarityScore"`
	Status          string                      `json:"status"` // PENDING, CONTACTED, RESOLVED
	IdentifiedAt    time.Time                   `json:"identifiedAt"`
}
