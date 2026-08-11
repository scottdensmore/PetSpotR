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

	t.Run("found pet events use an authenticated private push subscription", func(t *testing.T) {
		path := filepath.Join(root, "infra", "opentofu", "modules", "pubsub", "main.tf")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Pub/Sub module: %v", err)
		}

		required := []string{
			`resource "google_pubsub_subscription" "found_pet_matcher"`,
			"push_config",
			"oidc_token",
			`roles/run.invoker`,
			"dead_letter_policy",
			"retry_policy",
			`resource "google_pubsub_subscription" "match_found_backlog"`,
			`resource "google_pubsub_topic_iam_member" "found_pet_publisher"`,
			`resource "google_pubsub_topic_iam_member" "match_found_publisher"`,
			`roles/pubsub.publisher`,
		}
		for _, fragment := range required {
			if !strings.Contains(string(content), fragment) {
				t.Errorf("Pub/Sub module missing %q", fragment)
			}
		}
	})

	t.Run("pet matcher receives its authenticated push identity configuration", func(t *testing.T) {
		path := filepath.Join(root, "infra", "opentofu", "modules", "cloudrun", "main.tf")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Cloud Run module: %v", err)
		}

		for _, name := range []string{
			"PUBSUB_FOUND_SUBSCRIPTION",
			"PUBSUB_PUSH_SERVICE_ACCOUNT",
			`resource "google_service_account" "foundpet_runtime"`,
			`resource "google_service_account" "pet_matcher_runtime"`,
			`roles/datastore.user`,
			"min_instance_count = 1",
			"max_instance_count = 1",
			"cpu_idle = false",
		} {
			if !strings.Contains(string(content), name) {
				t.Errorf("Cloud Run module missing %s", name)
			}
		}
	})

	t.Run("Firestore indexes pending outbox scans", func(t *testing.T) {
		path := filepath.Join(root, "infra", "opentofu", "modules", "firestore", "main.tf")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Firestore module: %v", err)
		}

		for _, fragment := range []string{`resource "google_firestore_index" "pending_outbox"`, "createdAt", "topic", "status"} {
			if !strings.Contains(string(content), fragment) {
				t.Errorf("Firestore module missing %q", fragment)
			}
		}
	})
}
