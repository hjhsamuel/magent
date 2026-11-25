package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
	corev1 "k8s.io/api/core/v1"
)

func (a *Api) CheckWorkspaceStatus(ctx *gin.Context) {
	auth, _ := utils.GetAuth(ctx)

	status, err := a.srv.CheckWorkspaceStatus(auth)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	rsp := entities.CheckWorkspaceStatus{}
	switch status {
	case corev1.PodRunning:
		rsp.Status = "running"
	case corev1.PodPending:
		rsp.Status = "pending"
	case corev1.PodSucceeded, corev1.PodFailed:
		rsp.Status = "error"
	default:
		rsp.Status = "none"
	}
	ctx.JSON(http.StatusOK, utils.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) CreateWorkspace(ctx *gin.Context) {
	auth, _ := utils.GetAuth(ctx)

	err := a.srv.CreateWorkspace(auth)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	} else {
		ctx.JSON(http.StatusOK, utils.Response{
			Code: http.StatusOK,
			Data: "OK",
		})
	}
}
