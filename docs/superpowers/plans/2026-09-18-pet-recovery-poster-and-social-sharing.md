# Pet Recovery Poster Generator & Dynamic Social Sharing Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide lost pet owners with an instant, printable 8.5" x 11" recovery flyer with scannable tear-off tabs and dynamic QR codes, a server-rendered OpenGraph social card for viral neighborhood network sharing, and a streamlined mobile landing page for QR scanners to immediately report found pets or send messages.

**Architecture:** A pure Go vector QR engine generates crisp SVG QR codes pointing to a shortlink (`/p/:id`). A server-side OpenGraph renderer produces 1200x630 social preview SVG cards. A client-side CSS Paged Media print engine renders 8.5x11 printable flyers with single-page guarantees, and a mobile-optimized finder landing page enables 1-tap photo matching and mediated contact.

**Tech Stack:** Go 1.26.5, CSS Paged Media (`@page`, `@media print`), Web Share API (`navigator.share()`), HTML5 Geolocation, Playwright E2E test suite.

**Spec:** `docs/superpowers/specs/2026-09-18-pet-recovery-poster-and-social-sharing-design.md`

## Global Constraints

- Pinned Go toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Zero external client-side NPM dependencies; pure vanilla JavaScript with standard browser APIs.
- Strict Content Security Policy (CSP): `script-src 'self'`, `connect-src 'self' https://storage.petspotr.io`, `img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org`.
- Zero-PII by default: Flyers, QR codes, and landing pages identify participants strictly via PetSpotR mediated channels; direct phone numbers on tabs are strictly opt-in by the pet owner.
- Single-page physical print guarantee: All print styles must constrain to exactly 1 sheet of 8.5" x 11" Letter paper without overflow (`page-break-inside: avoid; break-inside: avoid;`).
- All checks in `make verify` and Playwright tests must pass with 100% success.

---

### Task 1: Pure Go Vector QR Code Engine (`pkg/qrcode`)

**Files:**
- Create: `pkg/qrcode/qrcode.go`
- Create: `pkg/qrcode/svg.go`
- Test: `pkg/qrcode/qrcode_test.go`

**Interfaces:**
- Consumes: Standard Go standard library (`bytes`, `fmt`, `image`).
- Produces: `GenerateSVG(content string, size int, margin int) ([]byte, error)` producing valid SVG XML containing `<path d="M..."/>`.

- [ ] **Step 1: Write failing tests in `pkg/qrcode/qrcode_test.go`**

