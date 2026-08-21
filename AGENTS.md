# AGENTS.md

This file is the single source of truth for this repository. The managed block
at the end defines the workflow every agent follows; everything above it is the
project half — commands, layout, criteria, and traps — that the workflow relies
on. `CLAUDE.md` and `GEMINI.md` only point here, and the workflow's subagents
carry no project knowledge on purpose, so a rule recorded anywhere else is
invisible to whichever agent reads a different entrypoint. Record it here.

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
  headless, and a change confined to them skips UI review.

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
| Markdown lint | `npx --yes markdownlint-cli@0.49.1 --config .markdownlint.json "AGENTS.md" "CLAUDE.md" "GEMINI.md" "README.md" "docs/**/*.md"` | Markdown changed |
| Infra | `cd infra/opentofu && tofu fmt -check -recursive && tofu init -backend=false && tofu validate` | `infra/` changed |
| Playwright | Bring up the three services (see [Local Setup](#local-setup)), then `cd tests/playwright && npm ci && npx playwright install chromium && npx playwright test` | HTTP behavior, rendered pages, or contracts changed |

Toolchains, checked before running anything:

- **Go `1.26.5`** — `go version` must report it. `go.mod` declares `go 1.25.8`,
  which is the *language* version and pins nothing; if the local toolchain
  differs, prefix the Go commands with `GOTOOLCHAIN=go1.26.5`. Never report a Go
  gate as passed on a different toolchain.
- **Node `24`** — `node -v` reports the system default, so a different major
  silently resolves different transitive dependencies than CI. Route the
  markdownlint and Playwright commands through the pinned one instead:
  `mise x node@24 -- npx ...` or your version manager's equivalent.
- **OpenTofu `1.12.5`** — `tofu fmt` output differs between versions.

**The baseline is green.** On 2026-08-20, on a host running Go 1.26.6 routed
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
service-account JSON, `*.tfstate`, `.env` files, API keys, or real GCP project
identifiers; if a task appears to need one, stop and ask.

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
- **The markdownlint file list is hardcoded in `ci.yml`.** Renaming or adding a
  top-level doc requires updating the workflow in the same commit, or CI lints a
  file that no longer exists — or silently never lints a new one.
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
| `*.md` at the root or under `docs/`, or `.markdownlint.json` | markdownlint only — no Go, Playwright, or OpenTofu command reads Markdown |
| `infra/` | `tofu fmt -check -recursive`, `tofu init -backend=false && tofu validate` |
| `tests/playwright/` | the Playwright suite |
| `go.mod`, `go.sum`, `.golangci.yml`, `Dockerfile`, `docker-compose.yml`, `.github/workflows/` | the complete gate |
| anything else | the complete gate |

## Git & CI

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
  | `static-checks` | `Static Analysis & Markdown Linting` | yes |
  | `go-checks` | `Go Build & Unit Tests` | yes |
  | `infra-checks` | `OpenTofu Format & Validate` | yes |
  | `e2e-playwright-tests` | `Playwright API Journeys` | yes |

- **All five checks are required; none can be bypassed.** The Playwright job
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
- **File out-of-scope discoveries** with `gh issue create` rather than widening
  the slice: title them as Conventional Commits, label them `agent-found`, and
  reference the branch that surfaced them. This is pre-authorized.
- **CI is not the verification stage.** A green pipeline does not replace the
  independent `verifier` report the workflow requires before code review.

<!-- The managed block below is written by agent-workflow-skills and is
     overwritten on every re-adoption, so it cannot be reflowed to this
     repository's Markdown style. Exempt it rather than editing it. -->
<!-- markdownlint-disable MD013 MD060 -->
<!-- agent-skills:begin workflow 185672e4 — managed block, edits here are overwritten -->
## Development Workflow

Follow these stages in order (governed by the global `agent-workflow-skills`). Scale the pipeline to the
size of the change using the triage table — skipping a stage is a decision to
state out loud, never a shortcut taken silently. A stage in parentheses applies
only when its own entry says it does.

| Track | When | Stages |
|---|---|---|
| **Trivial** | Docs, comments, typos, config with no logic change | 1 → 6 → 9 |
| **Single fix** | One bug or small change with a clear, contained cause | 1 → 2 → 5 → 6 → (7) → 8 → 9 |
| **Feature** | New behavior, several files, or an architectural choice | All stages; repeat 5–8 per slice |

**Division of labor.** The main agent runs only focused checks — the single test
it just wrote, a formatter over the files it just touched. Whole suites, builds,
dependency audits, and repository-wide lint go to the **`verifier`** subagent;
reviews go to **`code-reviewer`** and **`ui-reviewer`**. Each follows the skill
of the same job (`verifier`, `code-review`, `ui-review`), reads this file for
what the project's commands and criteria are, and is declared without
file-editing tools — a read-only sandbox where the host supports one. This is
not ceremony: it keeps routine command output out of the implementation context,
and it means each gate is read by something that has not already convinced
itself the change is correct. If a subagent is unavailable, run the stage
inline against the same skill and say that you did.

**Stages end.** Every delegated stage returns a verdict, and a verdict is acted
on once. Fix what came back, then rerun only the stage whose inputs your fix
touched. If the same finding survives two attempts, stop and report it with what
you tried — do not loop. Never rerun a stage against a state it has already
seen; an unchanged tree yields an unchanged verdict.

**Preserve what you did not change.** A worktree may hold work that is not yours.
Never stage, revert, or "clean up" a change you did not make; when something
unrelated is in the way, name it and leave it alone.

**Claim only what you observed.** A gate licenses a statement about exactly
what it measured and nothing more: a green build says the code compiles, not
that the feature works; a passing test says that test passed, not that the bug
is gone. If you did not run it, say you did not. "I believe this fixes it" is a
usable sentence; "fixed and verified" without a command and its output is not.

**Say what you assumed.** When a choice would change what gets delivered and the
request does not settle it, ask before building rather than after. When it is
too small to be worth asking, decide, and write the assumption where a reviewer
will see it. An assumption nobody can see is indistinguishable from a mistake.

**Instructions are part of the change.** When a command, a behavior, or a
constraint changes, the file that documents it changes in the same commit —
`AGENTS.md`, the Verification Map, the README, whichever is now wrong. Stale
instructions are worse than missing ones, because the next agent follows them
confidently.

1. **Inspect & Branch**: Inspect `git status`, the current branch, and every
   applicable instruction file before touching anything. Note unrelated staged,
   unstaged, and untracked work so you can preserve it. Fetch the base branch
   (`git fetch origin main`) and create a dedicated branch:
   `git checkout -b <owner>/<type>/<short-description> origin/main`.
   `<owner>` is your GitHub login (`gh api user --jq .login`); `<type>` is one of
   `feat`, `fix`, `refactor`, `chore`, `test`, `docs`. Never commit to `main`.
2. **Plan & Slice (`plan-and-prototype`)**:
   - **Read before you plan.** Open the code the change will touch, its tests, and
     its call sites. A plan written without reading them is a guess about a
     codebase rather than a plan for this one.
   - Formulate a clear step-by-step plan before writing code. Define the smallest
     end-to-end slice that can be reviewed, tested, and shipped independently; if
     the work is too large for one pull request, order the slices and complete only
     the current one.
   - **A slice is vertical, not horizontal.** It goes through every layer of one
     narrow thing and ends in something you can actually verify: "add the new field
     end to end, with tests" is a slice; "rename the field everywhere" is a sweep.
     One concern per branch — if a change spans unrelated concerns, that is two
     branches.
   - **A new dependency is an architectural decision, not an implementation
     detail.** Say what it replaces, why writing that yourself is the worse option,
     and what its license and maintenance status are. Adding one silently is how a
     project acquires a liability nobody chose.
3. **Prototype Options (if needed)**: When facing architectural choices, unfamiliar
   APIs, or UX alternatives, spike lightweight prototypes and compare trade-offs
   before committing to an approach.
4. **Track Bugs & Follow-ups**: When bugs, edge cases, technical debt, or follow-up
   tasks surface mid-change, file them immediately (`gh issue create`, the project's
   tracker, or `ISSUES.md` when none is configured) instead of expanding the current
   slice.
5. **Test-Driven Development (`tdd-workflow`)**:
   - Write/update a focused test first → confirm it fails for the expected reason →
     minimal implementation → iterate until passing → refactor. A test that passes
     before the code exists is testing the wrong thing.
   - **When the change replaces an existing contract, find the tests pinning the old
     one first.** A new failing test proves the new behavior is missing; it says
     nothing about tests still asserting the behavior being removed. Search for
     assertions on the symbol, attribute, label, or role being changed and update
     them inside the same red/green loop. Skipping this is silently safe — the new
     test goes green, the loop looks complete, and the contradiction only surfaces a
     full gate cycle later.
   - **A test that has never failed is not evidence of anything.** When you add a
     regression detector, break the thing it guards and confirm it catches it, then
     put it back. A detector that cannot be shown to fire is decoration.
   - Run only the test you authored or changed, filtered by file and name. Whole
     suites are stage 6's job.
   - Pure logic (calculations, state machines, business rules) must be unit-tested.
     Non-testable areas (rendering, audio) must be visually/interactively verified.
6. **Verification (`verifier` subagent → `verifier` skill)**:
   - Run the project's full gate: lint, type-check, test suites, build. Focused runs
     from stage 5 do not substitute for it.
   - **Know what green looked like before you started.** If you do not know the
     gate passed on the state you began from, establish that first. Without it a
     failure is ambiguous — you cannot tell what you broke from what you inherited,
     and every later decision rests on that distinction.
   - **Measure the thing you ship, not a proxy for it.** A gate that checks part of
     the output, or a stand-in for it, reads exactly like one that checks all of it
     — and certifies the rest by silence. If a command covers less than it appears
     to, say what it left out.
   - The subagent runs and reports; fixing is yours. Resolve every actionable
     finding before code review. When a fix changes code, rerun the affected focused
     tests, then ask for only the gate commands whose inputs the fix touched — see
     **Verification Map** below if this project defines one. The complete gate must
     run in full at least once on the state that enters code review.
   - Some findings are environmental and no code change resolves them (browsers that
     will not install, no network, a missing credential). Resolving those means
     naming them precisely — what ran, what did not, and why — not retrying them.
7. **UI Review (`ui-reviewer` → `ui-review`)**:
   - Runs after verification, so the tree builds before anyone looks at it.
   - **Check whether this stage applies before delegating.** It applies only when
     the change can alter something a person sees or interacts with. A change
     confined to documentation, comments, configuration, build scripts, CI, tests,
     or code with no rendered output does not qualify — skip the stage, record one
     line saying which of those it was, and move on. A docs-only or test-only diff
     never needs a UI review.
   - When it does apply, audit layout, visual hierarchy, contrast (WCAG AA),
     interaction states, and accessibility according to the project's UI domain.
   - A project whose UI domain is headless or backend skips this stage every time.
   - Never invent findings to justify the stage, and never describe an appearance
     that was not observed running.
8. **Code Review (`code-reviewer` → `code-review`)**:
   - The reviewer reads the complete change: `git diff origin/main...HEAD`,
     plus staged and unstaged edits (`git diff HEAD`) and untracked files (`git
     status --porcelain`). It reports; it does not edit. **You** remove the
     accidental or unrelated edits it names, and preserve anything that is the
     user's.
   - Enforce architectural boundaries, language idioms, defensive error handling,
     and zero committed secrets.
   - Do not repeat this review on an unchanged state. Rerun it only when the
     reviewed content actually changed.
9. **Commit & PR Lifecycle (`slice-and-pr`)**:
   - **Close the loop against the request.** Re-read what was actually asked for,
     and state how this change satisfies it — and what it deliberately does not.
     Every gate above proves the code works; none of them prove it is the thing
     that was wanted. A green pipeline on the wrong feature is the most expensive
     outcome available.
   - Commit using Conventional Commits (`<type>(<scope>): <summary>`). Stage files
     explicitly; never `git add -A` when unrelated work is present.
   - **Match the stopping point to the request.** A request that only asks to
     commit stops after the local commit. A request that asks to use, follow, or
     complete the workflow—including "commit based on the workflow"—includes the
     reversible remote steps: push the branch, open the PR, and watch its checks.
     It does not authorize a merge or any action named under **Stop there and
     report**.
   - Open the PR with `gh pr create` and watch CI with `gh pr checks --watch`.
   - **The description carries the evidence.** Say why the change exists, what it
     changes grouped by concern rather than by file, and how it was tested — the
     command you actually ran and its actual result. "Should work" is not a test
     result. If a test was added, say what it would have caught.
   - **Stop there and report.** Anything you cannot take back needs explicit
     approval from the user in the current conversation: merging (`gh pr merge`),
     force-pushing, rewriting shared history, deleting a branch or tag, dropping
     or migrating data, removing files wholesale, and publishing or deploying.
     Approval for one of them is not approval for the next.
   - **Squash, unless this project says otherwise.** One reviewed slice lands as
     one commit on the base branch. The false starts, the fixups and the "address
     review" commits are how the work got made, not what it is; keeping them turns
     the base branch's history into a diary and makes a revert an archaeology
     exercise. Because the PR description is what survives, it has to carry the
     reasoning — see above. A project that requires merge commits or a rebase says
     so in its own section, and that wins.
   - **A merge takes its branch with it.** Once a merge is approved and done,
     delete that branch — remote and local, in the same step. It is the one
     deletion the merge approval covers, because it is the merge finishing rather
     than a separate act; no other branch is included. A merged branch left
     behind is a decoy: it looks like work in flight, and the next person cannot
     tell it from the real thing without checking.
   - Verify before deleting, and be aware of the squash case: a squash merge
     writes a new commit rather than joining histories, so git sees no ancestry
     and `git branch -d` refuses a branch whose every line is already merged.
     Confirm with `git diff <base> <branch>` — empty output means nothing is
     lost — and then `-D` is correct rather than reckless. If that diff is *not*
     empty, stop: something did not make it in.
<!-- agent-skills:end workflow -->
<!-- markdownlint-enable MD013 MD060 -->
