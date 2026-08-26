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
| `## Project overview` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Repo Map` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Development Commands` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Local Setup` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Architecture & Conventions` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Gotchas & Troubleshooting` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Verification Map` | owns if `profiled`; fills if placeholder, `unverified`, or missing |
| `## Notes & Learned Patterns` | never — belongs to the humans |
| anything between `agent-skills:begin/end workflow` | **never** — overwritten on next adopt |
| any other section the project wrote | never edits; may propose |

### Who owns a section

Three cases, and each is decidable by looking at the section's own bytes:

1. **It ends with an `agent-skills:profiled` marker** — this skill wrote it. Rewrite
   it freely, and stamp it again.
2. **It is missing, empty, or still carries placeholders** (`<command>`,
   `[Key architectural boundary 1]`) **or an `<!-- unverified -->` marker** — write
   it, and stamp it.
3. **It has content and no marker** — a human wrote it. Report what you found and
   what you would change, and leave the edit to them.

Deleting the marker is how someone takes a section back. Say so if you rewrite a
section they may have wanted to keep.

### The marker

Stamp the last line of every section body you write:

```markdown
<!-- agent-skills:profiled 2026-08-21 sources=package.json@3c17e1e8,.github/workflows/ci.yml@ef015b7c -->
```

- The date is the day of the run, `YYYY-MM-DD`.
- `sources` lists the files the section's facts came from — the manifest or lockfile
  you read the commands out of, the CI workflow, the task runner's config. List only
  files you actually opened; a source you did not read is a claim you cannot support.
  In particular, the config a `nothing` row rests on is a **source**, not just
  something to mention in prose: only `sources` is fingerprinted, so a config
  named in the text and left out of the list can be widened without flagging
  the row it was holding up.
- Each fingerprint is the first 8 characters of `git hash-object --no-filters <path>`.
  Use that command; it is the same value the adoption tool computes, and a different
  one makes every future run report rot that is not there.
- A note may follow the source list, so the marker can tell the next reader what
  deleting it does:

  ```markdown
  <!-- agent-skills:profiled 2026-08-21 sources=package.json@3c17e1e8 — delete this line to take ownership -->
  ```

  Two rules, and each exists because breaking it fails quietly:

  - **No spaces inside the list.** `sources=a.json@1111,b.json@2222`, never
    `, b.json@2222`. The list ends at the first space, so a space truncates it —
    and rather than record half a list, the tool records none and reports the
    section stale.
  - **Close the comment.** A marker whose `-->` is missing usually records
    nothing: the search for it runs on into the document and stops at the next
    `<!--`, which is the following marker or the managed block. Markers after it
    are still read either way, so one bad stamp costs one section, not the file.

  The shape up to the sources is fixed — `agent-skills:profiled`, the date, then
  `sources=` — and everything after the list is free text. The note may contain
  `>`, and the closing `-->` may wrap to the next line. The one thing it may not
  contain is `<!--`: that reads as a marker running past its own end, and the
  section is reported stale.

  Break a rule and the marker records nothing, which reads as stale. That is the
  safe direction — a section re-derived needlessly rather than a rotted one
  trusted — but it is still a marker doing nothing, so it is worth getting right.

This is the whole re-run mechanism. Without it, a run that fills a section also
deletes the `unverified` marker that identified the section as this skill's work,
so the next run sees prose it cannot attribute, correctly declines to touch it
under case 3, and the file can never be updated again.

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

Then **the file a command resolves *through* is a source**, not only the file
you found the command declared in. `Lint | ./scripts/lint.sh` is confirmed by
that script existing, and existing is all a later run can re-confirm: rewrite
the script and every recorded fingerprint still matches while the command does
something else. Record `scripts/lint.sh` and the rewrite flags the section.

The same sentence as the `nothing` row's config, for the same reason — a claim
is only as durable as the fingerprint watching what it rests on. Where the
command and its definition share a file this is already true: `make test` is
confirmed by a target name, but the `Makefile` is the source, so a changed
recipe moves it. It is a shim, a task file calling another file, or a script
invoked from CI that slips through.

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

## 4. Re-profiling a file that is already filled

Most runs after the first are not about blanks — they are about rot. A section
written a year ago is not wrong because it was careless; it is wrong because the
project moved.

**Start by asking what moved, before re-deriving anything.** For each section
carrying an `agent-skills:profiled` marker, recompute
`git hash-object --no-filters <path>` for every source it records:

- Every fingerprint still matches → that section is current. Say so in the report
  and skip it. Nothing it was derived from has changed, so re-reading the project
  to reach the same conclusion costs a run and proves nothing.
- Any fingerprint differs, or the file is gone → that section is suspect. Re-derive
  it, and re-stamp it with the new fingerprints.
- No marker at all → the section predates this mechanism, or a human took it back.
  Treat it under case 3 above: read it, report, do not edit.

The cost of a re-run then scales with what actually changed rather than with the
size of the project, which is what makes running it often practical. It scales
with how *widely a source is cited*, though, not with how small the edit was:
the manifest tends to be both the most-edited file and the most-cited source, so
adding a dependency can legitimately flag every section at once.

