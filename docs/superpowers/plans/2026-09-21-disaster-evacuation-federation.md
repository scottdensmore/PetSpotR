# Milestone 10.3: Municipal & Multi-Agency Disaster Evacuation Federation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a municipal and multi-agency disaster evacuation federation system in PetSpotR featuring dynamic crisis shelter and evacuation hub capacity management, bulk CSV/JSON pet roster intake with automated microchip reconciliation, an immutable cross-agency mutual aid transfer ledger, a prioritized disaster reunification queue, and an interactive operations dashboard at `/evacuation`.

**Architecture:**
- `pkg/domain`: Domain models for evacuation hubs, transfer manifests, crisis reunification queue items, and bulk intake results.
- `pkg/store`: Store collection constants for `evacuation_hubs`, `transfer_manifests`, `crisis_intakes`, and `crisis_reunifications`.
- `pkg/sheltersync`: Streaming bulk CSV/JSON roster parser with flexible column mapping, microchip normalization (`pkg/microchip`), and batch summary generation.
- `internal/app/webfrontend`: REST handlers for hub management, batch intake, transfer custody lifecycle, and crisis reunification dispatch, plus HTML view at `/evacuation`.
- `internal/app/webfrontend/static/js`, `templates`, `styles.css`: Tabbed emergency operations dashboard with metrics strip, hub capacity progress bars, drag-and-drop CSV importer, transfer ledger, and crisis match queue.
- `tests/playwright/e2e`: Automated end-to-end journey tests covering bulk intake, microchip reconciliation, transfer manifests, and crisis owner dispatch.

**Tech Stack:** Go 1.26 (pure standard library, no CGO), HTML5 drag-and-drop, vanilla JavaScript, Playwright TypeScript test runner.

