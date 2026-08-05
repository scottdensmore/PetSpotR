variable "project_id" {
  description = "Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "GCP Deployment Region"
  type        = string
  default     = "us-central1"
}

variable "lostpet_image" {
  description = "Container image for lostpet-service"
  type        = string
  default     = "gcr.io/petspotr/lostpet-service:latest"
}

variable "foundpet_image" {
  description = "Container image for foundpet-service"
  type        = string
  default     = "gcr.io/petspotr/foundpet-service:latest"
}
