---
name: slice-and-pr
description: >-
  Vertical slice planning, branch creation, conventional commit formatting,
  and GitHub CLI pull request lifecycle automation. Use to manage feature breakdown,
  commits, and PR merges.
---

# Slice and PR Lifecycle Skill

This skill governs the operational lifecycle of changes: decomposing large issues into thin vertical slices, enforcing dedicated branch creation, writing structured Conventional Commits, tracking bugs/follow-up issues, and managing pull requests via GitHub CLI (`gh`).

---

## 1. Lifecycle Workflow

```mermaid
flowchart TD
    A["1. Inspect & Branch (Fetch base branch, create branch)"] --> B["2. Choose Thin Vertical Slice"]
    B --> C["3. Track Discoveries (Create issues for side bugs / follow-ups)"]
    C --> D["4. Implement & Test (TDD + Reviews)"]
    D --> E["5. Conventional Commit (after every stage your track includes)"]
    E --> F["6. Push Branch (git push)"]
    F --> G["7. Open PR via GitHub CLI (gh pr create)"]
    G --> H["8. Watch CI (gh pr checks --watch)"]
    H --> I["9. Report PR + CI status — STOP; merge needs explicit approval"]
```

---

## 2. Rules & Protocols

### A. Branch Creation

- Resolve the repository's base branch first — never assume `main`:
  `git symbolic-ref --short refs/remotes/origin/HEAD` (falls back to whatever
  `AGENTS.md` records under **Base Branch**).
- Always fetch the latest upstream state before branching: `git fetch origin <base>`.
- Create a dedicated branch: `git checkout -b <owner>/<type>/<short-description> origin/<base>`.
  `<owner>` is your GitHub login (`gh api user --jq .login`).
- Allowed branch types: `feat`, `fix`, `refactor`, `chore`, `test`, `docs`.
- Never commit directly to the base branch.

### B. Thin Vertical Slices

- Define the smallest end-to-end slice that delivers a coherent outcome and can be independently reviewed, tested, and shipped.
- Avoid large horizontal layers (e.g. implementing 10 database models before any UI or business logic).
- Keep pull requests small enough for reliable review, verification, and fast rollback.

### C. Capturing Discoveries, Bugs & Follow-ups

- When unexpected bugs, edge cases, technical debt, or follow-up tasks are identified during the course of development:
  - **Do NOT expand the scope of the current slice** (prevent scope creep).
  - **Create a tracked issue immediately** using the project's source control management / issue system:
    - **GitHub**: `gh issue create --title "<type>(<scope>): <summary>" --body "<details & reproduction>"`
    - **GitLab / Jira / Azure DevOps**: Use the respective CLI (`glab`, `jira`, `az`) or tracker.
    - **Local / No Tracker**: Record in a project markdown file (`ISSUES.md`, `ROADMAP.md`, or `## Notes & Learned Patterns` in `AGENTS.md`).
  - Keep the current branch focused on completing its single reviewed slice.

### D. Conventional Commits

Commit only after every stage your track includes has **passed**, returned `N/A`, or
been recorded `NOT RUN` per § E. A verdict alone is not enough:
`CHANGES_REQUESTED` is a verdict, and it is the one outcome that must be acted on
before you commit rather than recorded.

`VERIFIED (partial)` is a fourth thing and counts as passed on one condition: every
`NOT RUN` row it names is carried into the PR description per § E. The word
`VERIFIED` inside it is not the verdict — a partial result you do not pass on is a
full one you invented.

A stage the triage table excluded is not a missing precondition — the Trivial track
runs neither review, and a stage that was never owed cannot be missing.

A stage that *applies* and cannot alter what it judges is a different case, and it
is not excluded: a docs-only change on the Feature track still owes UI review an
`N/A`, which the rule at the top of this section already accepts. It is a verdict,
not a skip.

The **Single fix** track is where the two are hardest to tell apart, because its
stages 7 and 8 are parenthesized. Parentheses are not exclusion: the workflow says
a stage in them "applies only when its own entry says it does", and those entries
end in `N/A` rather than in silence. So that track owes the same line.

```text
<type>(<scope>): <imperative summary>

[optional body explaining why this change was made]
```

- Subject line <= 72 characters.
- Stage files explicitly; never run blind `git add -A` if unrelated work exists in the worktree.

