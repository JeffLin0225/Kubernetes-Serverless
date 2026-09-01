package main

import (
	"log"

	"sep/common/config"
	"sep/common/kube"
	"sep/services/engine/router"
	"sep/services/engine/service"
)

func main() {
	// 載入設定檔物件
	cfg := config.LoadEngineConfig()

	// 初始化 K8s Clientset
	clientSet, err := kube.NewKubeClient()
	if err != nil {
		log.Fatalf("初始化 K8s 連線失敗: %v", err)
	}

	// 建立 Job Launcher
	jobLauncher := service.NewJobLauncher(cfg, clientSet)

	// 路由註冊
	r := router.SetupRouter(cfg, jobLauncher)

	// 啟動 Server Listener
	log.Printf("🚀 [Engine] 服務啟動於 Port: %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服務啟動失敗: %v", err)
	}
}
