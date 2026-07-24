# Identity for the in-cluster OpenTelemetry Collector so it can export traces to
# Cloud Trace and logs to Cloud Logging via Workload Identity — no SA keys, same
# keyless pattern as the storage SA. Bound to the `medmarket-collector` KSA in
# the staging namespace.
resource "google_service_account" "collector" {
  account_id   = "medmarket-collector"
  display_name = "otel collector -> cloud trace/logging"
}

# Project-level roles: write spans to Cloud Trace, write log entries to Cloud
# Logging. Granted to the GSA (not on the SA resource itself — these are project
# permissions, unlike the tokenCreator self-binding on the storage SA).
resource "google_project_iam_member" "collector_trace" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.collector.email}"
}

resource "google_project_iam_member" "collector_logging" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.collector.email}"
}

# Let the collector KSA impersonate this GSA (the Workload Identity link).
resource "google_service_account_iam_member" "collector_workload_identity" {
  service_account_id = google_service_account.collector.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:project-8628faf2-7f2e-46d0-a01.svc.id.goog[staging/medmarket-collector]"
}