**A standing `unverified` marker is checked on every run, whatever the
fingerprints say.** Two rules point opposite ways here and this is the
precedence between them: a section can carry a `profiled` marker whose sources
all match *and* an `unverified` marker inside it, which case 2 calls a section
to write. Fingerprints decide whether to **re-derive** the section; they say
nothing about the open question. Most `unverified` markers are about the
**environment** rather than a file — "eslint is not installed here", "the
credential does not exist yet" — and the environment is not a source, so no
fingerprint will ever move and a marker gated on one would stand forever while
the section reported as current. Retry the marker; leave the rest of the section
alone unless a fingerprint moved.

Retrying means re-running the *evidence*, under the same limits as the first
time: the declaration may now exist, the config file may have been added, the
`grep` may now find a caller. Then either confirm it or narrow the marker to
what is still genuinely unconfirmed. **Never lift the §2 limits to close one** —
if confirming still needs an install, a network call, or anything that mutates,
the marker stays, and the note says which command a human should run once to
settle it.

**A `nothing` row whose justification is missing from `sources` is repaired on
every run, whatever the fingerprints say** (a `nothing` row is a Verification Map
row that reruns no command; § 5 defines one and says what may justify it) — like the `unverified` rule above,
this overrides "every fingerprint still matches → skip it", and says so itself
rather than borrowing the point. The reason is the same: a marker stamped before that rule has its justifying config
absent from `sources` by hypothesis, so nothing moves, nothing flags, and the
sweep below is never entered for it. Saying "whether or not the section was
flagged" inside a loop over flagged sections is a no-op — the first draft of
this rule was itself inert, for exactly the failure it describes. Add the
missing config now. If there is no config you can point at — the row rested
on a reading of the tool rather than its configuration — then the row was
never safe, and it becomes `the complete gate`, which is the same answer the
fill rule gives.

For each section the check flagged, sweep for the three kinds of decay, in this
order. (`unverified` markers are **not** among them: they are handled above, in
every section rather than only the flagged ones, which is a rule a step inside
this loop could not state.)

1. **Commands that no longer resolve.** A script renamed, a task removed, a
   package manager swapped. Check each row of Development Commands the same way
   you confirmed it originally — declaration first, `--help` at most.
2. **Paths that no longer exist, and justifications that no longer hold.**
   Every path in the Repo Map and the Verification Map — a moved directory
   silently makes a map worse than no map, because it is confidently wrong.
   Then the claims those maps rest on, which existence cannot answer: a
   `nothing` row rests on a config that **still exists but now excludes less**,
   which invalidates the row while every path in it is still there. Re-read
   each such config against the row it justifies.

   Note that **not every entry in either map is a path**. A row may route on a
   kind of change rather than a location — `anything else`, or an installed
   copy that was edited rather than deleted — and there is nothing to check the
   existence of. Read those against whether the routing is still true.
3. **Gotchas whose cause is gone.** A workaround for a bug that has since been
   fixed sends the next agent around an obstacle that is no longer there. Where
   history shows the fix landed, say so rather than deleting the entry outright:
   a struck-through gotcha with the fix noted is more useful than silence,
   because it answers the question a second time.

Apply the fixes you can prove and **report the rest** — a section carrying no
`agent-skills:profiled` marker is theirs, even when it has gone stale. Say what is
now untrue, what you verified it should say, and leave the edit to them. In a
section you do own, fix it and re-stamp; the marker is what makes that decidable
rather than a judgment about who probably wrote which paragraph.

---

## 5. Filling each section

### Project overview

The one section every other section is read against, and the only owned one that
had no guide here. Five things, each answerable from the tree:

- **What this is**, in a sentence a newcomer could repeat. Not the marketing line
  — what it does and for whom.
- **Stack and toolchain**, from the manifest and the CI file rather than the
  README, which is the most confidently stale source in most repositories.
- **UI domain**, honestly. `Headless / Backend` when nothing renders, which makes
  stage 7 `N/A` every time rather than argued about per change.
- **Locales**, and — the load-bearing part — whether the project has *decided* it
  will not localize. A decision written down is a standing `N/A` for stage 8; an
  empty catalog is not, and the two look identical from here.
- **Base Branch**, confirmed rather than inherited. The template ships `main` as a
  literal, with no placeholder and no `unverified` marker, so nothing flags it and
  a project on `develop` or `trunk` reads as profiled. Resolve it with
  `git symbolic-ref --short refs/remotes/origin/HEAD`, which answers
  `origin/<name>` — record the part after the **first** slash, since a branch
  name may contain more (`origin/release/2.0` is the branch `release/2.0`).
  Recording what the
  command literally prints puts `origin/main` in a field every later step reads
  as a branch, which is this bullet's own failure one step further on.

  **That ref is unset more often than you would expect** — a local-only
  repository, a remote added but never fetched, a fresh `git init` — and it then
  prints nothing rather than failing. Do not fall back to the template's `main`:
  that reproduces the bug this bullet exists to prevent and then stamps a
  `profiled` marker over it, which is worse than the unflagged state, because
  something now asserts it. Walk the same tiers adoption walks: a remote-tracking
  `origin/<name>`, then a local branch, for `main`, `master`, `trunk`, `develop`
  in that order; failing all of those, the branch you are actually on
  (`git symbolic-ref --short HEAD`, which answers before the first commit exists).
  If none of that resolves, record what you found and leave the row
  `<!-- unverified -->`. This is the field with the worst failure history in this
  repository: the wrong branch lands in five places in the generated file, and
  every one of them looks like a fact.

