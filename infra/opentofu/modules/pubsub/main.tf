data "google_project" "current" {
  project_id = var.project_id
}

locals {
  pubsub_service_agent = "service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

resource "google_pubsub_topic" "lost_pet" {
  name = "lostPet"
}

resource "google_pubsub_topic" "found_pet" {
  name = "foundPet"
}

resource "google_pubsub_topic" "match_found" {
  name = "matchFound"
}

resource "google_pubsub_topic" "found_pet_dead_letter" {
  name = "foundPet-dead-letter"
}

resource "google_pubsub_topic_iam_member" "found_pet_publisher" {
  topic  = google_pubsub_topic.found_pet.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${var.foundpet_runtime_service_account}"
}

resource "google_pubsub_topic_iam_member" "match_found_publisher" {
  topic  = google_pubsub_topic.match_found.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${var.pet_matcher_runtime_service_account}"
}

resource "google_service_account" "pet_matcher_invoker" {
  account_id   = "pubsub-pet-matcher-invoker"
  display_name = "Pub/Sub pet-matcher push invoker"
}

resource "google_service_account_iam_member" "pubsub_token_creator" {
  service_account_id = google_service_account.pet_matcher_invoker.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${local.pubsub_service_agent}"
}

resource "google_cloud_run_v2_service_iam_member" "pet_matcher_invoker" {
  project  = var.project_id
  location = var.region
  name     = var.pet_matcher_name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.pet_matcher_invoker.email}"
}

resource "google_pubsub_subscription" "found_pet_matcher" {
  name  = "found-pet-matcher"
  topic = google_pubsub_topic.found_pet.id

  ack_deadline_seconds       = 600
  message_retention_duration = "604800s"

  expiration_policy {
    ttl = ""
  }

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.found_pet_dead_letter.id
    max_delivery_attempts = 10
  }

  push_config {
    push_endpoint = "${trimsuffix(var.pet_matcher_url, "/")}/pubsub/found-pet"

    oidc_token {
      service_account_email = google_service_account.pet_matcher_invoker.email
      audience              = var.pet_matcher_url
    }
  }

  depends_on = [
    google_cloud_run_v2_service_iam_member.pet_matcher_invoker,
    google_pubsub_topic_iam_member.dead_letter_publisher,
    google_service_account_iam_member.pubsub_token_creator,
  ]
}

resource "google_pubsub_topic_iam_member" "dead_letter_publisher" {
  topic  = google_pubsub_topic.found_pet_dead_letter.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${local.pubsub_service_agent}"
}

resource "google_pubsub_subscription_iam_member" "found_pet_subscriber" {
  subscription = google_pubsub_subscription.found_pet_matcher.name
  role         = "roles/pubsub.subscriber"
  member       = "serviceAccount:${local.pubsub_service_agent}"
}

resource "google_pubsub_subscription" "found_pet_dead_letter_retention" {
  name                       = "found-pet-dead-letter-retention"
  topic                      = google_pubsub_topic.found_pet_dead_letter.id
  message_retention_duration = "2678400s"

  expiration_policy {
    ttl = ""
  }
}

# Preserve matchFound events until the notification push consumer is migrated
# in the next #108 slice. A subscription is required for Pub/Sub to retain them.
resource "google_pubsub_subscription" "match_found_backlog" {
  name                       = "match-found-notification-backlog"
  topic                      = google_pubsub_topic.match_found.id
  message_retention_duration = "2678400s"

  expiration_policy {
    ttl = ""
  }
}

variable "project_id" { type = string }
variable "region" { type = string }
variable "pet_matcher_name" { type = string }
variable "pet_matcher_url" { type = string }
variable "foundpet_runtime_service_account" { type = string }
variable "pet_matcher_runtime_service_account" { type = string }

output "lost_pet_topic" {
  value = google_pubsub_topic.lost_pet.name
}

output "found_pet_topic" {
  value = google_pubsub_topic.found_pet.name
}

output "match_found_topic" {
  value = google_pubsub_topic.match_found.name
}
