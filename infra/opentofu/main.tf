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
  source         = "./modules/cloudrun"
  region         = var.region
  lostpet_image  = var.lostpet_image
  foundpet_image = var.foundpet_image
}