The UI domain and Locales bullets are the two the table in **What the workflow
asks this project to answer** sends you here to fill; this is where they get
filled in. Base Branch has no row of its own in that table — the nearest,
*Branch, commit, and PR conventions*, is about a different field and points at
the project's own section — so nothing downstream will remind you about it.

### Development Commands

One row per stage the project actually has. Every command must run from the
repository root — prefix with `cd <dir> &&` when the project is nested. Drop the
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
| `docs/*.md` | nothing — `eslint.config.mjs` ignores it, `vitest.config.mts` and `tsconfig.json` cover only `app/` |
| anything else | the complete gate |
```

A "nothing" row is a strong claim: it is only safe when a tool's configuration
excludes the path, not when convention suggests it should. State which config
makes it true, **and record that config in the section's `sources`** — the row
is only as durable as the fingerprint watching its justification. If you cannot
show that, the row is `the complete gate`.

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
| *UI review applies only to what a person sees* | The project's UI domain, honestly. `Headless / Backend` if nothing renders; the stage is then `N/A` every time rather than argued about per change. That is a verdict, not a skip — the block draws the distinction and `slice-and-pr` acts on it. | `## Project overview` |
| *Localization review keeps the application localizable* | Which locales the project ships, where the catalogs live if any, and — the load-bearing part — **whether the project has decided it will never localize**. That decision is the only thing that makes the stage `N/A`, and it has to be written rather than inferred: an internal CLI or a service with no human-facing text says `single locale, not intended`. A missing catalog is **not** an `N/A`; it is the case the stage exists for, since keeping the door open is cheapest before a second locale exists. Say which of the two a bare `single locale` means, because the reviewer cannot tell. | `## Project overview` |
| The reviewers judge against this file | Where the review criteria live — the invariants, contracts, and traps a change is checked against, with `file:line` anchors where they exist. | `## Architecture & Conventions`, `## Gotchas` |
| Branch, commit, and PR conventions | Any place this project's rules differ from the generic block — branch naming, commit format, PR sections. State plainly which one wins. | the project's own section (often `## Git`) |

These are questions, not a template: if the project has no answer for a row —
no partial-success trap, no self-rewriting file — leave it out. An invented
answer is worse than an absent one, and a row you cannot source is an invention.

---

## 6. Editing AGENTS.md without corrupting it

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
edit goes wrong.

**Most of these judge the half of the file you may edit**, so something inside
the managed block that looks like a violation is adoption's shape rather than
your splice. Three collided with it before that was stated, and the reader
reaction was the same each time: a check that reports a violation on a correct
run is one people stop reading, and the next thing it reports is real.

**Two are the exception, and they are the ones the block is *for*.** `exactly
one agent-skills:end workflow` and `byte-identical` take the block as their
subject: a violation there is not adoption's shape, it is you having damaged
the thing you were told not to touch. Read those two literally.

- Every `##` heading **you are responsible for** appears exactly once. Adoption
  itself can leave two `## Development Workflow` headings — where a project's
  older workflow section was kept alongside the new block, which it reports as
  `coexists` — and neither copy is yours to remove.
- The file contains **exactly one** `agent-skills:end workflow`.
- The managed block is **byte-identical** to before your edit.
- No placeholder token survives **outside the managed block**. Inside it,
  `<owner>`, `<type>`, `<base>` and the rest are metavariables in the commands
  the workflow documents — `git checkout -b <owner>/<type>/<short-description>`
  — not blanks anyone fills.
- Every heading **outside the managed block** is preceded by a blank line — no
  `text.## Heading` splices. Inside it, `## Development Workflow` sits directly
  under the begin marker: adoption's shape, not a splice.
- Nothing that was in the file before is gone unless you replaced a generated
  draft. Diff old against new and account for every removed line.
- **Every section you set out to write is actually in the file, with its body.**
  Check the content, not just the heading. Structural checks pass happily on a
  file where nothing was written.

Then re-run adoption against the repository with `--dry-run --keep-existing`. It
must report `AGENTS.md already current` and end with `Dry run — no files were
written.` If it wants to change something, your edit broke a structure it depends
on.

Both flags, always. `--keep-existing` alone is not a check — it is an install, and
it writes. In a repository adopted with `--skip-skills` or `--no-pointers` it puts
back everything that was deliberately left out: skills and subagents across every
host directory, and a `GEMINI.md` the project may have declined. That lands in the
middle of someone's unrelated slice, against the workflow's own rule to preserve
what you did not change.

## 7. Report

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
