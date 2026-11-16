package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"

	"sep/common/config"
	"sep/common/model"
)

// systemIDPattern：system_id 會成為 Cloud Run job 名稱的一部分，只能是小寫英數和 "-"
var systemIDPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// labelInvalidChars：GCP label 的值只能是小寫英數、"_"、"-"
var labelInvalidChars = regexp.MustCompile(`[^a-z0-9_-]`)

// cloudRunQuota 對應環境變數 SYSTEM_QUOTAS_JSON 裡每個系統的設定。
// Cloud Run 只有 limit、沒有 request，所以比 K8s 版少兩個欄位。
type cloudRunQuota struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type CloudRunLauncher struct {
	client *run.JobsClient
	cfg    *config.EngineConfig
	quotas map[string]cloudRunQuota
}

// NewCloudRunLauncher 啟動時載入一次配額表（K8s 版是每次請求都讀 ConfigMap）
func NewCloudRunLauncher(ctx context.Context, cfg *config.EngineConfig) (*CloudRunLauncher, error) {
	if cfg.GCPProject == "" || cfg.GCPRegion == "" || cfg.TaskServiceAccount == "" {
		return nil, fmt.Errorf("Cloud Run 模式需要設定 GCP_PROJECT、GCP_REGION、TASK_SERVICE_ACCOUNT")
	}

	quotas := map[string]cloudRunQuota{}
	if err := json.Unmarshal([]byte(cfg.SystemQuotasJSON), &quotas); err != nil {
		return nil, fmt.Errorf("SYSTEM_QUOTAS_JSON 格式錯誤: %w", err)
	}
	if len(cfg.AllowedImagePrefixes) == 0 {
		return nil, fmt.Errorf("ALLOWED_IMAGE_PREFIXES 不可為空，否則任何 image 都無法執行")
	}

	// 憑證來自執行環境：Cloud Run 上是 sep-engine 這個 Service Account，本機是 ADC
	client, err := run.NewJobsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("建立 Cloud Run Jobs client 失敗: %w", err)
	}

	log.Printf("[INFO] Cloud Run launcher 已載入 %d 個系統配額，image 白名單: %v", len(quotas), cfg.AllowedImagePrefixes)
	return &CloudRunLauncher{client: client, cfg: cfg, quotas: quotas}, nil
}

func (l *CloudRunLauncher) Launch(ctx context.Context, req model.RunRequest) (string, error) {
	// 1. 驗證請求
	if !systemIDPattern.MatchString(req.SystemID) {
		return "", fmt.Errorf("%w: system_id 只能是小寫英數和 '-'", ErrRejected)
	}
	quota, ok := l.quotas[req.SystemID]
	if !ok || quota.CPU == "" || quota.Memory == "" {
		return "", fmt.Errorf("%w: 系統 '%s' 未在配額表註冊或配置不完整", ErrRejected, req.SystemID)
	}
	if !l.imageAllowed(req.Image) {
		return "", fmt.Errorf("%w: image '%s' 不在白名單內", ErrRejected, req.Image)
	}

	// 2. 建立 job（只是定義，還不會執行）
	parent := fmt.Sprintf("projects/%s/locations/%s", l.cfg.GCPProject, l.cfg.GCPRegion)
	jobID := fmt.Sprintf("job-%s-%d", req.SystemID, time.Now().UnixMilli())

	// CreateJob 是長時間操作，設定上限避免卡住整個 HTTP 請求
	createCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	createOp, err := l.client.CreateJob(createCtx, &runpb.CreateJobRequest{
		Parent: parent,
		JobId:  jobID,
		Job:    l.buildJob(req, quota),
	})
	if err != nil {
		return "", fmt.Errorf("建立 Cloud Run Job 失敗: %w", err)
	}
	job, err := createOp.Wait(createCtx)
	if err != nil {
		return "", fmt.Errorf("等待 Cloud Run Job 建立完成失敗: %w", err)
	}

	// 3. 執行 job。只送出執行請求，不等任務跑完
	if _, err := l.client.RunJob(ctx, &runpb.RunJobRequest{Name: job.GetName()}); err != nil {
		// job 已建立但沒跑起來，留給 Cleaner 當孤兒回收
		return "", fmt.Errorf("執行 Cloud Run Job '%s' 失敗: %w", jobID, err)
	}

	log.Printf("[Success] 已建立並執行系統 '%s' 的 Cloud Run Job: '%s'", req.SystemID, jobID)
	return jobID, nil
}

func (l *CloudRunLauncher) imageAllowed(image string) bool {
	for _, prefix := range l.cfg.AllowedImagePrefixes {
		if strings.HasPrefix(image, prefix) {
			return true
		}
	}
	return false
}

func (l *CloudRunLauncher) buildJob(req model.RunRequest, quota cloudRunQuota) *runpb.Job {
	return &runpb.Job{
		Labels: map[string]string{
			model.LabelManagedBy: model.ManagedByEngine,
			model.LabelSystemID:  sanitizeLabel(req.SystemID),
			model.LabelTaskID:    sanitizeLabel(req.TaskID),
		},
		Template: &runpb.ExecutionTemplate{
			Template: &runpb.TaskTemplate{
				// 對應 K8s 版的 backoffLimit: 0，批次任務不盲目重試
				Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 0},
				// 任務容器用沒有任何權限的身分跑
				ServiceAccount: l.cfg.TaskServiceAccount,
				Containers: []*runpb.Container{
					{
						Image:   req.Image,
						Command: req.Command,
						Resources: &runpb.ResourceRequirements{
							Limits: map[string]string{
								"cpu":    quota.CPU,
								"memory": quota.Memory,
							},
						},
					},
				},
			},
		},
	}
}

// sanitizeLabel 把字串轉成合法的 GCP label 值：小寫、非法字元換成 "-"、最長 63 字元
func sanitizeLabel(s string) string {
	s = labelInvalidChars.ReplaceAllString(strings.ToLower(s), "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}
