# Veterinary Clinic & Shelter HL7 FHIR / PIMS Exchange Protocol — Design Specification

- **Milestone:** 12.2
- **Date:** 2026-10-16
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

In **Milestone 11.4**, PetSpotR introduced offline Emergency Veterinary Passports (`VeterinaryPassport`) and Field Crisis Medical Triage (`TriageAssessment`). However, when a field SAR medic stabilizes a critically injured animal (`TRIAGE_RED` or `TRIAGE_YELLOW`) and transports it to an emergency veterinary hospital or municipal shelter clinic, there is currently no automated, standards-compliant clinical handoff protocol between field operations and Practice Information Management Systems (PIMS) such as IDEXX Cornerstone, ezyVet, Avimark, or Shelterluv.

**Milestone 12.2** introduces the **Veterinary Clinic & Shelter HL7 FHIR / PIMS Exchange Protocol**:
1. **HL7 FHIR R4 Veterinary Bundle Serializer & Validator (`pkg/veterinary/fhir.go`):**
   - Maps `domain.VeterinaryPassport` and `domain.TriageAssessment` into a standards-compliant HL7 FHIR R4 `Bundle` (`resourceType: "Bundle"`, `type: "document"`) signed with a canonical SHA-256 `BundleDigestHex`:
     - `Patient` resource featuring the official HL7 `http://hl7.org/fhir/StructureDefinition/patient-animal` extension (`species`, `breed`) and microchip/rabies identifiers.
     - `Encounter` resource (`class: "EMER"`, priority mapped from `TRIAGE_RED`/`YELLOW`/`GREEN`/`BLACK`, trauma notes, and receiving clinic reference).
     - `Observation` resources for each assessed physiological vital sign (`HeartRateBPM` LOINC `8867-4`, `RespiratoryRateBPM` LOINC `9279-1`, `TemperatureF` LOINC `8310-5`, `CapillaryRefillSec`, `MucousMembrane`, `GlasgowComaScale`, and `WeightKg` LOINC `29463-7`).
     - `AllergyIntolerance` resources mapped from `VeterinaryPassport.Allergies` (including `criticality: "high"` for `ANAPHYLACTIC`).
     - `Immunization` resources mapped from `VeterinaryPassport.Vaccinations`.
     - `MedicationAdministration` resources mapped from `TriageAssessment.AdministeredTreatments`.
   - Also provides a flat **PIMS Exchange Adapter** (`PIMSIntakeRecord` JSON & CSV export) compatible with Cornerstone / ezyVet / Shelterluv bulk intake schemas.
2. **Bidirectional Pre-Arrival ER Handoff & Clinic Discharge Reconciliation:**
   - **Outbound Pre-Arrival Handoff (`POST /api/v1/veterinary/handoffs`):** Field medics dispatch a `ClinicalHandoff` with receiving `ClinicID`, `ETAMinutes`, `TransportUnit`, `AssessmentID`, and `PetID`, generating a signed FHIR R4 Bundle and PIMS record and alerting the clinic via real-time SSE (`/api/v1/veterinary/triage/stream`) and WebRTC mesh (`s.signalingHub`).
   - **Inbound Clinic PIMS Webhook (`POST /api/v1/veterinary/handoffs/{id}/pims-webhook`):** Receiving veterinary clinics post intake/stabilization/discharge updates (`IN_TREATMENT`, `STABILIZED`, `CLEARED_FOR_REUNIFICATION`), verified microchip scans (`VerifiedMicrochipID`), attending veterinarian notes, and discharge summaries.
3. **Integrated FHIR Handoff Cockpit & Inspector UI (`/triage` & `/veterinary/fhir`):**
   - Adds a **Pre-Arrival ER Handoff & FHIR R4 Exchange** section directly in the `/triage` cockpit (plus a dedicated `/veterinary/fhir` Clinical Interoperability Portal) with registered 24/7 Emergency Vet Clinics, one-click ER Handoff dispatch, interactive FHIR R4 Bundle JSON & PIMS CSV inspector, and an Inbound PIMS Discharge Simulator.
   - Strict CSP compliance (zero inline `<script>` or `eval()`) and WCAG AAA (`> 7:1`) contrast across light and dark themes.

---

## 2. Domain Models & State Store Collections

### 2.1 Store Collections (`pkg/store/store.go`)
- `CollectionVeterinaryClinics = "veterinary_clinics"`
- `CollectionClinicalHandoffs = "clinical_handoffs"`

### 2.2 Domain Models (`pkg/domain/veterinary_fhir.go`)

