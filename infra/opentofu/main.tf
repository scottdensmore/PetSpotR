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

module "storage" {
  source          = "./modules/storage"
  project_id      = var.project_id
  region          = var.region
  allowed_origins = var.image_cors_allowed_origins
  depends_on      = [google_project_service.storage]
}

module "firestore" {
  source = "./modules/firestore"
  region = var.region
}

module "cloudrun" {
  source                     = "./modules/cloudrun"
  project_id                 = var.project_id
  region                     = var.region
  web_frontend_image         = var.web_frontend_image
  lostpet_image              = var.lostpet_image
  foundpet_image             = var.foundpet_image
  pet_matcher_image          = var.pet_matcher_image
  notification_service_image = var.notification_service_image
  image_bucket_name          = module.storage.bucket_name
  depends_on                 = [google_project_service.iam_credentials]
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
