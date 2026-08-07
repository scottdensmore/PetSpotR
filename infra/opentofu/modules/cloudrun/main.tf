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
    containers {
      image = var.lostpet_image
    }
  }
}

resource "google_cloud_run_v2_service" "foundpet_service" {
  name     = "foundpet-service"
  location = var.region

  template {
    containers {
      image = var.foundpet_image
    }
  }
}

resource "google_cloud_run_v2_service" "pet_matcher" {
  name     = "pet-matcher"
  location = var.region

  template {
    containers {
      image = var.pet_matcher_image
    }
  }
}

resource "google_cloud_run_v2_service" "notification_service" {
  name     = "notification-service"
  location = var.region

  template {
    containers {
      image = var.notification_service_image
    }
  }
}

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

output "notification_service_url" {
  value = google_cloud_run_v2_service.notification_service.uri
}
