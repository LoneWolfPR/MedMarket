terraform {
  required_version = ">= 1.15.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }

  backend "gcs" {
    bucket = "project-8628faf2-7f2e-46d0-a01-tfstate"
    prefix = "medmarket/state"
  }
}