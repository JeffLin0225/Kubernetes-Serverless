terraform {
  # bucket 名稱不寫在這裡，init 時用 -backend-config="bucket=..." 傳入
  backend "gcs" {
    prefix = "sep"
  }
}
