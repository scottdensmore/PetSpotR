# CLAUDE.md

## Agent workflow

**Follow [`AGENTS.md`](./Agents.md).** It is the single source of truth for how
coding agents work in this repository — branching, thin vertical slices, TDD,
review gates, commit format, and merge rules. Read it before making changes and
do not duplicate its rules here; fix them there.

This file holds only the repo-specific facts an agent needs to execute that
workflow.

## What this project is

Event-driven Go microservices for reporting and matching lost pets, running on
GCP (Cloud Run, Pub/Sub, Firestore, GCS) and locally via `docker-compose` +
Ollama `gemma2:2b`.

There is **no frontend**. All four services expose JSON HTTP endpoints or run as
Pub/Sub workers. The Playwright suite exercises HTTP APIs, not a browser UI.

## Layout

| Path | Contents |
| --- | --- |
| `cmd/` | Service entrypoints: `lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service` |
| `pkg/` | Shared packages: `domain`, `store`, `pubsub`, `blob`, `ollama`, `scoring` |
| `e2e/` | Go end-to-end event-cascade tests (needs the stack running) |
| `tests/playwright/` | API journey tests (needs the stack running) |
| `infra/opentofu/` | GCP infrastructure modules |
| `deploy/cloudrun/` | Cloud Run manifests |
| `docs/` | `DEVELOPMENT.md`, `MIGRATION_PLAN.md` |

## Commands

Static checks and unit tests:

```bash
go vet ./...
go test -race -cover ./...
```

Lint. `.golangci.yml` uses the **v2** config schema, so a v1 binary will fail to
parse it. CI pins `v2.12.2`; match it locally:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run
```

Markdown lint. CI pins `0.49.1`; match it locally:

```bash
npx --yes markdownlint-cli@0.49.1 --config .markdownlint.json \
  "AGENTS.md" "CLAUDE.md" "README.md" "docs/**/*.md"
```

Full local stack — required before running `e2e/` or Playwright:

```bash
ollama pull gemma2:2b
docker-compose up --build
```

Playwright journeys (against a running stack):

```bash
cd tests/playwright && npm install && npx playwright test
```

Infrastructure checks. CI pins OpenTofu `1.12.5`; `tofu fmt` output can differ
between versions, so match it locally:

```bash
cd infra/opentofu
tofu fmt -check -recursive
tofu init -backend=false && tofu validate
```

## CI

`.github/workflows/ci.yml` runs four jobs on pull requests. All four are
intended to be required status checks on `main` — see
[Branch protection](#branch-protection) for whether that is in force.

- `pr-title` — validates the **PR title** as a Conventional Commit. Because
  short-lived branches are squash-merged, the PR title becomes the commit
  subject on `main`.
- `static-checks` — markdownlint. The file list is hardcoded in the workflow;
  renaming or adding a top-level doc requires updating it in the same commit.
- `go-checks` — `go vet ./...`, `go test -race -cover ./...`, `golangci-lint`.
- `infra-checks` — `tofu fmt -check -recursive` and `tofu validate`.

**Every tool version in CI is pinned.** Reproduce a CI failure by running the
pinned version, not `@latest` — a floating linter produced a green local run and
a red CI once already.

| Tool | Pinned version |
| --- | --- |
| golangci-lint | `v2.12.2` (v2 config schema; v1 cannot parse `.golangci.yml`) |
| markdownlint-cli | `0.49.1` |
| OpenTofu | `1.12.5` |
| Go | `1.22` |

CI still does **not** run the Playwright suite or `e2e/`: both need the full
stack plus `ollama pull gemma2:2b`. Those remain verifier-owned local steps, so
a green CI is still narrower than the verification `AGENTS.md` asks for.

## Branch protection

`main` has **no ruleset yet**, so `AGENTS.md` step 11 ("never bypass a failing
or pending required check") is currently honor-system: nothing stops a direct
push or a merge over red checks. Until a ruleset exists, treat step 11 as a
rule you enforce yourself.

To put it in force, run the `gh api ... rulesets` command recorded in the
project notes. It requires the four check contexts above to match the job
`name:` values in `ci.yml` exactly — if a job is renamed, update both.
