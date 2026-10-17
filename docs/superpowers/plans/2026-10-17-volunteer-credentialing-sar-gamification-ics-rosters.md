# Volunteer Credentialing, SAR Gamification & Incident Command (ICS) Shift Rosters — Implementation Plan

- **Milestone:** 12.3
- **Spec:** `docs/superpowers/specs/2026-10-17-volunteer-credentialing-sar-gamification-ics-rosters-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-ics-volunteer-credentialing` (`feat/ics-volunteer-credentialing`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collections & Validation (`pkg/domain/ics_roster.go`, `pkg/store/store.go`)
- **Files:**
  - Create `pkg/domain/ics_roster.go`
  - Create `pkg/domain/ics_roster_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `SARRoleCredential`, `SARMeritBadge`, `ICSShiftStatus`, `FatigueAlertLevel`, `SignedCredential`, `VolunteerProfile`, and `ICSShiftEntry`.
  - Add `CollectionVolunteerProfiles = "volunteer_profiles"` and `CollectionICSShiftEntries = "ics_shift_entries"` in `pkg/store/store.go`.
  - Write unit tests for validation and JSON round-trip serialization in `pkg/domain/ics_roster_test.go`.

### Task 2: Pure-Go Cryptographic Credential Signer, Fatigue Guardrail Evaluator & Gamification Engine (`pkg/ics`)
- **Files:**
  - Create `pkg/ics/credentials.go`
  - Create `pkg/ics/fatigue.go`
  - Create `pkg/ics/gamification.go`
  - Create `pkg/ics/ics_test.go`
- **Details:**
  - `SignRoleCredential`, `VerifyRoleCredential`, and `HasVerifiedRole` using HMAC-SHA256 and `subtle.ConstantTimeCompare`.
  - `EvaluateShiftFatigue(activeHours, distanceKm)` enforcing `< 6h` (`NORMAL_FIT`), `6h–<8h` (`FATIGUE_WARNING`), and `>= 8h` or `>= 25km` (`MANDATORY_REST`).
  - `ExportICS204RosterCSV(shifts []domain.ICSShiftEntry) (string, error)` generating RFC 4180 ICS-204 / ICS-211 CSV exports.
  - `EvaluateVolunteerProgression(profile domain.VolunteerProfile, nightShift bool) domain.VolunteerProfile` computing deterministic `TotalXP`, `RankTitle`, and `EarnedBadges`.

### Task 3: WebFrontend REST API, Credential Role Gate, Shift Progression & ICS-204 Export (`internal/app/webfrontend/ics_handlers.go`)
- **Files:**
  - Create `internal/app/webfrontend/ics_handlers.go`
  - Create `internal/app/webfrontend/ics_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
- **Details:**
  - Register routes in `server.go`:
    - `GET /ics-command` -> `s.handleICSCommandPage`
    - `GET /api/v1/ics/volunteers` -> `s.handleListICSVolunteers`
    - `POST /api/v1/ics/volunteers` -> `s.handleCreateICSVolunteer`
    - `POST /api/v1/ics/volunteers/{id}/credentials` -> `s.handleIssueVolunteerCredential`
    - `POST /api/v1/ics/shifts` -> `s.handleCreateICSShift`
    - `GET /api/v1/ics/shifts` -> `s.handleListICSShifts`
    - `POST /api/v1/ics/shifts/{id}/progress` -> `s.handleUpdateICSShiftProgress`
    - `GET /api/v1/ics/stream` -> `s.handleICSStream`
  - Seed 3 default credentialed SAR responders on first lookup.
  - Enforce credential role gate on `POST /api/v1/ics/shifts` (`403 Forbidden` if volunteer lacks verified credential for specialized role).
  - Broadcast `ics_shift_updated` and `ics_volunteer_updated` on SSE (`"ics"` topic) and WebRTC mesh (`s.signalingHub`).

### Task 4: ICS Command & SAR Gamification Cockpit UI (`/ics-command`) & WCAG AAA CSS
- **Files:**
  - Create `internal/app/webfrontend/templates/ics_command.html`
  - Create `internal/app/webfrontend/static/js/ics-command.js`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/ics-command.spec.ts`
- **Details:**
  - Build `/ics-command` featuring:
    - **Section 1: SAR Responder Registry, Credential Issuance & Leaderboard (`#volunteers-leaderboard`, `#form-register-volunteer`, `#form-issue-credential`)**
    - **Section 2: ICS-211 Shift Check-In & Sector Assignment (`#form-checkin-shift`)** with role-gate alert banner (`#role-gate-error-alert`)
    - **Section 3: Active ICS-204 Shift Roster & Fatigue Safety Monitor (`#ics-shifts-list`, `#btn-export-ics204-csv`)** with one-click telemetry/fatigue simulation (`+2h Patrol`, `Trigger 8.5h Mandatory Rest`, `Check Out & Award XP`)
  - Enforce WCAG AAA (`> 7:1`) contrast for fatigue and shift status badges (`.badge-fatigue-normal`, `.badge-fatigue-warning`, `.badge-fatigue-rest`, `.badge-sar-rank`) across dark and light modes.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/ics-command-journey.spec.ts`
- **Details:**
  - 6-step E2E journey verifying responder registration, role-gate enforcement (`403` when attempting `UAV_PART107_PILOT` shift without credential), cryptographic credential issuance (`UAV_PART107_PILOT`), successful shift check-in, fatigue escalation (`FATIGUE_WARNING` at 6.5h -> `MANDATORY_REST` at 8.5h), shift check-out with XP & merit badge progression (`FIRST_RESPONDER`, `AERIAL_SPOTTER`, `GRID_MASTER_50KM`), and ICS-204 CSV export verification.

### Task 6: Final Whole-Branch Code Review & Verification Gate
- **Details:**
  - Run whole-branch review, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass, push, open PR, and squash-merge.
