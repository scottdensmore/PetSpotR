# Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Engine) — Implementation Plan

- **Milestone:** 13.2
- **Spec:** `docs/superpowers/specs/2026-10-17-trust-safety-scam-hoax-shield-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-trust-safety-shield` (`feat/trust-safety-shield`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collection & Validation (`pkg/domain/trust_safety.go`, `pkg/store/store.go`)
- **Files:**
  - Create `pkg/domain/trust_safety.go`
  - Create `pkg/domain/trust_safety_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `TrustScreeningVerdict`, `ThreatAnomalyFlag`, and `SightingTrustScreening` with validation methods (`IsValid()`, `Validate()`).
  - Add `CollectionSightingTrustScreening = "sighting_trust_screenings"` in `pkg/store/store.go`.
  - Write unit tests in `pkg/domain/trust_safety_test.go`.

### Task 2: Pure-Go Multi-Modal Scam, Velocity & Synthetic Heuristics Engine (`pkg/trustsafety`)
- **Files:**
  - Create `pkg/trustsafety/shield.go`
  - Create `pkg/trustsafety/shield_test.go`
- **Details:**
  - Implement `EvaluateSightingTrust` and `ApplyModeratorDecision` in `pkg/trustsafety/shield.go`.
  - Write unit tests in `pkg/trustsafety/shield_test.go` covering extortion payment demands, 6-digit PIN phishing, biological teleportation velocity (`> 35 km/h`), synthetic GAN image artifacts, clean EXIF-verified sightings, and moderator override decisions.

### Task 3: WebFrontend REST API, Auto-Quarantine, Sighting Promotion & SSE/Mesh Stream (`internal/app/webfrontend/trust_safety_handlers.go`)
- **Files:**
  - Create `internal/app/webfrontend/trust_safety_handlers.go`
  - Create `internal/app/webfrontend/trust_safety_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
  - Create stub `internal/app/webfrontend/templates/trust_safety.html`
- **Details:**
  - Register routes in `server.go`:
    - `GET /trust-safety` -> `s.handleTrustSafetyPage`
    - `GET /api/v1/trust-safety/screenings` -> `s.handleListTrustScreenings`
    - `POST /api/v1/trust-safety/screenings` -> `s.handleCreateTrustScreening`
    - `POST /api/v1/trust-safety/screenings/{id}/decision` -> `s.handleAdjudicateTrustScreening`
    - `GET /api/v1/trust-safety/stream` -> `s.handleTrustSafetyStream`

### Task 4: Trust & Safety Moderator Quarantine Cockpit UI (`/trust-safety`) & WCAG AAA CSS
- **Files:**
  - Update `internal/app/webfrontend/templates/trust_safety.html`
  - Create `internal/app/webfrontend/static/js/trust-safety.js`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/trust-safety.spec.ts`
- **Details:**
  - Build `/trust-safety` featuring:
    - **Section 1: Inbound Sighting Trust & Scam Shield Simulator (`#form-submit-trust-screening`, `#btn-sim-extortion-scam`, `#btn-sim-teleport-hoax`)**
    - **Section 2: Moderator Quarantine & Verification Queue (`#trust-screenings-list`)** with action buttons (`✅ Approve & Promote to Sighting`, `🚫 Confirm Scam / Hoax & Block`)
    - **Section 3: Multi-Factor Threat & Kinematic Telemetry Inspector (`#trust-inspector-panel`)**
  - Enforce WCAG AAA (`> 7:1`) contrast across dark and light modes.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/trust-safety-journey.spec.ts`
- **Details:**
  - 6-step E2E journey verifying seeded screenings, submitting a reward-extortion + synthetic GAN sighting and verifying auto-quarantine (`QUARANTINED_SCAM_RISK`) with owner protection banner, submitting a biological teleportation velocity anomaly (`HOLD_FOR_MODERATION`), adjudicating a held sighting as `VERIFIED_AUTHENTIC` and confirming promotion to a live Community Sighting (`sighting-trust-...`), adjudicating the extortion attempt as `REJECTED_CONFIRMED_HOAX` (reporter trust score `0`), and verifying zero CSP errors.

### Task 6: Final Whole-Branch Code Review & Verification Gate
- **Details:**
  - Run whole-branch review, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass, push, open PR (#287), and squash-merge into `main`.
