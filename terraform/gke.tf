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
    machine_type = "e2-medium"
    disk_size_gb = 30
    disk_type    = "pd-standard"
    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }
}