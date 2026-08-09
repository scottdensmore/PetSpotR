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
		"11. **Complete the Codex GitHub review loop.**",
		"chatgpt-codex-connector[bot]",
		"`eyes`",
		"`+1`",
		"cutoff timestamp immediately before",
		"reaction's `created_at`",
		"PR head still matches the recorded SHA",
		"conversation comments, review bodies",
		"thread-aware inline comments",
		"resolve every addressed thread",
		"12. **Merge only clean, passing pull requests.**",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("AGENTS.md missing Codex review workflow %q", fragment)
		}
	}
}
