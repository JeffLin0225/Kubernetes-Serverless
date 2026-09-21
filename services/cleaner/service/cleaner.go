package service

import (
	"context"
	"log"
	"time"

	"sep/common/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CleanerService struct {
	client               *kubernetes.Clientset
	namespace            string
	interval             time.Duration
	normalWaitingReasons map[string]bool // 正常啟動過程中的 Waiting 狀態白名單，不在此清單中的一律視為異常（來自 .env 的 NORMAL_WAITING_REASONS）
}

func NewCleanerService(cfg *config.CleanerConfig, client *kubernetes.Clientset) *CleanerService {
	normalWaitingReasons := make(map[string]bool, len(cfg.NormalWaitingReasons))
	for _, reason := range cfg.NormalWaitingReasons {
		normalWaitingReasons[reason] = true
	}

	return &CleanerService{
		client:               client,
		namespace:            cfg.TargetNamespace,
		interval:             cfg.ScanInterval,
		normalWaitingReasons: normalWaitingReasons,
	}
}

// Start 啟動定時巡檢與自動收割
func (s *CleanerService) Start(ctx context.Context) {
	log.Printf("🚀 [Cleaner] 異常 Pod 收割微服務啟動，巡檢週期: %v, 目標 Namespace: '%s' (空字串代表全叢集)", s.interval, s.namespace)

	// 啟動時先立即執行一次掃描
	s.scanAndClean(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Cleaner] 收到關閉信號，停止監控")
			return
		case <-ticker.C:
			s.scanAndClean(ctx)
		}
	}
}

// scanAndClean 掃描並清理異常卡住的 Pod
func (s *CleanerService) scanAndClean(parentCtx context.Context) {
	// 每次巡檢設定獨立 Timeout，避免 API Server 異常時阻塞主巡檢 Loop
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	// 過濾由 Serverless Engine 發起的 Pods（支援 system_id 標籤或 managed-by 標籤）
	// （相當於 SQL 的 WHERE 條件 或 kubectl 指令的篩選參數）
	listOptions := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=sep-engine",
		// 走 apiserver 本地 watch cache 讀取，不直接打 etcd quorum read。
		// 巡檢本來就是每隔一段時間跑一次，晚個幾百毫秒的資料無感，換取大幅降低 etcd/apiserver 負擔。
		ResourceVersion: "0",
	}

	pods, err := s.client.CoreV1().Pods(s.namespace).List(ctx, listOptions)
	if err != nil {
		log.Printf("[ERROR] 查詢 Pod 清單失敗: %v", err)
		return
	}

	cleanedCount := 0
	for _, pod := range pods.Items { // range 會同時回傳 兩個值：第一個是索引（0, 1, 2...），第二個是元素實體（pod）
		// 檢查每個容器的狀態
		for _, cs := range pod.Status.ContainerStatuses { // ContainerStatuses 是一個陣列，代表這個 Pod 裡面所有容器的執行狀態。 (cs 是 ContainerStatus 的縮寫。)
			if cs.State.Waiting != nil && !s.normalWaitingReasons[cs.State.Waiting.Reason] { // 只要不是白名單內的正常等待狀態（如 ContainerCreating），一律視為異常。
				cleanedCount++
				reason := cs.State.Waiting.Reason
				systemID := pod.Labels["system_id"]
				taskID := pod.Labels["task_id"]
				jobName := pod.Labels["job-name"]
				if jobName == "" {
					jobName = pod.Labels["batch.kubernetes.io/job-name"]
				}
				if jobName == "" {
					for _, owner := range pod.OwnerReferences {
						if owner.Kind == "Job" {
							jobName = owner.Name
							break
						}
					}
				}

				log.Printf("🚨 [ALERT 告警] 發現異常死 Pod！\n  - Pod: %s (Namespace: %s)\n  - SystemID: %s, TaskID: %s\n  - 失敗原因: %s",
					pod.Name, pod.Namespace, systemID, taskID, reason)

				// 執行收割清理
				s.cleanupStuckPodAndJob(ctx, pod.Namespace, pod.Name, jobName)
			}
		}
	}

	if cleanedCount == 0 {
		if len(pods.Items) == 0 {
			log.Printf("🔍 [Cleaner] 巡檢完成：目前無任何任務 Pod，系統正常")
		} else {
			log.Printf("🔍 [Cleaner] 巡檢完成：共巡檢 %d 個任務 Pod，狀態皆正常，無需清除", len(pods.Items))
		}
	}
}

// cleanupStuckPodAndJob 刪除 Job 與 Pod 釋放配額
func (s *CleanerService) cleanupStuckPodAndJob(ctx context.Context, namespace, podName, jobName string) {
	// 1. 優先刪除 Job（K8s 會自動連帶 Cascade 刪除底下 Pod）
	if jobName != "" {
		propagation := metav1.DeletePropagationBackground
		err := s.client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		})
		if err == nil {
			log.Printf("🧹 [CLEAN] 已成功刪除 Job '%s' (Namespace: '%s')，已釋放 CPU/RAM 預留配額！", jobName, namespace)
			return
		}
		log.Printf("[WARN] 刪除 Job '%s' 失敗: %v，嘗試直接刪除 Pod", jobName, err)
	}

	// 2. 兜底：直接強制刪除 Pod
	err := s.client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("[ERROR] 刪除 Pod '%s' 失敗: %v", podName, err)
	} else {
		log.Printf("🧹 [CLEAN] 已直接刪除 Pod '%s'", podName)
	}
}
