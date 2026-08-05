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

variable "region" { type = string }
variable "lostpet_image" { type = string }
variable "foundpet_image" { type = string }

output "lostpet_url" {
  value = google_cloud_run_v2_service.lostpet_service.uri
}

output "foundpet_url" {
  value = google_cloud_run_v2_service.foundpet_service.uri
}
