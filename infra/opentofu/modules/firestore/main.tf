resource "google_firestore_database" "database" {
  project                           = var.project_id
  name                              = "(default)"
  location_id                       = var.region
  type                              = "FIRESTORE_NATIVE"
  delete_protection_state           = var.deletion_protection ? "DELETE_PROTECTION_ENABLED" : "DELETE_PROTECTION_DISABLED"
  point_in_time_recovery_enablement = "POINT_IN_TIME_RECOVERY_ENABLED"
}

resource "google_firestore_backup_schedule" "daily" {
  project   = var.project_id
  database  = google_firestore_database.database.name
  retention = "604800s" # 7 days
  daily_recurrence {}
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

resource "google_firestore_index" "lost_pet_candidates" {
  database   = google_firestore_database.database.name
  collection = "lostPets"

  fields {
    field_path = "lostStatus"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostGeocodingStatus"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostReportedAt"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostLatitude"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostLongitude"
    order      = "ASCENDING"
  }

  fields {
    field_path = "key"
    order      = "ASCENDING"
  }
}

resource "google_firestore_index" "lost_pet_candidates_by_species" {
  database   = google_firestore_database.database.name
  collection = "lostPets"

  fields {
    field_path = "lostStatus"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostGeocodingStatus"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostSpecies"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostReportedAt"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostLatitude"
    order      = "ASCENDING"
  }

  fields {
    field_path = "lostLongitude"
    order      = "ASCENDING"
  }

  fields {
    field_path = "key"
    order      = "ASCENDING"
  }
}

variable "project_id" {
  description = "Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "GCP Deployment Region"
  type        = string
}

variable "deletion_protection" {
  description = "Enable deletion protection on Firestore database"
  type        = bool
  default     = true
}

output "database_name" {
  value = google_firestore_database.database.name
}
