output "storage_bucket" {
  value = module.storage.bucket_name
}

output "lostpet_service_url" {
  value = module.cloudrun.lostpet_url
}

output "foundpet_service_url" {
  value = module.cloudrun.foundpet_url
}
