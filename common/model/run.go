package model

type RunRequest struct {
	// SystemID 限制最大長度 40，確保 job-<system_id>-<timestamp> 不會超過 K8s 資源名稱 63 字元上限
	SystemID  string   `json:"system_id" binding:"required,max=40"`
	// TaskID 限制最大長度 63，符合 K8s Label value 上限
	TaskID    string   `json:"task_id" binding:"required,max=63"`
	Namespace string   `json:"namespace,omitempty"`
	Image     string   `json:"image" binding:"required"`
	Command   []string `json:"command,omitempty"`
}

type RunResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JobName string `json:"job_name,omitempty"`
}
