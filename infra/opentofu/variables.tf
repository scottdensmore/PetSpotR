variable "project_id" {
  description = "Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "GCP Deployment Region"
  type        = string
  default     = "us-central1"
}

variable "image_cors_allowed_origins" {
  description = "Exact browser origins allowed to upload and read signed pet images"
  type        = list(string)
  default     = []
}

variable "web_frontend_image" {
  description = "Container image for web-frontend"
  type        = string
  default     = "gcr.io/petspotr/web-frontend:latest"
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

variable "pet_matcher_image" {
  description = "Container image for pet-matcher"
  type        = string
  default     = "gcr.io/petspotr/pet-matcher:latest"
}

variable "notification_service_image" {
  description = "Container image for notification-service"
  type        = string
  default     = "gcr.io/petspotr/notification-service:latest"
}
