# Design Specification: Milestone 11.4 — Emergency Veterinary Passport & Crisis Medical Triage

## 1. Executive Summary & Objective

In natural disasters (wildfires, hurricanes, earthquakes) and wilderness rescue operations, displaced animals frequently suffer acute injuries (smoke inhalation, burns, fractures, hypothermia, shock) or have chronic conditions (insulin-dependent diabetes, Addison's disease, epilepsy) and life-threatening allergies. Field rescue medics and pop-up emergency evacuation shelters frequently receive animals without immediate owner contact, medical records, or internet connectivity.

Milestone 11.4 introduces the **Emergency Veterinary Passport & Crisis Medical Triage** subsystem to PetSpotR:
1. **Offline Emergency Veterinary Passport**: An ultra-compact, cryptographically signed digital health record (ECDSA P-256, CBOR, and Zlib compressed binary payload $< 380$ bytes) encoded into an offline QR code. This allows any field responder with a camera to instantly verify rabies status, vaccinations, chronic conditions, and critical allergies without internet access.
2. **Species-Aware Crisis Medical Triage Engine (VECC / SALT protocol)**: Deterministic evaluation of vital signs (Heart Rate, Respiratory Rate, Temperature, Capillary Refill Time, Mucous Membrane color, Glasgow Coma Scale) against species-specific canine vs. feline physiological baselines. Automatically categorizes patient acuity into standard triage levels (`TRIAGE_RED`, `TRIAGE_YELLOW`, `TRIAGE_GREEN`, `TRIAGE_BLACK`) and provides weight-based resuscitation and drug dosage guidance.
3. **Multi-Agency & P2P Mesh Federation**: Integrates with Disaster Evacuation Hubs (`HubTypeMobileTriage`, `HubTypePopUpCrisisCenter`), dispatches real-time SSE updates (`triage_assessment_created`), and broadcasts peer-to-peer WebRTC frames (`mesh:medical-triage`) to field personnel (`MeshRoleMedic`).
4. **Accessible Triage Cockpit & Printable Passport**: A glassmorphic Crisis Triage Cockpit (`/triage`) with live vitals gauges and treatment logs, paired with printable high-contrast emergency passport cards (`/p/{id}/passport`) adhering to strict WCAG AAA contrast requirements ($> 7:1$).

---

## 2. Architecture & Domain Models

### 2.1 Domain Enums and Core Entities (`pkg/domain/veterinary.go`)

```go
package domain

import "time"

// TriageCategory defines the emergency medical acuity tag.
type TriageCategory string

const (
	TriageCategoryRed    TriageCategory = "TRIAGE_RED"    // Immediate / Life-Threatening
	TriageCategoryYellow TriageCategory = "TRIAGE_YELLOW" // Delayed / Serious
	TriageCategoryGreen  TriageCategory = "TRIAGE_GREEN"  // Minor / Walking Wounded
	TriageCategoryBlack  TriageCategory = "TRIAGE_BLACK"  // Deceased / Expectant
)

// MucousMembraneColor represents the physical perfusion and oxygenation indicator.
type MucousMembraneColor string

const (
	MMColorPink     MucousMembraneColor = "MM_PINK"     // Normal perfusion
	MMColorPale     MucousMembraneColor = "MM_PALE"     // Shock, vasoconstriction, anemia
	MMColorCyanotic MucousMembraneColor = "MM_CYANOTIC" // Severe hypoxia, respiratory distress
	MMColorIcteric  MucousMembraneColor = "MM_ICTERIC"  // Jaundice, hepatic dysfunction
	MMColorBrickRed MucousMembraneColor = "MM_BRICK_RED" // Sepsis, hyperthermia, vasodilation
)

// AllergySeverity defines the clinical consequence of allergen exposure.
type AllergySeverity string

const (
	AllergySeverityMild         AllergySeverity = "MILD"
	AllergySeverityModerate     AllergySeverity = "MODERATE"
	AllergySeverityAnaphylactic AllergySeverity = "ANAPHYLACTIC"
)

// ClinicalAllergy represents a diagnosed medication or environmental allergen.
type ClinicalAllergy struct {
	Allergen            string          `json:"allergen"`
	Severity            AllergySeverity `json:"severity"`
	ReactionDescription string          `json:"reactionDescription"`
}

// VaccinationRecord represents an immunization history entry.
type VaccinationRecord struct {
	VaccineName      string    `json:"vaccineName"`
	AdministeredDate time.Time `json:"administeredDate"`
	ExpirationDate   time.Time `json:"expirationDate"`
	BatchNumber      string    `json:"batchNumber,omitempty"`
	ClinicName       string    `json:"clinicName"`
	Verified         bool      `json:"verified"`
}

// ChronicCondition documents ongoing illnesses and active treatments.
type ChronicCondition struct {
	ConditionName string    `json:"conditionName"`
	DiagnosedDate time.Time `json:"diagnosedDate"`
	Medications   []string  `json:"medications"`
	CriticalFlag  bool      `json:"criticalFlag"`
}

// VitalSigns represents quantifiable physiological measurements.
type VitalSigns struct {
	HeartRateBPM       int                 `json:"heartRateBpm"`
	RespiratoryRateBPM int                 `json:"respiratoryRateBpm"`
	TemperatureF       float64             `json:"temperatureF"`
	CapillaryRefillSec float64             `json:"capillaryRefillSec"`
	MucousMembrane     MucousMembraneColor `json:"mucousMembrane"`
	GlasgowComaScale   int                 `json:"glasgowComaScale"` // 3 to 18
}

// TreatmentRoute defines the pharmacological administration pathway.
type TreatmentRoute string

const (
	RouteIV      TreatmentRoute = "IV"
	RouteIM      TreatmentRoute = "IM"
	RouteSC      TreatmentRoute = "SC"
	RoutePO      TreatmentRoute = "PO"
	RouteTopical TreatmentRoute = "TOPICAL"
)

// ClinicalTreatment records an emergency procedure or medication administered.
type ClinicalTreatment struct {
	TreatmentID    string         `json:"treatmentId"`
	MedicationName string         `json:"medicationName"`
	Dosage         string         `json:"dosage"`
	Route          TreatmentRoute `json:"route"`
	AdministeredBy string         `json:"administeredBy"`
	AdministeredAt time.Time      `json:"administeredAt"`
	Notes          string         `json:"notes,omitempty"`
}

// TriageAssessment records an emergency clinical evaluation in the field.
type TriageAssessment struct {
	AssessmentID           string              `json:"assessmentId"`
	PetID                  string              `json:"petId"`
	HubID                  string              `json:"hubId,omitempty"`
	MedicID                string              `json:"medicId"`
	MedicName              string              `json:"medicName"`
	Species                string              `json:"species"`
	WeightKg               float64             `json:"weightKg"`
	Category               TriageCategory      `json:"category"`
	Vitals                 VitalSigns          `json:"vitals"`
	TraumaNotes            string              `json:"traumaNotes"`
	AdministeredTreatments []ClinicalTreatment `json:"administeredTreatments"`
	AssessedAt             time.Time           `json:"assessedAt"`
	ReassessedAt           *time.Time          `json:"reassessedAt,omitempty"`
}

// VeterinaryPassport is the verifiable digital health record for an animal.
type VeterinaryPassport struct {
	PassportID        string              `json:"passportId"`
	PetID             string              `json:"petId"`
	PetName           string              `json:"petName"`
	Species           string              `json:"species"`
	Breed             string              `json:"breed"`
	MicrochipID       string              `json:"microchipId,omitempty"`
	RabiesTagID       string              `json:"rabiesTagId,omitempty"`
	BloodType         string              `json:"bloodType,omitempty"`
	WeightKg          float64             `json:"weightKg"`
	Vaccinations      []VaccinationRecord `json:"vaccinations"`
	Allergies         []ClinicalAllergy   `json:"allergies"`
	ChronicConditions []ChronicCondition  `json:"chronicConditions"`
	EmergencyContact  string              `json:"emergencyContact"`
	PrimaryClinic     string              `json:"primaryClinic"`
	PublicKeyHex      string              `json:"publicKeyHex"`
	SignatureHex      string              `json:"signatureHex"`
	IssuedAt          time.Time           `json:"issuedAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
}
```

### 2.2 State Store Constants (`pkg/store/store.go`)
- `CollectionVeterinaryPassports = "veterinary_passports"`
- `CollectionTriageAssessments = "triage_assessments"`

---

## 3. Cryptographic Passport Engine (`pkg/veterinary/passport.go`)

### 3.1 Digital Signatures with ECDSA P-256
The passport uses standard `crypto/ecdsa` over the NIST P-256 curve with SHA-256. 
- Signing Authority: Veterinary clinics or municipal animal control authorities hold private ECDSA keys.
- Verification: Public keys are stored with the passport or distributed in the public directory.

### 3.2 Canonical Binary Payload & Compression
To ensure compact QR codes that scan under field conditions with smudged screens or dim lighting:
1. Canonical payload extracts critical fields: `PassportID`, `PetID`, `MicrochipID`, `RabiesTagID`, `WeightKg`, expiration dates, critical allergy names, and active vaccine names.
2. Canonical bytes are signed using ECDSA P-256 ($R, S$ 64-byte IEEE P1363 signature).
3. Payload is serialized with a compact byte envelope and compressed using standard `compress/zlib` (deflate).
4. The compressed binary is encoded as base64url and prefixed with `VP1:` (Veterinary Passport Version 1).
5. Typical payload size: $240\text{--}320\text{ bytes}$, well within standard QR Version 6–8 with Error Correction Level M.

### 3.3 QR Matrix Generation
- Utilizes `pkg/qrcode.GenerateMatrix` (and `skip2/go-qrcode` for PNG data URI generation) to render clean vector or raster QR images.

---

## 4. Species-Aware Clinical Triage Engine (`pkg/veterinary/triage.go`)

### 4.1 Physiological Normal Baselines

| Vital Measurement | Canine (Adult) | Feline (Adult) | Critical Abnormal Boundary (RED) |
|---|---|---|---|
| **Heart Rate (HR)** | 60–140 BPM | 140–220 BPM | Dog: $< 50$ or $> 220$; Cat: $< 100$ or $> 260$ |
| **Respiratory Rate (RR)** | 10–30 BPM | 20–40 BPM | Dog: $< 8$ or $> 60$; Cat: $< 12$ or $> 80$ |
| **Body Temperature** | 100.0–102.5 °F | 100.0–102.5 °F | $< 96.0^\circ\text{F}$ (severe hypothermia) or $> 105.0^\circ\text{F}$ |
| **Capillary Refill Time** | $< 2.0\text{ sec}$ | $< 2.0\text{ sec}$ | $\ge 3.0\text{ sec}$ (severe hypoperfusion/shock) |
| **Mucous Membranes** | Pink | Pink | Cyanotic (blue/grey), Brick Red (sepsis) |
| **Glasgow Coma Scale** | 15–18 | 15–18 | $\le 8$ (stupor, comatose) |

### 4.2 Acuity Calculation Algorithm
Function `EvaluateTriage(species string, vitals domain.VitalSigns, traumaFlags TraumaIndicators) (domain.TriageCategory, []string)`:
1. **Immediate Evaluation for `TRIAGE_BLACK`**:
   - Absence of heartbeat, absence of respirations, fixed dilated pupils, and absence of corneal reflex.
2. **Immediate Evaluation for `TRIAGE_RED`**:
   - Any critical abnormal boundary met: Cyanotic/Brick-Red MM, CRT $\ge 3.0\text{s}$, critical HR/RR boundaries, GCS $\le 8$, active arterial hemorrhage, flail chest, or $> 30\%$ burn surface area.
3. **Evaluation for `TRIAGE_YELLOW`**:
   - Pale MM with CRT $2.0\text{--}3.0\text{s}$, open non-arterial fractures, deep lacerations, burns $10\text{--}30\%$, GCS $9\text{--}14$, or moderate hypothermia ($96.0\text{--}99.5^\circ\text{F}$).
4. **Default to `TRIAGE_GREEN`**:
   - Vitals within normal limits, ambulatory, superficial abrasions, GCS $15\text{--}18$.

### 4.3 Emergency Resuscitation & Dosage Helper
Provides standard veterinary emergency formulas:
- **Resuscitation Fluid Bolus (Crystalloid)**:
  - Canine: $15\text{ mL/kg}$ IV over 15 minutes.
  - Feline: $7.5\text{ mL/kg}$ IV over 15 minutes.
- **Epinephrine 1:1000 ($1\text{ mg/mL}$)**:
  - Low dose CPR: $0.01\text{ mg/kg}$ ($0.01\text{ mL/kg}$).
- **Buprenorphine ($0.3\text{ mg/mL}$)**:
  - Analgesia: $0.02\text{ mg/kg}$ ($0.067\text{ mL/kg}$) IV/IM/sublingual.

---

## 5. WebFrontend REST Endpoints & Mesh Broadcast

### 5.1 REST Endpoints (`internal/app/webfrontend/veterinary_handlers.go`)
- `POST /api/v1/veterinary/passports`: Create and sign a passport; returns passport details and `qrDataUri`.
- `POST /api/v1/veterinary/passports/verify`: Verify signature and decode compressed QR string.
- `GET /api/v1/veterinary/passports/{id}`: Fetch passport record.
- `POST /api/v1/veterinary/triage`: Submit emergency triage evaluation; triggers SSE and P2P mesh frames.
- `GET /api/v1/veterinary/triage/{petId}`: Fetch chronological triage and treatment timeline.
- `POST /api/v1/veterinary/triage/{assessmentId}/treatments`: Append treatment record.

### 5.2 SSE Event Envelope
```json
{
  "event": "triage_assessment_created",
  "data": {
    "assessmentId": "triage-9921",
    "petId": "pet-104",
    "hubId": "hub-red-cross-1",
    "category": "TRIAGE_RED",
    "species": "Dog",
    "vitals": {
      "heartRateBpm": 210,
      "respiratoryRateBpm": 65,
      "mucousMembrane": "MM_CYANOTIC"
    },
    "assessedAt": "2026-09-23T20:30:00Z"
  }
}
```

### 5.3 P2P Mesh Broadcast (`mesh:medical-triage`)
- Medics operating off-grid broadcast assessment envelopes over WebRTC DataChannels using `pkg/domain/mesh.go` and IndexedDB offline caching.

---

## 6. Frontend UI & Accessibility Specifications

### 6.1 Crisis Triage Cockpit (`/triage`)
- Route: `GET /triage`.
- Species selector button group with live threshold visual cues.
- Sliders and numeric steppers for HR, RR, Temperature, and GCS.
- Triage Category Badges with WCAG AAA ($> 7:1$) contrast:
  - `.badge-triage-red`: Background `#8A0000`, Text `#FFFFFF` (Contrast $8.4:1$).
  - `.badge-triage-yellow`: Background `#594200`, Text `#FFFBEA` (Contrast $7.3:1$).
  - `.badge-triage-green`: Background `#0F5132`, Text `#FFFFFF` (Contrast $7.1:1$).
  - `.badge-triage-black`: Background `#121212`, Text `#FFFFFF`, Border `#666666` (Contrast $18.2:1$).
