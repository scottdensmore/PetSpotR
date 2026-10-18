# Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger — Implementation Plan

- **Milestone:** 13.1
- **Spec:** `docs/superpowers/specs/2026-10-17-pet-custody-chain-reunion-ledger-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-custody-chain-ledger` (`feat/custody-chain-ledger`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collection & Validation (`pkg/domain/custody_ledger.go`, `pkg/store/store.go`)
- **Files:**
  - Create `pkg/domain/custody_ledger.go`
  - Create `pkg/domain/custody_ledger_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `CustodyStage`, `OwnershipProofMethod`, `CustodyTransferHop`, and `PetCustodyChain` with validation methods (`IsValid()`, `Validate()`).
  - Add `CollectionPetCustodyChains = "pet_custody_chains"` in `pkg/store/store.go`.
  - Write unit tests in `pkg/domain/custody_ledger_test.go`.

### Task 2: Pure-Go SHA-256 Hash-Chain Ledger, Tamper Detector & Dual-Party PIN Escrow Engine (`pkg/custody`)
- **Files:**
  - Create `pkg/custody/ledger.go`
  - Create `pkg/custody/escrow.go`
  - Create `pkg/custody/custody_test.go`
- **Details:**
  - Implement `ComputeHopHashes`, `AppendCustodyHop`, `VerifyCustodyChainIntegrity`, `GenerateHandoverPIN`, and `VerifyAndCompleteReunionHandover`.
  - Write unit tests in `pkg/custody/custody_test.go` covering genesis & multi-hop hash chaining, tamper detection when a hop's custodian/timestamp/hash is altered, deterministic 6-digit PIN generation, wrong-PIN rejection (`ErrInvalidHandoverPIN`), and completed reunion certificate issuance.

### Task 3: WebFrontend REST API, Dual-Party PIN Gate, Integrity Verifier & SSE/Mesh Stream (`internal/app/webfrontend/custody_handlers.go`)
- **Files:**
  - Create `internal/app/webfrontend/custody_handlers.go`
  - Create `internal/app/webfrontend/k9_or_custody_test` -> `internal/app/webfrontend/custody_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
  - Create stub `internal/app/webfrontend/templates/custody_ledger.html`
- **Details:**
  - Register routes in `server.go`:
    - `GET /custody-ledger` -> `s.handleCustodyLedgerPage`
    - `GET /api/v1/custody/chains` -> `s.handleListCustodyChains`
    - `POST /api/v1/custody/chains` -> `s.handleCreateCustodyChain`
    - `POST /api/v1/custody/chains/{id}/transfer` -> `s.handleAppendCustodyTransfer`
    - `POST /api/v1/custody/chains/{id}/reunite` -> `s.handleCompleteReunionHandover`
    - `GET /api/v1/custody/chains/{id}/verify` -> `s.handleVerifyCustodyChain`
    - `GET /api/v1/custody/stream` -> `s.handleCustodyStream`

### Task 4: Pet Custody Chain & Cryptographic Handover Cockpit UI (`/custody-ledger`), Printable Certificate & WCAG AAA CSS
- **Files:**
  - Update `internal/app/webfrontend/templates/custody_ledger.html`
  - Create `internal/app/webfrontend/static/js/custody-ledger.js`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/custody-ledger.spec.ts`
- **Details:**
  - Build `/custody-ledger` featuring:
    - **Section 1: Active Pet Custody Chains & SHA-256 Integrity Monitor (`#custody-chains-list`, `#form-create-custody-chain`)**
    - **Section 2: Custody Transfer Hop Appender & Dual-Party PIN Reunion Escrow (`#form-append-custody-hop`, `#form-complete-reunion`, `#pin-gate-error-alert`)**
    - **Section 3: Hash-Chain Timeline Inspector & Printable Custody Waiver Certificate (`#custody-hops-timeline`, `#reunion-certificate-card`)**
  - Enforce WCAG AAA (`> 7:1`) contrast across dark and light modes.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/custody-ledger-journey.spec.ts`
- **Details:**
  - 6-step E2E journey verifying seeded custody chains, initiating a new pet custody chain with genesis SHA-256 hash, appending intermediate custody transfer hops (`FIELD_SAR_ESCORT` -> `OWNER_REUNIFICATION_ESCROW`), verifying `403 Forbidden` rejection on wrong Handover PIN, completing Dual-Party Reunion Handover with the valid 6-digit PIN + microchip proof, and verifying the Printable Custody Waiver Certificate (`CERT-REUNION-...`) and cryptographic audit endpoint (`/api/v1/custody/chains/{id}/verify`).

### Task 6: Final Whole-Branch Code Review & Verification Gate
- **Details:**
  - Run whole-branch review, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass, push, open PR (#286), and squash-merge into `main`.