**Spec:** [`docs/superpowers/specs/2026-09-21-disaster-evacuation-federation-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-21-disaster-evacuation-federation-design.md)

## Global Constraints
- Pure Go standard library for all processing (zero CGO, zero external GIS/parser dependencies).
- Toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Full backward compatibility with existing shelter and pet record endpoints.
- Strict CSP compliance: Zero inline `<script>` tags, zero `eval()`.
- WCAG AAA compliance across all new dashboard UI elements and status badges.
- Zero lint issues (`make verify`) and 100% passing Playwright test suite.

---

### Task 1: Domain Models, Store Constants & Pure Go Bulk Intake Engine (`pkg/domain`, `pkg/store`, `pkg/sheltersync`)

**Files:**
- Create: `pkg/domain/evacuation.go`
- Modify: `pkg/store/store.go`
- Create: `pkg/sheltersync/bulk_intake.go`
- Create: `pkg/sheltersync/bulk_intake_test.go`

**Interfaces:**
- Consumes: `domain.LocationPoint`, `domain.FoundPetRecord`, `domain.LostPetRecord`, `microchip.ValidateAndNormalize`
- Produces: `domain.EvacuationHub`, `domain.TransferManifest`, `domain.CrisisReunificationItem`, `sheltersync.ParseBulkIntakeCSV(r io.Reader, hubID string, hubName string) (BulkIntakeBatchSummary, []domain.FoundPetRecord, error)`, `sheltersync.ParseBulkIntakeJSON(data []byte, hubID string, hubName string) (BulkIntakeBatchSummary, []domain.FoundPetRecord, error)`

- [ ] **Step 1: Write failing tests in `pkg/sheltersync/bulk_intake_test.go`**
```go
package sheltersync

import (
	"strings"
	"testing"
)

func TestParseBulkIntakeCSV(t *testing.T) {
	csvData := `species,breed,primary_color,gender,microchip_id,address,notes
dog,Golden Retriever,Golden,male,985141000123456,"2061 15th Ave W, Seattle, WA","Found near Interbay"
cat,Domestic Shorthair,Tabby,female,456789123,"Mercer St, Seattle, WA","Scanned Avid chip"
dog,Poodle,White,male,invalid-chip,"Rainier Ave, Seattle, WA","Triage check ok"
`
	summary, records, err := ParseBulkIntakeCSV(strings.NewReader(csvData), "hub-test-1", "Test Crisis Center")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.TotalProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 3 {
		t.Errorf("expected 3 ingested, got %d", summary.IngestedCount)
	}
	if summary.MicrochipCount != 2 {
		t.Errorf("expected 2 valid microchips, got %d", summary.MicrochipCount)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 found pet records, got %d", len(records))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/sheltersync/... -run TestParseBulkIntakeCSV`
Expected: Compilation failure due to missing types and functions.

- [ ] **Step 3: Implement domain types and store constants**
Create `pkg/domain/evacuation.go` with `EvacuationHub`, `TransferManifest`, `CrisisReunificationItem`, and related enums.
In `pkg/store/store.go`, add `EvacuationHubsCollection`, `TransferManifestsCollection`, `CrisisIntakesCollection`, `CrisisReunificationsCollection`.

- [ ] **Step 4: Implement bulk intake engine in `pkg/sheltersync/bulk_intake.go`**
Implement CSV/JSON parser with header normalization, microchip validation via `pkg/microchip`, and `FoundPetRecord` translation.

- [ ] **Step 5: Run tests to verify they pass**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/sheltersync/...`
Expected: PASS.

- [ ] **Step 6: Commit**
```bash
git add pkg/domain/ pkg/store/ pkg/sheltersync/
git commit -m "feat(evacuation): add domain models, store constants, and bulk intake engine"
```

---

### Task 2: Backend REST Endpoints & Mutual Aid Transfer Ledger (`internal/app/webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/evacuation.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/evacuation_test.go`

**Interfaces:**
- Consumes: `sheltersync.ParseBulkIntakeCSV`, `sheltersync.ParseBulkIntakeJSON`, `store.StateStore`, `microchip.ValidateAndNormalize`
- Produces: `GET /api/v1/evacuations/hubs`, `POST /api/v1/evacuations/hubs`, `POST /api/v1/evacuations/intake-batch`, `GET /api/v1/evacuations/transfers`, `POST /api/v1/evacuations/transfers`, `PUT /api/v1/evacuations/transfers/{id}/status`, `GET /api/v1/evacuations/reunification-queue`, `POST /api/v1/evacuations/reunification-queue/{matchId}/contact`

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/evacuation_test.go`**
Test hub creation, batch intake with instant microchip match staging, transfer manifest creation and status progression (`STAGED` -> `IN_TRANSIT` -> `RECEIVED`), and crisis reunification contact dispatch.

- [ ] **Step 2: Run tests to verify they fail**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... -run Evacuation`
Expected: FAIL due to missing handlers.

- [ ] **Step 3: Implement handlers in `internal/app/webfrontend/evacuation.go` and register in `server.go`**
Implement all REST handlers and wire default Seattle evacuation hubs (e.g. Seattle Exhibition Center, Magnuson Community Center).

- [ ] **Step 4: Run tests to verify they pass**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend/... -run Evacuation`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/app/webfrontend/
git commit -m "feat(webfrontend): add disaster evacuation federation REST endpoints and transfer ledger"
```

---

### Task 3: Emergency Operations Dashboard Template & Glassmorphic CSS (`evacuation.html`, `layout.html`, `styles.css`)

**Files:**
- Create: `internal/app/webfrontend/templates/evacuation.html`
- Modify: `internal/app/webfrontend/templates/layout.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`

**Interfaces:**
- Produces: `/evacuation` HTML route, emergency navigation badge, responsive glassmorphic tab panels, metrics HUD, and accessible tables.

- [ ] **Step 1: Create `templates/evacuation.html`**
Author operations dashboard template containing metrics HUD (`#metric-evacuated-total`, etc.), tab navigation (`role="tablist"`), and panels for Facilities, Bulk Intake, Mutual Aid Transfers, and Crisis Reunifications.

- [ ] **Step 2: Update `templates/layout.html`**
Add navigation link to `/evacuation` in the primary header with an emergency beacon indicator (`🚨 Disaster Hub`).

- [ ] **Step 3: Add styles in `static/css/styles.css`**
Author styling for `.evacuation-dashboard`, `.metrics-hud`, `.hub-card`, `.progress-bar-occupancy`, `.intake-dropzone`, `.transfer-ledger-table`, `.badge-transfer-*`, and `.crisis-match-card` ensuring WCAG AAA compliance.

- [ ] **Step 4: Verify and Commit**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
```bash
git add internal/app/webfrontend/templates/ internal/app/webfrontend/static/css/
git commit -m "feat(ui): add disaster evacuation dashboard template, navigation link, and emergency styles"
```

---

### Task 4: Client Dashboard Controller & Batch Upload Engine (`evacuation.js`)

**Files:**
- Create: `internal/app/webfrontend/static/js/evacuation.js`
- Modify: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: REST endpoints `/api/v1/evacuations/*`
- Produces: Interactive client dashboard, tab navigation, drag-and-drop CSV batch upload, real-time results table rendering, transfer status updates, and owner notification triggers.

- [ ] **Step 1: Implement `internal/app/webfrontend/static/js/evacuation.js`**
Author client controller managing tabs, drag-and-drop file ingestion, batch submission, transfer manifest lifecycle actions, and crisis contact triggers.

- [ ] **Step 2: Add static asset test in `server_test.go`**
Verify `/evacuation` renders with HTTP 200, contains expected metric element IDs, and loads `evacuation.js`.

- [ ] **Step 3: Run tests to verify they pass**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS.

- [ ] **Step 4: Commit**
```bash
git add internal/app/webfrontend/static/js/ internal/app/webfrontend/server_test.go
git commit -m "feat(frontend): add evacuation dashboard controller, CSV dropzone, and transfer actions"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/disaster-evacuation-journey.spec.ts`

**Interfaces:**
- End-to-end user journey test verifying:
  1. Register lost dog with known microchip ID.
  2. Open `/evacuation` dashboard and inspect operational metrics.
  3. Upload CSV batch with 3 animals (one matching the registered lost dog's microchip).
  4. Verify batch results table shows valid microchips and instant match alert.
  5. Open Crisis Reunifications tab, verify matched pair, and trigger owner alert.
  6. Open Mutual Aid Transfers tab, stage a transfer to a secondary hub, transition status to In Transit and Received, and verify capacity updates.

- [ ] **Step 1: Author `tests/playwright/e2e/disaster-evacuation-journey.spec.ts`**
Author Playwright test using web-first assertions without brittle timeouts.

- [ ] **Step 2: Run new journey test**
Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/disaster-evacuation-journey.spec.ts`
Expected: 1/1 passed.

- [ ] **Step 3: Run full Playwright test suite**
Run: `cd tests/playwright && npx playwright test`
Expected: 124/124 passed (100%).

- [ ] **Step 4: Run full repository verification**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: 0 lint issues, OpenTofu valid, yamllint clean, race tests pass.

- [ ] **Step 5: Commit**
```bash
git add tests/playwright/e2e/disaster-evacuation-journey.spec.ts
git commit -m "test(playwright): add E2E user journey tests for disaster evacuation federation and batch intake"
```

---

### Task 6: Final Whole-Branch Code Review, PR Creation & Squash Merge

- [ ] **Step 1: Perform whole-branch code review with `pro` model**
Review complete git diff against spec requirements, concurrency safety, and performance constraints. Address any findings.

- [ ] **Step 2: Push branch and create Pull Request**
Create PR targeting `main`.

- [ ] **Step 3: Squash-merge PR and sync `main`**
Merge PR, delete feature branch, remove worktree, pull `main`, and re-run verification.
