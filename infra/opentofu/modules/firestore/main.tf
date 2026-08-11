resource "google_firestore_database" "database" {
  name        = "(default)"
  location_id = var.region
  type        = "FIRESTORE_NATIVE"
}

resource "google_firestore_index" "pending_outbox" {
  database   = google_firestore_database.database.name
  collection = "eventOutbox"

  fields {
    field_path = "topic"
    order      = "ASCENDING"
  }

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "createdAt"
    order      = "ASCENDING"
  }

  fields {
    field_path = "key"
    order      = "ASCENDING"
  }
}

variable "region" { type = string }

output "database_name" {
  value = google_firestore_database.database.name
}
