package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"kubernetes-serverless/common/config"
	"kubernetes-serverless/common/kube"
	"kubernetes-serverless/services/cleaner/service"
)

func main() {
	log.Println("[INFO] 正在初始化 Pod Cleaner 監控微服務...")

	// 載入 Cleaner 微服務設定
	cfg := config.LoadCleanerConfig()

	// 初始化共用 K8s Clientset
	clientSet, err := kube.NewKubeClient()
	if err != nil {
		log.Fatalf("[FATAL] 初始化 K8s 連線失敗: %v", err)
	}

	cleanerService := service.NewCleanerService(cfg, clientSet)

	// 支援優雅停機 (Graceful Shutdown)
	// ctx 生命週期控制牌 => 等同於 C# 的 CancellationToken
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cleanerService.Start(ctx)
}
