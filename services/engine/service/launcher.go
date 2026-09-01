package service

import (
	"context"
	"fmt"
	"time"
	"log"

	"sep/common/config"
	"sep/common/model"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type JobLauncher struct {
	client *kubernetes.Clientset
	cfg    *config.EngineConfig
}

func NewJobLauncher(cfg *config.EngineConfig, client *kubernetes.Clientset) *JobLauncher {
	return &JobLauncher{
		client: client,
		cfg:    cfg,
	}
}

/**
 * 建立JOB
 */
func (l *JobLauncher) CreateJob(ctx context.Context, req model.RunRequest) (*batchv1.Job, error) {
	// 1. 根據 SystemID 取得該系統核准的資源配額
	reqRes, limitRes, err := l.resolveSystemResources(ctx, req.SystemID)
	if err != nil {
		log.Printf("[WARN] Quota 驗證失敗: %v", err)
		return nil, err
	}

	// 決定目標 Namespace：若未指定則預設使用 cfg 中的 Namespace
	targetNs := req.Namespace
	if targetNs == "" {
		targetNs = l.cfg.Namespace
	}

	jobSpecs := l.buildJobSpec(req, targetNs, reqRes, limitRes)

	createdJob, err := l.client.BatchV1().Jobs(targetNs).Create(ctx, jobSpecs, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("建立 Kubernetes Job 失敗: %w", err)
	}

	log.Printf("[Success] 已建立系統 '%s' 的 Job: '%s' (Namespace: '%s')", req.SystemID, createdJob.Name, targetNs)
	return createdJob, nil
}

/**
 * （檢查/取） ConfigMap 資源配置
 */
func (l *JobLauncher) resolveSystemResources(ctx context.Context, systemID string) (corev1.ResourceList, corev1.ResourceList, error) {
	cmName := l.cfg.SystemQuotasCM

	// 取 ConfigMap
	cm, err := l.client.CoreV1().ConfigMaps(l.cfg.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("無法取得 ConfigMap '%s': %w", cmName, err)
	}

	// 定義該系統在 ConfigMap 裡必須存在的 4 個 key
	kCPUReq := fmt.Sprintf("%s.cpu_request", systemID)
	kMemReq := fmt.Sprintf("%s.memory_request", systemID)
	kCPULim := fmt.Sprintf("%s.cpu_limit", systemID)
	kMemLim := fmt.Sprintf("%s.memory_limit", systemID)

	// 檢查是否註冊過這個系統
	if cm.Data[kCPUReq] == "" || cm.Data[kMemReq] == "" || cm.Data[kCPULim] == "" || cm.Data[kMemLim] == "" {
		return nil, nil, fmt.Errorf("系統 '%s' 未在配額表註冊或配置不完整", systemID)
	}

	cpuReq, err := resource.ParseQuantity(cm.Data[kCPUReq])
	if err != nil {
		return nil, nil, fmt.Errorf("%s 格式錯誤: %w", kCPUReq, err)
	}
	memReq, err := resource.ParseQuantity(cm.Data[kMemReq])
	if err != nil {
		return nil, nil, fmt.Errorf("%s 格式錯誤: %w", kMemReq, err)
	}
	cpuLimit, err := resource.ParseQuantity(cm.Data[kCPULim])
	if err != nil {
		return nil, nil, fmt.Errorf("%s 格式錯誤: %w", kCPULim, err)
	}
	memLimit, err := resource.ParseQuantity(cm.Data[kMemLim])
	if err != nil {
		return nil, nil, fmt.Errorf("%s 格式錯誤: %w", kMemLim, err)
	}

	return corev1.ResourceList{
			corev1.ResourceCPU:    cpuReq,
			corev1.ResourceMemory: memReq,
		}, corev1.ResourceList{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memLimit,
		}, nil
}

/**
組建 JOB 參數
 */
func (l *JobLauncher) buildJobSpec(req model.RunRequest, targetNs string, reqRes, limitRes corev1.ResourceList) *batchv1.Job {
	// 通用 Job 命名：格式為 job-<system_id>-<timestamp>
	jobName := fmt.Sprintf("job-%s-%d", req.SystemID, time.Now().UnixMilli())

	backoffLimit := int32(0) // 批次任務不盲目重試
	ttl := int32(300)        // 執行完畢 5 分鐘後自動被 kubernetes 回收

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: targetNs,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sep-engine",
				"system_id":                    req.SystemID,
				"task_id":                      req.TaskID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "sep-engine",
						"system_id":                    req.SystemID,
						"task_id":                      req.TaskID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "task-runner",
							Image:           req.Image,
							Command:         req.Command,
							ImagePullPolicy: corev1.PullPolicy(l.cfg.ImagePullPolicy),
							Resources: corev1.ResourceRequirements{
								Requests: reqRes,
								Limits:   limitRes,
							},
						},
					},
				},
			},
		},
	}
}
