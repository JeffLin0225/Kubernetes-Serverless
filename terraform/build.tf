# Cloud Build 用的身分。程式碼是 Cloud Build 自己從 GitHub clone 的（public repo，免憑證），
# 不需要上傳本機原始碼，所以不用原始碼 bucket。

resource "google_service_account" "builder" {
  account_id   = "sep-builder"
  display_name = "SEP Cloud Build"
}

# builder 只能 push 到 sep 這一個 repo
resource "google_artifact_registry_repository_iam_member" "builder_writer" {
  location   = google_artifact_registry_repository.sep.location
  repository = google_artifact_registry_repository.sep.name
  role       = "roles/artifactregistry.writer"
  member     = google_service_account.builder.member
}

# 寫 build log 到 Cloud Logging
resource "google_project_iam_member" "builder_logs" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = google_service_account.builder.member
}
