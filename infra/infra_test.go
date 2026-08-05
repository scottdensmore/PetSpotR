package infra_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenTofuInfrastructureModules(t *testing.T) {
	root := ".."

	t.Run("main.tf, variables.tf, and outputs.tf exist in infra/opentofu", func(t *testing.T) {
		files := []string{"main.tf", "variables.tf", "outputs.tf"}
		for _, f := range files {
			path := filepath.Join(root, "infra", "opentofu", f)
			_, err := os.Stat(path)
			if err != nil {
				t.Errorf("missing OpenTofu file %s: %v", f, err)
			}
		}
	})

	t.Run("modules directory contains required GCP submodules", func(t *testing.T) {
		modules := []string{"storage", "pubsub", "firestore", "cloudrun"}
		for _, m := range modules {
			path := filepath.Join(root, "infra", "opentofu", "modules", m, "main.tf")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("missing submodule main.tf for %s: %v", m, err)
				continue
			}

			str := string(content)
			if !strings.Contains(str, "resource \"google_") {
				t.Errorf("submodule %s main.tf missing google resource definition", m)
			}
		}
	})
}
