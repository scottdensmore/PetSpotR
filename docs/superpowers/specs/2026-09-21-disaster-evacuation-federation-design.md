# Milestone 10.3: Municipal & Multi-Agency Disaster Evacuation Federation — Design Specification

## 1. Executive Summary & Problem Statement
During regional natural disasters (wildfires, severe coastal flooding, catastrophic storms), municipal animal shelters, animal control agencies, and humane societies face sudden surges of displaced pets. Emergency teams operate temporary pop-up crisis centers, fairground staging shelters, and mobile triage hubs under severe time and capacity constraints.

Milestone 10.3 establishes a unified **Disaster Evacuation Federation** within PetSpotR. The system enables:
1. **Dynamic Crisis Shelter & Evacuation Hub Management**: Tracking pop-up crisis facilities, fairground holding pens, capacity thresholds, and real-time dog/cat occupancy.
2. **High-Throughput Bulk Intake Processing**: Rapid ingestion of animal rosters via interactive CSV drag-and-drop dropzone or REST API, featuring automated microchip standard validation (`pkg/microchip`) and deduplication.
3. **Mutual Aid Transfer Ledger**: An immutable custody manifest system tracking interstate and inter-agency animal convoys (`STAGED` $\to$ `IN_TRANSIT` $\to$ `RECEIVED` $\to$ `RECONCILED`), automatically updating animal custody and locations upon verified delivery.
4. **Disaster Reunification Escalation**: An instant matching queue prioritizing evacuated animals against active lost reports in the disaster perimeter, triggering immediate owner emergency notifications for verified microchip and visual matches.
5. **Interactive Operations Dashboard (`/evacuation`)**: A high-contrast, responsive emergency management interface for shelter managers and rescue coordinators.

---

## 2. Architecture & Domain Models

### 2.1 Domain Types (`pkg/domain/evacuation.go`)
```go
package domain

import "time"

type EvacuationHubType string

const (
	HubTypePermanentShelter  EvacuationHubType = "PERMANENT_SHELTER"
	HubTypePopUpCrisisCenter EvacuationHubType = "POP_UP_CRISIS_CENTER"
	HubTypeFairgroundStaging EvacuationHubType = "FAIRGROUND_STAGING"
	HubTypeMobileTriage      EvacuationHubType = "MOBILE_TRIAGE"
)

type EvacuationHubStatus string

const (
	HubStatusActive  EvacuationHubStatus = "ACTIVE"
	HubStatusFull    EvacuationHubStatus = "FULL"
	HubStatusStandby EvacuationHubStatus = "STANDBY"
	HubStatusClosed  EvacuationHubStatus = "CLOSED"
)

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

type TransferStatus string

const (
	TransferStatusStaged     TransferStatus = "STAGED"
	TransferStatusInTransit  TransferStatus = "IN_TRANSIT"
	TransferStatusReceived   TransferStatus = "RECEIVED"
	TransferStatusReconciled TransferStatus = "RECONCILED"
)

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

type CrisisReunificationPriority string

const (
	CrisisPriorityMicrochipMatch   CrisisReunificationPriority = "MICROCHIP_EXACT"
	CrisisPriorityHighSimilarity   CrisisReunificationPriority = "HIGH_SIMILARITY"
	CrisisPriorityProximityAlert   CrisisReunificationPriority = "PROXIMITY_ALERT"
)

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
```

### 2.2 Store Collections (`pkg/store/store.go`)
- `EvacuationHubsCollection = "evacuation_hubs"`
- `TransferManifestsCollection = "transfer_manifests"`
- `CrisisIntakesCollection = "crisis_intakes"`
- `CrisisReunificationsCollection = "crisis_reunifications"`

---

## 3. Bulk Intake Engine & Microchip Reconciliation

