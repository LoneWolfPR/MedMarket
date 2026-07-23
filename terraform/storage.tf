resource "google_storage_bucket" "medmarket" {
  name                        = "project-8628faf2-7f2e-46d0-a01-prescriptions"
  location                    = "us-central1"
  uniform_bucket_level_access = true
  force_destroy               = true
}

resource "google_service_account" "medmarket" {
  account_id   = "medmarket-storage"
  display_name = "prescription file storage"
}

resource "google_storage_bucket_iam_member" "medmarket" {
  bucket = google_storage_bucket.medmarket.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.medmarket.email}"
}

resource "google_service_account_iam_member" "token_creator" {
  service_account_id = google_service_account.medmarket.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.medmarket.email}"
}

resource "google_service_account_iam_member" "workload_identity" {
  service_account_id = google_service_account.medmarket.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:project-8628faf2-7f2e-46d0-a01.svc.id.goog[staging/medmarket-backend]"
}