package model

type RunRequest struct {

	// 加上 binding:"required"，Gin 會在欄位為空時自動報錯
	SystemID string            `json:"system_id" binding:"required"` // 系統 ID (例如: crawler, analytics, etl)
	TaskID   string            `json:"task_id" binding:"required"`   // 該任務執行的唯一識別碼 (對 Prefect 來說就是 flow_run_id)
	Image    string            `json:"image" binding:"required"`     // 要運行的映像檔
	Env      map[string]string `json:"env" binding:"required"`       // 選填：額外注入的環境變數
	Command  []string          `json:"command,omitempty"`            // 選填：執行指令 (例如: ["python", "main.py"])
}

type RunResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JobName string `json:"job_name,omitempty"`
}
