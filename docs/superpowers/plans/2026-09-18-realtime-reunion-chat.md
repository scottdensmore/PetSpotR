# Real-Time Owner-Finder Secure Chat & Reunion Room Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a secure, real-time mediated communication tunnel ("Reunion Room") between pet owners and pet finders featuring live Server-Sent Events (SSE) message streaming, verification photo sharing, typing and presence indicators, and in-chat resolution actions with zero personal PII exposure.

**Architecture:** Server-Sent Events (SSE) multiplexed over HTTP/2 deliver real-time message, presence, and resolution streams to verified match participants. A thread-safe in-memory `ReunionHub` bridges local client connections with Cloud Pub/Sub broadcast distribution across webfrontend instances. Message sending, presence pings, and photo staging flow through authenticated, rate-limited REST endpoints with CSRF and idempotency protection.

**Tech Stack:** Go 1.26.5, Server-Sent Events (SSE), Cloud Firestore, Cloud Pub/Sub, Cloud Storage, Vanilla JavaScript (EventSource API, DOM APIs), Playwright E2E.

**Spec:** `docs/superpowers/specs/2026-09-18-realtime-reunion-chat-design.md`

## Global Constraints

- Pinned toolchain: `export GOTOOLCHAIN=go1.26.5`
- Zero external client-side NPM dependencies; pure vanilla JavaScript with native browser APIs (`EventSource`, `fetch`, `FormData`).
- Strict Content Security Policy (CSP): `script-src 'self'`, `connect-src 'self' https://storage.petspotr.io ...`.
- Zero-PII Wire Contract: Neither party's real email, phone number, name, or Google sub ID is ever exposed; participants are strictly identified by role (`"reporter"` or `"finder"`).
- All background tasks and async operations must be fully verified via `make verify` and Playwright tests before committing.

---

### Task 1: Domain Model Extensions & Real-Time Event Envelopes

**Files:**
- Modify: `pkg/domain/match_thread.go`
- Create: `pkg/domain/reunion_events.go`
- Test: `pkg/domain/match_thread_test.go`

**Interfaces:**
- Consumes: Standard Go packages (`time`, `errors`).
- Produces: `MediatedMatchMessage.Images`, `ReunionStreamEvent`, `ReunionPresencePayload`.

- [ ] **Step 1: Write failing tests in `pkg/domain/match_thread_test.go`**

```go
func TestAppendMediatedMessageWithImages(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	reporter := domain.PrincipalRef{Issuer: "https://accounts.google.com", Subject: "owner-1"}
	finder := domain.PrincipalRef{Issuer: "https://accounts.google.com", Subject: "finder-1"}

	record := domain.MatchParticipantRecord{
		MatchID:    "match-123",
		LostPetID:  "lost-456",
		FoundPetID: "found-789",
		Reporter:   &reporter,
		Finder:     &finder,
	}

	t.Run("appends valid images", func(t *testing.T) {
		images := []string{"images/reunions/match-123/collar.jpg"}
		next, msg, created, err := record.AppendMediatedMessageWithImages(
			reporter, "idem-1", "Here is the collar photo", images, now,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatal("expected message to be created")
		}
		if len(msg.Images) != 1 || msg.Images[0] != images[0] {
			t.Fatalf("expected 1 image %q, got %v", images[0], msg.Images)
		}
		if len(next.Messages) != 1 {
			t.Fatalf("expected 1 message in participant record, got %d", len(next.Messages))
		}
	})

	t.Run("rejects more than 3 images", func(t *testing.T) {
		images := []string{"img1.jpg", "img2.jpg", "img3.jpg", "img4.jpg"}
		_, _, _, err := record.AppendMediatedMessageWithImages(
			reporter, "idem-2", "Too many photos", images, now,
		)
		if !errors.Is(err, domain.ErrInvalidMediatedMessage) {
			t.Fatalf("expected ErrInvalidMediatedMessage, got %v", err)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestAppendMediatedMessageWithImages ./pkg/domain/...`
