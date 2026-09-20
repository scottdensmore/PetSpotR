# Milestone 8.2: EXIF Geolocation Auto-Extraction & AI Image Quality Enhancer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a seamless, privacy-first imaging pipeline featuring a pure Go EXIF reader to automatically extract geographic and chronological data to pre-fill report fields, alongside an on-the-fly Image Quality Analyzer that evaluates underexposed photos and applies dynamic range normalization and orientation corrections.

**Architecture:** The `pkg/imaging` package handles pure Go memory-safe parsing and image manipulation. `internal/app/webfrontend` exposes rate-limited REST endpoints for metadata extraction and image enhancement without permanent storage of temporary processing bytes. Client-side JS integrates a dropzone with EXIF detection prompts and an auto-enhance slider UI under strict CSP.

**Tech Stack:** Go 1.26.5 (`export GOTOOLCHAIN=go1.26.5`), Playwright E2E testing framework, Vanilla JS/CSS.

**Spec:** `docs/superpowers/specs/2026-09-19-exif-geolocation-and-image-enhancer-design.md`

## Global Constraints

- Go toolchain: Pinned to `export GOTOOLCHAIN=go1.26.5`.
- Content Security Policy compliance: `script-src 'self'` (no inline scripts, no eval).
- Zero-PII wire contract: Strip camera serial/hardware IDs from EXIF, explicit user confirmation for location.
- Memory/CPU limits: Enforce strict boundaries on parsed image dimensions and buffer sizes.
- 100% verification pass rate across `make verify` and all Playwright test specifications before completion.
- User Review Required: None (standard SDD automation).

---

### Task 1: Pure Go EXIF Metadata Extraction Engine (`pkg/imaging`)

**Files:**
- Create: `pkg/imaging/exif.go`
- Create: `pkg/imaging/exif_test.go`

**Interfaces:**
- Produces: `imaging.ExtractMetadata(data []byte) (*imaging.ImageMetadata, error)`, `imaging.ImageMetadata` (GPS coords, timestamp, orientation, dimensions).

- [ ] **Step 1: Write failing extraction logic tests**

In `pkg/imaging/exif_test.go`:
```go
package imaging_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/imaging"
)

func TestExtractMetadata(t *testing.T) {
	t.Parallel()
	
	// Synthetic byte fixture or small dummy JPEG logic
	data := []byte("dummy jpeg bytes with EXIF")
	
	_, err := imaging.ExtractMetadata(data)
	if err == nil {
		t.Fatalf("expected error for invalid dummy bytes, got nil")
	}
	
	// TODO: Add proper tests for valid coordinates and timestamp parsing
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/imaging/...`
Expected: FAIL (types and functions not defined).

- [ ] **Step 3: Implement domain models and extraction logic**

In `pkg/imaging/exif.go`:
Implement `ExtractMetadata` utilizing purely Go-native binary parsing for EXIF markers. Convert rational coordinates to decimal float64, parse strings to `time.Time`, and handle dimension constraints to avoid out-of-memory errors.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/imaging/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/imaging
git commit -m "feat(imaging): implement pure Go EXIF metadata extraction engine"
```

---

### Task 2: Image Quality Analyzer & Contrast Normalization Engine (`pkg/imaging`)

**Files:**
- Create: `pkg/imaging/enhance.go`
- Create: `pkg/imaging/enhance_test.go`

**Interfaces:**
- Consumes: Raw image bytes, extracted orientation.
- Produces: `imaging.EnhanceImage(data []byte) ([]byte, error)`.

- [ ] **Step 1: Write failing enhancement tests**

In `pkg/imaging/enhance_test.go`:
```go
package imaging_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/imaging"
)

