# Milestone 8.3: Automated Neighborhood Webhooks & Localized GeoRSS/Atom Feeds Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement automated, location-filtered Webhook dispatcher with SSRF protections and HMAC-SHA256 signatures, alongside standard W3C GeoRSS/Atom feeds for integrating lost pet alerts with third-party community sites and CRM partners.

**Architecture:** A core delivery engine in `pkg/webhook` handles background webhook deliveries, retry backoffs, SSRF validation, and HMAC signing. Standard RFC 4287 Atom 1.0 feeds with W3C GeoRSS point annotations are served via `internal/app/webfrontend`. Web frontend manages subscription registrations and exposes REST endpoints.

**Tech Stack:** Go 1.26.5 (`export GOTOOLCHAIN=go1.26.5`), Playwright E2E testing framework.

**Spec:** `docs/superpowers/specs/2026-09-19-neighborhood-webhooks-and-georss-feeds-design.md`

## 1. Header & Proposed Changes
- **User Review Required:** None (standard SDD automation)
- **Global Constraints:** 
  - Pin to `export GOTOOLCHAIN=go1.26.5`. 
  - Strict CSP (`script-src 'self'`). 
  - SSRF URL validation (blocking RFC 1918 / loopback / cloud metadata IPs).
  - 100% verification pass rate across `make verify` and all Playwright test specifications before completion.

---

## 2. Tasks

### Task 1: Webhook Models, HMAC-SHA256 Signer & SSRF Validator

**Files:**
- Create: `pkg/webhook/subscription.go`
- Create: `pkg/webhook/validator.go`
- Create: `pkg/webhook/signer.go`
- Create: `pkg/webhook/webhook_test.go`

**Interfaces:**
- Consumes: None
- Produces: `webhook.WebhookSubscription`, `webhook.GeoFence`, `webhook.ValidateURL(url string) error`, `webhook.GenerateSignature(payload []byte, secret string) string`

- [ ] **Step 1: Write failing test code**

In `pkg/webhook/webhook_test.go`:
```go
package webhook_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestValidator_SSRF(t *testing.T) {
	t.Parallel()

	invalidURLs := []string{
		"http://127.0.0.1/admin",
		"https://10.0.0.5/api",
		"http://169.254.169.254/latest/meta-data/",
	}

	for _, u := range invalidURLs {
		if err := webhook.ValidateURL(u); err == nil {
			t.Errorf("expected SSRF validation error for %s, got nil", u)
		}
	}

	validURLs := []string{
		"https://api.example.com/webhook",
	}

	for _, u := range validURLs {
		if err := webhook.ValidateURL(u); err != nil {
			t.Errorf("expected no error for %s, got %v", u, err)
		}
	}
}

func TestSigner_HMAC(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"event":"pet_lost"}`)
	secret := "supersecret"
	
	sig := webhook.GenerateSignature(payload, secret)
	if sig == "" {
		t.Error("expected signature, got empty string")
	}
}
```

- [ ] **Step 2: Run test to fail**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/...`
Expected: FAIL (types and functions not defined).

- [ ] **Step 3: Implementation code**

1. In `pkg/webhook/subscription.go`: Define `WebhookSubscription` and `GeoFence` models, and constants for collections.
2. In `pkg/webhook/validator.go`: Implement `ValidateURL` parsing IPs and blocking private network boundaries.
3. In `pkg/webhook/signer.go`: Implement `GenerateSignature` using HMAC-SHA256 and returning hex encoded hash formatted as `sha256=<hash>`.

- [ ] **Step 4: Run test to pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/...`
Expected: PASS.

- [ ] **Step 5: Git commit**

```bash
git add pkg/webhook
git commit -m "feat(webhook): add models, SSRF validator, and HMAC signer"
```

---

### Task 2: Background Webhook Delivery Worker with Backoff Retries

**Files:**
- Create: `pkg/webhook/dispatcher.go`
- Create: `pkg/webhook/dispatcher_test.go`

**Interfaces:**
- Consumes: `pkg/webhook/subscription.go`, `pkg/webhook/validator.go`, `pkg/webhook/signer.go`
- Produces: `webhook.Dispatcher` instance that handles delivery, geospatial filtering, and retries.

- [ ] **Step 1: Write failing test code**

In `pkg/webhook/dispatcher_test.go`:
```go
package webhook_test

