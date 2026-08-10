package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentationFilesExist(t *testing.T) {
	root := ".."

	t.Run("docs/DEVELOPMENT.md exists and contains local & GCP deployment steps", func(t *testing.T) {
		path := filepath.Join(root, "docs", "DEVELOPMENT.md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read docs/DEVELOPMENT.md: %v", err)
		}

		str := string(content)
		keywords := []string{"docker compose", "Ollama", "gemma4", "Cloud Run", "OpenTofu", "go test"}
		for _, kw := range keywords {
			if !strings.Contains(str, kw) {
				t.Errorf("docs/DEVELOPMENT.md missing keyword %s", kw)
			}
		}
	})

	t.Run("README.md is updated with Go architecture references", func(t *testing.T) {
		path := filepath.Join(root, "README.md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read README.md: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "Go") || !strings.Contains(str, "Ollama") {
			t.Errorf("README.md missing Go or Ollama architecture references")
		}
	})
}

func TestAgentWorkflowIncludesCodexReviewGate(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}

	workflow := string(content)
	required := []string{
		"## Code Review Rules",
		"Pub/Sub handlers must remain idempotent under redelivery",
		"Event schema changes must remain backward compatible",
		"Reporter contact details and state-changing actions",
		"11. **Complete the Codex GitHub review loop.**",
		"chatgpt-codex-connector[bot]",
		"`@codex review`",
		"`eyes`",
		"`+1`",
		"exact trigger comment",
		"GitHub reports that SHA as the PR head",
		"trigger comment's",
		"`created_at` value as the cutoff",
		"review's `commit.oid` equals the expected full SHA",
		"PR head still matches the expected SHA",
		"conversation comments, review bodies",
		"thread-aware inline comments",
		"resolve every addressed thread",
		"repository policy gate",
		"GitHub required status check",
		"self-contained in-process cascade coverage",
		"docker compose up --build --detach lostpet-service foundpet-service web-frontend",
		"Neither CI nor the documented verifier commands exercise a live Ollama",
		"12. **Merge only clean, passing pull requests.**",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("AGENTS.md missing Codex review workflow %q", fragment)
		}
	}

	forbidden := []string{
		"There is **no frontend**",
		"All four services",
		"docker-compose up --build",
		"ollama pull gemma4:e2b",
		"reviewer runs after a pull request opens and after every push",
		"both the trigger comment and the pull request",
		"CI does **not** run `e2e/`",
		"Both need the full stack",
		"no local stack for `e2e/`",
	}
	for _, fragment := range forbidden {
		if strings.Contains(workflow, fragment) {
			t.Errorf("AGENTS.md contains stale workflow guidance %q", fragment)
		}
	}
}

func TestRegisteredSubagentsDeferToAgentSourceOfTruth(t *testing.T) {
	paths := []string{
		filepath.Join("..", ".claude", "agents", "ui-review.md"),
		filepath.Join("..", ".claude", "agents", "verifier.md"),
		filepath.Join("..", ".claude", "agents", "code-review.md"),
	}
	forbidden := []string{
		"currently has **no frontend**",
		"docker-compose up --build",
		"gemma2:2b",
		"golangci-lint run",
		"markdownlint-cli --config",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}

		entrypoint := string(content)
		if !strings.Contains(entrypoint, "AGENTS.md") {
			t.Errorf("%s does not defer to AGENTS.md", path)
		}
		for _, fragment := range forbidden {
			if strings.Contains(entrypoint, fragment) {
				t.Errorf("%s contains stale guidance %q", path, fragment)
			}
		}
	}
}
