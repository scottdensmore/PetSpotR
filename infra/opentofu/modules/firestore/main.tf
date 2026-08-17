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

variable "region" { type = string }

output "database_name" {
  value = google_firestore_database.database.name
}
