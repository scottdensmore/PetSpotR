# Agent Workflow Guidelines

This document defines the standard workflow required for all AI coding agents
working on this codebase. It is the single source of truth; `CLAUDE.md` points
here and adds only repo-specific commands and layout.

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
   [Applicability](#applicability) for when this gate is live — it is currently
   dormant, because the repository has no frontend yet. When it applies,
   exercise the changed journey in the rendered application at representative
   phone, tablet, and desktop viewports; inspect interaction, loading, empty,
   error, focus, keyboard, contrast, and responsive states; and capture
   screenshots or equivalent visual evidence. Address every actionable finding
   before running the `verifier`. When the gate is dormant or the change has no
   UI impact, record one line stating that and move on — do not fabricate a
   review.

7. **Run `verifier` before code review.** Invoke the `verifier` sub-agent to run
   the builds, static checks, tests, and journey coverage appropriate for the
   change. The verifier must report failures, flakes, missing coverage, and
   environment issues, and must state explicitly which suites it could not run
   and why (for example, no local stack for `e2e/`). Fix or explicitly resolve
   every actionable finding before starting code review. If a verifier finding
   requires a code change, rerun the verifier after addressing it.

8. **Run `code-review` before every commit.** Invoke the `code-review`
   sub-agent against the current branch diff and every staged, unstaged, and
   untracked file. Address every actionable finding before committing. If
   review findings cause changes, rerun the affected tests and the `verifier`,
   then obtain a fresh `code-review` approval for the changed state.

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

## Applicability

Not every gate applies to every change. Record the reason when one does not.

| Gate | Applies when | Status in this repo |
| --- | --- | --- |
| `ui-review` | The change affects rendered UI — templates, static assets, styling, or a client app | **Dormant.** No frontend exists yet. Activate in the same pull request that introduces the first rendered surface. |
| `verifier` | Always | Live |
| `code-review` | Always | Live |

The Playwright suite in `tests/playwright/` is **API journey coverage**, not UI
coverage: it drives HTTP endpoints with `request.post(...)` and asserts on
status codes and JSON. It belongs to the `verifier`, not to `ui-review`. When a
real UI lands, add browser-driven specs and reassign them accordingly.

## Verification scope

The `verifier` owns these commands. CI runs a subset; see `CLAUDE.md` for the
current split and for how to bring up the local stack.

- `go vet ./...` and `go test -race -cover ./...` — always.
- `golangci-lint run` — always, when installed.
- markdownlint over `AGENTS.md`, `CLAUDE.md`, and `docs/**/*.md` — when
  Markdown changes.
- `tofu fmt -check -recursive` and `tofu validate` — when `infra/` changes.
- `go test ./e2e/...` and the Playwright suite — when service behavior or
  contracts change, and a local stack is running. If no stack is available, the
  verifier must say so rather than silently skipping.

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
  - Documents UI findings, or states plainly that the gate is dormant or the
    change has no UI impact.
  - Identifies out-of-scope UI defects and files issues per step 5.

### `verifier`

- **Role**: Build, static check, test, and journey coverage verifier.
- **Responsibilities**:
  - Executes the commands in [Verification scope](#verification-scope).
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
