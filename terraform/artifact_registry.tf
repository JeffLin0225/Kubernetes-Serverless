resource "google_artifact_registry_repository" "sep" {
  location      = var.region
  repository_id = "sep"
  format        = "DOCKER"
  description   = "SEP engine / cleaner images"

  # 免費額度只有 0.5 GB：刪除超過 1 天的舊 image，但每個 image 名稱保留最新 3 個版本
  # KEEP 的優先權高於 DELETE，所以最新 3 個不會被刪
  cleanup_policies {
    id     = "delete-old"
    action = "DELETE"
    condition {
      tag_state  = "ANY"
      older_than = "86400s"
    }
  }

  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"
    most_recent_versions {
      keep_count = 3
    }
  }
}
