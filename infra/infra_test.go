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
			"PUBSUB_PUSH_SUBSCRIPTION",
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

	t.Run("match found events use an authenticated private notification push subscription", func(t *testing.T) {
		pubsubPath := filepath.Join(root, "infra", "opentofu", "modules", "pubsub", "main.tf")
		pubsubContent, err := os.ReadFile(pubsubPath)
		if err != nil {
			t.Fatalf("read Pub/Sub module: %v", err)
		}
		for _, fragment := range []string{
			`resource "google_pubsub_subscription" "match_found_backlog"`,
			`resource "google_pubsub_topic" "match_found_dead_letter"`,
			`resource "google_service_account" "notification_invoker"`,
			`resource "google_cloud_run_v2_service_iam_member" "notification_invoker"`,
			`push_endpoint = "${trimsuffix(var.notification_service_url, "/")}/pubsub/match-found"`,
			"dead_letter_policy",
			"retry_policy",
		} {
			if !strings.Contains(string(pubsubContent), fragment) {
				t.Errorf("Pub/Sub module missing %q", fragment)
			}
		}

		cloudRunPath := filepath.Join(root, "infra", "opentofu", "modules", "cloudrun", "main.tf")
		cloudRunContent, err := os.ReadFile(cloudRunPath)
		if err != nil {
			t.Fatalf("read Cloud Run module: %v", err)
		}
		for _, fragment := range []string{
			`resource "google_service_account" "notification_runtime"`,
			`resource "google_project_iam_member" "notification_datastore"`,
			`resource "google_cloud_run_v2_service" "notification_service"`,
			`ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"`,
			`name  = "PUBSUB_PUSH_SUBSCRIPTION"`,
			`value = "projects/${var.project_id}/subscriptions/match-found-notification-backlog"`,
			`value = "pubsub-notification-invoker@${var.project_id}.iam.gserviceaccount.com"`,
		} {
			if !strings.Contains(string(cloudRunContent), fragment) {
				t.Errorf("Cloud Run module missing %q", fragment)
			}
		}
	})

	t.Run("lost pet events use managed publication and authenticated notification push", func(t *testing.T) {
		pubsubPath := filepath.Join(root, "infra", "opentofu", "modules", "pubsub", "main.tf")
		pubsubContent, err := os.ReadFile(pubsubPath)
		if err != nil {
			t.Fatalf("read Pub/Sub module: %v", err)
		}
		for _, fragment := range []string{
			`resource "google_pubsub_topic_iam_member" "lost_pet_publisher"`,
			`member = "serviceAccount:${var.lostpet_runtime_service_account}"`,
			`resource "google_pubsub_topic" "lost_pet_dead_letter"`,
			`resource "google_pubsub_subscription" "lost_pet_notification"`,
			`push_endpoint = "${trimsuffix(var.notification_service_url, "/")}/pubsub/lost-pet"`,
			"dead_letter_policy",
			"retry_policy",
		} {
			if !strings.Contains(string(pubsubContent), fragment) {
				t.Errorf("Pub/Sub module missing %q", fragment)
			}
		}

		cloudRunPath := filepath.Join(root, "infra", "opentofu", "modules", "cloudrun", "main.tf")
		cloudRunContent, err := os.ReadFile(cloudRunPath)
		if err != nil {
			t.Fatalf("read Cloud Run module: %v", err)
		}
		for _, fragment := range []string{
			`resource "google_service_account" "lostpet_runtime"`,
			`resource "google_project_iam_member" "lostpet_datastore"`,
			`service_account = google_service_account.lostpet_runtime.email`,
			`name  = "PUBSUB_LOST_SUBSCRIPTION"`,
			`value = "projects/${var.project_id}/subscriptions/lost-pet-notification"`,
		} {
			if !strings.Contains(string(cloudRunContent), fragment) {
				t.Errorf("Cloud Run module missing %q", fragment)
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
