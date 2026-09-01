package controller

import (
	"fmt"
	"log"
	"net/http"

	"sep/common/model"
	"sep/services/engine/service"

	"github.com/gin-gonic/gin"
)

type RunController struct {
	jobLauncher *service.JobLauncher
}

func NewRunController(jobLauncher *service.JobLauncher) *RunController {
	return &RunController{
		jobLauncher: jobLauncher,
	}
}

func (ctrl *RunController) HandleRun(c *gin.Context) {
	var req model.RunRequest

	// ShouldBindJSON 會自動解析 Body 並驗證 required 條件
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "無效的請求資料: " + err.Error(),
		})
		return
	}

	// 呼叫 Service 執行 K8s 操作
	job, err := ctrl.jobLauncher.CreateJob(c.Request.Context(), req)
	if err != nil {
		log.Printf("[ERROR] 建立 Job 失敗，TaskID=%s SystemID=%s err=%v", req.TaskID, req.SystemID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("建立 Job 失敗，請聯繫管理員，TaskID=%s SystemID=%s", req.TaskID, req.SystemID),
		})
		return
	}

	c.JSON(http.StatusOK, model.RunResponse{
		Status:  "ok",
		Message: "已成功接收 TaskID:" + req.TaskID,
		JobName: job.Name,
	})
}
