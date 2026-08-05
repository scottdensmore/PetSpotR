# Agent Workflow Guidelines

This document defines the standard workflow required for all AI coding
agents working on this codebase.

## Workflow

1. **Inspect before changing anything.** Inspect the repository, current
   Git state, and all applicable instruction files before making changes.
   Preserve unrelated staged, unstaged, and untracked work.

2. **Create a branch first.** Create a dedicated feature, fix, refactor, chore,
   test, or documentation branch before making code changes. Never commit
   directly to `main`, and create the branch from the latest appropriate `main`
   state.

3. **Choose a thin vertical slice.** Before implementing a tracked issue or
   feature, define the smallest end-to-end slice that can be reviewed,
   tested, shipped, and merged independently. Prefer one coherent
   user-visible or operational outcome over a broad horizontal layer. If the
   next issue is too large for one pull request, split it into ordered slices
   and complete only the current slice. Keep pull requests small enough for
   thorough review, reliable verification, and quick rollback.

4. **Use test-driven development when behavior or structure is testable.**
   - Add or update a focused test before implementation.
   - Run it and confirm it fails for the expected reason.
   - Implement the smallest appropriate change.
   - Run focused tests while iterating.
   - Refactor only while the relevant tests remain green.

5. **Inspect the complete diff and track out-of-scope discoveries.** Review
   the branch diff plus all staged, unstaged, and untracked files. Remove
   accidental or unrelated changes while preserving work that belongs to the
   user. Whenever you discover bugs, technical debt, missing features, or
   refactoring opportunities that are not aligned with the current work or
   vertical slice, open an issue to track them rather than expanding the
   scope of the current branch.

6. **Run `ui-review` before verification.** After the main agent completes an
   implementation pass, invoke the `ui-review` sub-agent. The `ui-review`
   sub-agent must act as an expert in website design, usability,
   responsiveness, and accessibility. Address every actionable finding before
   running the `verifier`. For UI-affecting changes, exercise the changed
   journey in the rendered application at representative phone, tablet, and
   desktop viewports; inspect interaction, loading, empty, error, focus,
   keyboard, contrast, and responsive states as applicable; and capture
   screenshots or equivalent visual evidence. For changes with no UI impact,
   explicitly record that rendered UI review is not applicable. If a finding is
   not applicable, record the concrete reason rather than silently ignoring it.

7. **Run `verifier` before code review.** Invoke the `verifier` sub-agent to
   run the builds, static checks, tests, and journey coverage appropriate for
   the change. The verifier must report failures, flakes, missing coverage, and
   environment issues. Fix or explicitly resolve every actionable finding before
   starting code review. If a verifier finding requires a code change, rerun
   the verifier after addressing it.

8. **Run `code-review` before every commit.** Invoke the `code-review`
   sub-agent against the current branch diff and every staged, unstaged, and
   untracked file. The reviewer must act as an expert in the languages and
   frameworks used by this application. Address every actionable finding before
   committing. If review findings cause changes, rerun the appropriate tests
   and the `verifier`, then obtain a fresh `code-review` approval for the
   changed state.

9. **Commit after approval.** Commit only after verification and code review are
   complete. Use Conventional Commits:

   ```text
   <type>(<scope>): <imperative summary>
   ```

   Keep the subject at 72 characters or fewer, describe why in the body
   when useful, and do not combine unrelated work.

10. **Create pull requests from the reviewed state.**
    - Confirm that local verification remains valid.
    - Rerun `code-review` only if the reviewed state changed after the
      pre-commit review.
    - A changed state includes code, tests, documentation, generated files,
      conflict resolution, or any other staged, unstaged, or untracked content.
    - Do not repeat code review when the already-reviewed diff and worktree
      remain unchanged.
    - Push and create the pull request only after local verification and any
      required code review are complete.
    - Open a normal, ready-for-review pull request by default. Do not open
      draft pull requests unless the user explicitly asks for a draft.

11. **Merge only clean, passing pull requests.** Merge only after GitHub
    reports a clean merge state and every configured check passes. Never
    bypass a failing or pending required check. Self-merges are allowed when
    these conditions are met. Use squash merge for short-lived development
    branches to keep `main` linear, then delete the merged branch.

## Registered Subagents

The following subagents are defined and available for invocation:

### `ui-review`

- **Role**: Website design, usability, responsiveness, and accessibility
  expert.
- **Responsibilities**:
  - Exercises UI user journeys across mobile, tablet, and desktop viewports.
  - Inspects interaction, loading, empty, error, focus, keyboard navigation,
    contrast, and responsive layout states.
  - Captures visual evidence/screenshots.
  - Formally documents UI findings or provides explicit rationale if UI review
    is not applicable.
  - Identifies out-of-scope UI defects or enhancements and opens issues to track
    them.

### `verifier`

- **Role**: Build, static check, test, and journey coverage verifier.
- **Responsibilities**:
  - Executes project builds, linters, static analysis tools, unit tests,
    integration tests, and end-to-end tests.
  - Reports failures, flakes, missing coverage, and environment configuration
    issues.
  - Validates code fixes before initiating code review.
  - Flags pre-existing test debt or environmental issues outside the current
    scope and opens issues to track them.

### `code-review`

- **Role**: Language, framework, and architectural code review expert.
- **Responsibilities**:
  - Inspects branch diffs, staged, unstaged, and untracked files against best
    practices, security standards, and performance patterns.
  - Verifies conventional commit messages, API design consistency, and clean
    code principles.
  - Issues formal approvals or actionable revision requests prior to commits
    and pull requests.
  - Identifies unrelated code smells, technical debt, or bugs and opens issues
    to track them rather than coupling them to the PR.
