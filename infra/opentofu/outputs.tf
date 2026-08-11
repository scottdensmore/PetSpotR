output "web_frontend_url" {
  value       = module.cloudrun.web_frontend_url
  description = "Public URL of web-frontend Cloud Run service"
}

output "lostpet_service_url" {
  value       = module.cloudrun.lostpet_url
  description = "Public URL of lostpet-service Cloud Run service"
}

output "foundpet_service_url" {
  value       = module.cloudrun.foundpet_url
  description = "Public URL of foundpet-service Cloud Run service"
}

output "pet_matcher_url" {
  value       = module.cloudrun.pet_matcher_url
  description = "Authenticated internal URL of pet-matcher Cloud Run service"
}

output "notification_service_url" {
  value       = module.cloudrun.notification_service_url
  description = "Authenticated internal URL of notification-service Cloud Run service"
}
