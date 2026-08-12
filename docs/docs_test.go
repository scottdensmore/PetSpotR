// Package docs_test guards AGENTS.md against two classes of drift.
//
// The first is a value duplicated verbatim across two files: version pins, the
// Compose service list, and the markdownlint file list, each encoded by both
// AGENTS.md and the CI workflow. These drift silently, producing a green local
// run against a red CI.
//
// The second has no second copy to diff — the project table describes the tree,
// and the tree is the only other record of itself. That class went unguarded
// long enough for four pkg/ packages and a doc to land unlisted while every
// test here stayed green, so the table is compared against the filesystem in
// both directions.
//
// Prose assertions were removed deliberately. Matching English could not tell a
// rule from its negation — a guard on "must remain idempotent under redelivery"
// still passed once the text read "may be non-idempotent under redelivery".
//
// Guards fail CLOSED: when a pattern stops matching its target the test fails,
// rather than falling back to a broader search and reporting green while
// watching nothing.
package docs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

const (
	agentsPath = "../AGENTS.md"
	ciPath     = "../.github/workflows/ci.yml"
)

// stepBoundary splits ci.yml into step blocks. Scoping every lookup to one
// block stops a generic key like `version:` binding to an unrelated step;
// RE2 has no lookahead, so a `.*?` gap cannot be bounded inside the pattern.
var stepBoundary = regexp.MustCompile(`(?m)^\s*- (?:name|uses):`)

// yamlComment matches a trailing or whole-line YAML comment.
var yamlComment = regexp.MustCompile(`(?m)(^|\s)#.*$`)

// readCI returns ci.yml with comments stripped. Without this, a pin that was
// commented out rather than deleted still satisfies its pattern, and the guard
// reports agreement with a value CI no longer uses. Only ci.yml is stripped —
// `#` starts a heading in Markdown.
func readCI(t *testing.T) string {
	t.Helper()
	return yamlComment.ReplaceAllString(readFile(t, ciPath), "")
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(content)
}

// ciStep returns the one step block containing marker, failing if it is
// missing or ambiguous — returning the first of several would let a stray
// mention in an earlier step shadow the real one.
func ciStep(t *testing.T, ci, marker string) string {
	t.Helper()

	var matched []string
	for _, block := range stepBoundary.Split(ci, -1) {
		if strings.Contains(block, marker) {
			matched = append(matched, block)
		}
	}

	switch len(matched) {
	case 1:
		return matched[0]
	case 0:
		t.Fatalf("no step in %s references %q — the step was renamed or removed, so this guard is no longer watching anything", ciPath, marker)
	default:
		t.Fatalf("%d steps in %s reference %q — the marker is ambiguous and could bind to the wrong step; make it more specific", len(matched), ciPath, marker)
	}
	return ""
}

// versionPin ties one pinned tool version in CI to every place AGENTS.md
// restates it.
type versionPin struct {
	name string

	// step scopes the CI lookup. Every pin sets one: an unscoped search is how
	// a key that moves or gets commented out rebinds to another tool.
	step string

	// ci captures the version from within that step.
	ci *regexp.Regexp

	// docs captures the version from AGENTS.md. Each pattern must match at
	// least once and every match must agree — checking only the aggregate
	// would let one pattern lose its anchor while others keep the test green.
	// Versions stated in unrecognized forms are invisible here.
	docs []*regexp.Regexp
}

