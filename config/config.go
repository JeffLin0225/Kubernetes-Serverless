package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PrefectAPIURL   string
	NameSpace       string
	Image           string
	Port            string
	ImagePullPolicy string
	SystemQuotasCM  string // 配額 ConfigMap 名稱
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	return &Config{
		PrefectAPIURL:   os.Getenv("PREFECT_API_URL"),
		NameSpace:       os.Getenv("NAMESPACE"),
		Image:           os.Getenv("IMAGE"),
		Port:            os.Getenv("PORT"),
		ImagePullPolicy: getEnv("IMAGE_PULL_POLICY"),
		SystemQuotasCM:  getEnv("SYSTEM_QUOTAS_CONFIGMAP"),
	}
}
