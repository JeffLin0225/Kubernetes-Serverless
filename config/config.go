package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Namespace       string
	Port            string
	ImagePullPolicy string
	SystemQuotasCM  string // 配額 ConfigMap 名稱
}

func LoadConfig() *Config {
	// 本機開發：從 .env 載入環境變數
	// K8s 環境：.env 不存在屬正常，env 由 ConfigMap 注入，直接略過
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] 未找到 .env 檔，將使用系統環境變數（K8s 模式）")
	}

	return &Config{
		Namespace:       mustGetEnv("NAMESPACE"),
		Port:            mustGetEnv("PORT"),
		ImagePullPolicy: mustGetEnv("IMAGE_PULL_POLICY"),
		SystemQuotasCM:  mustGetEnv("SYSTEM_QUOTAS_CONFIGMAP"),
	}
}

// mustGetEnv 取得環境變數，若不存在則直接 Fatal
func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("缺少必要的環境變數: %s", key)
	}
	return val
}