// TestPinnedVersionsMatchCI is the assertion this file exists for. ci.yml
// carries "Keep in sync with AGENTS.md" on all four pins — an invariant both
// files assert and neither enforced. CI is the source of truth.
func TestPinnedVersionsMatchCI(t *testing.T) {
	agents := readFile(t, agentsPath)
	ci := readCI(t)

	pins := []versionPin{
		{
			name: "Go toolchain",
			step: "actions/setup-go",
			// Quoting is optional in YAML; accept either form.
			ci: regexp.MustCompile(`go-version:\s*['"]?(\d+\.\d+\.\d+)['"]?`),
			docs: []*regexp.Regexp{
				regexp.MustCompile("Go toolchain — `(\\d+\\.\\d+\\.\\d+)`"),
				regexp.MustCompile(`must report go(\d+\.\d+\.\d+)`),
				// The recovery instruction restates the pin a third time. Left
				// unwatched, a bump updates the two above and leaves agents
				// with a stale GOTOOLCHAIN to paste.
				regexp.MustCompile(`GOTOOLCHAIN=go(\d+\.\d+\.\d+)`),
			},
		},
		{
			name: "golangci-lint",
			step: "golangci-lint-action",
			ci:   regexp.MustCompile(`version:\s*(v\d+\.\d+\.\d+)`),
			docs: []*regexp.Regexp{
				regexp.MustCompile(`golangci-lint@(v\d+\.\d+\.\d+)`),
			},
		},
		{
			name: "markdownlint-cli",
			step: "Install & Run Markdown Linter",
			ci:   regexp.MustCompile(`markdownlint-cli@(\d+\.\d+\.\d+)`),
			docs: []*regexp.Regexp{
				regexp.MustCompile(`markdownlint-cli@(\d+\.\d+\.\d+)`),
			},
		},
		{
			name: "OpenTofu",
			step: "opentofu/setup-opentofu",
			ci:   regexp.MustCompile(`tofu_version:\s*['"]?(\d+\.\d+\.\d+)['"]?`),
			docs: []*regexp.Regexp{
				regexp.MustCompile("OpenTofu `(\\d+\\.\\d+\\.\\d+)`"),
			},
		},
	}

	for _, pin := range pins {
		t.Run(pin.name, func(t *testing.T) {
			block := ciStep(t, ci, pin.step)

			ciMatch := pin.ci.FindStringSubmatch(block)
			if ciMatch == nil {
				t.Fatalf("could not locate the %s version inside its CI step.\nPattern: %s\n"+
					"The key was removed or renamed. Update this pattern so the guard keeps watching.",
					pin.name, pin.ci)
			}
			want := ciMatch[1]

			// Per-pattern, not aggregate: a rewritten sentence must fail
			// loudly instead of leaving that mention unguarded.
			for _, pattern := range pin.docs {
				matches := pattern.FindAllStringSubmatch(agents, -1)
				if len(matches) == 0 {
					t.Errorf("%s is pinned to %s in %s, but AGENTS.md no longer states it in the form %s.\n"+
						"Either restore that wording or update this pattern — as written, that mention is unguarded.",
						pin.name, want, ciPath, pattern)
					continue
				}

				for i, m := range matches {
					if m[1] != want {
						t.Errorf("%s mention %d of %d matching %s is %s, but %s pins %s; bump every mention in the same commit",
							pin.name, i+1, len(matches), pattern, m[1], ciPath, want)
					}
				}
			}
		})
	}
}

var (
	// nodeVersionCI matches every node-version key in ci.yml. Deliberately
	// unscoped, unlike the pins above: two jobs set up Node, and them drifting
	// apart is itself the bug this catches. Majors only — a step pinned to
	// 24.11.0 and another to 24 compare equal, which is the intended
	// granularity while the repo pins bare majors.
	nodeVersionCI = regexp.MustCompile(`node-version:\s*['"]?(\d+)['"]?`)

	// nodeVersionDocs matches the pin everywhere AGENTS.md states it: the pin
	// itself, and the recovery command that restates it. Node is pinned to a
	// major only, so it cannot reuse the three-part pin machinery. Each pattern
	// must match at least once and every match must agree, so a bump that
	// updates the pin and forgets the recovery command fails here.
	//
	// The prose about Node 20 reaching end-of-life is deliberately in neither
	// form: it is history, not a pin, and must not be dragged forward on a bump.
	nodeVersionDocs = []*regexp.Regexp{
		regexp.MustCompile("Node — `(\\d+)`"),
		regexp.MustCompile(`node@(\d+)`),
	}
)

