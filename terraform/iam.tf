# 執行期用的 4 個身分。builder 在 build.tf。
# 這個檔不依賴任何 image，可以在第一次 build 之前就 apply。

resource "google_service_account" "engine" {
  account_id   = "sep-engine"
  display_name = "SEP Engine runtime"
}

resource "google_service_account" "cleaner" {
  account_id   = "sep-cleaner"
  display_name = "SEP Cleaner runtime"
}

# 任務容器用的身分：故意不給任何權限
resource "google_service_account" "task" {
  account_id   = "sep-task"
  display_name = "SEP task container (no permissions)"
}

resource "google_service_account" "scheduler" {
  account_id   = "sep-scheduler"
  display_name = "SEP Cloud Scheduler trigger"
}

# Engine 要能建立和執行 Cloud Run job
resource "google_project_iam_member" "engine_run_developer" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = google_service_account.engine.member
}

# Engine 建 job 時會指定「這個 job 用 sep-task 身分跑」，需要有權限借用該身分
resource "google_service_account_iam_member" "engine_act_as_task" {
  service_account_id = google_service_account.task.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.engine.member
}

# Engine 建 job 時，GCP 會檢查 Engine 本身能不能讀該 image，所以要給系統 repo 的讀取權限
resource "google_artifact_registry_repository_iam_member" "engine_reader_demo_go" {
  location   = google_artifact_registry_repository.demo_go.location
  repository = google_artifact_registry_repository.demo_go.name
  role       = "roles/artifactregistry.reader"
  member     = google_service_account.engine.member
}

# Cleaner 要能列出和刪除 Cloud Run job
resource "google_project_iam_member" "cleaner_run_developer" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = google_service_account.cleaner.member
}

# sep-scheduler 的權限（只能觸發 cleaner 這一個 job）寫在 cleaner.tf，因為要先有那個 job
