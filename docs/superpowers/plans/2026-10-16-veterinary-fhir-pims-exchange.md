# Veterinary Clinic & Shelter HL7 FHIR / PIMS Exchange Protocol — Implementation Plan

- **Milestone:** 12.2
- **Spec:** `docs/superpowers/specs/2026-10-16-veterinary-fhir-pims-exchange-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-veterinary-fhir-pims-exchange` (`feat/veterinary-fhir-pims-exchange`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collections & Validation (`pkg/domain/veterinary_fhir.go`, `pkg/store/store.go`)
- **Files:**
  - Create `pkg/domain/veterinary_fhir.go`
  - Create `pkg/domain/veterinary_fhir_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `PIMSProvider`, `HandoffStatus`, `VeterinaryClinic`, `FHIRResourceEntry`, `FHIRVeterinaryBundle`, `PIMSIntakeRecord`, and `ClinicalHandoff`.
  - Add `CollectionVeterinaryClinics = "veterinary_clinics"` and `CollectionClinicalHandoffs = "clinical_handoffs"` in `pkg/store/store.go`.
  - Write unit tests for validation and JSON serialization in `pkg/domain/veterinary_fhir_test.go`.

### Task 2: Pure-Go HL7 FHIR R4 Bundle Serializer, Validator & PIMS CSV Exporter (`pkg/veterinary/fhir.go`)
- **Files:**
  - Create `pkg/veterinary/fhir.go`
  - Create `pkg/veterinary/fhir_test.go`
- **Details:**
  - `BuildFHIRVeterinaryBundle(assessment domain.TriageAssessment, passport *domain.VeterinaryPassport, clinic domain.VeterinaryClinic) (domain.FHIRVeterinaryBundle, error)`:
    - Generates `Patient` resource with `http://hl7.org/fhir/StructureDefinition/patient-animal` extension, `Encounter` (`class: "EMER"`), `Observation` entries for vitals (`HeartRateBPM`, `RespiratoryRateBPM`, `TemperatureF`, `CapillaryRefillSec`, `MucousMembrane`, `GlasgowComaScale`, `WeightKg`), `AllergyIntolerance` entries, `Immunization` entries, and `MedicationAdministration` entries.
    - Computes deterministic SHA-256 `BundleDigestHex` over the canonical entries JSON.
  - `ValidateFHIRVeterinaryBundle(bundle domain.FHIRVeterinaryBundle) error`: verifies `ResourceType == "Bundle"`, non-empty `Entry`, `Patient` animal extension presence, and SHA-256 digest integrity.
  - `BuildPIMSIntakeRecord(...) domain.PIMSIntakeRecord` and `FormatPIMSRecordCSV(records []domain.PIMSIntakeRecord) (string, error)`.

### Task 3: WebFrontend REST Endpoints, Outbound Pre-Arrival Handoff & Inbound PIMS Webhook (`internal/app/webfrontend/veterinary_fhir_handlers.go`)
- **Files:**
  - Create `internal/app/webfrontend/veterinary_fhir_handlers.go`
  - Create `internal/app/webfrontend/veterinary_fhir_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
- **Details:**
  - Register routes in `server.go`:
    - `GET /veterinary/fhir` -> `s.handleVeterinaryFHIRPage`
    - `GET /api/v1/veterinary/clinics` -> `s.handleListVeterinaryClinics`
    - `POST /api/v1/veterinary/clinics` -> `s.handleCreateVeterinaryClinic`
    - `POST /api/v1/veterinary/handoffs` -> `s.handleCreateClinicalHandoff`
    - `GET /api/v1/veterinary/handoffs` -> `s.handleListClinicalHandoffs`
    - `GET /api/v1/veterinary/handoffs/{id}/fhir` -> `s.handleExportClinicalHandoffFHIR`
    - `POST /api/v1/veterinary/handoffs/{id}/pims-webhook` -> `s.handleInboundPIMSWebhook`
  - Seed 3 default 24/7 Emergency Veterinary Clinics (`IDEXX_CORNERSTONE`, `EZYVET`, `SHELTERLUV`) on first clinic list/lookup if empty.
  - Broadcast `clinical_handoff_dispatched` and `clinical_handoff_updated` on SSE (`"triage"` topic) and WebRTC mesh (`s.signalingHub`).

### Task 4: Triage Cockpit Handoff Integration, FHIR Inspector Portal & WCAG AAA CSS
- **Files:**
  - Create `internal/app/webfrontend/templates/veterinary_fhir.html`
  - Modify `internal/app/webfrontend/templates/triage.html`
  - Modify `internal/app/webfrontend/static/js/veterinary-triage.js`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/veterinary-fhir.spec.ts`
- **Details:**
  - Add a link/button in `/triage` header & patient treatment panel to dispatch a Pre-Arrival ER Handoff or open `/veterinary/fhir`.
  - Build `/veterinary/fhir` (`templates/veterinary_fhir.html`) featuring:
    - Receiving ER Clinic Directory & Registration (`#clinic-register-form`, `#clinics-list`)
    - Pre-Arrival ER Handoff Dispatcher (`#form-dispatch-handoff`)
    - Active Clinical Handoffs & FHIR R4 / PIMS Inspector (`#handoffs-list`, `#fhir-bundle-viewer`, `#btn-download-pims-csv`)
    - Inbound PIMS Stabilization & Discharge Webhook Simulator (`#form-pims-webhook-sim`)
  - Enforce WCAG AAA (`> 7:1`) contrast for handoff status badges (`.badge-handoff-enroute`, `.badge-handoff-treatment`, `.badge-handoff-stabilized`, `.badge-handoff-cleared`) across light and dark modes.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/veterinary-fhir-journey.spec.ts`
- **Details:**
  - 6-step E2E user journey verifying passport + triage creation, outbound Pre-Arrival ER Handoff dispatch, FHIR R4 `Bundle` inspection (`patient-animal` extension, `AllergyIntolerance`, `Observation`, `MedicationAdministration`), PIMS CSV export verification, inbound PIMS webhook updating status to `CLEARED_FOR_REUNIFICATION` (`readyForReunion: true`), and WCAG AAA contrast compliance.

### Task 6: Final Whole-Branch Code Review & Verification Gate
- **Details:**
  - Run whole-branch review, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass, push, open PR, and squash-merge.