### E. GitHub CLI (`gh`) Automation

Use Git for branch transport and the GitHub CLI (`gh`) for GitHub operations:

- **Finish at a pull request by default.** After verification and review pass,
  finish the reversible lifecycle by committing, pushing the branch, opening a
  ready-for-review PR, and watching its checks. Stop after the local commit only
  when the user explicitly asks for a local-only or commit-only result. Creating
  a PR does not authorize a merge or any action in § F.
- **Recording a stage that did not pass.** `N/A` is one line naming why the stage
  does not apply — which track excluded it, or what about the change means it
  judges nothing. It goes in the PR description and needs nothing else.
- **A command that did not run travels with the verdict.** A verifier returning
  `VERIFIED (partial)` names the commands it could not run and what stopped each.
  Copy that list into the description under its own heading. The stage passed and
  the PR is ready for review — but a command nobody ran, inside a stage that
  otherwise passed, is the quiet version of a stage nobody ran, and the reader
  decides what it is worth rather than never learning of it.
- **What the description has to carry.** Why the change exists; what it changes,
  grouped by concern rather than by file; and how it was tested — the command you
  actually ran and its actual result. "Should work" is not a test result. If a
  test was added, say what it would have caught. This matters more under a squash
  (§ G): the intermediate commits do not survive, so the description is the only
  record of how the change was reasoned about.
- **A gate that did not run is named in the title.** A stage marked `NOT RUN` — a
  subagent you invoked and could not reach, or one the user waived — opens
  **ready for review** with `[NOT RUN: <stage>]` leading the title and a
  `## NOT RUN` section giving the subagent, the host, the exact invocation and the
  exact error. Drop the prefix once the stage has actually run.

  **A draft is what a user asks for, never what an unrun gate forces.** A draft
  hides the change from the people best placed to say whether the missing gate
  mattered, and reads as "not finished" when the work is finished and one gate is
  not. The marker says the true thing instead, in the one place a reviewer cannot
  scroll past. A gate you could not run never blocks the work, and is never
  described as passed.

  **Remove the prefix before merging.** It announces an unrun gate, and merging is
  the moment that stops being true — or the moment someone decides it no longer
  matters. Whether it also reaches the commit subject depends on a repository
  setting: `squash_merge_commit_title` is `COMMIT_OR_PR_TITLE` by default, which
  takes the *commit's* subject when the pull request has exactly one commit and
  the title only when it has more. Since one reviewed slice lands as one commit,
  the prefix usually never reaches the subject and a `commitlint` step never sees
  it — so do not rely on that as the reason. On a multi-commit branch, or where
  the setting is `PR_TITLE`, it does reach it.

  This is a *stage*, not a command. A verifier returning `VERIFIED (partial)` has
  passed its stage; its unrun commands go in the description and earn no prefix.

```bash
# 1. Push Branch
git push -u origin <branch>

# 2. Create Pull Request — every applicable gate passed
gh pr create --title "<type>(<scope>): <summary>" --body "<why this change was made, what was verified, and any command the verifier could not run>"

# 2b. Or, with a stage that could not run — still ready for review
gh pr create --title "[NOT RUN: <stage>] <type>(<scope>): <summary>" --body "<the above, plus a ## NOT RUN section naming the subagent, host, invocation and error>"

# 3. Monitor CI status
gh pr checks --watch
```

- Never bypass failing or pending CI checks.
- A PR opens ready for review unless the user asks for a draft. An unrun gate is
  never the reason: it is disclosed in the title and the description, never by
  withholding the PR — the reviewer decides what the gap is worth, which they
  cannot do if they never see it.

### F. Actions That Require Explicit Approval

The agent stops after reporting PR status. These commands are **never** run on the
agent's own initiative — only when the user asks for them in the current conversation:

```bash
gh pr merge --squash --delete-branch   # merging
git push --force / --force-with-lease  # rewriting published history
gh pr close / gh issue close           # closing others' work
```

**These three are not the whole list** — they are the ones you reach for most
often. The workflow's own **Stop there and report** bullet is the boundary, and it
names merging, force-pushing, rewriting shared history, deleting a branch or tag,
closing an issue or pull request, dropping or migrating data, removing files
wholesale, and publishing or deploying.