import (
	"context"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestDispatcher_Delivery(t *testing.T) {
	t.Parallel()
	
	dispatcher := webhook.NewDispatcher(nil) // Add dependencies as needed
	if dispatcher == nil {
		t.Fatal("expected non-nil dispatcher")
	}

	// Mocking and testing retry behavior
}
```

- [ ] **Step 2: Run test to fail**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/dispatcher_test.go`
Expected: FAIL (dispatcher not implemented).

- [ ] **Step 3: Implementation code**

In `pkg/webhook/dispatcher.go`:
Implement `Dispatcher` structure, consumer logic for event bus, `Deliver` function that includes HTTP dispatch with `X-PetSpotR-Signature`, Haversine filtering, and retry logic (3 retries at 5s, 15s, 45s). Logs deliveries to database.

- [ ] **Step 4: Run test to pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/...`
Expected: PASS.

- [ ] **Step 5: Git commit**

```bash
git add pkg/webhook/dispatcher.go pkg/webhook/dispatcher_test.go
git commit -m "feat(webhook): implement dispatcher with backoff and retries"
```

---

### Task 3: GeoRSS & Atom Syndication Serializers

**Files:**
- Create: `pkg/webhook/feed.go`
- Create: `pkg/webhook/feed_test.go`

**Interfaces:**
- Consumes: Application domain models (e.g. `domain.LostPetRecord`)
- Produces: `application/atom+xml` compliant output with GeoRSS.

- [ ] **Step 1: Write failing test code**

In `pkg/webhook/feed_test.go`:
```go
package webhook_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestFeed_AtomGeneration(t *testing.T) {
	t.Parallel()

	feedXML, err := webhook.GenerateAtomFeed(nil) // pass mock data
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(feedXML, "<georss:point>") {
		t.Error("expected georss point in output")
	}
}
```

- [ ] **Step 2: Run test to fail**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/feed_test.go`
Expected: FAIL.

- [ ] **Step 3: Implementation code**

In `pkg/webhook/feed.go`:
Implement struct tags and serialization logic for Atom 1.0 compliant XML using `encoding/xml`. Ensure `georss:point` is injected for coordinates.

- [ ] **Step 4: Run test to pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/webhook/feed_test.go`
Expected: PASS.

- [ ] **Step 5: Git commit**

```bash
git add pkg/webhook/feed.go pkg/webhook/feed_test.go
git commit -m "feat(webhook): add Atom and GeoRSS serialization"
```

---

### Task 4: Web Frontend REST & Feed Endpoints

**Files:**
- Create: `internal/app/webfrontend/webhook.go`
- Create: `internal/app/webfrontend/feed.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/webhook_test.go`

**Interfaces:**
- Consumes: `pkg/webhook` capabilities.
- Produces: 
  - `POST /api/v1/webhooks`
  - `GET /api/v1/webhooks`
  - `DELETE /api/v1/webhooks/{id}`
  - `POST /api/v1/webhooks/{id}/test`
  - `GET /feeds/lost-pets.atom`
  - `GET /feeds/sightings.atom`

- [ ] **Step 1: Write failing test code**

In `internal/app/webfrontend/webhook_test.go`:
```go
package webfrontend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestWebhookEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{})

	req := httptest.NewRequest(http.MethodGet, "/feeds/lost-pets.atom", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to fail**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/webhook_test.go`
Expected: FAIL (routes not found).

- [ ] **Step 3: Implementation code**

1. In `internal/app/webfrontend/webhook.go`: Implement CRUD handlers for webhooks and `/test` ping handler.
2. In `internal/app/webfrontend/feed.go`: Implement feed generation handlers utilizing `pkg/webhook.GenerateAtomFeed`.
3. In `internal/app/webfrontend/server.go`: Register all new routes.

- [ ] **Step 4: Run test to pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Git commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(webfrontend): add webhook REST and Atom feed endpoints"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Verification

**Files:**
- Create: `tests/playwright/e2e/webhooks-and-georss-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend.
- Produces: End-to-end verification of registering a webhook, trigger event, and checking feeds.

- [ ] **Step 1: Write failing test code**

In `tests/playwright/e2e/webhooks-and-georss-journey.spec.ts`:
```typescript
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Webhooks & GeoRSS E2E Journey', () => {
  test('Partner configures webhook and pulls RSS feed', async ({ page, request }) => {
    // Navigate and register webhook
    const webhookResp = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks`, {
        data: { targetUrl: "https://httpbin.org/post", filterEvents: ["pet_lost"] }
    });
    expect(webhookResp.status()).toBe(201);

    // Verify Atom feed
    const feedResp = await request.get(`${WEB_FRONTEND_URL}/feeds/lost-pets.atom`);
    expect(feedResp.status()).toBe(200);
    const text = await feedResp.text();
    expect(text).toContain('feed');
  });
});
```

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/webhooks-and-georss-journey.spec.ts`
Expected: FAIL.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% PASS across all specs.

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (`go vet`, `golangci-lint`, Go race tests).

- [ ] **Step 5: Git commit**

```bash
git add tests/playwright/e2e/webhooks-and-georss-journey.spec.ts
git commit -m "test(e2e): add automated user journey for webhooks and georss feeds"
```
