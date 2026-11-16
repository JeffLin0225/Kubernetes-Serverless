terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }
}

# T1 新增
provider "google" {
  project = var.project_id
  region  = var.region
}

# T2 新增
locals {
  # Artifact Registry 的 image 路徑前綴，例如 asia-east1-docker.pkg.dev/<project>/sep
  repo_url = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.sep.repository_id}"

  # T5 新增：Engine 的 image 白名單 = 自己的 repo + 額外允許的前綴
  allowed_image_prefixes = concat(["${local.repo_url}/"], var.extra_allowed_image_prefixes)
}
