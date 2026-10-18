# Milestone 13.3: Humane Live-Trap IoT Telemetry, Bait Decay & Rapid Dispatch Cockpit — Implementation Plan

## Overview
This plan breaks Milestone 13.3 into 5 sequential, TDD-driven implementation tasks executed via Subagent-Driven Development (SDD), followed by a final whole-branch review, full Playwright suite verification, PR creation, and squash-merge.

## Tasks
1. **Task 1: Domain Models, State Store Collection & Validation (`pkg/domain/humane_trap.go`, `pkg/store/store.go`)**
   - Define `HumaneTrapType`, `HumaneTrapStatus`, `BaitFreshnessState`, `HumaneTrapAlertFlag`, and `HumaneTrapDeployment` with `Validate()`.
   - Add `CollectionHumaneTrapDeployments = "humane_trap_deployments"` to `pkg/store/store.go` and `DefaultCollections()`.
   - Unit test validation rules in `pkg/domain/humane_trap_test.go`.

2. **Task 2: Pure-Go Humane Trap Telemetry, Bait Decay, Thermal Safety & SLA Engine (`pkg/humanetrap`)**
   - Implement `EvaluateBaitFreshness`, `EvaluateTrapTelemetry`, `ProcessDoorTripEvent`, `DispatchTrapResponder`, and `ResolveTrapInspection` in `pkg/humanetrap/telemetry.go`.
   - Unit test all telemetry, bait spoilage, thermal lockout, weight-plate target vs bycatch heuristics, 15-minute SLA deadline, and responder resolution transitions in `pkg/humanetrap/telemetry_test.go`.

3. **Task 3: WebFrontend REST API, Seed Deployments, Sighting Promotion & SSE/Mesh Stream (`internal/app/webfrontend/humane_trap_handlers.go`)**
   - Implement `GET /humane-traps`, `GET /api/v1/humane-traps`, `POST /api/v1/humane-traps`, `POST /api/v1/humane-traps/{id}/telemetry`, `POST /api/v1/humane-traps/{id}/dispatch`, and `GET /api/v1/humane-traps/stream`.
   - Seed 3 deterministic traps (`trap-ht-101`, `trap-ht-102`, `trap-ht-103`) and promote `TARGET_PET_CAPTURED` outcomes to `store.SightingsCollection`.
   - Unit test all endpoints and SSE streaming in `internal/app/webfrontend/humane_trap_handlers_test.go`.

4. **Task 4: Humane Live-Trap Telemetry & Rapid Dispatch Cockpit UI (`/humane-traps`) & WCAG AAA CSS**
   - Build `internal/app/webfrontend/templates/humane_traps.html` and `internal/app/webfrontend/static/js/humane-traps.js`.
   - Add WCAG AAA (`> 7.1:1`) badge and urgent SLA banner classes in `internal/app/webfrontend/static/css/styles.css`.
   - Write and pass 5 Playwright component/unit tests in `tests/playwright/unit/humane-traps.spec.ts`.

5. **Task 5: Automated Playwright E2E User Journey & Full Repository Verification**
   - Implement the 6-step end-to-end user journey in `tests/playwright/e2e/humane-traps-journey.spec.ts`.
   - Run `make verify` and the full Playwright suite (`202/202` tests).
