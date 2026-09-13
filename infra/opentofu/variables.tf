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

variable "cloudrun_max_instances" {
  description = "Default maximum instance count for Cloud Run services"
  type        = number
  default     = 10
}

variable "cloudrun_concurrency" {
  description = "Default maximum concurrent requests per Cloud Run instance"
  type        = number
  default     = 80
}

variable "web_frontend_max_instances" {
  description = "Maximum instance count for web-frontend (defaults to cloudrun_max_instances when null)"
  type        = number
  default     = null
}

variable "web_frontend_concurrency" {
  description = "Maximum concurrent requests per instance for web-frontend (defaults to cloudrun_concurrency when null)"
  type        = number
  default     = null
}

variable "lostpet_max_instances" {
  description = "Maximum instance count for lostpet-service (defaults to cloudrun_max_instances when null)"
  type        = number
  default     = null
}

variable "lostpet_concurrency" {
  description = "Maximum concurrent requests per instance for lostpet-service (defaults to cloudrun_concurrency when null)"
  type        = number
  default     = null
}

variable "foundpet_max_instances" {
  description = "Maximum instance count for foundpet-service (defaults to cloudrun_max_instances when null)"
  type        = number
  default     = null
}

variable "foundpet_concurrency" {
  description = "Maximum concurrent requests per instance for foundpet-service (defaults to cloudrun_concurrency when null)"
  type        = number
  default     = null
}

variable "pet_matcher_max_instances" {
  description = "Maximum instance count for pet-matcher (defaults to cloudrun_max_instances when null)"
  type        = number
  default     = null
}

variable "pet_matcher_concurrency" {
  description = "Maximum concurrent requests per instance for pet-matcher (defaults to 10 when null for inference protection)"
  type        = number
  default     = null
}

variable "notification_service_max_instances" {
  description = "Maximum instance count for notification-service (defaults to cloudrun_max_instances when null)"
  type        = number
  default     = null
}

variable "notification_service_concurrency" {
  description = "Maximum concurrent requests per instance for notification-service (defaults to cloudrun_concurrency when null)"
  type        = number
  default     = null
}

variable "firestore_deletion_protection" {
  description = "Enable deletion protection on Firestore database in production"
  type        = bool
  default     = true
}

variable "pet_inference_image" {
  description = "Container image for pet-inference"
  type        = string
  default     = "ollama/ollama:0.32.5"
}

variable "pet_inference_concurrency" {
  description = "Maximum concurrent requests per instance for pet-inference"
  type        = number
  default     = 4
}

variable "pet_inference_gpu_type" {
  description = "GPU accelerator type for pet-inference"
  type        = string
  default     = "nvidia-l4"
}

variable "pet_inference_gpu_count" {
  description = "Number of GPUs allocated per pet-inference instance"
  type        = number
  default     = 1
}

variable "pet_inference_cpu" {
  description = "CPU limit for pet-inference container"
  type        = string
  default     = "4"
}

variable "pet_inference_memory" {
  description = "Memory limit for pet-inference container"
  type        = string
  default     = "16Gi"
}

variable "pet_inference_min_instances" {
  description = "Minimum instance count for pet-inference"
  type        = number
  default     = null
}

variable "pet_inference_max_instances" {
  description = "Maximum instance count for pet-inference"
  type        = number
  default     = null
}

variable "ollama_model" {
  description = "Default Ollama model identifier for multimodal pet matching"
  type        = string
  default     = "gemma4:e2b"
}
