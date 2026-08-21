---
name: project-profile
description: >-
  Fill in the project-specific half of AGENTS.md — commands, architecture,
  gotchas, and the verification map — by inspecting the repository rather than
  guessing. Use after adopting the agent workflow, when build or test commands
  change, or when AGENTS.md still contains placeholders or unverified entries.
---

# Project Profile Skill

Adoption writes the universal half of `AGENTS.md`: the workflow contract, inside
a managed block. It fills the project-specific half from filename pattern matching,
which is a first draft and is marked `<!-- unverified -->` because pattern matching
cannot run `xcodebuild -list`, read `package.json` scripts, or open a CI config.

This skill replaces that draft with facts.

---

## 1. Scope

| Section | This skill |
|---|---|
| `## Project overview` | fills if placeholder |
| `## Repo Map` | fills if placeholder or `unverified` |
| `## Development Commands` | fills if placeholder or `unverified` |
| `## Local Setup` | fills if placeholder or `unverified` |
| `## Architecture & Conventions` | fills if placeholder |
| `## Gotchas & Troubleshooting` | fills if placeholder |
| `## Verification Map` | fills if placeholder |
| `## Notes & Learned Patterns` | never — belongs to the humans |
| anything between `agent-skills:begin/end workflow` | **never** — overwritten on next adopt |
| any other section the project wrote | never edits; may propose |

**Fill blanks, propose the rest.** A section that is empty or still carries
placeholders (`<command>`, `[Key architectural boundary 1]`) or an
`<!-- unverified -->` marker is yours to write. A section already written by a
human is not: report what you found and what you would change, and leave the edit
to them.

---

## 2. The evidence rule

> Every command recorded must be one you confirmed exists. If you cannot confirm
> it, write it with `<!-- unverified: <what you could not confirm> -->` rather
> than a plausible guess.

Guessing here is worse than leaving a blank, because `verifier` treats `AGENTS.md`
as canonical and will run whatever it finds.

### Confirming without breaking anything

Prefer **declaration** over **execution**. A command is confirmed when you have
seen it declared:

- `package.json` → `scripts`
- `Makefile` → target names (`^target:`)
- `pyproject.toml` → `[project.scripts]`, tool sections
- `Cargo.toml`, `go.mod`, `build.gradle`
- a committed CI workflow that invokes it
- an executable file that exists (`./scripts/lint.sh`)

When you must execute, execute only inspection: `--help`, `--version`, `-list`,
`--dry-run`, `-n`. **Never run a command that could mutate anything** — no
`deploy`, `publish`, `release`, `migrate`, `--fix`, `--write`, no installs, no
`make` targets whose behavior you have not read. If confirming a command requires
running it for real, mark it unverified and say why.

---

## 3. Discovery order

Work down this list. Earlier sources beat later ones when they disagree, and note
the disagreement.

1. **CI configuration** (`.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`).
   This is the project's real gate — what must pass before merge. If CI runs
   `npm run validate`, that is the verification command, whatever the README says.
2. **Task definitions**: `package.json` scripts, `Makefile`, `justfile`, `Taskfile`,
   `pyproject.toml`, `scripts/`.
3. **Toolchain queries**: `xcodebuild -list`, `swift package describe`,
   `go list ./...`, `cargo metadata`, `npm run` with no arguments.
4. **Existing documentation**: `README`, `CONTRIBUTING`, `docs/`. Often correct,
   often stale — treat as a claim to verify, not a source.
5. **The tree itself**: layout, entry points, test locations.

---

## 4. Filling each section

### Development Commands
One row per stage the project actually has. Every command must run from the
repository root — prefix with `cd <dir> && ` when the project is nested. Drop the
`<!-- unverified -->` marker only for rows you confirmed; keep it, narrowed, for
rows you could not.

### Verification Map
The table stage 6 of the workflow uses to rerun only what a fix could have
invalidated. For each gate command, determine which paths it **reads**:

- config the command loads (`eslint.config.*`, `.prettierignore`, `vitest.config.*`,
  `tsconfig.json`, `.swiftlint.yml`)
- the source and test roots it collects from
- files a custom check script opens — read the script

Then invert it: for each path group, list the commands that read it.

