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

module "storage" {
  source     = "./modules/storage"
  project_id = var.project_id
  region     = var.region
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
}

module "pubsub" {
  source                              = "./modules/pubsub"
  project_id                          = var.project_id
  region                              = var.region
  pet_matcher_name                    = module.cloudrun.pet_matcher_name
  pet_matcher_url                     = module.cloudrun.pet_matcher_url
  notification_service_name           = module.cloudrun.notification_service_name
  notification_service_url            = module.cloudrun.notification_service_url
  foundpet_runtime_service_account    = module.cloudrun.foundpet_runtime_service_account
  pet_matcher_runtime_service_account = module.cloudrun.pet_matcher_runtime_service_account
}
