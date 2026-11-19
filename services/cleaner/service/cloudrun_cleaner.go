package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/iterator"

	"sep/common/config"
	"sep/common/model"
)

// CloudRunCleaner 取代 K8s 版的 ttlSecondsAfterFinished：
// Cloud Run Job 跑完不會自己消失，要有人把它刪掉。
type CloudRunCleaner struct {
	client    *run.JobsClient
	parent    string
	retention time.Duration
}

func NewCloudRunCleaner(ctx context.Context, cfg *config.CleanerConfig) (*CloudRunCleaner, error) {
	if cfg.GCPProject == "" || cfg.GCPRegion == "" {
		return nil, fmt.Errorf("Cloud Run 模式需要設定 GCP_PROJECT、GCP_REGION")
	}

	client, err := run.NewJobsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("建立 Cloud Run Jobs client 失敗: %w", err)
	}

	return &CloudRunCleaner{
		client:    client,
		parent:    fmt.Sprintf("projects/%s/locations/%s", cfg.GCPProject, cfg.GCPRegion),
		retention: cfg.JobRetention,
	}, nil
}

// RunOnce 掃描一次就結束。常駐輪詢改由 Cloud Scheduler 定時觸發這個 job。
func (c *CloudRunCleaner) RunOnce(ctx context.Context) error {
	log.Printf("🚀 [Cleaner] Cloud Run 模式啟動，保留時間: %v，範圍: %s", c.retention, c.parent)

	scanned, deleted := 0, 0

	// ListJobs 不支援用 label 篩選，只能全部列出來再自己過濾
	it := c.client.ListJobs(ctx, &runpb.ListJobsRequest{Parent: c.parent})
	for {
		job, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("列出 Cloud Run Job 失敗: %w", err)
		}

		// 只處理 Engine 建立的任務 job。Cleaner 自己（sep-cleaner）沒有這個 label，不會被刪
		if job.GetLabels()[model.LabelManagedBy] != model.ManagedByEngine {
			continue
		}
		scanned++

		reason, expired := c.expiredReason(job)
		if !expired {
			continue
		}

		if _, err := c.client.DeleteJob(ctx, &runpb.DeleteJobRequest{Name: job.GetName()}); err != nil {
			log.Printf("[ERROR] 刪除 Job '%s' 失敗: %v", job.GetName(), err)
			continue
		}
		deleted++
		log.Printf("🧹 [CLEAN] 已刪除 Job '%s'（%s，SystemID: %s，TaskID: %s）",
			job.GetName(), reason, job.GetLabels()[model.LabelSystemID], job.GetLabels()[model.LabelTaskID])
	}

	log.Printf("🔍 [Cleaner] 巡檢完成：共 %d 個任務 Job，刪除 %d 個", scanned, deleted)
	return nil
}

// expiredReason 判斷 job 是否該刪除，並回傳原因
func (c *CloudRunCleaner) expiredReason(job *runpb.Job) (string, bool) {
	exec := job.GetLatestCreatedExecution()

	// 從沒執行過：Engine 建了 job 但 RunJob 失敗留下的孤兒
	if exec.GetName() == "" {
		if time.Since(job.GetCreateTime().AsTime()) > c.retention {
			return "從未執行的孤兒 job", true
		}
		return "", false
	}

	// 還在跑
	if exec.GetCompletionTime() == nil {
		return "", false
	}

	// 已結束（不論成功或失敗），而且超過保留時間
	if time.Since(exec.GetCompletionTime().AsTime()) > c.retention {
		return fmt.Sprintf("執行結束狀態: %s", exec.GetCompletionStatus()), true
	}
	return "", false
}
