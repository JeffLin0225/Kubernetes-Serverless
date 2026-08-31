package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config 聚合整個 Monorepo 的所有微服務設定
type Config struct {
	Engine  EngineConfig
	Cleaner CleanerConfig
}

// EngineConfig - Serverless Engine 微服務專屬配置
type EngineConfig struct {
	Namespace       string
	Port            string
	ImagePullPolicy string
	SystemQuotasCM  string // 配額 ConfigMap 名稱
}

// CleanerConfig - 異常 Pod 收割微服務專屬配置
type CleanerConfig struct {
	TargetNamespace string
	ScanInterval    time.Duration
}

// // LoadConfig 載入全專案設定
// func LoadConfig() *Config {
// 	// 支援載入當前目錄或上一層根目錄的 .env
// 	if err := godotenv.Load(".env", "../.env"); err != nil {
// 		log.Println("[INFO] 未找到 engine .env 檔，將使用系統環境變數（K8s 模式）")
// 	}

// 	return &Config{
// 		Engine:  loadEngineConfig(),
// 		Cleaner: loadCleanerConfig(),
// 	}
// }

// LoadEngineConfig 專門載入 Engine 微服務設定
func LoadEngineConfig() *EngineConfig {
	if err := godotenv.Load(".env", "../.env"); err != nil {
		log.Println("[INFO] 未找到 .env 檔，將使用系統環境變數（K8s 模式）")
	}
	cfg := loadEngineConfig()
	return &cfg
}

// LoadCleanerConfig 專門載入 Cleaner 微服務設定
func LoadCleanerConfig() *CleanerConfig {
	if err := godotenv.Load(".env", "../.env"); err != nil {
		log.Println("[INFO] 未找到 cleaner .env 檔，將使用系統環境變數（K8s 模式）")
	}
	cfg := loadCleanerConfig()
	return &cfg
}

func loadEngineConfig() EngineConfig {
	return EngineConfig{
		Namespace:       getEnv("NAMESPACE", "default"),
		Port:            getEnv("PORT", "8080"),
		ImagePullPolicy: getEnv("IMAGE_PULL_POLICY", "Never"),
		SystemQuotasCM:  getEnv("SYSTEM_QUOTAS_CONFIGMAP", "serverless-system-quotas"),
	}
}

func loadCleanerConfig() CleanerConfig {
	return CleanerConfig{
		TargetNamespace: getEnv("TARGET_NAMESPACE", ""),
		ScanInterval:    getDurationEnv("SCAN_INTERVAL", 5*time.Second),
	}
}

// getEnv 取得環境變數，若為空則回傳預設值
func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// getDurationEnv 取得 time.Duration 型別環境變數
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("[WARN] %s 格式無效 (%s)，使用預設值 %v", key, val, defaultVal)
		return defaultVal
	}
	return d
}
