resource "google_storage_bucket" "pet_images" {
  name                        = "${var.project_id}-pet-images"
  location                    = var.region
  force_destroy               = true
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }
}

variable "project_id" { type = string }
variable "region" { type = string }

output "bucket_name" {
  value = google_storage_bucket.pet_images.name
}
