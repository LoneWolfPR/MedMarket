resource "google_artifact_registry_repository" "medmarket" {
  location      = "us-central1"
  repository_id = "medmarket"
  format        = "DOCKER"
  depends_on    = [google_project_service.artifact_registry]
}
