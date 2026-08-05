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

Markdown lint (matches CI):

```bash
npx --yes markdownlint-cli "Agents.md" "docs/**/*.md"
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

Infrastructure checks:

```bash
cd infra/opentofu && tofu fmt -check -recursive && tofu validate
```

## CI

`.github/workflows/ci.yml` runs three jobs on pull requests:

- `pr-title` — validates the **PR title** as a Conventional Commit. Because
  short-lived branches are squash-merged, the PR title becomes the commit
  subject on `main`.
- `static-checks` — markdownlint over `AGENTS.md` and `docs/**/*.md`. This path
  is hardcoded; renaming `AGENTS.md` requires updating the workflow in the same
  commit.
- `go-checks` — `go vet ./...` and `go test -v ./...`.

CI does **not** currently run `-race`, coverage, `golangci-lint`, the Playwright
suite, or OpenTofu validation. Run those locally; a green CI is weaker than the
verification `AGENTS.md` asks for.
