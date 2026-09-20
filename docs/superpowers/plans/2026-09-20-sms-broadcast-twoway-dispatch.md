# Milestone 9.2: Automated Neighborhood SMS/RCS Broadcast & Two-Way Interactive Dispatch Implementation Plan

- **Target Branch:** `feat/sms-broadcast-twoway`
- **Worktree:** `.worktrees/track-c`

---

## Proposed Tasks

### Task 1: SMS Provider Engine, Zero-PII Tokenizer & Opt-Out Store (`pkg/sms`, `pkg/store`)
- Implement `pkg/sms/provider.go` with `MockProvider`, `TokenizeNumber`, and E.164 normalization.
- Implement `pkg/sms/optout.go` managing opt-out compliance with unit tests.
- Add store constants `store.SMSOptOutCollection = "smsOptOuts"` and `store.SMSDeliveriesCollection = "smsDeliveries"`.

### Task 2: Inbound SMS Webhook Handler & Command Parser (`internal/app/webfrontend`)
- Implement `POST /api/v1/webhooks/sms/inbound` handling `CLAIM`, `SIGHTED`, `STATUS`, and `STOP`.
- Connect `CLAIM` command to `store.SectorAssignmentsCollection` and `ReunionHub` SSE broadcast.
- Connect `SIGHTED` command to `store.SightingsCollection` and trajectory updates.
- Write unit tests in `internal/app/webfrontend/sms_webhook_test.go`.

### Task 3: Hyper-Local Radius Dispatch Worker (`pkg/sms`, `internal/app/notification`)
- Implement geo-fenced alert broadcast query filtering opted-in phones within 2 miles of a lost pet report.
- Dispatch templated alerts via `Provider` with rate limiting and opt-out checks.
- Write unit tests in `pkg/sms/broadcast_test.go`.

### Task 4: Frontend Volunteer SMS Opt-in UI in Notification Center
- Add phone number input with SMS alert preference checkbox in `#modal-alert-preferences`.
- Send verification test SMS via `POST /api/v1/sms/subscribe`.
- Add responsive styling in `styles.css`.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- Create `tests/playwright/e2e/sms-twoway-dispatch-journey.spec.ts`.
- Test volunteer SMS subscription, inbound webhook command simulation (`CLAIM`, `SIGHTED`, `STOP`), and SSE broadcast synchronization.
- Run `make verify` ensuring all linters and tests pass cleanly.
