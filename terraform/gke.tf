resource "google_container_cluster" "medmarket" {
  name                     = "medmarket"
  location                 = "us-central1-a"
  remove_default_node_pool = true
  initial_node_count       = 1
  workload_identity_config {
    workload_pool = "project-8628faf2-7f2e-46d0-a01.svc.id.goog"
  }
  ip_allocation_policy {}
  network_policy {
    enabled = true
  }
  # HTTP load balancing (the L7 Ingress controller) was enabled out-of-band via
  # gcloud; captured here for reproducibility. Every currently-enabled addon is
  # declared explicitly because terraform resets unlisted addons to their
  # defaults — omitting gce_persistent_disk_csi_driver would disable it and break
  # the StatefulSet PVCs. Run `terraform plan` and confirm nothing here is being
  # DISABLED before applying.
  addons_config {
    http_load_balancing {
      disabled = false
    }
    horizontal_pod_autoscaling {
      disabled = false
    }
    gce_persistent_disk_csi_driver_config {
      enabled = true
    }
    dns_cache_config {
      enabled = true
    }
    network_policy_config {
      disabled = false
    }
  }
  release_channel {
    channel = "REGULAR"
  }
  deletion_protection = false
}

resource "google_container_node_pool" "medmarket" {
  name     = "medmarket-pool"
  cluster  = google_container_cluster.medmarket.name
  location = google_container_cluster.medmarket.location
  autoscaling {
    min_node_count = 0
    max_node_count = 2
  }
  node_config {
    # e2-standard-2 = 2 dedicated vCPU (~1930m allocatable). e2-medium is
    # shared-core (only 940m allocatable), nearly all consumed by GKE system
    # DaemonSets, leaving no room for the app.
    machine_type = "e2-standard-2"
    disk_size_gb = 30
    disk_type    = "pd-standard"
    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }
}

# The pool runs as the default Compute Engine SA; grant it the roles nodes need
# to pull from Artifact Registry and report telemetry.
data "google_compute_default_service_account" "default" {}

resource "google_project_iam_member" "node_default_sa" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/container.defaultNodeServiceAccount"
  member  = "serviceAccount:${data.google_compute_default_service_account.default.email}"
}

resource "google_project_iam_member" "node_artifact_reader" {
  project = "project-8628faf2-7f2e-46d0-a01"
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${data.google_compute_default_service_account.default.email}"
}