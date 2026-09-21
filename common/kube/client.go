package kube

import (
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewKubeClient 初始化 K8s Clientset（優先 In-Cluster，失敗自動 Fallback 至 ~/.kube/config）
func NewKubeClient() (*kubernetes.Clientset, error) {
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

	// 設定 Client 連線超時時間，避免 K8s API Server 連線異常時無限制掛住
	k8sConfig.Timeout = 10 * time.Second

	// client-go 預設 QPS=5 / Burst=10 是給「一堆 controller 共用同個叢集」情境的保守值。
	// 這支 clientset 只給 cleaner 這一支微服務用，拉高上限避免異常事故時大量刪除操作被自己的限速器卡住。
	k8sConfig.QPS = 20
	k8sConfig.Burst = 40

	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("建立 Kubernetes Clientset 失敗: %w", err)
	}

	return clientSet, nil
}
