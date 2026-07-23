output "prescriptions_bucket" {
  value = google_storage_bucket.medmarket.name
}

output "artifact_registry_repo" {
  value = google_artifact_registry_repository.medmarket.repository_id
}

output "storage_service_account_email" {
  value = google_service_account.medmarket.email
}
