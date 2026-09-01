package router

import (
	"sep/common/config"
	"sep/services/engine/controller"
	"sep/services/engine/service"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.EngineConfig, jobLauncher *service.JobLauncher) *gin.Engine {
	// 初始化 Gin 引擎
	r := gin.Default()

	runCtrl := controller.NewRunController(jobLauncher)

	// 註冊路由群組
	r.GET("/health", controller.HealthCheck)

	api := r.Group("/api")
	{
		api.POST("/run", runCtrl.HandleRun)
	}

	return r
}
