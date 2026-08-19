package controller

import (
	"net/http"
	"kubernetes-serverless/model"
	"kubernetes-serverless/service/kube"

	"github.com/gin-gonic/gin"
)

type RunController struct {
	kubeCli *kube.KubeCli
}

func NewRunController(kubeCli *kube.KubeCli) *RunController {
	return &RunController{
		kubeCli: kubeCli,
	}
}

func (ctrl *RunController) HandleRun(c *gin.Context) {
	var req model.RunRequest

	// ShouldBindJSON 會自動解析 Body 並驗證 required 條件
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error",
			"error": "無效的請求資料:" + err.Error()})
		return
	}

	// 呼叫 Service 執行 K8s 操作（先前寫好的邏輯完全相容）
	_ = ctrl.kubeCli

	c.JSON(http.StatusOK, model.RunResponse{
		Status:  "ok",
		Message: "已成功接收 FlowRunID:" + req.FlowRunID,
	})

}