- Live patient triage board with drag-and-drop or status update capabilities.

### 6.2 Printable Veterinary Passport (`/p/{id}/passport`)
- Route: `GET /p/{id}/passport`.
- High-resolution QR code rendered via SVG/canvas.
- High-visibility Critical Allergy Alert box: Red border, high contrast, alert icon.
- Complete vaccination audit table.
- Printable styling via `@media print`.

---

## 7. Verification Strategy & Acceptance Criteria

### 7.1 Automated Playwright Journey (`veterinary-triage-journey.spec.ts`)
1. **Passport Generation**: Issue passport via API with rabies vaccine and penicillin allergy; assert valid ECDSA signature.
2. **Offline Payload Verification**: Verify `VP1:` QR string via verify API; assert decoded data match.
3. **Cockpit Intake & Live Evaluation**: Open `/triage`, select Canine, input shock vitals, assert live gauge shows `TRIAGE_RED`.
4. **Assessment Submission**: Submit triage form; assert patient card appears in active queue with `.badge-triage-red`.
5. **Emergency Treatment Log**: Administer shock bolus treatment; assert entry in timeline.
6. **Printable Passport Card**: Load `/p/{id}/passport`; assert QR code, vaccination list, and critical allergy alert banner.

### 7.2 Full Repository Verification
- Clean pass on `export GOTOOLCHAIN=go1.26.5 && make verify`.
- 100% pass on all 150+ Playwright tests with zero race conditions.
