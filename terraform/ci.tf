# Keyless CI auth: GitHub Actions presents its OIDC token, exchanges it for a
# short-lived GCP token that impersonates the medmarket-ci SA. No SA keys — the
# org policy forbids them, and this is the recommended pattern anyway.

# The pool is the trust boundary for external (non-GCP) identities.
resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Actions"
}

# The provider teaches the pool to trust GitHub's OIDC issuer. The attribute
# condition is the security gate: only tokens from THIS repo are accepted, so a
# fork or another repo can never mint a token for our SA.
resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub OIDC"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = "assertion.repository == 'LoneWolfPR/MedMarket'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# The identity CI acts as once the token is exchanged.
resource "google_service_account" "ci" {
  account_id   = "medmarket-ci"
  display_name = "GitHub Actions deployer"
}

# Push images to Artifact Registry.
resource "google_project_iam_member" "ci_artifact_writer" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.ci.email}"
}

# Read cluster credentials + apply Kubernetes objects.
resource "google_project_iam_member" "ci_gke_developer" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/container.developer"
  member  = "serviceAccount:${google_service_account.ci.email}"
}

# Let identities from this repo (via the pool) impersonate the CI SA. principalSet
# scopes it to the repository attribute — the same repo named in the condition.
resource "google_service_account_iam_member" "ci_workload_identity" {
  service_account_id = google_service_account.ci.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/LoneWolfPR/MedMarket"
}

# The workflow needs these two values (google-github-actions/auth inputs).
output "ci_workload_identity_provider" {
  value = google_iam_workload_identity_pool_provider.github.name
}

output "ci_service_account_email" {
  value = google_service_account.ci.email
}