// TestNodeVersionMatchesCI guards the one pin TestPinnedVersionsMatchCI cannot.
// That test scopes each lookup to a single CI step, and `actions/setup-node`
// appears in two, so its ambiguity check fails closed rather than picking one.
// Left unguarded, the Node pin drifts exactly like the others: markdownlint and
// Playwright both run under whatever major CI installs, and npx resolves
// different transitive dependencies across majors.
func TestNodeVersionMatchesCI(t *testing.T) {
	ci := readCI(t)

	// Counting the steps, not just the matches, is what keeps this closed. A
	// step that stops spelling its version as a bare integer — commented out,
	// switched to node-version-file, or set to lts/* — would otherwise leave
	// the guard silently watching whichever step still matched. readCI strips
	// comments for the same reason, and aggregating alone defeats it.
	setupSteps := strings.Count(ci, "actions/setup-node")
	if setupSteps == 0 {
		t.Fatalf("no actions/setup-node step in %s; the Node setup was removed or renamed, so this guard is no longer watching anything", ciPath)
	}

	ciMatches := nodeVersionCI.FindAllStringSubmatch(ci, -1)
	if len(ciMatches) != setupSteps {
		t.Fatalf("%s has %d actions/setup-node steps but %d readable node-version pins; a step using node-version-file, lts/*, or a commented-out key would go unwatched",
			ciPath, setupSteps, len(ciMatches))
	}

	want := ciMatches[0][1]
	for i, match := range ciMatches {
		if match[1] != want {
			t.Errorf("node-version %d of %d in %s is %s, but the first is %s; the jobs would run different majors",
				i+1, len(ciMatches), ciPath, match[1], want)
		}
	}

	agents := readFile(t, agentsPath)

	// Per-pattern, not aggregate, matching TestPinnedVersionsMatchCI: a
	// rewritten sentence must fail loudly instead of leaving that mention
	// unguarded while the others keep the test green.
	for _, pattern := range nodeVersionDocs {
		docMatches := pattern.FindAllStringSubmatch(agents, -1)
		if len(docMatches) == 0 {
			t.Errorf("Node is pinned to %s in %s, but AGENTS.md no longer states it in the form %s.\n"+
				"Either restore that wording or update this pattern — as written, that mention is unguarded.",
				want, ciPath, pattern)
			continue
		}

		for i, match := range docMatches {
			if match[1] != want {
				t.Errorf("Node mention %d of %d matching %s is %s, but %s pins %s; bump every mention in the same commit",
					i+1, len(docMatches), pattern, match[1], ciPath, want)
			}
		}
	}
}

// TestLocalStackCommandMatchesCI compares every occurrence on both sides.
// Exact equality, not containment: a truncated service list is a prefix of the
// full one and would otherwise pass.
func TestLocalStackCommandMatchesCI(t *testing.T) {
	assertCommandMatches(t,
		regexp.MustCompile(`docker compose up --build --detach[^\n]*`),
		"Compose stack command",
		"agents copy it verbatim to bring up the Playwright stack",
	)
}

// markdownlintInvocation captures the command across however many backslash
// continuation lines it uses. Consuming a fixed number would leave every later
// line unread — and since the file list is already near the wrap width, the
// next entry added is the one that would create that blind spot.
var markdownlintInvocation = regexp.MustCompile(`markdownlint-cli@(?:[^\n]*\\\n)*[^\n]*`)

var quotedArg = regexp.MustCompile(`"([^"]+)"`)

// markdownlintFiles returns the sorted set of quoted paths passed to
// markdownlint. Comparing sets rather than matching the raw line is what makes
// this insensitive to where an entry is inserted: anchoring on the first
// element would miss anything prepended before it.
func markdownlintFiles(t *testing.T, content, source string) []string {
	t.Helper()

	invocation := markdownlintInvocation.FindString(content)
	if invocation == "" {
		t.Fatalf("could not locate the markdownlint invocation in %s; update this pattern if the command changed", source)
	}

	var files []string
	for _, m := range quotedArg.FindAllStringSubmatch(invocation, -1) {
		files = append(files, m[1])
	}
	if len(files) == 0 {
		t.Fatalf("the markdownlint invocation in %s lists no files", source)
	}

	sort.Strings(files)
	return files
}

// TestMarkdownlintFileListMatchesCI guards the invariant AGENTS.md calls a
// landmine: the file list is hardcoded in the workflow, so adding a doc to one
// list and not the other leaves the new file silently unlinted — or documents a
// command that fails on a file CI never checks.
func TestMarkdownlintFileListMatchesCI(t *testing.T) {
	ciFiles := markdownlintFiles(t, readCI(t), ciPath)
	docFiles := markdownlintFiles(t, readFile(t, agentsPath), agentsPath)

	if !slices.Equal(ciFiles, docFiles) {
		t.Errorf("markdownlint file lists disagree:\n  %s: %v\n  AGENTS.md: %v\n"+
			"a doc listed in only one place is either silently unlinted or fails the documented command",
			ciPath, ciFiles, docFiles)
	}
}