Expected: Compile error / undefined `AppendMediatedMessageWithImages`.

- [ ] **Step 3: Implement domain extensions in `match_thread.go` and `reunion_events.go`**

1. In `pkg/domain/match_thread.go`:
   - Add `Images []string` to `MediatedMatchMessage`:
     ```go
     type MediatedMatchMessage struct {
         MessageID  string               `json:"messageId"`
         SenderRole MatchParticipantRole `json:"senderRole"`
         Message    string               `json:"message"`
         Images     []string             `json:"images,omitempty"`
         SentAt     time.Time            `json:"sentAt"`
     }
     ```
   - Implement `AppendMediatedMessageWithImages` with max 3 images limit and validation.
   - Update `AppendMediatedMessage` to delegate to `AppendMediatedMessageWithImages(..., nil, ...)`.

2. Create `pkg/domain/reunion_events.go`:
   ```go
   package domain

   import "time"

   type ReunionEventType string

   const (
       ReunionEventMessageCreated ReunionEventType = "message"
       ReunionEventPresence       ReunionEventType = "presence"
       ReunionEventResolved       ReunionEventType = "reunion_resolved"
   )

   type ReunionStreamEvent struct {
       EventID   string           `json:"eventId"`
       Type      ReunionEventType `json:"type"`
       MatchID   string           `json:"matchId"`
       Timestamp time.Time        `json:"timestamp"`
       Payload   any              `json:"payload"`
   }

   type ReunionPresencePayload struct {
       SenderRole MatchParticipantRole `json:"senderRole"`
       Status     string               `json:"status"`
   }
   ```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/domain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/match_thread.go pkg/domain/reunion_events.go pkg/domain/match_thread_test.go
git commit -m "feat(domain): add image attachments to mediated match messages and reunion stream event models"
```

---

### Task 2: In-Memory Reunion Hub & Concurrency Management

**Files:**
- Create: `internal/app/webfrontend/reunion_hub.go`
- Test: `internal/app/webfrontend/reunion_hub_test.go`

**Interfaces:**
- Consumes: `pkg/domain/reunion_events.go`, standard `sync`, `context`.
- Produces: `ReunionHub` struct providing thread-safe `Subscribe`, `Unsubscribe`, and `BroadcastLocal`.

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/reunion_hub_test.go`**

