terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_project_service" "storage" {
  project            = var.project_id
  service            = "storage.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam_credentials" {
  project            = var.project_id
  service            = "iamcredentials.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "firestore" {
  project            = var.project_id
  service            = "firestore.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "pubsub" {
  project            = var.project_id
  service            = "pubsub.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudrun" {
  project            = var.project_id
  service            = "run.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "secretmanager" {
  project            = var.project_id
  service            = "secretmanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudtrace" {
  project            = var.project_id
  service            = "cloudtrace.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "artifactregistry" {
  project            = var.project_id
  service            = "artifactregistry.googleapis.com"
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "petspotr" {
  project       = var.project_id
  location      = var.region
  repository_id = "petspotr"
  description   = "PetSpotR regional container image repository"
  format        = "DOCKER"

  depends_on = [google_project_service.artifactregistry]
}

module "storage" {
  source          = "./modules/storage"
  project_id      = var.project_id
  region          = var.region
  allowed_origins = var.image_cors_allowed_origins
  depends_on      = [google_project_service.storage]
}

module "firestore" {
  source              = "./modules/firestore"
  project_id          = var.project_id
  region              = var.region
  deletion_protection = var.firestore_deletion_protection
  depends_on          = [google_project_service.firestore]
}

module "cloudrun" {
  source                             = "./modules/cloudrun"
  project_id                         = var.project_id
  region                             = var.region
  web_frontend_image                 = var.web_frontend_image
  lostpet_image                      = var.lostpet_image
  foundpet_image                     = var.foundpet_image
  pet_matcher_image                  = var.pet_matcher_image
  notification_service_image         = var.notification_service_image
  image_bucket_name                  = module.storage.bucket_name
  web_frontend_max_instances         = coalesce(var.web_frontend_max_instances, var.cloudrun_max_instances)
  web_frontend_concurrency           = coalesce(var.web_frontend_concurrency, var.cloudrun_concurrency)
  lostpet_max_instances              = coalesce(var.lostpet_max_instances, var.cloudrun_max_instances)
  lostpet_concurrency                = coalesce(var.lostpet_concurrency, var.cloudrun_concurrency)
  foundpet_max_instances             = coalesce(var.foundpet_max_instances, var.cloudrun_max_instances)
  foundpet_concurrency               = coalesce(var.foundpet_concurrency, var.cloudrun_concurrency)
  pet_matcher_max_instances          = coalesce(var.pet_matcher_max_instances, var.cloudrun_max_instances)
  pet_matcher_concurrency            = coalesce(var.pet_matcher_concurrency, 10)
  notification_service_max_instances = coalesce(var.notification_service_max_instances, var.cloudrun_max_instances)
  notification_service_concurrency   = coalesce(var.notification_service_concurrency, var.cloudrun_concurrency)
  depends_on                         = [google_project_service.iam_credentials]
}

module "pubsub" {
  source                               = "./modules/pubsub"
  project_id                           = var.project_id
  region                               = var.region
  pet_matcher_name                     = module.cloudrun.pet_matcher_name
  pet_matcher_url                      = module.cloudrun.pet_matcher_url
  notification_service_name            = module.cloudrun.notification_service_name
  notification_service_url             = module.cloudrun.notification_service_url
  lostpet_runtime_service_account      = module.cloudrun.lostpet_runtime_service_account
  web_frontend_runtime_service_account = module.cloudrun.web_frontend_runtime_service_account
  foundpet_runtime_service_account     = module.cloudrun.foundpet_runtime_service_account
  pet_matcher_runtime_service_account  = module.cloudrun.pet_matcher_runtime_service_account
}

resource "google_storage_bucket_iam_member" "foundpet_objects" {
  bucket = module.storage.bucket_name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${module.cloudrun.foundpet_runtime_service_account}"
}

resource "google_storage_bucket_iam_member" "lostpet_objects" {
  bucket = module.storage.bucket_name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${module.cloudrun.lostpet_runtime_service_account}"
}

resource "google_storage_bucket_iam_member" "pet_matcher_reader" {
  bucket = module.storage.bucket_name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${module.cloudrun.pet_matcher_runtime_service_account}"
}
