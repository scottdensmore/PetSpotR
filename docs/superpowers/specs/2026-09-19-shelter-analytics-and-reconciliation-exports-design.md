# Milestone 6.4: Shelter Analytics & Automated Reconciliation Exports Design Specification

## 1. Executive Summary
Milestone 6.4 expands PetSpotR's municipal shelter integration by providing real-time operational analytics, coalition return-to-owner (RTO) performance indicators, and automated reconciliation exports in standard CSV and GIS-ready GeoJSON formats. This empowers municipal shelter directors, rescue coalitions, and animal control agencies to measure reunification velocity, evaluate microchip scanning penetration, and seamlessly integrate PetSpotR data into external CRM and GIS platforms without exposing PII.

---

## 2. Core Principles & Constraints
1. **Zero-PII Wire Contract**: All exported datasets and analytics views must strictly maintain privacy. Microchips are masked as `<Registry> ••••<last4>` (e.g. `HomeAgain ••••3456`), and personal owner/finder contact information is strictly omitted from public and coalition exports.
2. **Deterministic & Pure Calculations**: Analytics aggregation in `pkg/analytics` relies on pure Go business logic that processes `domain.FoundPetRecord` and `domain.MatchRecord` collections deterministically.
3. **Standards-Compliant Exports**:
   - Tabular exports comply strictly with RFC 4180 CSV specifications.
   - Spatial exports comply strictly with RFC 7946 GeoJSON FeatureCollection specifications.
4. **Zero Client-Side JavaScript Dependencies**: Dashboard UI relies exclusively on semantic HTML5, CSS custom properties, and native DOM APIs adhering to strict Content Security Policy (`script-src 'self'`).
5. **Continuous Verification**: 100% test coverage across unit tests, HTTP integration tests, and Playwright E2E browser journey tests.

---

## 3. Package Architecture & Analytics Engine (`pkg/analytics`)

### 3.1 Data Structures
```go
package analytics

import (
    "time"
    "github.com/scottdensmore/petspotr/pkg/domain"
)

// FilterOptions defines criteria for filtering analytics.
type FilterOptions struct {
    ShelterID string    `json:"shelterId,omitempty"`
    StartDate time.Time `json:"startDate,omitempty"`
    EndDate   time.Time `json:"endDate,omitempty"`
}

// ShelterKPISummary captures top-level recovery and intake metrics.
type ShelterKPISummary struct {
    TotalIntakes               int     `json:"totalIntakes"`
    ActiveInShelterCare        int     `json:"activeInShelterCare"`
    ReunitedCount              int     `json:"reunitedCount"`
    ReturnToOwnerRate          float64 `json:"returnToOwnerRate"`          // ReunitedCount / TotalIntakes (0.0 to 1.0)
    MicrochippedCount          int     `json:"microchippedCount"`
    MicrochipScanRate          float64 `json:"microchipScanRate"`          // MicrochippedCount / TotalIntakes (0.0 to 1.0)
    DeterministicMatchCount    int     `json:"deterministicMatchCount"`
    MultimodalMatchCount       int     `json:"multimodalMatchCount"`
    DeterministicMatchRatio    float64 `json:"deterministicMatchRatio"`    // Deterministic / (Deterministic + Multimodal)
    MedianIntakeToMatchHours   float64 `json:"medianIntakeToMatchHours"`   // Median hours from intake to match
    MedianIntakeToReunionHours float64 `json:"medianIntakeToReunionHours"` // Median hours from intake to reunion
}

// MunicipalShelterStats aggregates metrics for a single shelter.
type MunicipalShelterStats struct {
    ShelterID         string  `json:"shelterId"`
    ShelterName       string  `json:"shelterName"`
    IntakeCount       int     `json:"intakeCount"`
    ActiveCareCount   int     `json:"activeCareCount"`
    ReunitedCount     int     `json:"reunitedCount"`
    ReturnToOwnerRate float64 `json:"returnToOwnerRate"`
    MicrochipScanRate float64 `json:"microchipScanRate"`
}

// ShelterInfo provides identity metadata for shelter filter pickers.
type ShelterInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// AnalyticsReport is the complete aggregated analytics payload.
type AnalyticsReport struct {
    GeneratedAt      time.Time               `json:"generatedAt"`
    Filter           FilterOptions           `json:"filter"`
    OverallKPIs      ShelterKPISummary       `json:"overallKpis"`
    ShelterBreakdown []MunicipalShelterStats `json:"shelterBreakdown"`
    AvailableShelters []ShelterInfo          `json:"availableShelters"`
}
```

