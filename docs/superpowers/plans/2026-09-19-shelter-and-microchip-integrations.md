# Shelter & Microchip Registry Integrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement universal microchip transponder validation, registry lookup attribution, deterministic 100% match interception, shelter intake feed ingestion (`POST /api/v1/shelter-intakes/ingest` and background sync), and shelter reclaim UX with privacy-preserving masking.

**Architecture:** Pure Go microchip engine (`pkg/microchip`) validates ISO 15-digit, Avid 9-digit, and Euro 10-digit RFID transponders and routes ICAR prefixes to issuing registries. Domain aggregates are extended with microchip and shelter custody metadata. The scoring engine intercepts identical microchips to fix an exact 1.0 deterministic match score. A shelter intake ingestion pipeline converts shelter feeds into found pet records with `CustodyStatus: "Shelter Care"`, triggering real-time owner alerts. Frontend wizards and match dashboards expose live format validation, privacy-masked chip badges, and direct shelter contact actions.

**Tech Stack:** Go 1.26.5, standard library `html/template`, `crypto/rand`, Playwright E2E test suite, Google Cloud Pub/Sub event pipeline.

**Spec:** `docs/superpowers/specs/2026-09-19-shelter-and-microchip-integrations-design.md`

## Global Constraints

- Pinned Go toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Zero-PII wire contract: Raw microchip numbers are never exposed in public DTOs or public HTML views; masked as `HomeAgain (••••3456)` or `ISO (••••3456)`.
- Zero client NPM dependencies: Pure vanilla ES2020+ JavaScript for wizard validation and match dashboard interactions.
- Strict Content Security Policy: `script-src 'self'`.
- 100% test pass rate across unit test suites, `make verify`, and Playwright E2E journeys.

---

### Task 1: Pure Go Microchip Engine & Registry Lookup Architecture (`pkg/microchip`)

**Files:**
- Create: `pkg/microchip/microchip.go`
- Create: `pkg/microchip/registry.go`
- Create: `pkg/microchip/client.go`
- Create: `pkg/microchip/microchip_test.go`

**Interfaces:**
- Produces:
  - `ValidateAndNormalize(raw string) ValidationResult`
  - `IdentifyIssuingRegistry(normalizedID string) RegistryInfo`
  - `MaskMicrochip(raw string) string`
  - `RegistryLookupClient` interface and `NewMockRegistryLookupClient() RegistryLookupClient`

- [ ] **Step 1: Write failing tests in `pkg/microchip/microchip_test.go`**

```go
package microchip_test

import (
	"context"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/microchip"
)

func TestValidateAndNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantValid    bool
		wantStandard microchip.Standard
		wantNormalized string
	}{
		{
			name:         "Valid ISO 15-digit FDX-B",
			input:        " 985141000123456 ",
			wantValid:    true,
			wantStandard: microchip.StandardISO15,
			wantNormalized: "985141000123456",
		},
		{
			name:         "Valid Avid 9-digit with asterisks",
			input:        "123*456*789",
			wantValid:    true,
			wantStandard: microchip.StandardAvid9,
			wantNormalized: "123456789",
		},
		{
			name:         "Valid Euro 10-digit alphanumeric",
			input:        "00064a12b3",
			wantValid:    true,
			wantStandard: microchip.StandardEuro10,
			wantNormalized: "00064A12B3",
		},
		{
			name:         "Invalid length",
			input:        "12345",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:         "Invalid characters in ISO",
			input:        "98514100012345X",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := microchip.ValidateAndNormalize(tt.input)
			if res.Valid != tt.wantValid {
				t.Fatalf("expected valid=%v, got %v (err=%s)", tt.wantValid, res.Valid, res.ErrorMessage)
			}
			if res.Standard != tt.wantStandard {
				t.Errorf("expected standard=%v, got %v", tt.wantStandard, res.Standard)
			}
			if tt.wantValid && res.NormalizedID != tt.wantNormalized {
				t.Errorf("expected normalized=%q, got %q", tt.wantNormalized, res.NormalizedID)
			}
		})
	}
}

func TestIdentifyIssuingRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip         string
		wantRegistry string
	}{
		{"985141000123456", "HomeAgain"},
		{"981010000123456", "AKC Reunite"},
		{"977200000123456", "PetLink"},
		{"982000000123456", "24Petwatch"},
		{"965000000123456", "BuddyID"},
		{"123456789", "Avid Identification Systems"},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			reg := microchip.IdentifyIssuingRegistry(tt.chip)
			if reg.RegistryName != tt.wantRegistry {
				t.Errorf("expected registry %q, got %q", tt.wantRegistry, reg.RegistryName)
			}
		})
	}
}

func TestMaskMicrochip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip     string
		wantMask string
	}{
		{"985141000123456", "HomeAgain ••••3456"},
		{"123456789", "Avid ••••6789"},
		{"00064A12B3", "Euro ••••12B3"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			got := microchip.MaskMicrochip(tt.chip)
			if got != tt.wantMask {
				t.Errorf("expected mask %q, got %q", tt.wantMask, got)
			}
		})
	}
}

func TestMockRegistryLookupClient(t *testing.T) {
	t.Parallel()

	client := microchip.NewMockRegistryLookupClient()
	res, err := client.Lookup(context.Background(), "985141000123456")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if res.Registry.RegistryName != "HomeAgain" {
		t.Errorf("expected HomeAgain, got %s", res.Registry.RegistryName)
	}
	if res.Status != "registered" {
		t.Errorf("expected registered status, got %s", res.Status)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/microchip/...`
