resource "google_firestore_database" "database" {
  name        = "(default)"
  location_id = var.region
  type        = "FIRESTORE_NATIVE"
}

variable "region" { type = string }

output "database_name" {
  value = google_firestore_database.database.name
}
