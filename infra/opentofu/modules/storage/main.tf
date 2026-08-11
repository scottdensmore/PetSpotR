resource "google_storage_bucket" "pet_images" {
  name                        = "${var.project_id}-pet-images"
  location                    = var.region
  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = true
  }

  dynamic "cors" {
    for_each = length(var.allowed_origins) == 0 ? [] : [var.allowed_origins]
    content {
      origin          = cors.value
      method          = ["GET", "HEAD", "POST"]
      response_header = ["Content-Type", "ETag", "x-goog-generation"]
      max_age_seconds = 3600
    }
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age            = 1
      matches_prefix = ["uploads/"]
      with_state     = "ANY"
    }
  }
}

variable "project_id" { type = string }
variable "region" { type = string }
variable "allowed_origins" { type = list(string) }
output "bucket_name" {
  value = google_storage_bucket.pet_images.name
}
