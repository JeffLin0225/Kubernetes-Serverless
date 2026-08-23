package main

import (
	"log"
	"kubernetes-serverless/config"
	"kubernetes-serverless/router"
	"kubernetes-serverless/service/kube"
)

func main() {

	// 載入設定檔物件
	cfg := config.LoadConfig()

	// 建立 Kube Cluster 連線物件
	// go 是依照資料夾分類，所以是 kube 非kubeCli
	kubeCli, err := kube.NewKubeCli(cfg)
	if err != nil {
		log.Fatal("初始化 kubeCli 失敗！", err)
	}

	// 路由註冊
	r := router.SetupRouter(cfg, kubeCli)

	// 啟動 Server Listener
	log.Printf("服務啟動於 Port: %s ", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服務啟動失敗 %v", err)
	}

	log.Printf("連線 Prefect Server API: %s", cfg.PrefectAPIURL)
}
