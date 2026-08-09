package tests_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaywrightUserJourneysExist(t *testing.T) {
	root := ".."

	t.Run("playwright.config.ts exists and sets test directory", func(t *testing.T) {
		path := filepath.Join(root, "tests", "playwright", "playwright.config.ts")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read playwright.config.ts: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "testDir: './e2e'") {
			t.Errorf("playwright.config.ts missing testDir definition")
		}
	})

	t.Run("package.json exists and defines Playwright dependency", func(t *testing.T) {
		path := filepath.Join(root, "tests", "playwright", "package.json")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read package.json: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "@playwright/test") {
			t.Errorf("package.json missing @playwright/test dev dependency")
		}
	})

	t.Run("Lost Pet user journey spec exists", func(t *testing.T) {
		path := filepath.Join(root, "tests", "playwright", "e2e", "lost-pet-journey.spec.ts")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read lost-pet-journey.spec.ts: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "/lostPet") || !strings.Contains(str, "201") {
			t.Errorf("lost-pet-journey.spec.ts missing endpoint or status assertion")
		}
	})

	t.Run("Found Pet user journey spec exists", func(t *testing.T) {
		path := filepath.Join(root, "tests", "playwright", "e2e", "found-pet-matching-journey.spec.ts")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read found-pet-matching-journey.spec.ts: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "/foundPet") || !strings.Contains(str, "201") {
			t.Errorf("found-pet-matching-journey.spec.ts missing endpoint or status assertion")
		}
	})
}

func TestCIWorkflowRunsPlaywrightAPIJourneys(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "ci.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read CI workflow: %v", err)
	}

	workflow := string(content)
	required := []string{
		"e2e-playwright-tests:",
		"name: Playwright API Journeys",
		"docker compose up --build --detach lostpet-service foundpet-service web-frontend",
		"npm ci",
		"LOSTPET_SERVICE_URL: http://localhost:8080",
		"FOUNDPET_SERVICE_URL: http://localhost:8081",
		"WEB_FRONTEND_URL: http://localhost:8082",
		"npx playwright test",
		"actions/upload-artifact@v4",
		"docker compose down",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("CI workflow missing Playwright journey configuration %q", fragment)
		}
	}
}
