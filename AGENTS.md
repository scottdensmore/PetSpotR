# Agent Workflow Guidelines

This document defines the standard workflow required for all AI coding agents
working on this codebase, and it is the single source of truth for that
workflow. Every agent reads this file. Tool-specific entrypoints such as
`CLAUDE.md` only point here — never add rules, commands, or project facts to
them, because agents that read a different entrypoint will not see them. Add it
here instead. Registered subagent definitions may describe their role and
reporting format, but they must defer project facts, gate applicability, and
verification commands to this file.

## The project

Event-driven Go microservices for reporting and matching lost pets, running on
GCP (Cloud Run, Pub/Sub, Firestore, GCS) and locally via Docker Compose plus
Ollama `gemma4:e2b`.

The repository has five services. Four expose JSON HTTP endpoints or run as
Pub/Sub workers; `web-frontend` serves rendered HTML, CSS, JavaScript, and JSON
API endpoints. The Playwright suite contains both API-level request journeys
and browser-driven page coverage.

| Path | Contents |
| --- | --- |
| `cmd/` | Service entrypoints: `lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service`, `web-frontend` |
| `pkg/` | Shared packages: `domain`, `store`, `pubsub`, `outbox`, `delivery`, `blob`, `ollama`, `scoring`, `runtimeconfig`, `telemetry` |
| `e2e/` | Event-cascade tests: in-memory/test-server cascades plus emulator-gated Firestore and Pub/Sub contracts |
| `tests/playwright/` | API and browser journeys using three local HTTP services |
| `infra/opentofu/` | GCP infrastructure modules |
| `deploy/cloudrun/` | Cloud Run manifests |
| `docs/` | `DEVELOPMENT.md` |

## Code Review Rules

These rules guide local reviewers. Keep deterministic formatting, lint, build,
and test checks in the verifier and CI.

- Pub/Sub handlers must remain idempotent under redelivery. Flag writes,
  notifications, or state transitions that can be duplicated when the same
  event is delivered more than once; use a stable event or deduplication key,
  or make the operation inherently idempotent.
- Event schema changes must remain backward compatible with in-flight
  messages. Prefer additive fields with tolerant readers; otherwise introduce
  an explicit version and preserve a decoder or migration path for the prior
  schema.
- Reporter contact details and state-changing actions must not cross an
  unauthenticated or unauthorized boundary. Apply this rule to new or changed
  boundaries: return redacted public DTOs and require authentication plus
  ownership or equivalent authorization before exposing contact data or
  mutating pet and match state. Existing pre-authentication demo behavior is
  roadmap debt, not precedent for expanding the exposed boundary.
- Automated tests must exercise product code or product behavior. Do not add
  tests that inspect repository files such as documentation, agent guidance,
  CI workflows, dependency metadata, Docker or deployment manifests, or
  OpenTofu source. Validate those artifacts with their native lint, build,
  configuration, or validation commands instead.

## Ground rules

These apply at every step, not just at the gate where they are mentioned.

- **Never commit directly to `main`.** Never force-push, never amend or rebase
  a commit that has already been pushed, and never discard user work with
  `git checkout --`, `git restore`, `git clean`, or `git reset --hard`.
- **Preserve unrelated work.** Staged, unstaged, and untracked files that you
  did not create belong to the user. Leave them alone.
- **Never commit secrets.** No service-account JSON, `*.tfstate`, `.env` files,
  API keys, or real GCP project identifiers. If you need a credential to
  proceed, stop and ask.
- **Stop and ask the user** rather than pushing through when: a gate fails
  twice for the same reason; the task requires a credential, a paid resource,
  or a destructive operation; the change would alter published API contracts,
  infrastructure state, or CI enforcement; or the request is ambiguous enough
  that two readings produce materially different work.

## Workflow

1. **Inspect before changing anything.** Inspect the repository, current Git
   state, and all applicable instruction files before making changes.

