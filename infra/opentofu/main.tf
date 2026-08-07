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

module "pubsub" {
  source = "./modules/pubsub"
}

module "firestore" {
  source = "./modules/firestore"
  region = var.region
}

module "cloudrun" {
  source                     = "./modules/cloudrun"
  region                     = var.region
  web_frontend_image         = var.web_frontend_image
  lostpet_image              = var.lostpet_image
  foundpet_image             = var.foundpet_image
  pet_matcher_image          = var.pet_matcher_image
  notification_service_image = var.notification_service_image
}
