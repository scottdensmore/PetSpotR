// Package docs_test compares machine-readable values that AGENTS.md and the CI
// workflow both encode: version pins, the Compose service list, and the
// markdownlint file list. Each is duplicated verbatim across two files and
// drifts silently, producing a green local run against a red CI.
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
	required := markdownlintFiles(t, readCI(t), ciPath)
	required = append(required, "docs/DEVELOPMENT.md", "docs/MIGRATION_PLAN.md", "docs/ROADMAP.md")

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