2. **Choose a thin vertical slice.** Before implementing a tracked issue or
   feature, define the smallest end-to-end slice that can be reviewed, tested,
   shipped, and merged independently. Prefer one coherent user-visible or
   operational outcome over a broad horizontal layer. If the next issue is too
   large for one pull request, split it into ordered slices and complete only
   the current slice. Keep pull requests small enough for thorough review,
   reliable verification, and quick rollback.

3. **Create a branch for that slice.** Branch from the latest `main` using
   `<type>/<short-kebab-summary>`, where `<type>` is one of `feat`, `fix`,
   `refactor`, `chore`, `test`, or `docs` — matching the Conventional Commit
   type the work will land under. Example: `feat/foundpet-image-upload`.

4. **Use test-driven development when behavior or structure is testable.**
   - Add or update a focused test before implementation.
   - Run it and confirm it fails for the expected reason.
   - Implement the smallest appropriate change.
   - Run focused tests while iterating.
   - Refactor only while the relevant tests remain green.
   - Follow [Test execution ownership](#test-execution-ownership) so the main
     agent retains only focused Go test output and the verifier owns noisy or
     repository-wide execution.

5. **Inspect the complete diff and track out-of-scope discoveries.** Review the
   branch diff plus all staged, unstaged, and untracked files. Remove
   accidental or unrelated changes while preserving work that belongs to the
   user. Whenever you discover bugs, technical debt, missing features, or
   refactoring opportunities not aligned with the current slice, file them with
   `gh issue create` rather than expanding scope. Title them as Conventional
   Commits, label them `agent-found`, and reference the branch that surfaced
   them. Filing these issues is pre-authorized; you do not need to ask first.

6. **Run `ui-review` when the change can affect rendered UI.** Invoke the
   `ui-review` sub-agent after an implementation pass. See
   [Applicability](#applicability) for when this gate is live. When it applies,
   exercise the changed journey in the rendered application at representative
   phone, tablet, and desktop viewports; inspect interaction, loading, empty,
   error, focus, keyboard, contrast, and responsive states; and capture
   screenshots or equivalent visual evidence. Address every actionable finding
   before running the `verifier`. When the change has no UI impact, record one
   line stating that and move on — do not fabricate a review.

   Bring the rendered app up with `docker compose up --build -d web-frontend`
   (port 8082; it needs neither Ollama nor the other services), then stop only
   what you started with `docker compose rm -sf web-frontend` — a project-wide
   `down` would tear down a stack the user or an earlier step is using. The `-d`
   spelling is deliberate: it keeps this line from colliding with the guarded
   `--detach` stack command in [Verification scope](#verification-scope).
   Starting the app so the gate can run is not test execution, so the Compose
   restriction in [Test execution ownership](#test-execution-ownership) does not
   apply to it.

7. **Run `verifier` before code review.** Invoke the `verifier` sub-agent to run
   the builds, static checks, tests, and journey coverage appropriate for the
   change. The verifier must report failures, flakes, missing coverage, and
   environment issues, and must state explicitly which suites it could not run
   and why (for example, the local HTTP services required by Playwright could
   not be started). Fix or explicitly resolve every actionable finding before
   starting code review. If a verifier finding requires a code change, rerun
   the verifier after addressing it. A focused-support verifier run during TDD
   does not satisfy this gate; invoke the verifier fresh against the settled
   worktree for the complete applicable battery.

8. **Run the local `code-review` before every commit.** Invoke the
    `code-review` sub-agent against the current branch diff and every staged,
    unstaged, and untracked file. Address every actionable finding before
    committing. If review findings cause changes, rerun the affected tests and
    the `verifier`, then obtain a fresh `code-review` approval for the changed
    state.

9. **Commit after approval.** Commit only after verification and code review are
   complete. Use Conventional Commits:

   ```text
   <type>(<scope>): <imperative summary>
   ```

   Keep the subject at 72 characters or fewer, describe why in the body when
   useful, and do not combine unrelated work.

10. **Create pull requests from the reviewed state.**
    - Confirm that local verification remains valid.
    - Rerun `code-review` only if the reviewed state changed after the
      pre-commit review. A changed state includes code, tests, documentation,
      generated files, conflict resolution, or any other staged, unstaged, or
      untracked content. Do not repeat code review when the already-reviewed
      diff and worktree remain unchanged.
    - Title the pull request as a Conventional Commit. CI enforces this, and
      because short-lived branches are squash-merged, the pull request title
      becomes the commit subject on `main`.
    - Link the tracked issue in the pull request body: `Closes #<n>` only when
      this slice completes the issue, `Part of #<n>` for every earlier slice of
      an ordered split. The squash merge carries the body onto `main`, so
      `Closes` on an intermediate slice closes the issue while later slices are
      still outstanding.
    - Push and open a normal, ready-for-review pull request. Do not open draft
      pull requests unless the user explicitly asks for a draft.

11. **Merge only clean, passing pull requests.** Merge only after GitHub reports
    a clean merge state and every configured check passes. Never bypass a
    failing or pending required check. Self-merges are allowed when these
    conditions are met. Use squash merge for short-lived development branches to
    keep `main` linear, then delete the merged branch.

## Gate discipline

The `ui-review` → `verifier` → `code-review` sequence can otherwise loop
forever. It is bounded by these rules.

- **Actionable** means correctness, security, data-loss, accessibility, or
  contract defects, plus anything the sub-agent marks as blocking. Style
  preferences and speculative refactors are not actionable; note them, or file
  them per step 5, and move on.
- **Two rounds maximum.** If the same gate reports actionable findings on a
  third pass, stop and hand the disagreement to the user with both positions
  summarized.
- **Rerun only what the change invalidated.** A documentation-only fix after a
  review does not require rerunning the full verifier suite; say what you
  reran and why.
- **Never self-certify a gate.** If a sub-agent is unavailable, say so
  explicitly and state what you checked manually instead. Do not report a gate
  as passed when it did not run.

## Test execution ownership

Keep the main agent's context focused on design and implementation. Split test
execution by phase and weight:

| Phase | Main agent | `verifier` sub-agent |
| --- | --- | --- |
| Go TDD red/green | Writes the test and runs only the exact focused Go test, scoped by package and `-run` | Not required |
| Integration or journey TDD red/green | Writes the focused contract and interprets the concise result | Runs only the named emulator, Docker, or Playwright test in focused-support mode |
| Settled implementation gate | Does not duplicate the full battery | Starts fresh and runs every applicable command in [Verification scope](#verification-scope) |

The main agent may rerun a tightly related set of named Go tests while
refactoring, but it does not run repository-wide tests, race/coverage, lint,
Compose rebuilds, emulator batteries, infrastructure validation, or
Playwright — the one exception being the single-service `ui-review` bring-up in
step 6, which starts the app rather than running tests. A focused Go test
already compiles its package; do not add a broad build merely to prove
compilation during the TDD loop.

All automated Playwright execution belongs to the verifier, including a single
spec or test selected for journey TDD. In focused-support mode, the verifier
reports the command, the red or green result, and the decisive failure evidence
without claiming the formal gate. After the implementation settles, use a
fresh verifier invocation for the full applicable suite. Manual viewport and
accessibility inspection remains the separate `ui-review` responsibility.

## Applicability

Not every gate applies to every change. Record the reason when one does not.

| Gate | Applies when | Status in this repo |
| --- | --- | --- |
| `ui-review` | The change affects rendered UI — templates, static assets, styling, or a client app | **Live.** Active for `internal/app/webfrontend` templates, CSS, and client-side JavaScript (`static/js/`, `static/sw.js`). |
| `verifier` | Always | Live |
| `code-review` | Always | Live |

The Playwright suite in `tests/playwright/` contains both API request specs and
browser-driven page specs. The verifier owns all automated Playwright coverage;
browser specs complement but do not replace the manual, viewport-based
`ui-review` gate for rendered UI changes.

## Verification scope

The `verifier` owns these commands.

**Every tool version is pinned to match CI.** Reproduce a CI failure with the
pinned version, never `@latest` — a floating linter once produced a green local
run and a red CI on the same commit. If you change a version here, change it in
`.github/workflows/ci.yml` in the same commit.

Go toolchain — `1.26.5`, matching `actions/setup-go` in CI. The Go version is
pinned for the same reason the linters are: `go vet`'s analyzer set and stdlib
behavior shift between releases, so a newer local toolchain can pass checks that
CI rejects. `go.mod` declares `go 1.25.8` as the **language** version, which is a
separate thing and does not pin the toolchain. Check yours before verifying:

```bash
go version   # must report go1.26.5
```

If it reports anything else, prefix the Go commands with
`GOTOOLCHAIN=go1.26.5` — Go fetches the pinned toolchain automatically, and the
language version in `go.mod` stops `GOTOOLCHAIN=auto` from doing it for you.
Never report a Go gate as passed on a different toolchain.

Node — `24`, the active LTS, matching both `actions/setup-node` steps in CI.
Node is dev tooling only here — it runs markdownlint and Playwright, and no
service or rendered page depends on it. Check with `node -v` before those
commands; a different local major can resolve different transitive dependencies
than CI.

If it reports a different major, run them under the pinned one rather than the
default — `mise x node@24 -- npx ...` or your version manager's equivalent.
This matters more than it looks: `node -v` reports the system default, so the
commands below run under whatever that happens to be unless you route them.

Track the LTS line. Node 20 was pinned here until it reached end-of-life, which
is how a supported runtime becomes an unsupported one without anyone deciding
to.

Static checks and unit tests — always:

```bash
go vet ./...
go test -race -cover ./...
```

Go lint — always. `.golangci.yml` uses the **v2** config schema, which a v1
binary cannot parse:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run
```

Markdown lint — when Markdown changes:

```bash
npx --yes markdownlint-cli@0.49.1 --config .markdownlint.json \
  "AGENTS.md" "CLAUDE.md" "README.md" "docs/**/*.md"
```

Infrastructure — when `infra/` changes. `tofu fmt` output can differ between
versions, so use OpenTofu `1.12.5`:

```bash
cd infra/opentofu
tofu fmt -check -recursive
tofu init -backend=false && tofu validate
```

The always-on `go test -race -cover ./...` command includes `e2e/`. Its
in-memory cascade tests need no Compose, Ollama, or GCP credentials.

Emulator-gated contracts across `pkg/store`, `pkg/outbox`, `pkg/runtimeconfig`,
`pkg/identity`, `internal/app/petmatcher`, `internal/app/notification`, and `e2e/` **skip silently**
unless the emulator host variable for that area — `FIRESTORE_EMULATOR_HOST`,
`PUBSUB_EMULATOR_HOST`, `FIREBASE_AUTH_EMULATOR_HOST`, or the documented
combination — is set. The package still reports `ok`, so a green run is not
evidence that those contracts ran. When human identity changes, run the pinned
Firebase Authentication emulator journey in `docs/DEVELOPMENT.md`. When
durable state, the outbox, Pub/Sub delivery, or idempotency changes, start the
emulators and run the contracts named for that area in `docs/DEVELOPMENT.md`.
Otherwise report emulator contracts `NOT RUN` with the reason.

Playwright journey coverage — when HTTP service behavior, rendered pages, or
contracts change. It needs only the three HTTP services exercised by the suite;
if they cannot run, the verifier must report `NOT RUN` with the reason rather
than silently skipping:

```bash
docker compose up --build --detach lostpet-service foundpet-service web-frontend

cd tests/playwright
npm ci
npx playwright install chromium
npx playwright test
```

## CI

`.github/workflows/ci.yml` runs five jobs on pull requests. The first four are
required before merge:

- `pr-title` — validates the **PR title** as a Conventional Commit. Because
  short-lived branches are squash-merged, the PR title becomes the commit
  subject on `main`.
- `static-checks` — markdownlint. The file list is hardcoded in the workflow;
  renaming or adding a top-level doc requires updating it in the same commit.
- `go-checks` — `go vet`, `go test -race -cover`, `golangci-lint`.
- `infra-checks` — `tofu fmt -check -recursive` and `tofu validate`.
- `e2e-playwright-tests` — builds the three HTTP services used by the Playwright
  API and browser journeys, waits for them to become ready, runs the suite, and
  uploads its report, traces, and service logs on failure. This job is not a
  required check until the repository ruleset is updated separately.

The `go-checks` job runs the in-memory portion of the `e2e/` package as part of
`go test -race -cover ./...`. It starts no emulator, so the emulator-gated files
in `e2e/` skip and the job reports green without them; see
[Verification scope](#verification-scope). The Playwright job needs only
`lostpet-service`, `foundpet-service`, and `web-frontend`, so it deliberately
avoids downloading a model. Neither CI nor the documented verifier commands
exercise a live Ollama, real Pub/Sub, or deployed GCP cascade; do not claim that
coverage unless a task defines and runs an explicit integration command for it.
CI results also do not replace the independent verifier report required by
step 7.

## Branch protection

`main` is protected by an active ruleset: pull request required, squash-only
merges, no deletion, no force-push, and all four checks green before merge.
There are no bypass actors — it applies to repository owners too, which is what
makes step 11 enforceable rather than aspirational.

Required status checks are **strict**: a branch must also be up to date with
`main` before it can merge. If GitHub reports `BLOCKED` while every check is
green, the branch is behind — update it from `main`, let CI rerun, and confirm
a clean state again. Merging `main` in changes the reviewed state, so step 10's
rerun rule applies: a conflict resolution needs a fresh `code-review`, while a
conflict-free update does not — the incoming commits were already reviewed on
`main`.

The required check contexts must match the job `name:` values in `ci.yml`
exactly. Renaming a job without updating the ruleset leaves a required check
that can never report, which blocks every pull request.

## Registered subagents

These are defined in `.claude/agents/` and are invocable by name. If you are
running under a tool that cannot load them, follow the responsibilities below
inline and say that you did so.

### `ui-review`

- **Role**: Website design, usability, responsiveness, and accessibility
  expert.
- **Responsibilities**:
  - Exercises UI user journeys across mobile, tablet, and desktop viewports.
  - Inspects interaction, loading, empty, error, focus, keyboard navigation,
    contrast, and responsive layout states.
  - Captures visual evidence and screenshots.
  - Documents UI findings, or states plainly that the change has no UI impact.
  - Identifies out-of-scope UI defects and files issues per step 5.

### `verifier`

- **Role**: Build, static check, test, and journey coverage verifier.
- **Responsibilities**:
  - Supports TDD by running an explicitly named integration, emulator, Docker,
    or Playwright test when requested, returning a concise result without
    treating it as the formal gate.
  - Executes the commands in [Verification scope](#verification-scope).
  - Runs the formal gate fresh against the settled worktree even when it ran
    focused-support checks earlier in the slice.
  - Reports failures, flakes, missing coverage, and environment configuration
    issues, and names every suite it could not run.
  - Validates code fixes before code review begins.
  - Flags pre-existing test debt or environmental issues outside the current
    scope and files issues per step 5.

### `code-review`

- **Role**: Language, framework, and architectural code review expert.
- **Responsibilities**:
  - Inspects branch diffs and all staged, unstaged, and untracked files against
    best practices, security standards, and performance patterns.
  - Verifies Conventional Commit messages, API design consistency, and clean
    code principles.
  - Issues a formal approval or actionable revision request before commits and
    pull requests.
  - Identifies unrelated code smells, technical debt, or bugs and files issues
    per step 5 rather than coupling them to the pull request.
