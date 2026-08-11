resource "google_service_account" "lostpet_runtime" {
  account_id   = "lostpet-runtime"
  display_name = "Lost-pet Cloud Run runtime"
}

resource "google_service_account" "foundpet_runtime" {
  account_id   = "foundpet-runtime"
  display_name = "Found-pet Cloud Run runtime"
}

resource "google_project_iam_member" "lostpet_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.lostpet_runtime.email}"
}

resource "google_service_account" "pet_matcher_runtime" {
  account_id   = "pet-matcher-runtime"
  display_name = "Pet matcher Cloud Run runtime"
}

resource "google_service_account" "notification_runtime" {
  account_id   = "notification-runtime"
  display_name = "Notification Cloud Run runtime"
}

resource "google_project_iam_member" "foundpet_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.foundpet_runtime.email}"
}

resource "google_project_iam_member" "pet_matcher_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.pet_matcher_runtime.email}"
}

resource "google_project_iam_member" "notification_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.notification_runtime.email}"
}

resource "google_cloud_run_v2_service" "web_frontend" {
  name     = "web-frontend"
  location = var.region

  template {
    containers {
      image = var.web_frontend_image
    }
  }
}

resource "google_cloud_run_v2_service" "lostpet_service" {
  name     = "lostpet-service"
  location = var.region

  template {
    service_account = google_service_account.lostpet_runtime.email

    scaling {
      min_instance_count = 1
      max_instance_count = 1
    }

    containers {
      image = var.lostpet_image

      resources {
        cpu_idle = false
      }
    }
  }
}

resource "google_cloud_run_v2_service" "foundpet_service" {
  name     = "foundpet-service"
  location = var.region

  template {
    service_account = google_service_account.foundpet_runtime.email

    scaling {
      min_instance_count = 1
      max_instance_count = 1
    }

    containers {
      image = var.foundpet_image

      resources {
        cpu_idle = false
      }
    }
  }
}

resource "google_cloud_run_v2_service" "pet_matcher" {
  name     = "pet-matcher"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  template {
    service_account = google_service_account.pet_matcher_runtime.email
    timeout         = "600s"

    containers {
      image = var.pet_matcher_image

      env {
        name  = "PUBSUB_PUSH_SUBSCRIPTION"
        value = "projects/${var.project_id}/subscriptions/found-pet-matcher"
      }

      env {
        name  = "PUBSUB_PUSH_SERVICE_ACCOUNT"
        value = "pubsub-pet-matcher-invoker@${var.project_id}.iam.gserviceaccount.com"
      }
    }
  }
}

resource "google_cloud_run_v2_service" "notification_service" {
  name     = "notification-service"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  template {
    service_account = google_service_account.notification_runtime.email

    containers {
      image = var.notification_service_image

      env {
        name  = "PUBSUB_PUSH_SUBSCRIPTION"
        value = "projects/${var.project_id}/subscriptions/match-found-notification-backlog"
      }

      env {
        name  = "PUBSUB_PUSH_SERVICE_ACCOUNT"
        value = "pubsub-notification-invoker@${var.project_id}.iam.gserviceaccount.com"
      }

      env {
        name  = "PUBSUB_LOST_SUBSCRIPTION"
        value = "projects/${var.project_id}/subscriptions/lost-pet-notification"
      }
    }
  }
}

variable "project_id" { type = string }
variable "region" { type = string }
variable "web_frontend_image" { type = string }
variable "lostpet_image" { type = string }
variable "foundpet_image" { type = string }
variable "pet_matcher_image" { type = string }
variable "notification_service_image" { type = string }

output "web_frontend_url" {
  value = google_cloud_run_v2_service.web_frontend.uri
}

output "lostpet_url" {
  value = google_cloud_run_v2_service.lostpet_service.uri
}

output "foundpet_url" {
  value = google_cloud_run_v2_service.foundpet_service.uri
}

output "pet_matcher_url" {
  value = google_cloud_run_v2_service.pet_matcher.uri
}

output "pet_matcher_name" {
  value = google_cloud_run_v2_service.pet_matcher.name
}

output "foundpet_runtime_service_account" {
  value = google_service_account.foundpet_runtime.email
}

output "lostpet_runtime_service_account" {
  value = google_service_account.lostpet_runtime.email
}

output "pet_matcher_runtime_service_account" {
  value = google_service_account.pet_matcher_runtime.email
}

output "notification_service_url" {
  value = google_cloud_run_v2_service.notification_service.uri
}

output "notification_service_name" {
  value = google_cloud_run_v2_service.notification_service.name
}

output "notification_runtime_service_account" {
  value = google_service_account.notification_runtime.email
}
