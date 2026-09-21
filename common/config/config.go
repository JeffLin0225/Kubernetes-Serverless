package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 聚合整個 Monorepo 的所有微服務設定
type Config struct {
	Engine  EngineConfig
	Cleaner CleanerConfig
}

// EngineConfig - SEP Engine (Serverless Execution Platform) 微服務專屬配置
type EngineConfig struct {
	Namespace       string
	Port            string
	ImagePullPolicy string
	SystemQuotasCM  string // 配額 ConfigMap 名稱
}

// CleanerConfig - SEP 異常 Pod 收割微服務專屬配置
type CleanerConfig struct {
	TargetNamespace      string
	ScanInterval         time.Duration
	NormalWaitingReasons []string // 正常啟動過程中的 Waiting 狀態白名單，不在此清單中的一律視為異常
}

// LoadEngineConfig 專門載入 Engine 微服務設定
func LoadEngineConfig() *EngineConfig {
	loaded := false
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Println("[INFO] 未找到 .env 檔，將使用系統環境變數（K8s 模式）")
	}
	cfg := loadEngineConfig()
	return &cfg
}

// LoadCleanerConfig 專門載入 Cleaner 微服務設定
func LoadCleanerConfig() *CleanerConfig {
	loaded := false
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Println("[INFO] 未找到 cleaner .env 檔，將使用系統環境變數（K8s 模式）")
	}
	cfg := loadCleanerConfig()
	return &cfg
}

func loadEngineConfig() EngineConfig {
	return EngineConfig{
		Namespace:       getEnv("NAMESPACE", "ns-sep"),
		Port:            getEnv("PORT", "8080"),
		ImagePullPolicy: getEnv("IMAGE_PULL_POLICY", "Never"),
		SystemQuotasCM:  getEnv("SYSTEM_QUOTAS_CONFIGMAP", "sep-system-quotas"),
	}
}

func loadCleanerConfig() CleanerConfig {
	// LookupEnv 可以區分「沒設定」跟「設定成空字串」兩種情況：
	// - 沒設定（本機開發未帶 .env）→ 使用預設值 "ns-sep"（只掃本機叢集）
	// - 設定成空字串 (TARGET_NAMESPACE=) → 代表掃全叢集所有 Namespace
	targetNamespace := "ns-sep" // 預設：本機開發用
	if val, ok := os.LookupEnv("TARGET_NAMESPACE"); ok {
		targetNamespace = val // 有設定（含空字串）就直接用，空字串 = 全叢集
	}

	return CleanerConfig{
		TargetNamespace:      targetNamespace,
		ScanInterval:         getDurationEnv("SCAN_INTERVAL", 5*time.Second),
		NormalWaitingReasons: getSliceEnv("NORMAL_WAITING_REASONS", []string{"ContainerCreating", "PodInitializing"}),
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

// getSliceEnv 取得逗號分隔的字串陣列型別環境變數
func getSliceEnv(key string, defaultVal []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}
