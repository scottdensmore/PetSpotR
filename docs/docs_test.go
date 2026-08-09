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
