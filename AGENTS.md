# AGENTS.md

This file is the single source of truth for this repository: commands,
layout, criteria, and traps. `CLAUDE.md` and `GEMINI.md` only point here,
so a rule recorded anywhere else is invisible to whichever agent reads a
different entrypoint. Record it here.

## Project overview

Event-driven Go microservices for reporting and matching lost pets, running on
GCP (Cloud Run, Pub/Sub, Firestore, GCS) and locally via Docker Compose plus
Ollama `gemma4:e2b`. Five services: four expose JSON HTTP endpoints or run as
Pub/Sub workers; `web-frontend` serves rendered HTML, CSS, and JavaScript
alongside JSON API endpoints. The sixth entrypoint, `demo-seed`, is a one-shot
CLI rather than a service.

- **Base branch**: `main`, protected — see [Git & CI](#git--ci).
- **UI Domain**: Responsive Web, but only for `internal/app/webfrontend`
  (templates, `static/css`, `static/js`, `static/sw.js`). Every other package is
  headless.

## Repo Map

| Path | Contents |
| --- | --- |
| `cmd/` | Entrypoints: `lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service`, `web-frontend`, `demo-seed` |
| `internal/app/` | Service implementations: `lostpet`, `foundpet`, `petmatcher`, `notification`, `outboxrecovery`, `webfrontend` (the last owns `templates/` and `static/`) |
| `pkg/` | Shared packages: `domain`, `store`, `pubsub`, `outbox`, `delivery`, `blob`, `ollama`, `scoring`, `runtimeconfig`, `identity`, `telemetry` |
| `e2e/` | Event-cascade tests: in-memory cascades plus emulator-gated Firestore and Pub/Sub contracts |
| `tests/playwright/e2e/` | API request journeys and browser page specs (`*.spec.ts`) |
| `infra/opentofu/`, `deploy/cloudrun/` | GCP infrastructure modules and Cloud Run manifests |
| `docs/DEVELOPMENT.md` | Runtime modes and the per-area emulator journeys |

Go tests live beside their source as `*_test.go`; copy the shape of a neighbour
in the package you are changing. Nothing is vendored. The only tool-written
files are the two lockfiles — `go.sum` (regenerate with `go mod tidy`) and
`tests/playwright/package-lock.json` (`npm install`) — which are updated by
their tool, never edited by hand.

## Development Commands

**Every version below is pinned to match CI** (`.github/workflows/ci.yml`).
Reproduce a CI failure with the pinned version, never `@latest` — a floating
linter once produced a green local run and a red CI on the same commit. Changing
a version here means changing it in `ci.yml` in the same commit. Green is
exit code 0.

| Gate | Command | When |
| --- | --- | --- |
| Vet | `go vet ./...` | Always |
| Tests | `go test -race -cover ./...` | Always |
| Go lint | `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run` | Always |
| Infra | `cd infra/opentofu && tofu fmt -check -recursive && tofu init -backend=false && tofu validate` | `infra/` changed |
| Playwright | Bring up the three services (see [Local Setup](#local-setup)), then `cd tests/playwright && npm ci && npx playwright install chromium && npx playwright test` | HTTP behavior, rendered pages, or contracts changed |

Toolchains, checked before running anything:

- **Go `1.26.5`** — `go version` must report it. `go.mod` declares `go 1.25.8`,
  which is the *language* version and pins nothing; if the local toolchain
  differs, prefix the Go commands with `GOTOOLCHAIN=go1.26.5`. Never report a Go
  gate as passed on a different toolchain.
- **Node `24`** — `node -v` reports the system default, so a different major
  silently resolves different transitive dependencies than CI. Route the
  Playwright commands through the pinned one instead:
  `mise x node@24 -- npx ...` or your version manager's equivalent.
- **OpenTofu `1.12.5`** — `tofu fmt` output differs between versions.

**The baseline is green.** On 2026-09-11, on a host running Go 1.26.6 routed
through `GOTOOLCHAIN=go1.26.5`, all three always-on gates exited 0 —
`go vet ./...` silent, `go test -race -cover ./...` with every package `ok`, and
`golangci-lint run` printing `0 issues.`. A failure in those three is yours, not
inherited. When a gate's binary is absent (`tofu` and `gcloud` are not installed
everywhere), report it `NOT RUN` with that reason rather than as a skip.

What a green run does **not** license a claim about: the emulator-gated
contracts, which skip silently (see [Gotchas](#gotchas--troubleshooting)); live
Ollama, real Pub/Sub, or a deployed GCP cascade, which no documented command
exercises; and color contrast, which nothing in the repository checks at all
(see [Architecture & Conventions](#architecture--conventions)). Coverage
percentages are informational, not a gate — no threshold is enforced anywhere.

## Local Setup

No credentials are needed for any command documented here. Never commit
service-account JSON, `*.tfstate`, `.env` files, API keys, real GCP project
identifiers, or agent tooling scratch state (such as `.superpowers/`); if a task
appears to need one, stop and ask.

**Runtime mode.** The five stateful processes choose their state backend from
`PETSPOTR_RUNTIME_MODE`: `memory` (no configuration, and the default outside
Cloud Run), `local-emulator` (needs `GOOGLE_CLOUD_PROJECT` plus the emulator
hosts below), or `gcp` (Application Default Credentials; Cloud Run selects it
automatically and rejects the other two). `docs/DEVELOPMENT.md` § *State Runtime
Modes* is the contract.

| Purpose | Command | Host ports |
| --- | --- | --- |
| Playwright services | `docker compose up --build --detach lostpet-service foundpet-service web-frontend` | 8080, 8081, 8082 |
| UI bring-up | `docker compose up --build -d web-frontend`, then `docker compose rm -sf web-frontend` | 8082 |
| Full stack | the above plus `pet-matcher`, `notification-service`, `ollama` | 8083, 8084; Ollama is network-internal |

Stop only the services you started. Ollama pulls `${OLLAMA_MODEL:-gemma4:e2b}`
on first start, so the Playwright and UI sets deliberately leave it out.

**Emulators.** Export the host variable for the area under test —
`FIRESTORE_EMULATOR_HOST=127.0.0.1:8085`,
`PUBSUB_EMULATOR_HOST=127.0.0.1:8086`,
`FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099` (that port is fixed by
`firebase.json`). Only the Auth emulator has a documented, pinned start command:

```bash
mise x node@24 -- npx --yes firebase-tools@15.27.0 \
  emulators:start --only auth --project demo-petspotr-auth
```

<!-- unverified: no start command for the Firestore or Pub/Sub emulator is
documented anywhere in the repository, and gcloud was absent on the host this
was profiled from — supply the endpoints above from whatever emulator you run -->

The per-area `go test -run ...` invocations live in `docs/DEVELOPMENT.md`: run
the identity journey when human identity changes, and the durable-state, outbox,
delivery, or idempotency contracts when those change.

**Fixtures.** `go run ./cmd/demo-seed` writes the fixed-ID match fixtures
`match-101` and `match-102` into a `local-emulator` Firestore. It refuses
`memory` and `gcp` so it cannot write ephemeral or deployed data, and rerunning
it replaces both documents — use a dedicated emulator project.

**Playwright URLs.** The specs read `LOSTPET_SERVICE_URL`,
`FOUNDPET_SERVICE_URL`, and `WEB_FRONTEND_URL`, falling back to `BASE_URL` and
then to `localhost:8080`, `:8081`, `:8082`. Those defaults match the Compose
ports, so a local run needs no environment at all.

## Architecture & Conventions

These are the invariants a review is judged against. Deterministic formatting,
lint, build, and test checks belong to the gate, not to a reviewer.

- **Pub/Sub handlers stay idempotent under redelivery.** Flag writes,
  notifications, or state transitions that duplicate when the same event
  arrives twice; use a stable event or deduplication key, or make the operation
  inherently idempotent.
- **Event schema changes stay backward compatible with in-flight messages.**
  Prefer additive fields with tolerant readers; otherwise version the schema
  explicitly and keep a decoder or migration path for the prior one.
- **Intentionally breaking a published contract is a stop-and-ask, not a review
  item.** The rule above is what a reviewer applies to a change that means to
  stay compatible. A change that knowingly breaks or versions an event or HTTP
  schema reaches messages already in flight and consumers already deployed —
  which no gate in this repository can catch and no revert can recall. Say what
  breaks, and get agreement, before implementing it.
- **Reporter contact details and state-changing actions never cross an
  unauthenticated or unauthorized boundary.** New or changed boundaries return
  redacted public DTOs and require authentication plus ownership, or equivalent
  authorization, before exposing contact data or mutating pet and match state.
  Existing pre-authentication demo behavior is roadmap debt, not precedent for
  widening the exposed boundary.
- **WCAG AA contrast is a review criterion, because no gate checks it.** The
  Playwright suite mechanizes most of the rendered-UI rubric — no horizontal
  overflow and 44px touch targets at 390px
  (`tests/playwright/e2e/mobile-navigation.spec.ts:13,43`), header containment
  and keyboard focus order across 769–1280px
  (`tests/playwright/e2e/web-frontend-api.spec.ts:817`) — but nothing anywhere
  in the repository computes contrast: there is no `axe`, no `getComputedStyle`
  contrast assertion, and no luminance helper. A rendered-UI change must have
  its text and interactive-icon contrast judged against WCAG AA by a reviewer,
  in **both** themes, because `.glass-nav` translucency and the theme toggle
  change effective background per surface. A green Playwright run is not
  evidence of contrast.
- **Tests exercise product code or product behavior.** Do not add tests that
  inspect repository files — documentation, agent guidance, CI workflows,
  dependency metadata, Docker or deployment manifests, OpenTofu source. Validate
  those with their own lint, build, or validation command instead.

## Gotchas & Troubleshooting

- **Emulator-gated contracts skip silently.** Files across `pkg/store`,
  `pkg/outbox`, `pkg/runtimeconfig`, `internal/app/petmatcher`,
  `internal/app/notification`, and `e2e/` skip unless their emulator host
  variable is set, and the package still reports `ok`. `pkg/identity` is **not**
  among them — its two test files carry no gating and always run; the Firebase
  Auth journeys it looks like it owns live in `e2e/identity_session_test.go` and
  `pkg/runtimeconfig/identity_test.go`. A green
  `go test -race -cover ./...` is not evidence that those contracts ran, and CI
  starts no emulator either.
- **`.golangci.yml` uses the v2 config schema**, which a v1 binary cannot parse.
  The pinned `github.com/golangci/golangci-lint/v2/...@v2.12.2` invocation is
  not interchangeable with an installed `golangci-lint`.
- **`golangci-lint` prints noise that is not findings.** Its cache can emit
  `level=warning ... no such file or directory` lines naming paths from a
  previous checkout location. The decisive output is the trailing `N issues.`
  line and the exit code; a run can print several warnings and still be clean.
- **`docker compose down` tears down the whole project**, including a stack the
  user or an earlier step is using. Stop only the services you started with
  `docker compose rm -sf <service>`.
- **`BLOCKED` with every check green means the branch is behind `main`.**
  Required checks are strict; update from `main`, let CI rerun, and confirm a
  clean state again.

## Verification Map

| A fix touches | Rerun |
| --- | --- |
| `cmd/`, `internal/`, `pkg/`, or `e2e/` | `go vet ./...`, `go test -race -cover ./...`, `golangci-lint run` |
| `internal/app/webfrontend/` or any HTTP handler contract | the Go gate above, plus the Playwright suite |
| `*.md` at the root or under `docs/` | none — no Go, Playwright, or OpenTofu command reads Markdown |
| `infra/` | `tofu fmt -check -recursive`, `tofu init -backend=false && tofu validate` |
| `tests/playwright/` | the Playwright suite |
| `go.mod`, `go.sum`, `.golangci.yml`, `Dockerfile`, `docker-compose.yml`, `.github/workflows/` | the complete gate |
| anything else | the complete gate |

## Git & CI

- **Always branch from `origin/main`.** Never commit directly to `main`. Every
  change starts by fetching `origin/main` and creating a dedicated branch:
  `git checkout -b <owner>/<type>/<short-description> origin/main` (where `<type>`
  is one of `feat`, `fix`, `refactor`, `chore`, `test`, `docs`).
- **Always deliver changes via Pull Request.** Never push directly to `main`.
  Push the branch and open a ready-for-review pull request (`gh pr create`).
- **Always squash merge.** All pull requests are squash-merged into `main`
  (`gh pr merge --squash --delete-branch`), keeping history linear. Once merged,
  delete the branch both remotely and locally.
- **`main` is protected** by the `main-protection` ruleset (id `20470880`) with
  no bypass actors — it applies to repository owners too. Pull request required,
  squash the only permitted merge method, no deletion, no force-push. Required
  checks are **strict**, so a branch must also be current with `main`. Zero
  approving reviews are required and PRs #220–#222 each merged with none, so
  self-merge works. The ruleset also sets
  `require_extra_approval_for_unattributed_changes`; nothing observed has been
  blocked by it, but it is the first thing to check if a green pull request ever
  stalls waiting for an approval.
- **A required check context is the job's `name:` value, not its id.** The
  ruleset stores the display names, so renaming a job in `ci.yml` without
  updating the ruleset leaves a required check that can never report, which
  blocks every pull request:

  | Job in `ci.yml` | Required check context | Required? |
  | --- | --- | --- |
  | `pr-title` | `Validate PR Title` | yes |
  | `go-checks` | `Go Build & Unit Tests` | yes |
  | `infra-checks` | `OpenTofu Format & Validate` | yes |
  | `e2e-playwright-tests` | `Playwright API Journeys` | yes |

- **All four checks are required; none can be bypassed.** The Playwright job
  builds the three HTTP services, waits for readiness, runs the suite, and
  uploads its report, traces, and service logs on failure. It is the only
  enforcement for the rendered-UI rubric the specs mechanize — viewports,
  overflow, stacking, touch targets, and keyboard focus order — so a change that
  breaks them can no longer merge. Budget roughly three minutes for it.
- **The PR title becomes the commit subject on `main`,** because branches are
  squash-merged; `pr-title` validates it as a Conventional Commit.
- **Link the tracked issue in the PR body**: `Closes #<n>` only when this slice
  completes the issue, `Part of #<n>` for every earlier slice of an ordered
  split. The squash carries the body onto `main`, so `Closes` on an intermediate
  slice closes an issue that is still outstanding.
- **Stop and ask before changing what enforces the gates.** Actions that alter
  `.github/workflows/`, the `main-protection` ruleset, or `infra/` state can
  weaken future checks without a diff that looks dangerous. In this repository
  those are a stop-and-ask regardless.
- **Keep the commit subject to 72 characters or fewer**, and open pull requests
  ready for review — no drafts unless the user asks for one.
- **File out-of-scope discoveries** with `gh issue create` rather than widening
  the slice: title them as Conventional Commits, label them `agent-found`, and
  reference the branch that surfaced them. This is pre-authorized.
- **Run local verification before opening a pull request.** Ensure all gates
  pass locally rather than relying solely on CI.