// TestListedDocsExist closes the gap left by removing main's
// TestDocumentationFilesExist. Its keyword assertions were dropped as
// meaningless — "Go" matched "Google" — but its os.ReadFile check was a real
// file-existence guard, and nothing else covers it: markdownlint-cli exits 0
// when a listed file is missing, so deleting README.md is green everywhere.
func TestListedDocsExist(t *testing.T) {
	// Non-glob entries from the markdownlint list, plus the docs AGENTS.md's
	// project table names. Globs are skipped: they match zero files silently.
	// The table names are read from the table rather than restated here — a
	// hardcoded mirror is the drift this file exists to prevent.
	required := markdownlintFiles(t, readCI(t), ciPath)
	for _, name := range backtickedNames(projectTableRow(t, readFile(t, agentsPath), "docs/")) {
		required = append(required, "docs/"+name)
	}

	for _, name := range required {
		if strings.ContainsAny(name, "*?[") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			// ReadFile rather than Stat: it also rejects a path replaced by a
			// directory, which Stat would accept.
			content, err := os.ReadFile(filepath.Join("..", name))
			if err != nil {
				t.Errorf("%s is referenced by CI or AGENTS.md but cannot be read: %v", name, err)
				return
			}
			if len(content) == 0 {
				t.Errorf("%s is referenced by CI or AGENTS.md but is empty", name)
			}
		})
	}
}

// projectTableRow returns the Contents cell of the project-table row whose Path
// cell is exactly path. Like ciStep it fails closed on both counts, missing and
// ambiguous: a renamed or reformatted row stops the guard rather than leaving it
// silently watching nothing, and a second row for the same path would otherwise
// bind the guard to whichever came first.
func projectTableRow(t *testing.T, agents, path string) string {
	t.Helper()

	row := regexp.MustCompile("(?m)^\\| `" + regexp.QuoteMeta(path) + "` \\| (.*) \\|$")

	matched := row.FindAllStringSubmatch(agents, -1)
	switch len(matched) {
	case 1:
		return matched[0][1]
	case 0:
		t.Fatalf("no project-table row for %q in AGENTS.md; the table was renamed or reformatted, so this guard is no longer watching anything", path)
	default:
		t.Fatalf("%d project-table rows in AGENTS.md claim the path %q; the guard would bind to the first and ignore the rest", len(matched), path)
	}
	return ""
}

// backtickedName extracts one `quoted` name from a table cell. Hoisted beside
// quotedArg, its direct analogue, rather than compiled per call.
var backtickedName = regexp.MustCompile("`([^`]+)`")

// backtickedNames returns the sorted `quoted` names in a table cell. Sorting is
// what lets the table stay in whatever order reads best — pkg/ is grouped by
// role, not alphabetically — while still comparing as a set. Duplicates are
// kept rather than collapsed, so a name listed twice fails against the tree
// instead of passing as a set that happens to match.
//
// Every backticked token in a guarded cell is read as a name, so those cells
// must carry no other inline code — a parenthetical like `main` package would
// be compared against the tree and fail.
func backtickedNames(cell string) []string {
	var names []string
	for _, m := range backtickedName.FindAllStringSubmatch(cell, -1) {
		names = append(names, m[1])
	}

	sort.Strings(names)
	return names
}

