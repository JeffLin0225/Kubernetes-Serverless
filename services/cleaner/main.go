package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sep/common/config"
	"sep/common/kube"
	"sep/services/cleaner/service"
)

func main() {
	log.Println("[INFO] 正在初始化 Cleaner 微服務...")

	// 載入 Cleaner 微服務設定
	cfg := config.LoadCleanerConfig()

	// 支援優雅停機 (Graceful Shutdown)
	// ctx 生命週期控制牌 => 等同於 C# 的 CancellationToken
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Platform {
	case "cloudrun":
		// Cloud Run Job：掃描一次就結束，由 Cloud Scheduler 定時觸發
		cleaner, err := service.NewCloudRunCleaner(ctx, cfg)
		if err != nil {
			log.Fatalf("[FATAL] 初始化 Cloud Run cleaner 失敗: %v", err)
		}
		if err := cleaner.RunOnce(ctx); err != nil {
			// 非 0 結束，Cloud Run 會把這次 execution 標記為失敗
			log.Fatalf("[FATAL] 巡檢失敗: %v", err)
		}
	case "k8s":
		// 初始化共用 K8s Clientset
		clientSet, err := kube.NewKubeClient()
		if err != nil {
			log.Fatalf("[FATAL] 初始化 K8s 連線失敗: %v", err)
		}
		cleanerService := service.NewCleanerService(cfg, clientSet)
		cleanerService.Start(ctx)
	default:
		log.Fatalf("[FATAL] 不支援的 PLATFORM: '%s'（只能是 k8s 或 cloudrun）", cfg.Platform)
	}
}