func TestEnhanceImage(t *testing.T) {
	t.Parallel()
	
	data := []byte("dummy raw image bytes")
	
	_, err := imaging.EnhanceImage(data)
	if err == nil {
		t.Fatalf("expected error for invalid dummy image bytes, got nil")
	}
	
	// TODO: Further tests for successful contrast stretching and rotation logic
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/imaging/...`
Expected: FAIL.

- [ ] **Step 3: Implement enhancement logic**

In `pkg/imaging/enhance.go`:
Implement brightness histogram evaluation, dynamic range contrast stretching, and EXIF orientation normalization without relying on CGO dependencies.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/imaging/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/imaging
git commit -m "feat(imaging): implement image quality analyzer and contrast normalizer"
```

---

### Task 3: Web Frontend Image Processing REST Endpoints (`internal/app/webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/imaging.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/imaging_test.go`

**Interfaces:**
- Consumes: `pkg/imaging`.
- Produces: `POST /api/v1/images/extract-metadata`, `POST /api/v1/images/enhance`.

- [ ] **Step 1: Write failing endpoint tests**

In `internal/app/webfrontend/imaging_test.go`:
```go
package webfrontend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestImagingEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{})

	t.Run("POST /api/v1/images/extract-metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/images/extract-metadata", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for missing multipart form, got %d", rec.Code)
		}
	})
	
	// TODO: Add test for /enhance endpoint
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestImagingEndpoints ./internal/app/webfrontend/...`
Expected: FAIL (404 Not Found or compilation failure).

- [ ] **Step 3: Implement REST endpoints**

In `internal/app/webfrontend/imaging.go`:
Implement stateless multipart handlers. Apply `ratelimit.ModerateLimit`. Strip sensitive metadata tags, handle errors cleanly.
In `internal/app/webfrontend/server.go`: Register endpoints.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(webfrontend): add imaging metadata and enhancement REST endpoints"
```

---

### Task 4: Client-Side Dropzone EXIF Detection & Auto-Enhance Slider UI (`webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/static/js/image-enhancer.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/report-lost.html`
- Modify: `internal/app/webfrontend/templates/report-found.html`
- Modify: `internal/app/webfrontend/templates/modal-report-sighting.html` (or equivalent modal template)

**Interfaces:**
- Consumes: `/api/v1/images/extract-metadata`, `/api/v1/images/enhance`.
- Produces: Interactive dropzone with pre-fill prompts, and auto-enhance slider UI.

- [ ] **Step 1: Write failing template test in `server_test.go`**

Verify `image-enhancer.js` is loaded and endpoints are structured properly.

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement UI and client-side logic**

In HTML templates: Add `#photo-dropzone`, EXIF pre-fill toast component, and a toggle for "✨ Auto-Enhance". Include `image-enhancer.js`.
In `image-enhancer.js`: Use standard fetch API to upload images, parse JSON response for metadata, and handle user opt-in for location coordinates. Fetch enhanced bytes and implement side-by-side or sliding comparison view. Follow strict CSP (`script-src 'self'`).

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(ui): add dropzone EXIF detection and auto-enhance slider interface"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Verification

**Files:**
- Create: `tests/playwright/e2e/exif-image-enhancer-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser testing framework, running webfrontend.
- Produces: Complete automated test validating the EXIF upload flow and the UI toggle for auto-enhance.

- [ ] **Step 1: Write `tests/playwright/e2e/exif-image-enhancer-journey.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: EXIF Auto-Extraction and Image Enhancer', () => {
  test('should prompt for EXIF geolocation and allow image enhancement', async ({ page }) => {
    // 1. Visit report-lost page
    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    
    // 2. Upload dummy image with EXIF data
    // Assuming file chooser interaction setup
    
    // 3. Verify EXIF pre-fill prompt appears
    // 4. Click Apply and verify coordinate fields populate
    
    // 5. Toggle Auto-Enhance slider
    // 6. Verify image updates with enhanced version
  });
});
```

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/exif-image-enhancer-journey.spec.ts`
Expected: PASS (once mock interactions are set up).

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% PASS across all specs.

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/exif-image-enhancer-journey.spec.ts
git commit -m "test(e2e): add user journey tests for EXIF extraction and auto-enhance features"
```