// treeEntries returns the sorted names in dir that satisfy keep. It does not
// recurse: the project table names top-level entries, so a nested doc is linted
// by the `docs/**/*.md` glob without being required in the table. That holds
// while docs/ stays flat; a subdirectory needs this to walk instead.
func treeEntries(t *testing.T, dir string, keep func(os.DirEntry) bool) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join("..", dir))
	if err != nil {
		t.Fatalf("failed to read %s: %v", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if keep(entry) {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)
	return names
}

// TestProjectTableMatchesTree closes the direction TestListedDocsExist leaves
// open. That guard asks whether everything named exists; this one asks whether
// everything that exists is named. The second question is the one that went
// unanswered: four pkg/ packages and a doc landed without reaching the table,
// and every guard in this file stayed green through all of it, because a table
// describing the tree has no second copy to diff against.
func TestProjectTableMatchesTree(t *testing.T) {
	agents := readFile(t, agentsPath)

	isDir := func(e os.DirEntry) bool { return e.IsDir() }
	isMarkdown := func(e os.DirEntry) bool {
		return !e.IsDir() && strings.HasSuffix(e.Name(), ".md")
	}

	rows := []struct {
		path string
		dir  string
		keep func(os.DirEntry) bool
		what string
	}{
		{"cmd/", "cmd", isDir, "service entrypoint"},
		{"pkg/", "pkg", isDir, "shared package"},
		{"docs/", "docs", isMarkdown, "document"},
	}

	for _, row := range rows {
		t.Run(row.path, func(t *testing.T) {
			listed := backtickedNames(projectTableRow(t, agents, row.path))
			actual := treeEntries(t, row.dir, row.keep)

			if !slices.Equal(listed, actual) {
				t.Errorf("the %s row of AGENTS.md's project table disagrees with the tree:\n  listed: %v\n  actual: %v\n"+
					"every %s must be named in the table — an unlisted one is invisible to an agent scoping work, and a listed one that no longer exists sends it looking for nothing",
					row.path, listed, actual, row.what)
			}
		})
	}
}

// assertCommandMatches requires a command to appear identically in AGENTS.md
// and ci.yml, in every place either file states it.
func assertCommandMatches(t *testing.T, pattern *regexp.Regexp, name, why string) {
	t.Helper()

	agents := readFile(t, agentsPath)
	ci := readCI(t)

	ciMatches := pattern.FindAllString(ci, -1)
	if len(ciMatches) == 0 {
		t.Fatalf("could not locate the %s in %s using %s; update this pattern if the job changed", name, ciPath, pattern)
	}

	docMatches := pattern.FindAllString(agents, -1)
	if len(docMatches) == 0 {
		t.Fatalf("could not locate the %s in AGENTS.md using %s — %s, so it must be documented", name, pattern, why)
	}

	want := strings.TrimSpace(ciMatches[0])
	for i, got := range append(ciMatches, docMatches...) {
		if strings.TrimSpace(got) != want {
			t.Errorf("%s occurrence %d disagrees.\n  want: %s\n  got:  %s\n%s",
				name, i+1, want, strings.TrimSpace(got), why)
		}
	}
}

// TestRegisteredSubagentsDeferToAgentSourceOfTruth enforces the rule at the top
// of AGENTS.md: subagents describe their role but defer pinned commands and
// versions to the source of truth, so there is only one place to update. It
// matches commands and version literals — mechanical strings, not prose.
func TestRegisteredSubagentsDeferToAgentSourceOfTruth(t *testing.T) {
	agentFiles := []string{"ui-review.md", "verifier.md", "code-review.md"}

	// Patterns, not literals: the real commands carry version pins
	// (`golangci-lint@v2.12.2 run`), so a bare-string check would never fire
	// on the copy-paste it is meant to catch.
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`golangci-lint(@v[\d.]+)?\s+run`),
		regexp.MustCompile(`markdownlint-cli(@[\d.]+)?\s+--config`),
		regexp.MustCompile(`docker[ -]compose up`),
		regexp.MustCompile(`tofu (fmt|validate)`),
		regexp.MustCompile(`go test -race`),
		regexp.MustCompile(`npx playwright test`),
		// Bare version literals, including the `go1.26.5` form AGENTS.md tells
		// agents to look for — there is no word boundary between `o` and `1`,
		// so a `\b`-anchored pattern would miss it.
		regexp.MustCompile(`\bgo\d+\.\d+\.\d+`),
		regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`),
	}

	for _, name := range agentFiles {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", ".claude", "agents", name)
			content := readFile(t, path)

			// A real reference, not a substring: `\b` matches inside
			// `NOT-AGENTS.md`, since `-` is already a non-word character.
			if !regexp.MustCompile(`(^|[^\w-])AGENTS\.md($|[^\w])`).MatchString(content) {
				t.Errorf("%s does not defer to AGENTS.md", path)
			}

			for _, pattern := range forbidden {
				if match := pattern.FindString(content); match != "" {
					t.Errorf("%s inlines %q instead of deferring to AGENTS.md, which owns the pinned versions and commands", path, match)
				}
			}
		})
	}
}