### 3.2 Calculation Rules
1. **Intake Scope**: Includes all `domain.FoundPetRecord` objects where `CustodyStatus == domain.CustodyShelterCare` or `ShelterID != ""`.
2. **Filtering**: If `FilterOptions.ShelterID` is specified, records are restricted to that shelter. If `StartDate` and/or `EndDate` are specified, records are filtered based on `FoundAt` (intake date).
3. **Reunion Status**: An intake is classified as `Reunited` if `FoundPetRecord.Status == domain.FoundPetStatusResolved` or if an associated `MatchRecord` has `Status == domain.MatchStatusReunited` or `domain.MatchStatusConfirmed`.
4. **Turnaround Velocity**:
   - `IntakeToMatch`: Difference between `FoundPetRecord.FoundAt` and `MatchRecord.MatchedAt`.
   - `IntakeToReunion`: Difference between `FoundPetRecord.FoundAt` and `MatchRecord.MatchedAt` (or resolution timestamp).
   - Statistical median is computed by sorting observed durations and picking the midpoint (or average of two midpoints). If no matches exist, returns `0.0`.

---

## 4. Reconciliation Export Serializers

### 4.1 Tabular CSV Serializer (`ExportReconciliationCSV`)
Streams a comma-separated values document using Go's standard `encoding/csv`:
- **Header Columns**:
  ```csv
  IntakeID,ShelterID,ShelterName,ReportedDate,Species,Breed,PrimaryColor,CustodyStatus,MicrochipStatus,MaskedMicrochip,MicrochipRegistry,MatchStatus,MatchType,MatchScore,ReunitedDate
  ```
- **Values**:
  - `MaskedMicrochip`: Generated strictly via `microchip.MaskMicrochip(raw)` (e.g. `HomeAgain ••••3456`).
  - Dates: Formatted as ISO 8601 / RFC 3339 strings (`2026-09-19T08:00:00Z`).
  - Match score: Formatted to 2 decimal places (`1.00`).

### 4.2 Spatial GeoJSON Serializer (`ExportReconciliationGeoJSON`)
Produces an RFC 7946-compliant `FeatureCollection`:
```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [-122.378, 47.648]
      },
      "properties": {
        "petId": "found-12345",
        "intakeId": "INT-2026-8819",
        "shelterId": "shelter-sea-01",
        "shelterName": "Seattle Animal Shelter",
        "species": "Dog",
        "breed": "Golden Retriever",
        "custodyStatus": "Shelter Care",
        "microchipStatus": "Verified",
        "maskedMicrochip": "HomeAgain ••••3456",
        "microchipRegistry": "HomeAgain",
        "matchStatus": "CONFIRMED",
        "intakeDate": "2026-09-19T08:00:00Z"
      }
    }
  ]
}
```
If an intake lacks valid geographic coordinates, it is omitted from the GeoJSON features array or assigned a null geometry.

---

## 5. Web Frontend Endpoints & Dashboard UI

### 5.1 HTTP Endpoints (`internal/app/webfrontend`)
- `GET /shelters/analytics`: Serves the responsive dashboard HTML template.
- `GET /api/v1/shelters/analytics`: Returns the aggregated `AnalyticsReport` JSON.
- `GET /api/v1/shelters/analytics/export.csv`: Streams the CSV reconciliation export file.
- `GET /api/v1/shelters/analytics/export.geojson`: Streams the GeoJSON reconciliation export file.

All API endpoints accept optional query parameters:
- `shelterId`: Unique shelter ID filter (e.g. `shelter-sea-01`).
- `range`: Relative window (`7d`, `30d`, `90d`, `all`).
- `startDate`: Explicit RFC 3339 start timestamp.
- `endDate`: Explicit RFC 3339 end timestamp.

### 5.2 Frontend UI Components
1. **Navigation Bar**: Added `Shelter Analytics` link (`/shelters/analytics`) to the main navigation menu.
2. **Filter Controls**:
   - `#shelter-filter`: Dropdown dynamically populated with available municipal shelters.
   - `#date-range-filter`: Select control for standard time windows.
   - Export buttons: `#btn-export-csv` and `#btn-export-geojson`.
3. **KPI Stat Cards**:
   - Return-to-Owner (RTO) Rate (`%`)
   - Median Match Velocity (`hours`)
   - Microchip Scanning Penetration (`%`)
   - Deterministic Microchip Match Ratio (`%`)
4. **Coalition Comparative Breakdown Table**:
   - Lists participating animal shelters with comparative metrics and responsive glassmorphism styles.

---

## 6. Verification & Testing Strategy
1. **Unit Tests (`pkg/analytics`)**:
   - `analytics_test.go`: Tests calculations with empty data, single shelter, multi-shelter, microchip proportions, and median velocity computation.
   - `export_test.go`: Tests CSV row counts, RFC 4180 compliance, header fields, Zero-PII assertions, and GeoJSON geometry validation.
2. **Web Frontend Integration Tests (`internal/app/webfrontend/shelter_analytics_test.go`)**:
   - Tests HTML rendering of `/shelters/analytics`.
   - Tests JSON API responses and query parameter filtering.
   - Tests streaming CSV and GeoJSON headers and bodies.
3. **Playwright E2E Journey (`tests/playwright/e2e/shelter-analytics-journey.spec.ts`)**:
   - End-to-end browser verification of dashboard navigation, filter interactions, KPI card rendering, and CSV/GeoJSON export downloads.
4. **Full Verification**:
   - `make verify` (`go vet`, `golangci-lint`, OpenTofu check, `yamllint`, Go race tests).
   - Full Playwright suite execution.
