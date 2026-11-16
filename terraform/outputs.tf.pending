output "engine_url" {
  description = "Engine 的網址（不公開，要透過 gcloud run services proxy 呼叫）"
  value       = google_cloud_run_v2_service.engine.uri
}

output "repo_url" {
  description = "Artifact Registry 的 image 路徑前綴"
  value       = local.repo_url
}

output "builder_service_account" {
  description = "gcloud builds submit --service-account 要用的完整資源名稱"
  value       = google_service_account.builder.id
}