Expected: FAIL (package does not exist yet).

- [ ] **Step 3: Implement `pkg/microchip/microchip.go`, `registry.go`, and `client.go`**

1. `microchip.go`: Define `Standard` constants, `ValidationResult`, and `ValidateAndNormalize(raw string)` using regex / string inspection.
2. `registry.go`: Define `RegistryInfo`, ICAR prefix map, `IdentifyIssuingRegistry(normalizedID string)`, and `MaskMicrochip(raw string) string`.
3. `client.go`: Define `RegistryLookupResult`, `RegistryLookupClient`, and `NewMockRegistryLookupClient()`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/microchip/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/microchip
git commit -m "feat(microchip): implement transponder validation, registry router, and masking"
```

---

### Task 2: Domain Model Extensions & Integration Payloads (`pkg/domain`)

**Files:**
- Modify: `pkg/domain/lost_report.go`
- Modify: `pkg/domain/found_report.go`
- Modify: `pkg/domain/match.go`
- Test: `pkg/domain/microchip_domain_test.go`

**Interfaces:**
- Consumes: `pkg/microchip`.
- Produces: Extended `LostPetRecord`, `FoundPetRecord`, `MatchRecord`, and DTO projections with `MicrochipID`, `MicrochipRegistry`, `ShelterID`, `ShelterName`, `IntakeID`, `CustodyStatus`, and `DeterministicMatch`.

- [ ] **Step 1: Write failing tests in `pkg/domain/microchip_domain_test.go`**

```go
package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMicrochipDomainExtensions(t *testing.T) {
	t.Parallel()

	lost := domain.LostPetRecord{
		PetID:             "lost-pet-1",
		PetName:           "Max",
		Species:           domain.SpeciesDog,
		MicrochipID:       "985141000123456",
		MicrochipRegistry: "HomeAgain",
	}

	data, err := json.Marshal(lost)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var roundtrip domain.LostPetRecord
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if roundtrip.MicrochipID != "985141000123456" || roundtrip.MicrochipRegistry != "HomeAgain" {
		t.Errorf("microchip fields not preserved in LostPetRecord")
	}

	found := domain.FoundPetRecord{
		PetID:             "found-shelter-sea-01-99",
		Species:           domain.SpeciesDog,
		MicrochipID:       "985141000123456",
		MicrochipRegistry: "HomeAgain",
		ShelterID:         "shelter-sea-01",
		ShelterName:       "Seattle Animal Shelter",
		IntakeID:          "INT-2026-8819",
		CustodyStatus:     "Shelter Care",
	}

	foundData, _ := json.Marshal(found)
	var foundRoundtrip domain.FoundPetRecord
	_ = json.Unmarshal(foundData, &foundRoundtrip)

	if foundRoundtrip.ShelterName != "Seattle Animal Shelter" || foundRoundtrip.IntakeID != "INT-2026-8819" {
		t.Errorf("shelter fields not preserved in FoundPetRecord")
	}

	match := domain.MatchRecord{
		MatchID:            "match-1",
		LostPetID:          lost.PetID,
		FoundPetID:         found.PetID,
		OverallScore:       1.0,
		DeterministicMatch: true,
		MatchType:          "deterministic_microchip",
		MatchedMicrochip:   "HomeAgain ••••3456",
	}

	matchData, _ := json.Marshal(match)
	var matchRoundtrip domain.MatchRecord
	_ = json.Unmarshal(matchData, &matchRoundtrip)

	if !matchRoundtrip.DeterministicMatch || matchRoundtrip.MatchType != "deterministic_microchip" {
		t.Errorf("deterministic match fields not preserved in MatchRecord")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestMicrochipDomainExtensions ./pkg/domain/...`
Expected: FAIL (fields not defined on structs).

- [ ] **Step 3: Update `pkg/domain/lost_report.go`, `found_report.go`, and `match.go`**

1. Add `MicrochipID` and `MicrochipRegistry` to `LostPetReport`, `LostPetRecord`, `PublicLostPetReport`.
2. Add `MicrochipID`, `MicrochipRegistry`, `ShelterID`, `ShelterName`, `IntakeID`, `CustodyStatus` to `FoundPetReport`, `FoundPetRecord`, `PublicFoundPetReport`.
3. Add `DeterministicMatch`, `MatchType`, `MatchedMicrochip` to `MatchRecord`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/domain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/lost_report.go pkg/domain/found_report.go pkg/domain/match.go pkg/domain/microchip_domain_test.go
git commit -m "feat(domain): add microchip and shelter custody models"
```

---

### Task 3: Deterministic Matching Interceptor in Scoring & Matcher (`pkg/scoring`, `internal/app/petmatcher`)

**Files:**
- Modify: `pkg/scoring/scorer.go`
- Modify: `pkg/scoring/scorer_test.go`
- Modify: `internal/app/petmatcher/matcher.go`
- Modify: `internal/app/petmatcher/matcher_test.go`

**Interfaces:**
- Consumes: `pkg/microchip`, `pkg/domain`.
- Produces: Deterministic match scoring function returning 1.0 on identical chips, 0.0 on conflicting chips, and triggering high-urgency notifications.

- [ ] **Step 1: Write failing tests in `pkg/scoring/scorer_test.go`**

```go
func TestCalculateDeterministicMatchScore(t *testing.T) {
	t.Parallel()

	t.Run("Identical microchips return score 1.0 and deterministic true", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985141000123456", "985141000123456")
		if !res.IsMatch || res.Score != 1.0 || !res.Deterministic {
			t.Fatalf("expected deterministic 1.0 match, got %+v", res)
		}
	})

	t.Run("Conflicting microchips return score 0.0 and mismatch true", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985141000123456", "981010000999999")
		if res.IsMatch || res.Score != 0.0 || !res.Mismatch {
			t.Fatalf("expected mismatch 0.0 penalty, got %+v", res)
		}
	})

	t.Run("Missing microchip in one or both delegates to probabilistic scoring", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985141000123456", "")
		if res.Deterministic || res.Mismatch {
			t.Fatalf("expected non-deterministic result when chip is missing, got %+v", res)
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestCalculateDeterministicMatchScore ./pkg/scoring/...`
Expected: FAIL.

- [ ] **Step 3: Implement microchip scoring interceptor in `pkg/scoring` & `pet-matcher`**

1. In `pkg/scoring/scorer.go`:
   - Implement `EvaluateMicrochipMatch(lostChip, foundChip string) MicrochipMatchResult`.
   - Update `CalculateHybridScore` (or candidate evaluator) to inspect microchips first: if matching, set `1.0` and `DeterministicMatch: true`.
2. In `internal/app/petmatcher/matcher.go`:
   - When saving `MatchRecord`, populate `DeterministicMatch` and `MatchType: "deterministic_microchip"`.
   - If deterministic match occurs, dispatch notification with urgency `"critical"` and type `"microchip_match"`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/scoring/... ./internal/app/petmatcher/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scoring internal/app/petmatcher
git commit -m "feat(scoring): implement deterministic microchip match interceptor"
```

---

### Task 4: Shelter Intake Ingestion Endpoint & Sync Poller (`pkg/sheltersync`, `webfrontend`)

**Files:**
- Create: `pkg/sheltersync/sheltersync.go`
- Create: `pkg/sheltersync/sheltersync_test.go`
- Create: `internal/app/webfrontend/shelter_endpoints.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/shelter_endpoints_test.go`

**Interfaces:**
- Consumes: `pkg/microchip`, `store.FoundPetsCollection`.
- Produces: Route `POST /api/v1/shelter-intakes/ingest` and background sync worker.

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/shelter_endpoints_test.go`**

```go
package webfrontend_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestShelterIntakeIngest(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	payload := map[string]interface{}{
		"shelterId":      "shelter-sea-01",
		"shelterName":    "Seattle Animal Shelter",
		"shelterAddress": "2061 15th Ave W, Seattle, WA 98119",
		"shelterPhone":   "(206) 386-7387",
		"intakeId":       "INT-2026-8819",
		"animal": map[string]interface{}{
			"species":     "dog",
			"breed":       "Golden Retriever",
			"description": "Found near Interbay",
			"microchipId": "985141000123456",
			"images": []map[string]string{
				{"url": "https://storage.petspotr.io/shelters/intake-8819.jpg", "view": "primary"},
			},
		},
		"location": map[string]interface{}{
			"address":   "Interbay, Seattle, WA",
			"latitude":  47.648,
			"longitude": -122.378,
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shelter-intakes/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 201 Created or 200 OK, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["intakeId"] != "INT-2026-8819" {
		t.Errorf("expected intakeId INT-2026-8819, got %v", resp["intakeId"])
	}
	if resp["custodyStatus"] != "Shelter Care" {
		t.Errorf("expected custodyStatus 'Shelter Care', got %v", resp["custodyStatus"])
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestShelterIntakeIngest ./internal/app/webfrontend/...`
Expected: FAIL / 404.

- [ ] **Step 3: Implement `pkg/sheltersync` and `internal/app/webfrontend/shelter_endpoints.go`**

1. `pkg/sheltersync`: Implement mock shelter intake generator and feed adapter.
2. `internal/app/webfrontend/shelter_endpoints.go`:
   - Implement `handleApiShelterIntakeIngest(w, r)`:
     - Parses JSON payload.
     - Validates and normalizes microchip via `microchip.ValidateAndNormalize`.
     - Creates `FoundPetRecord` with `CustodyStatus: "Shelter Care"` and shelter metadata.
     - Persists to `store.FoundPetsCollection`.
     - Publishes event and returns 201 Created.
3. In `server.go`: Register `s.mux.HandleFunc("/api/v1/shelter-intakes/ingest", s.handleApiShelterIntakeIngest)`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... ./pkg/sheltersync/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/sheltersync internal/app/webfrontend/shelter_endpoints.go internal/app/webfrontend/shelter_endpoints_test.go internal/app/webfrontend/server.go
git commit -m "feat(sheltersync): add shelter intake ingestion endpoint and sync adapter"
```

---

### Task 5: Frontend Wizards, Masked Card Badges & Shelter Reclaim UI (`webfrontend`)

**Files:**
- Modify: `internal/app/webfrontend/templates/report-lost.html`
- Modify: `internal/app/webfrontend/templates/report-found.html`
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/templates/matches.html`
- Modify: `internal/app/webfrontend/static/js/lost-wizard.js`
- Modify: `internal/app/webfrontend/static/js/found-report.js`
- Modify: `internal/app/webfrontend/static/js/match-dashboard.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `pkg/microchip`, DOM elements.
- Produces: Live validation for microchip inputs in report forms, privacy-masked chip badges on directory cards, `🎯 100% Verified Microchip Match` banner on `/matches`, and shelter reclaim contact buttons.

- [ ] **Step 1: Write failing test in `internal/app/webfrontend/server_test.go`**

```go
func TestMicrochipAndShelterUISnippets(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	// 1. Report Lost wizard contains microchip input
	reqLost := httptest.NewRequest(http.MethodGet, "/report-lost", nil)
	wLost := httptest.NewRecorder()
	srv.ServeHTTP(wLost, reqLost)
	if !strings.Contains(wLost.Body.String(), `id="lost-pet-microchip"`) {
		t.Errorf("expected id=\"lost-pet-microchip\" in /report-lost")
	}

	// 2. Report Found wizard contains microchip input
	reqFound := httptest.NewRequest(http.MethodGet, "/report-found", nil)
	wFound := httptest.NewRecorder()
	srv.ServeHTTP(wFound, reqFound)
	if !strings.Contains(wFound.Body.String(), `id="found-pet-microchip"`) {
		t.Errorf("expected id=\"found-pet-microchip\" in /report-found")
	}

	// 3. Matches page contains shelter alert & microchip badge styles
	reqMatches := httptest.NewRequest(http.MethodGet, "/matches", nil)
	wMatches := httptest.NewRecorder()
	srv.ServeHTTP(wMatches, reqMatches)
	if !strings.Contains(wMatches.Body.String(), `badge-microchip-match`) {
		t.Errorf("expected badge-microchip-match in /matches")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestMicrochipAndShelterUISnippets ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement UI updates and JavaScript controllers**

1. In `report-lost.html` & `report-found.html`:
   - Add `#lost-pet-microchip` / `#found-pet-microchip` inputs with validation badge container `#microchip-feedback`.
2. In `lost-wizard.js` & `found-report.js`:
   - Add client-side validator matching 15 digits (ISO), 9 digits (Avid), and 10 digits (Euro), displaying live registry badge (e.g., `✓ Valid ISO Microchip • HomeAgain`).
3. In `pets.html`:
   - Render masked microchip badge on cards: `{{ if .MicrochipID }}<span class="badge badge-microchip">🏷️ {{ .MicrochipRegistry }} (••••{{ slice .MicrochipID (minus (len .MicrochipID) 4) }})</span>{{ end }}`.
4. In `matches.html` & `match-dashboard.js`:
   - If `deterministicMatch`: render `🎯 100% Verified Microchip Match`.
   - If `custodyStatus == "Shelter Care"`: render shelter alert card with direct call button `tel:...` and directions link.
5. In `styles.css`:
   - Add styles for `.badge-microchip`, `.badge-microchip-match`, `.shelter-alert-card`, and `.btn-shelter-call`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestMicrochipAndShelterUISnippets ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates internal/app/webfrontend/static internal/app/webfrontend/server_test.go
git commit -m "feat(ui): add microchip entry validation, masked card badges, and shelter reclaim dashboard"
```

---

### Task 6: Playwright E2E User Journey Tests & Full Verification

**Files:**
- Create: `tests/playwright/e2e/shelter-microchip-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend and petmatcher.
- Produces: Automated verification of microchip registration, shelter intake ingestion, deterministic 100% match review, and full repository verification.

- [ ] **Step 1: Author `tests/playwright/e2e/shelter-microchip-journey.spec.ts`**

Implement automated user journeys:
1. `should register lost pet with microchip and display live format validation`:
   - Fills lost pet wizard with microchip `985141000123456`.
   - Asserts live feedback: "Valid 15-Digit ISO Microchip • HomeAgain".
   - Submits report; verifies masked chip badge on `/pets` (`HomeAgain (••••3456)`).
2. `should ingest shelter intake and trigger instant 100% deterministic microchip match`:
   - Calls `POST /api/v1/shelter-intakes/ingest` with matching microchip `985141000123456`.
   - Visits `/matches`, asserts `🎯 100% Verified Microchip Match`.
   - Asserts shelter name ("Seattle Animal Shelter"), intake ID ("INT-2026-8819"), and click-to-call button.
3. `should reject candidate match when microchips conflict`:
   - Ingests a found pet with a distinct valid microchip `981010000999999`.
   - Asserts that it is not presented as a match candidate for the lost pet.

- [ ] **Step 2: Run Playwright Journey Test**

Run:
```bash
cd tests/playwright && npx playwright test e2e/shelter-microchip-journey.spec.ts
```
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run:
```bash
cd tests/playwright && npx playwright test
```
Expected: 100% PASS across all spec files.

- [ ] **Step 4: Run Full Repository Verification**

Run:
```bash
export GOTOOLCHAIN=go1.26.5 && make verify
```
Expected: PASS (`go vet`, `golangci-lint`, OpenTofu, `yamllint`, race detector clean).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/shelter-microchip-journey.spec.ts
git commit -m "test(e2e): add end-to-end user journey tests for shelter intake and microchip matching"
```
