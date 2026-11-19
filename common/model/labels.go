package model

// Cloud Run 版 Engine 和 Cleaner 共用的 label。
// GCP 的 label key 不允許 "/" 和 "."，所以不能沿用 K8s 版的 app.kubernetes.io/managed-by。
const (
	LabelManagedBy  = "managed-by"
	ManagedByEngine = "sep-engine"
	LabelSystemID   = "system_id"
	LabelTaskID     = "task_id"
)