```go
func TestReunionHubConcurrency(t *testing.T) {
	t.Parallel()
	hub := NewReunionHub()

	ch1, unsub1 := hub.Subscribe("match-1")
	defer unsub1()
	ch2, unsub2 := hub.Subscribe("match-1")
	defer unsub2()
	chOther, unsubOther := hub.Subscribe("match-2")
	defer unsubOther()

	event := domain.ReunionStreamEvent{
		EventID:   "evt-1",
		Type:      domain.ReunionEventMessageCreated,
		MatchID:   "match-1",
		Timestamp: time.Now().UTC(),
	}

	hub.BroadcastLocal(event)

	select {
	case received := <-ch1:
		if received.EventID != "evt-1" {
			t.Fatalf("ch1 received wrong event: %v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ch1")
	}

	select {
	case received := <-ch2:
		if received.EventID != "evt-1" {
			t.Fatalf("ch2 received wrong event: %v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ch2")
	}

	select {
	case unexpected := <-chOther:
		t.Fatalf("chOther should not receive match-1 event, got: %v", unexpected)
	default:
		// Expected
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestReunionHub ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement `ReunionHub` in `reunion_hub.go`**

Implement `internal/app/webfrontend/reunion_hub.go`:
- Thread-safe connection registry with `sync.RWMutex`.
- Bounded channel buffer (`cap: 16`) per subscriber.
- Non-blocking `BroadcastLocal` with select default drop on full channels.
- Clean cleanup in `unsubscribe` removing channels and empty map keys.

- [ ] **Step 4: Run tests with race detector to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race -run TestReunionHub ./internal/app/webfrontend/...`
Expected: PASS (0 race conditions).

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/reunion_hub.go internal/app/webfrontend/reunion_hub_test.go
git commit -m "feat(webfrontend): implement thread-safe in-memory reunion event hub"
```

---

### Task 3: SSE Streaming Endpoint & Presence REST Handler

**Files:**
- Create: `internal/app/webfrontend/reunion_events.go`
- Modify: `internal/app/webfrontend/server.go`
- Modify: `internal/app/webfrontend/mediated_contact.go`
- Test: `internal/app/webfrontend/reunion_events_test.go`

**Interfaces:**
- Consumes: `ReunionHub`, HTTP request, session auth.
- Produces:
  - `GET /api/v1/reunions/events?matchId=...` (SSE stream with `: ping\n\n` keepalive)
  - `POST /api/v1/reunions/presence` (ephemeral presence events)
  - Extended `POST /api/v1/reunions/contact` with `images: [...]`

- [ ] **Step 1: Write failing tests in `reunion_events_test.go`**

- Test SSE handshake returns `200 OK`, `Content-Type: text/event-stream`, `Cache-Control: no-cache, no-transform`.
- Test unauthenticated or non-participant callers receive 404.
- Test `: ping\n\n` keepalive comment emitted periodically.
- Test presence endpoint dispatches `presence` event to subscribed SSE client.
- Test message creation with `images` dispatches `message` event to subscribed SSE client.

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestReunionEvents ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement SSE & presence handlers**

1. Create `internal/app/webfrontend/reunion_events.go`:
   - `handleApiReunionEvents(w, r)`:
     - Checks session principal and loads authorized mediated thread.
     - Verifies `http.Flusher`.
     - Sets SSE headers (`text/event-stream`, `no-cache`, `no-transform`, `X-Accel-Buffering: no`).
     - Subscribes to `ReunionHub`.
     - Sends initial connected comment.
     - Loops on 15s ticker for ping, subscriber channel for events, and `r.Context().Done()` for graceful disconnect.
   - `handleApiReunionPresence(w, r)`:
     - Validates session, participant role, matchId, and status.
     - Calls `hub.BroadcastLocal` or PubSub publish.
2. In `mediated_contact.go`:
   - Parse optional `images []string` from `ReunionContactRequest`.
   - Call `record.AppendMediatedMessageWithImages`.
   - On success, publish `ReunionEventMessageCreated` to hub.
3. In `server.go`:
   - Initialize `reunionHub = NewReunionHub()`.
   - Register routes in `registerRoutes`:
     - `/api/v1/reunions/events`
     - `/api/v1/reunions/presence`

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race -run TestReunionEvents ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/reunion_events.go internal/app/webfrontend/reunion_events_test.go internal/app/webfrontend/server.go internal/app/webfrontend/mediated_contact.go
git commit -m "feat(webfrontend): add sse streaming endpoint and presence handler for reunion room"
```

---

### Task 4: Reunion Room Template Markup & Responsive Styles

**Files:**
- Modify: `internal/app/webfrontend/templates/matches.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: HTML templates, CSS custom properties.
- Produces: Upgraded `#match-thread-modal` with live presence indicator, typing banner, image staging tray, compose row with attachment button, and celebratory resolution banner.

- [ ] **Step 1: Update `matches.html`**

Update `#match-thread-modal` structure:
- Add `#match-thread-presence` (dot and status text).
- Add `#match-thread-typing` (animated ellipsis and participant typing text).
- Add `#match-thread-resolved-banner` (celebratory resolution alert).
- Add `#btn-chat-resolve-reunion` in the modal header.
- Add photo attachment trigger `#match-thread-attach-btn`, file input `#match-thread-file-input`, and preview tray `#match-thread-staged-tray`.

