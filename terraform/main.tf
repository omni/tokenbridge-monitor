terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "3.85.0"
    }
  }

  required_version = ">= 0.14.9"
}

provider "google" {
  credentials = file(var.gcp_credentials)
  project     = var.project
  region      = var.region
  zone        = var.zone
}
