variable "project_id" {
  description = "GCP 專案 ID"
  type        = string
}

variable "region" {
  description = "Cloud Run 和 Artifact Registry 所在的 region"
  type        = string
  default     = "asia-east1"
}

# ---- 以下從 T4 之後才陸續加入，T1 只需要上面兩個 ----

variable "system_quotas" {
  description = "每個來源系統核准的資源上限，取代 K8s 版的 ConfigMap sep-system-quotas"
  type = map(object({
    cpu    = string
    memory = string
  }))
  default = {
    demo-go = {
      cpu    = "1"
      memory = "512Mi"
    }
  }
}

variable "extra_allowed_image_prefixes" {
  description = "除了本專案的 Artifact Registry 之外，額外允許的任務 image 前綴"
  type        = list(string)
  default     = []
}

variable "job_retention" {
  description = "任務 job 跑完後保留多久才由 Cleaner 刪除（Go duration 格式）"
  type        = string
  default     = "5m"
}

variable "cleaner_schedule" {
  description = "Cleaner 的觸發排程（cron 格式）"
  type        = string
  default     = "0 * * * *"
}
