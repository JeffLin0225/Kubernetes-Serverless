package controller

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"sep/common/model"
	"sep/services/engine/service"

	"github.com/gin-gonic/gin"
)

type RunController struct {
	launcher service.Launcher
}

func NewRunController(launcher service.Launcher) *RunController {
	return &RunController{
		launcher: launcher,
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

	// 呼叫 Service 啟動任務（K8s 或 Cloud Run，由 main.go 決定）
	jobName, err := ctrl.launcher.Launch(c.Request.Context(), req)
	if err != nil {
		log.Printf("[ERROR] 建立 Job 失敗，TaskID=%s SystemID=%s err=%v", req.TaskID, req.SystemID, err)

		// 請求本身不被允許（未註冊的系統、不在白名單的 image）回 400，並告知原因
		if errors.Is(err, service.ErrRejected) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("建立 Job 失敗，請聯繫管理員，TaskID=%s SystemID=%s", req.TaskID, req.SystemID),
		})
		return
	}

	c.JSON(http.StatusOK, model.RunResponse{
		Status:  "ok",
		Message: "已成功接收 TaskID:" + req.TaskID,
		JobName: jobName,
	})
}
