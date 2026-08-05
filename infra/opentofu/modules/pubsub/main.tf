resource "google_pubsub_topic" "lost_pet" {
  name = "lostPet"
}

resource "google_pubsub_topic" "found_pet" {
  name = "foundPet"
}

resource "google_pubsub_topic" "match_found" {
  name = "matchFound"
}

output "lost_pet_topic" {
  value = google_pubsub_topic.lost_pet.name
}

output "found_pet_topic" {
  value = google_pubsub_topic.found_pet.name
}

output "match_found_topic" {
  value = google_pubsub_topic.match_found.name
}