```markdown
| A fix touches | Rerun |
|---|---|
| `app/` or `tests/` | `npm run test`, `npm run lint`, `npm run build` |
| `eslint.config.mjs` | everything above |
| `docs/*.md` | nothing; no gate command reads it |
| anything else | the complete gate |
```

A "nothing" row is a strong claim: it is only safe when a tool's configuration
excludes the path, not when convention suggests it should. State which config
makes it true. If you cannot show that, the row is `the complete gate`.

### Repo Map

The first thing an agent needs and the least likely to be written down. Derive
each row from the tree and the tool configs, never from convention:

- **Code lives in** — the directories a change actually lands in. In a workspace
  or monorepo, which package owns what, since "where does this live" has more
  than one answer. Read the workspace config, not the folder names.
- **Entry points** — where execution starts: the binary's `main`, the server's
  bootstrap, the app's root component, the job's handler. A reader who knows the
  entry point can follow anything; one who does not, greps.
- **Tests live in** — the directory or the beside-the-source convention, the
  filename pattern, and one existing test to copy the shape of. Stage 5 writes a
  test before anything else; without this it invents a location, and a test in
  the wrong place may not run at all.
- **Generated — never edit** — anything a tool writes: API clients, ORM models,
  schema bindings, compiled assets, lockfiles. Give the regeneration command.
  This is the highest-value row in the table: an edit to a generated file looks
  correct, passes review, and vanishes on the next build.
- **Vendored or third-party** — checked-in dependencies, where a fix belongs
  upstream instead.

Find them from evidence: the build or workspace config for layout, the test
runner's config for test roots, `.gitattributes` `linguist-generated`, codegen
config, and a header comment saying "do not edit" in the file itself.

### Local Setup

What it takes to run the thing at all — the difference between an agent that can
verify its work and one that can only reason about it. Services it expects, the
environment variables it reads and **where their values come from**, and any seed
or fixture step.

Read `.env.example`, the compose or dev-container file, the CI service
definitions, and the config loader itself for the variable names. **Never copy a
real secret into `AGENTS.md`** — name the variable and its source. If a value
cannot be obtained locally, say so plainly: an agent that knows it cannot run the
integration suite reports that honestly, while one that does not will claim a
pass it never saw.

### Re-profiling a file that is already filled

Most runs after the first are not about blanks — they are about rot. A section
written a year ago is not wrong because it was careless; it is wrong because the
project moved. Sweep for the four kinds of decay, in this order:

1. **Commands that no longer resolve.** A script renamed, a task removed, a
   package manager swapped. Check each row of Development Commands the same way
   you confirmed it originally — declaration first, `--help` at most.
2. **Paths that no longer exist.** Every path in the Repo Map and the
   Verification Map. A moved directory silently makes a map worse than no map,
   because it is confidently wrong.
3. **`unverified` markers still standing.** Each one is a question someone could
   not answer then. Try again — the browser may now install, the credential may
   now exist — and either confirm it or narrow the marker to what is still
   genuinely unconfirmed.
4. **Gotchas whose cause is gone.** A workaround for a bug that has since been
   fixed sends the next agent around an obstacle that is no longer there. Where
   history shows the fix landed, say so rather than deleting the entry outright:
   a struck-through gotcha with the fix noted is more useful than silence,
   because it answers the question a second time.

Apply the fixes you can prove and **report the rest** — a section a human wrote
is still theirs, even when it has gone stale. Say what is now untrue, what you
verified it should say, and leave the edit to them.

### What the workflow asks this project to answer

The workflow states rules in the abstract and then relies on this file for the
particulars. Each rule below turns into a question only this repository can
answer; answer it in the section named, and the generic pipeline becomes usable
here. Leave one unanswered and an agent has to guess at exactly the moment it
should not.

| The workflow says | So record here | Where |
|---|---|---|
| *Know what green looked like before you started* | What a passing gate looks like — exit code, the line it prints, the artifacts it produces. If the tree is **not** green today, say so plainly and say which failures are pre-existing, so an agent can tell its own breakage from what it inherited. | `## Development Commands`, or `## Gotchas` when the baseline is red |
| *Measure the thing you ship, not a proxy* | Which commands are real gates and which only look like one — a script that always exits 0, a check that covers part of the output, a linter in report mode. Name them as informational so no one counts them as verification. | `## Development Commands` |
| *Claim only what you observed* | The strongest sentence a green gate licenses here, and what it does **not** cover — what still needs a human, a device, or a run to confirm. | `## Development Commands` |
| *Preserve what you did not change* | Files that a build or tool rewrites on its own, and that will show up dirty without anyone having edited them. | `## Gotchas & Troubleshooting` |
| *A verdict is acted on once* | Any gate that partially succeeds — where some artifacts appear even on failure — and which artifact actually proves success. | `## Gotchas & Troubleshooting` |
| *UI review applies only to what a person sees* | The project's UI domain, honestly. `Headless / Backend` if nothing renders; the stage is then skipped every time rather than argued about per change. | `## Project overview` |
| The reviewers judge against this file | Where the review criteria live — the invariants, contracts, and traps a change is checked against, with `file:line` anchors where they exist. | `## Architecture & Conventions`, `## Gotchas` |
| Branch, commit, and PR conventions | Any place this project's rules differ from the generic block — branch naming, commit format, PR sections. State plainly which one wins. | the project's own section (often `## Git`) |

These are questions, not a template: if the project has no answer for a row —
no partial-success trap, no self-rewriting file — leave it out. An invented
answer is worse than an absent one, and a row you cannot source is an invention.

### Architecture & Conventions
Boundaries a change must respect, stated so a reviewer could cite them: layer
separation, where logic belongs versus presentation, error handling rules,
naming. Derive from the code, not from aspiration. Three accurate lines beat
twelve generic ones.

This is also where the reviewers get their criteria. A subagent reviewing a
change reads this file and nothing else about the project, so an invariant that
lives only in someone's head produces a review that cannot see it. Where the
project has hard rules — a layout contract, an ownership or lifetime rule, a
format whose version must be bumped — state the rule and where in the source it
is enforced.

### Gotchas & Troubleshooting
Only things that have actually bitten someone: a formatter that must not touch a
directory, a test that needs a simulator booted, an env var without which the
suite fails oddly. Look in CI workarounds, `.gitignore` oddities, scripts with
defensive comments, and recent bug-fix commits. If you find none, say so and
leave the section as-is — inventing gotchas is worse than an empty section.

Three kinds are worth hunting for specifically, because an agent that does not
know them draws the wrong conclusion rather than none:

- **A gate that half-succeeds.** A build that keeps going after one step fails,
  leaving most artifacts on disk, reads as success to anything counting outputs.
  Name the artifact whose absence proves failure.
- **A file that rewrites itself.** Build stamps, lockfiles, generated headers,
  coverage reports — anything that turns up in `git status` without a person
  touching it. Otherwise it gets reported as an unexplained change, or worse,
  committed as one.
- **A red baseline.** If the project does not currently build or pass, that fact
  outranks every other gotcha: without it, an agent reads inherited breakage as
  its own and starts "fixing" the wrong thing. Say what fails, and since when.

---

## 5. Editing AGENTS.md without corrupting it

Rewrite **whole `##` sections, anchored on their headings.** Do not splice by byte
offset: computing offsets and then mutating the string invalidates every later
offset, which silently merges headings into the previous line and leaves orphaned
fragments behind. This is the most likely way to damage the file.

The managed block frequently sits *inside* the last section you are rewriting
(`## Verification Map` precedes it). Lift it out before editing and put it back
after:

1. Find the block, cut it out, leave a unique placeholder in its position.
2. Split the remainder into `##` sections; replace the bodies you own.
3. **Insert the sections that do not exist yet.** A repository whose `AGENTS.md`
   has no `## Development Commands` at all is the common case, not the rare one —
   a rewrite that only replaces existing headings silently writes nothing and
   still looks like it worked. Insert missing sections before the managed block.
4. Rejoin, then substitute the block back in for the placeholder. If the section
   you rewrote contained the placeholder, carry it into the new body.

### Self-check before you finish

Run every one of these. They take seconds and each corresponds to a real way this
edit goes wrong:

- Every `##` heading appears **exactly once** — no duplicates.
- The file contains **exactly one** `agent-skills:end workflow`.
- The managed block is **byte-identical** to before your edit.
- No placeholder token survives in the file.
- Every heading is preceded by a blank line — no `text.## Heading` splices.
- Nothing that was in the file before is gone unless you replaced a generated
  draft. Diff old against new and account for every removed line.
- **Every section you set out to write is actually in the file, with its body.**
  Check the content, not just the heading. Structural checks pass happily on a
  file where nothing was written.

Then re-run adoption against the repository with `--keep-existing`. It must report
`AGENTS.md already current` and write nothing. If it wants to change something,
your edit broke a structure it depends on.

## 6. Report

```markdown
### Project Profile
- **Repository**: `<name>` — `<stacks detected>`
- **Base branch**: `<branch>`

#### Filled
| Section | Source of truth |
|---|---|
| Development Commands | `.github/workflows/ci.yml` → `npm run validate` |
| Verification Map | `package.json`, `eslint.config.mjs`, `vitest.config.mts` |

#### Unverified
| Entry | Why |
|---|---|
| `npm run e2e` | declared in package.json, needs a browser install to confirm |

#### Proposed, not applied
| Section | Finding |
|---|---|
| Gotchas | line 42 says `main`; CI and origin/HEAD both say `master` |

- **Managed block**: untouched (`<version>`)
```

Close by naming what a human should look at: `git diff -- AGENTS.md`.