```go
package qrcode_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/qrcode"
)

func TestGenerateSVG(t *testing.T) {
	t.Parallel()

	t.Run("generates valid SVG for standard URL", func(t *testing.T) {
		url := "https://petspotr.io/p/lost-123"
		svgBytes, err := qrcode.GenerateSVG(url, 256, 2)
		if err != nil {
			t.Fatalf("unexpected error generating SVG: %v", err)
		}
		svg := string(svgBytes)
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(strings.TrimSpace(svg), "</svg>") {
			t.Fatalf("expected valid svg root tag, got:\n%s", svg)
		}
		if !strings.Contains(svg, `viewBox="0 0`) {
			t.Fatal("expected viewBox attribute in SVG")
		}
		if !strings.Contains(svg, `<path`) {
			t.Fatal("expected path element in SVG")
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		_, err := qrcode.GenerateSVG("", 256, 2)
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})

	t.Run("enforces minimum dimensions", func(t *testing.T) {
		svgBytes, err := qrcode.GenerateSVG("https://petspotr.io/p/test", 16, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(svgBytes) == 0 {
			t.Fatal("expected non-empty svg output")
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/qrcode/...`
Expected: Compile failure / package `pkg/qrcode` does not exist.

- [ ] **Step 3: Implement Pure Go Vector QR Engine in `pkg/qrcode`**

1. Create `pkg/qrcode/qrcode.go`:
   - Implement QR matrix generation with Error Correction Level M.
   - Encode byte-mode payload for HTTPS URLs.
2. Create `pkg/qrcode/svg.go`:
   - Construct clean SVG path representation: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 W H" shape-rendering="crispEdges">...`.
   - Add white background `<rect width="100%" height="100%" fill="#ffffff"/>` and dark foreground `<path d="..." fill="#111827"/>`.
   - Export `GenerateSVG(content string, size int, margin int) ([]byte, error)`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/qrcode/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/qrcode/
git commit -m "feat(qrcode): implement pure go vector qr code svg generator"
```

---

### Task 2: Backend REST & Vector Image Endpoints in WebFrontend

**Files:**
- Create: `internal/app/webfrontend/poster_endpoints.go`
- Modify: `internal/app/webfrontend/server.go`
- Test: `internal/app/webfrontend/poster_endpoints_test.go`

**Interfaces:**
- Consumes: `pkg/qrcode`, `store.StateStore`, `store.LostPetsCollection`.
- Produces:
  - `GET /api/v1/pets/:id/qr.svg`: vector QR code.
  - `GET /api/v1/pets/:id/share-card.svg`: 1200x630 OpenGraph SVG card.

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/poster_endpoints_test.go`**

```go
package webfrontend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestPosterEndpoints(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	pet := domain.LostPetRecord{
		PetID:        "lost-pet-404",
		PetName:      "Rusty",
		Species:      domain.SpeciesDog,
		Breed:        "Golden Retriever",
		PrimaryColor: "Golden",
		LastSeenLocation: domain.Location{
			Address:   "Capitol Hill, Seattle, WA",
			Latitude:  47.625,
			Longitude: -122.320,
		},
		LastSeenTime: time.Now().UTC().Add(-24 * time.Hour),
		ImageURL:     "images/lost/rusty.jpg",
	}
	data, _ := json.Marshal(pet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, pet.PetID, data)

	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	t.Run("GET /api/v1/pets/:id/qr.svg returns 200 and image/svg+xml", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/qr.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Fatalf("expected Content-Type image/svg+xml, got %q", ct)
		}
		if !strings.Contains(w.Body.String(), "<svg") {
			t.Fatal("expected SVG body")
		}
	})

	t.Run("GET /api/v1/pets/:id/share-card.svg returns 200 and 1200x630 card", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/share-card.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Fatalf("expected Content-Type image/svg+xml, got %q", ct)
		}
		body := w.Body.String()
		if !strings.Contains(body, `viewBox="0 0 1200 630"`) {
			t.Fatal("expected 1200x630 viewBox in social card")
		}
		if !strings.Contains(body, "Rusty") {
			t.Fatal("expected pet name Rusty in social card")
		}
	})

	t.Run("Returns 404 for nonexistent pet", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/unknown-pet/qr.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 NotFound, got %d", w.Code)
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestPosterEndpoints ./internal/app/webfrontend/...`
Expected: FAIL / 404 on unhandled endpoints.

- [ ] **Step 3: Implement `internal/app/webfrontend/poster_endpoints.go` & Register Routes in `server.go`**

1. Create `internal/app/webfrontend/poster_endpoints.go`:
   - `handleApiPetQR(w, r)`: extracts pet ID, loads record from `store.LostPetsCollection`, generates SVG via `qrcode.GenerateSVG`, sets `Cache-Control: public, max-age=86400, immutable`, and writes response.
   - `handleApiPetShareCard(w, r)`: extracts pet ID, loads record, constructs dynamic 1200x630 SVG with pet photo mask, name, breed, location, and reward badge, sets `Cache-Control: public, max-age=3600`, and writes response.
2. Register routes in `server.go`:
   - `s.mux.HandleFunc("/api/v1/pets/{petID}/qr.svg", s.handleApiPetQR)`
   - `s.mux.HandleFunc("/api/v1/pets/{petID}/share-card.svg", s.handleApiPetShareCard)`

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestPosterEndpoints ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/poster_endpoints.go internal/app/webfrontend/poster_endpoints_test.go internal/app/webfrontend/server.go
git commit -m "feat(webfrontend): add vector qr code and dynamic social card svg endpoints"
```

---

### Task 3: Printable Flyer Template & CSS Paged Media (`@media print`)

**Files:**
- Create: `internal/app/webfrontend/templates/poster.html`
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/server.go`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `domain.LostPetRecord`, CSS Paged Media.
- Produces:
  - Route `GET /pets/:id/poster`: Standalone printable flyer view.
  - `#poster-modal` in `pets.html` with live flyer preview and customization inputs.
  - Single-sheet print layout in `styles.css`.

- [ ] **Step 1: Write failing test in `internal/app/webfrontend/server_test.go`**

```go
func TestPrintablePosterPage(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	req := httptest.NewRequest(http.MethodGet, "/pets/demo-lost-1/poster", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	body := w.Body.String()
	snippets := []string{
		`class="printable-poster"`,
		`class="poster-header"`,
		`class="poster-tear-tabs"`,
		`qr.svg`,
	}
	for _, snippet := range snippets {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected %q in rendered poster HTML", snippet)
		}
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestPrintablePosterPage ./internal/app/webfrontend/...`
Expected: FAIL / 404.

- [ ] **Step 3: Implement `poster.html`, `pets.html`, `styles.css`, and handler in `server.go`**

1. Create `internal/app/webfrontend/templates/poster.html`:
   - Letter-size container `.printable-poster`.
   - Alert banner `.poster-header` (`★ LOST DOG ★` / `★ LOST CAT ★`).
   - Image container `.poster-photo`.
   - Pet identification block `.poster-details`.
   - Urgent callouts `.poster-reward-banner` and `.poster-emergency-note`.
   - Primary vector QR code `<img>` linking to `/p/{{.PetID}}`.
   - Tear-off tabs container `.poster-tear-tabs` with 10 vertical tabs (`.tear-tab`), micro QR codes, and scissors indicators.
2. In `pets.html`:
   - Add `#poster-modal` with customization inputs (Reward amount, Emergency instructions, Tab phone number mode) and side-by-side scaled preview.
   - Add **"📄 Poster & Share"** action button to lost pet card controls.
3. In `styles.css`:
   - Add `@page { size: letter portrait; margin: 0.35in; }`.
   - Add `@media print` rules enforcing single-page height budget, hiding site nav/footers, and setting `print-color-adjust: exact`.
   - Add vertical tab formatting (`writing-mode: vertical-rl; transform: rotate(180deg);`).
4. In `server.go`:
   - Add `s.mux.HandleFunc("/pets/{petID}/poster", s.handlePetPoster)`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestPrintablePosterPage ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/poster.html internal/app/webfrontend/templates/pets.html internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/server.go internal/app/webfrontend/server_test.go
git commit -m "feat(ui): add 8.5x11 printable recovery poster with tear-off tabs and print css"
```

---

### Task 4: Mobile Finder Quick-Action Landing View (`/p/{id}`)

**Files:**
- Create: `internal/app/webfrontend/templates/finder_landing.html`
- Modify: `internal/app/webfrontend/server.go`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `domain.LostPetRecord`.
- Produces: Route `GET /p/:id` rendering mobile quick-action page with OpenGraph social tags.

- [ ] **Step 1: Write failing test in `internal/app/webfrontend/server_test.go`**

```go
func TestFinderLandingPage(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	req := httptest.NewRequest(http.MethodGet, "/p/demo-lost-1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	body := w.Body.String()
	snippets := []string{
		`<meta property="og:image"`,
		`share-card.svg`,
		`id="btn-finder-found"`,
		`id="btn-finder-message"`,
		`id="btn-finder-sighting"`,
	}
	for _, snippet := range snippets {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected %q in rendered finder landing HTML", snippet)
		}
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestFinderLandingPage ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement `finder_landing.html` and register route in `server.go`**

1. Create `internal/app/webfrontend/templates/finder_landing.html`:
   - OpenGraph and Twitter card meta tags in `<head>` referencing `/api/v1/pets/{{.PetID}}/share-card.svg`.
   - Hero header with pet photo, name, status, reward callout, and last seen location.
   - Primary high-contrast CTA `#btn-finder-found` ("📷 I Have Found This Pet") with hidden camera file input.
   - Secondary CTA `#btn-finder-message` ("💬 Send Message to Owner").
   - Tertiary CTA `#btn-finder-sighting` ("📍 Report a Quick Sighting").
   - Quick social share links.
2. In `server.go`:
   - Add `s.mux.HandleFunc("/p/{petID}", s.handleFinderLanding)`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestFinderLandingPage ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/finder_landing.html internal/app/webfrontend/server.go internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/server_test.go
git commit -m "feat(webfrontend): add mobile finder quick-action landing page with opengraph social tags"
```

---

### Task 5: Client-Side Poster Customizer & Social Share Controller (`pet-share.js`)

**Files:**
- Create: `internal/app/webfrontend/static/js/pet-share.js`
- Modify: `internal/app/webfrontend/static/js/pet-directory.js`
- Test: JS syntax validation via `node -c`.

**Interfaces:**
- Consumes: Web Share API, Clipboard API, DOM elements.
- Produces: Interactive poster customization, live preview DOM synchronization, native share sheet, and print triggers.

- [ ] **Step 1: Implement `pet-share.js`**

1. `initPosterModal(petData)`:
   - Synchronizes reward input, emergency note input, and tab phone number mode with live preview in `#poster-modal`.
   - Formats tear-off tabs dynamically based on user selections.
2. `printPoster()`:
   - Triggers `window.print()`.
3. `sharePetAlert(petData)`:
   - Uses `navigator.share()` if available.
   - Fallback to `navigator.clipboard.writeText(shareUrl)` and display accessible toast.
   - Opens share popups for Nextdoor, Facebook, and X.

- [ ] **Step 2: Wire `pet-directory.js` to trigger `#poster-modal`**

- Wire **"📄 Poster & Share"** button on each lost pet card to populate and display `#poster-modal`.

- [ ] **Step 3: Validate JavaScript syntax**

Run: `node -c internal/app/webfrontend/static/js/pet-share.js && node -c internal/app/webfrontend/static/js/pet-directory.js`
Expected: 0 syntax errors.

- [ ] **Step 4: Commit**

```bash
git add internal/app/webfrontend/static/js/pet-share.js internal/app/webfrontend/static/js/pet-directory.js
git commit -m "feat(ui): implement client poster customization preview and web share controller"
```

---

### Task 6: Playwright E2E User Journey Tests & Full Verification

**Files:**
- Create: `tests/playwright/e2e/poster-social-share-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend.
- Produces: Automated verification of poster preview, customization, print stylesheet rules, QR navigation to `/p/:id`, and 1-tap photo reporting.

- [ ] **Step 1: Write `tests/playwright/e2e/poster-social-share-journey.spec.ts`**

Implement automated user journeys:
1. `should open poster customizer, update reward and emergency notes, and verify live preview`:
   - Navigates to `/pets`.
   - Clicks "📄 Poster & Share" on a lost pet card.
   - Types `$1,000` into reward input -> asserts preview updates.
   - Enters emergency note -> asserts preview updates.
   - Verifies print media styles declare `@page { size: letter portrait; }`.
2. `should navigate to mobile finder landing page via shortlink and render action controls`:
   - Navigates to `/p/demo-lost-1`.
   - Asserts OpenGraph tags (`og:image`, `og:title`).
   - Asserts primary CTAs (`#btn-finder-found`, `#btn-finder-message`, `#btn-finder-sighting`) are visible and interactive.
3. `should copy share link with toast feedback when Web Share is unsupported`:
   - Clicks copy link button -> asserts toast notification "Link copied to clipboard!".

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/poster-social-share-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% passing (71+ tests).

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (all linters, OpenTofu, race detector clean).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/poster-social-share-journey.spec.ts
git commit -m "test(e2e): add end-to-end user journey tests for poster generator and social sharing"
```
