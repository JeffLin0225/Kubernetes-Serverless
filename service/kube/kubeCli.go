package kube

import (
	"context"
	"fmt"
	"kubernetes-serverless/config"
	"kubernetes-serverless/model"
	"log"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeCli struct {
	client *kubernetes.Clientset
	cfg    *config.Config
}

func NewKubeCli(config *config.Config) (*KubeCli, error) {

	// 優先嘗試 In-Cluster config（跑在 K8s Pod 內時使用）
	// 若失敗（本機開發），自動 fallback 到 ~/.kube/config
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Println("[INFO] 非 In-Cluster 環境，改用 ~/.kube/config 連線（本機開發模式）")
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		k8sConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, &clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("無法建立 K8s 連線設定: %w", err)
		}
	}

	// 2. kubernetes.NewForConfig(k8sConfig)：
	// 根據第一步產生的連線設定 (k8sConfig)，正式建立一個 Clientset 實例。
	// Clientset 包含了所有與 K8s API Server 溝通的 REST 客戶端，
	// 後續所有建立 Pod、Job、Deployment 的 API 呼叫都是透過這個實例完成。
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}

	return &KubeCli{
		client: clientSet,
		cfg:    config,
	}, nil
}

func (k *KubeCli) CreateJob(ctx context.Context, req model.RunRequest) (*batchv1.Job, error) {
	// 1. 根據 SystemID 取得該系統核准的資源配額
	reqRes, limitRes, err := k.resolveSystemResources(ctx, req.SystemID)
	if err != nil {
		log.Printf("Quota Error 系統 '%s' 資源驗證失敗: '%v'", req.SystemID, err)
		return nil, fmt.Errorf("資源配額不合規: %w", err)
	}

	jobSpecs := k.buildJobSpec(req, reqRes, limitRes)

	createdJob, err := k.client.BatchV1().Jobs(k.cfg.Namespace).Create(ctx, jobSpecs, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("建立 Kubernetes Job 失敗: %w", err)
	}

	log.Printf("[Success] 已建立系統 '%s' 的 Job: '%s'", req.SystemID, createdJob.Name)
	return createdJob, nil
}

/**
 * （檢查/取） ConfigMap 資源配置
 */
func (k *KubeCli) resolveSystemResources(ctx context.Context, systemID string) (corev1.ResourceList, corev1.ResourceList, error) {

	cmName := k.cfg.SystemQuotasCM

	// 取 ConfigMap
	cm, err := k.client.CoreV1().ConfigMaps(k.cfg.Namespace).Get(ctx, cmName, metav1.GetOptions{})
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

func (k *KubeCli) buildJobSpec(req model.RunRequest, reqRes, limitRes corev1.ResourceList) *batchv1.Job {
	// 1. 通用 Job 命名：格式為 job-<system_id>-<timestamp>
	//（Printf 是直接印在終端機螢幕上，不會回傳字串）
	jobName := fmt.Sprintf("job-%s-%d", req.SystemID, time.Now().UnixNano()/1e6)

	backoffLimit := int32(0) // 批次任務不盲目重試
	ttl := int32(300)        // 執行完畢 5 分鐘後自動被 kubernetes 回收
	// activeDeadline  := int64(120) // Job 最多存活 120 秒，超時強制 Kill（防止 ImagePullBackOff 等卡死情況）

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: k.cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "serverless-engine",
				"system_id":                    req.SystemID,
				"task_id":                      req.TaskID,
			},
		},
		Spec: batchv1.JobSpec{
			// ActiveDeadlineSeconds:   &activeDeadline,
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"system_id": req.SystemID,
						"task_id":   req.TaskID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "task-runner",
							Image:           req.Image,
							ImagePullPolicy: corev1.PullPolicy(k.cfg.ImagePullPolicy),
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