**With one carve-out, and it is the one you will meet.** Deleting the branch an
approved merge just took is part of that merge, not a second act needing its own
approval — see § G. Read the three commands above as the shell you would type, not
as the edge of what needs asking.

Approval given for one PR does not carry to the next. When work is ready, report the
PR URL and CI status, then wait.

### G. Finish the Merge

**Squash by default.** `gh pr merge --squash --delete-branch`. One reviewed slice
becomes one commit on the base branch: the false starts, the fixups and the "address
review" commits describe how the work got made, not what it is. Keeping them turns the
base branch into a diary and makes reverting the slice an archaeology exercise instead
of a single revert.

The trade is that the intermediate commits go, so the PR description is what survives —
which is why it carries the reasoning and the evidence (§ E). If a project requires
merge commits or a rebase instead, its own `AGENTS.md` says so and that wins.

An approved merge takes its own branch with it — `--delete-branch` covers the remote,
and the local copy goes too. That deletion is part of the merge, not a second act
needing its own approval; no *other* branch is included in it.

A merged branch left lying around is a decoy. It reads as work in flight, and nobody
can tell it from the real thing without diffing it against the base.

Two things to know before deleting:

- **Check the paths the branch touched, not the whole tree.** `git diff <base>
  <branch>` is symmetric: it reports everything the *base* has that the branch
  lacks as well, so it is non-empty the moment anything else merges ahead of
  you. Measured after merging two pull requests in sequence: both branches were
  fully merged and byte-identical on `main`, and the plain diff read **3060** and
  **499** lines. A rule that says "stop if it is not empty" stops on the second
  merge of every pair. Restrict the comparison to the files the branch changed,
  and it answers the question you actually have — is my work in the base?
- **Squash merges look unmerged.** A squash writes a new commit instead of joining
  histories, so git sees no ancestry and `git branch -d` refuses a branch that is
  fully merged. Once the scoped check above is clean, `-D` is the correct tool
  there, not a force.

```bash
# Ask, per path the branch changed: does the base already have this content?
# Never assume `main`. If the remote was added but never fetched this exits 128
# and leaves `base` empty — the checks below then fail closed, but § A's fallback
# is better: use what AGENTS.md records under **Base Branch**.
base=$(git symbolic-ref --short refs/remotes/origin/HEAD)
git diff --name-only "$base...scottdensmore/feat/thing"     # the paths to check
git diff --quiet "$base" scottdensmore/feat/thing -- "<one path>"   # silence = base has it
```

**Do not expand those paths through an unquoted `$(...)`.** A path containing a
space splits into several pathspecs that match nothing, `git diff --quiet` exits
**0** on a pathspec that matches nothing, and a `&&` chain then deletes a branch
whose work was never merged. Measured on a fixture: an unmerged branch touching
`sub/my notes.md` reported clean. A glob character or a leading `-` in a filename
does the same. The old whole-tree rule failed *closed* — it stopped too often.
This one fails *open*, on the single action this skill lists as unrecoverable, so
check the paths one at a time and quote each.

Two more ways to get it wrong, both silent:

- **No paths at all — check the exit status, not the output.** An empty list
  and a *failed* enumeration are indistinguishable on stdout: a mistyped base
  exits 128 and prints nothing, exactly like a branch that changed nothing. One
  means there is nothing to lose; the other means you have not looked, and the
  branch may hold unmerged work. Only treat empty as empty when the command also
  **succeeded**. Anything else is "cannot enumerate safely" — leave the branch.
  (An empty pathspec list would also degrade to comparing the whole tree, which
  is the false alarm this check exists to remove.)
- **Renames.** `--name-only` lists the new path; the base has the old one. Check
  `git diff --name-status` and treat an `R` as work the base may not hold under
  that name.

**If any check is not clean, or you cannot enumerate the paths safely, stop and
leave the branch.** A decoy branch costs someone a minute; a deleted branch with
unmerged work costs whatever was in it.

```bash
# Only after every path above came back clean, from a command that succeeded.
git branch -D scottdensmore/feat/thing        # -d refuses after a squash merge
git push origin --delete scottdensmore/feat/thing
```

**Gate the deletion on the check, do not merely print it.** Running the check and
then deleting unconditionally is how the guard becomes decoration — which is
exactly what happened the day this was written.