```go
package domain

import "time"

type PIMSProvider string

const (
	PIMSCornerstone PIMSProvider = "IDEXX_CORNERSTONE"
	PIMSEzyVet      PIMSProvider = "EZYVET"
	PIMSAvimark     PIMSProvider = "AVIMARK"
	PIMSShelterluv  PIMSProvider = "SHELTERLUV"
	PIMSGenericFHIR PIMSProvider = "HL7_FHIR_R4"
)

type HandoffStatus string

const (
	HandoffStatusEnRoute                  HandoffStatus = "EN_ROUTE_PRE_ARRIVAL"
	HandoffStatusReceivedInTreatment      HandoffStatus = "IN_CLINIC_TREATMENT"
	HandoffStatusStabilized               HandoffStatus = "STABILIZED"
	HandoffStatusClearedForReunification  HandoffStatus = "CLEARED_FOR_REUNIFICATION"
)

type VeterinaryClinic struct {
	ClinicID          string       `json:"clinicId"`
	Name              string       `json:"name"`
	PIMS              PIMSProvider `json:"pims"`
	Emergency24Hr     bool         `json:"emergency24Hr"`
	Phone             string       `json:"phone"`
	Address           string       `json:"address"`
	Latitude          float64      `json:"latitude"`
	Longitude         float64      `json:"longitude"`
	AcceptingPatients bool         `json:"acceptingPatients"`
	CreatedAt         time.Time    `json:"createdAt"`
}

type FHIRResourceEntry struct {
	FullURL  string         `json:"fullUrl"`
	Resource map[string]any `json:"resource"`
}

type FHIRVeterinaryBundle struct {
	ResourceType    string              `json:"resourceType"` // "Bundle"
	ID              string              `json:"id"`
	Type            string              `json:"type"`         // "document"
	Timestamp       time.Time           `json:"timestamp"`
	BundleDigestHex string              `json:"bundleDigestHex"`
	Entry           []FHIRResourceEntry `json:"entry"`
}

type PIMSIntakeRecord struct {
	HandoffID           string         `json:"handoffId"`
	ClinicID            string         `json:"clinicId"`
	ClinicName          string         `json:"clinicName"`
	PIMS                PIMSProvider   `json:"pims"`
	PetID               string         `json:"petId"`
	PetName             string         `json:"petName"`
	Species             string         `json:"species"`
	Breed               string         `json:"breed"`
	WeightKg            float64        `json:"weightKg"`
	MicrochipID         string         `json:"microchipId,omitempty"`
	TriageCategory      TriageCategory `json:"triageCategory"`
	HeartRateBPM        int            `json:"heartRateBpm"`
	RespiratoryRateBPM  int            `json:"respiratoryRateBpm"`
	TemperatureF        float64        `json:"temperatureF"`
	CriticalAllergies   []string       `json:"criticalAllergies"`
	TreatmentsGiven     []string       `json:"treatmentsGiven"`
	AttendingVet        string         `json:"attendingVet,omitempty"`
	DischargeSummary    string         `json:"dischargeSummary,omitempty"`
}

type ClinicalHandoff struct {
	HandoffID           string               `json:"handoffId"`
	AssessmentID        string               `json:"assessmentId"`
	PetID               string               `json:"petId"`
	PassportID          string               `json:"passportId,omitempty"`
	ClinicID            string               `json:"clinicId"`
	ClinicName          string               `json:"clinicName"`
	PIMS                PIMSProvider         `json:"pims"`
	TriageCategory      TriageCategory       `json:"triageCategory"`
	TransportUnit       string               `json:"transportUnit"`
	ETAMinutes          int                  `json:"etaMinutes"`
	Status              HandoffStatus        `json:"status"`
	VerifiedMicrochipID string               `json:"verifiedMicrochipId,omitempty"`
	AttendingVet        string               `json:"attendingVet,omitempty"`
	DischargeSummary    string               `json:"dischargeSummary,omitempty"`
	ReadyForReunion     bool                 `json:"readyForReunion"`
	FHIRBundle          FHIRVeterinaryBundle `json:"fhirBundle"`
	PIMSRecord          PIMSIntakeRecord     `json:"pimsRecord"`
	DispatchedAt        time.Time            `json:"dispatchedAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}
```

---

## 3. REST API & Interoperability Endpoints (`internal/app/webfrontend/veterinary_fhir_handlers.go`)

1. `GET /veterinary/fhir` — Renders the Clinical Interoperability & HL7 FHIR R4 Exchange Portal (`templates/veterinary_fhir.html`).
2. `GET /api/v1/veterinary/clinics` — Lists registered emergency veterinary clinics (seeds default regional 24/7 ER clinics if empty).
3. `POST /api/v1/veterinary/clinics` — Registers a new veterinary hospital or shelter clinic partner.
4. `POST /api/v1/veterinary/handoffs` — Dispatches an outbound Pre-Arrival ER Handoff from a `TriageAssessment` (and optional `VeterinaryPassport`), generating the signed `FHIRVeterinaryBundle` and `PIMSIntakeRecord`, storing in `store.CollectionClinicalHandoffs`, and broadcasting `clinical_handoff_dispatched` on SSE (`/api/v1/veterinary/triage/stream`) and WebRTC mesh.
5. `GET /api/v1/veterinary/handoffs` — Lists clinical handoffs (optional `?petId=` or `?clinicId=` or `?status=` filter).
6. `GET /api/v1/veterinary/handoffs/{id}/fhir` — Returns the raw HL7 FHIR R4 `Bundle` JSON (`application/fhir+json`) or PIMS CSV (`?format=csv`).
7. `POST /api/v1/veterinary/handoffs/{id}/pims-webhook` — Processes an inbound clinic PIMS status update (`IN_CLINIC_TREATMENT`, `STABILIZED`, `CLEARED_FOR_REUNIFICATION`), records `VerifiedMicrochipID`, `AttendingVet`, and `DischargeSummary`, sets `ReadyForReunion = true` when cleared, and broadcasts `clinical_handoff_updated` on SSE and mesh.