- [ ] **Step 2: Add CSS in `styles.css`**

Add styles for:
- `.presence-badge`, `.presence-dot` (green for online, amber for connecting/typing).
- `.typing-indicator` with smooth bounce animation for dots.
- `.reunion-banner` with emerald accent border and glassmorphic background.
- `.compose-row`, `.btn-attach`, `.staged-tray`, `.staged-thumbs`.
- Respect `prefers-reduced-motion: reduce`.

- [ ] **Step 3: Update `server_test.go`**

Add assertion in `TestRenderedPagesDeclareOfflineUI` or new test verifying presence, attachment, and typing elements exist in rendered `/matches` HTML.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/matches.html internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/server_test.go
git commit -m "feat(ui): add reunion room presence, typing, image attachment, and resolution markup and styles"
```

---

### Task 5: Client Reunion Controller & Native EventSource Integration

**Files:**
- Modify: `internal/app/webfrontend/static/js/match-dashboard.js`

**Interfaces:**
- Consumes: Browser `EventSource`, `fetch`, DOM APIs.
- Produces: Live real-time chat, debounced typing events, photo staging and upload via presigned URLs, and in-chat resolution handling.

- [ ] **Step 1: Update `match-dashboard.js`**

1. In `openThreadModal(matchID)`:
   - Establish `new EventSource('/api/v1/reunions/events?matchId=' + encodeURIComponent(matchID))`.
   - Listen for `event: message` $\rightarrow$ append to `#match-thread-messages` and scroll into view.
   - Listen for `event: presence` $\rightarrow$ update typing indicator with 3-second auto-hide timeout.
   - Listen for `event: reunion_resolved` $\rightarrow$ show `#match-thread-resolved-banner`, lock compose input.
2. Typing detection on textarea `input`:
   - Debounced (every 1.5s) `POST /api/v1/reunions/presence` with `{ matchId, status: "typing" }`.
3. Photo attachment handling:
   - File picker populates `stagedPhotos` array (max 3).
   - On send, uploads Blobs via `/api/v1/uploads/presigned-url`, attaches tokens, and posts message.
4. On `closeThreadModal()`:
   - Calls `evtSource.close()`.
   - Sends `status: "idle"`.

- [ ] **Step 2: Validate JavaScript syntax**

Run: `node -c internal/app/webfrontend/static/js/match-dashboard.js`
Expected: 0 syntax errors.

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/js/match-dashboard.js
git commit -m "feat(webfrontend): integrate eventsource live streaming and photo attachments into match dashboard"
```

---

### Task 6: Playwright Multi-User Real-Time E2E Journey Tests & Full Verification

**Files:**
- Create: `tests/playwright/e2e/reunion-chat-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright multi-context (`browser.newContext()`), live backend services.
- Produces: Automated E2E verification of real-time multi-user reunion chat, typing indicators, photo sharing, and in-chat reunion resolution.

- [ ] **Step 1: Write `tests/playwright/e2e/reunion-chat-journey.spec.ts`**

Implement multi-user tests:
1. `should stream private messages and typing indicators in real-time between owner and finder`:
   - Creates Context A (Owner) and Context B (Finder).
   - Both open Match Dashboard and enter Reunion Room for Match #1.
   - Finder types in input $\rightarrow$ Owner sees "Finder is typing...".
   - Finder sends message $\rightarrow$ Owner receives message in real-time over SSE without reload.
2. `should share verification photos and mark reunion resolved inside chat`:
   - Finder stages and sends a photo $\rightarrow$ Owner receives message with thumbnail image.
   - Owner clicks "Mark as Reunited" $\rightarrow$ both windows receive `reunion_resolved` event and display the resolved banner.

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/reunion-chat-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% passing (67+ tests).

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: All checks passing cleanly.

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/reunion-chat-journey.spec.ts
git commit -m "test(e2e): add multi-user real-time reunion chat and resolution journey test"
```