### 3.1 Streaming Ingestion Parser (`pkg/sheltersync/bulk_intake.go`)
The bulk intake engine supports both JSON arrays and CSV streams:
- Normalizes flexible headers (`species`/`animal_type`, `microchip_id`/`rfid`/`chip`, `notes`/`triage_notes`, `address`/`found_location`).
- Validates microchips via `pkg/microchip.ValidateAndNormalize`, tagging records with standard (`iso_15`, `avid_9`, `euro_10`) or flagging invalid digits.
- Creates corresponding `FoundPetRecord` entries in `store.FoundPetsCollection` with shelter metadata.
- Matches against `store.LostPetsCollection` to detect instant exact microchip matches and stages candidates into `store.CrisisReunificationsCollection`.
- Returns a structured `BulkIntakeBatchSummary`:
```go
type BulkIntakeRecordResult struct {
	RowIndex          int                 `json:"rowIndex"`
	PetID             string              `json:"petId,omitempty"`
	Microchip         string              `json:"microchip,omitempty"`
	MicrochipValid    bool                `json:"microchipValid"`
	MicrochipStandard microchip.Standard  `json:"microchipStandard,omitempty"`
	Status            string              `json:"status"` // INGESTED, INVALID_MICROCHIP, ERROR
	ErrorMessage      string              `json:"errorMessage,omitempty"`
	MatchedLostPetID  string              `json:"matchedLostPetId,omitempty"`
}

type BulkIntakeBatchSummary struct {
	BatchID        string                   `json:"batchId"`
	HubID          string                   `json:"hubId"`
	HubName        string                   `json:"hubName"`
	TotalProcessed int                      `json:"totalProcessed"`
	IngestedCount  int                      `json:"ingestedCount"`
	ErrorCount     int                      `json:"errorCount"`
	MicrochipCount int                      `json:"microchipCount"`
	InstantMatches int                      `json:"instantMatches"`
	Results        []BulkIntakeRecordResult `json:"results"`
	ProcessedAt    time.Time                `json:"processedAt"`
}
```

---

## 4. Backend REST Endpoints

1. **`GET /api/v1/evacuations/hubs`**: List active evacuation facilities and capacities.
2. **`POST /api/v1/evacuations/hubs`**: Register an emergency staging hub.
3. **`POST /api/v1/evacuations/intake-batch`**: Ingest bulk animal roster (CSV file upload or JSON payload).
4. **`GET /api/v1/evacuations/transfers`**: Query mutual aid transfer ledger.
5. **`POST /api/v1/evacuations/transfers`**: Stage an inter-hub animal transfer convoy.
6. **`PUT /api/v1/evacuations/transfers/{id}/status`**: Progress transfer status (`IN_TRANSIT`, `RECEIVED`, `RECONCILED`), updating facility occupancy and pet custody locations.
7. **`GET /api/v1/evacuations/reunification-queue`**: Query prioritized disaster match candidates.
8. **`POST /api/v1/evacuations/reunification-queue/{matchId}/contact`**: Send emergency owner notification and mark contacted.
9. **`GET /evacuation`**: Render HTML operations dashboard view.

---

## 5. Operations Dashboard UI (`evacuation.html`, `evacuation.js`, `styles.css`)

1. **Emergency Operations Metrics HUD**:
   - Total Evacuated Animals (`#metric-evacuated-total`).
   - Active Emergency Facilities (`#metric-active-hubs`).
   - Regional Available Capacity (`#metric-open-capacity`).
   - Pending Crisis Reunifications (`#metric-reunifications-pending`).
2. **Tab Navigation**:
   - `[🏢 Evacuation Facilities | 📥 Bulk Intake Importer | 🚚 Mutual Aid Transfers | 🤝 Crisis Reunifications]`.
3. **Facilities Tab**:
   - Card grid with occupancy meters for overall, dog, and cat capacity.
   - "Register Pop-Up Hub" modal button.
4. **Bulk Intake Tab**:
   - Drag-and-drop CSV/JSON dropzone with file picker.
   - Target hub selector and ingest action button.
   - Dynamic results table displaying microchip validation badges and instant match indicators.
5. **Mutual Aid Transfers Tab**:
   - Manifest ledger table with status pills.
   - Action controls: `Mark In Transit`, `Confirm Delivery & Custody`.
6. **Crisis Reunifications Tab**:
   - Card list displaying paired evacuated pet and lost pet reports.
   - Priority badges (`🔥 Exact Microchip Match`).
   - One-tap "Alert Owner" dispatch button.

---

## 6. Verification & Quality Gates

1. **Unit & Race Tests**:
   - `pkg/sheltersync/bulk_intake_test.go`: CSV parser, microchip normalization, error handling.
   - `internal/app/webfrontend/evacuation_test.go`: Hub management, batch intake, transfer status transitions, crisis reunification queue.
   - Tested under `go test -race -v`.
2. **Playwright E2E User Journey Test**:
   - `tests/playwright/e2e/disaster-evacuation-journey.spec.ts`:
     - Creates registered lost dog report with microchip.
     - Opens `/evacuation` dashboard and verifies operational metrics.
     - Ingests bulk CSV roster with matching microchip.
     - Inspects batch results table.
     - Verifies Crisis Reunifications queue and triggers owner alert.
     - Stages and completes a mutual aid transfer manifest, verifying capacity adjustments.
3. **Pre-Push Quality Gate**:
   - `export GOTOOLCHAIN=go1.26.5 && make verify`: 0 lint issues, OpenTofu valid, yamllint clean, 100% tests passing.
   - Full Playwright suite passing (124+ tests).
