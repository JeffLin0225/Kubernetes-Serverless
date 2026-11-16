package main

import (
	"context"
	"log"

	"sep/common/config"
	"sep/common/kube"
	"sep/services/engine/router"
	"sep/services/engine/service"
)

func main() {
	// 載入設定檔物件
	cfg := config.LoadEngineConfig()

	// 依 PLATFORM 選擇任務要啟動在哪個平台
	var launcher service.Launcher
	switch cfg.Platform {
	case "cloudrun":
		cloudRunLauncher, err := service.NewCloudRunLauncher(context.Background(), cfg)
		if err != nil {
			log.Fatalf("初始化 Cloud Run launcher 失敗: %v", err)
		}
		launcher = cloudRunLauncher
	case "k8s":
		// 初始化 K8s Clientset
		clientSet, err := kube.NewKubeClient()
		if err != nil {
			log.Fatalf("初始化 K8s 連線失敗: %v", err)
		}
		launcher = service.NewJobLauncher(cfg, clientSet)
	default:
		log.Fatalf("不支援的 PLATFORM: '%s'（只能是 k8s 或 cloudrun）", cfg.Platform)
	}

	// 路由註冊
	r := router.SetupRouter(launcher)

	// 啟動 Server Listener
	log.Printf("🚀 [Engine] 服務啟動於 Port: %s（平台: %s）", cfg.Port, cfg.Platform)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服務啟動失敗: %v", err)
	}
}
