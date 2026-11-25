package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
	corev1 "k8s.io/api/core/v1"
)

// CheckWorkspaceStatus 查询当前用户的工作区状态。
// @Summary 查询工作区状态
// @Description data.status 的取值为 running（运行中）、pending（启动中）、error（Pod 已结束或失败）或 none（其他状态或无 Pod）。
// @Tags workspace
// @Produce json
// @Success 200 {object} utils.Response{data=entities.CheckWorkspaceStatus}
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/workspace [get]
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

// CreateWorkspace 创建当前用户的工作区。
// @Summary 创建工作区
// @Description 工作区处于 pending 或 running 状态时也返回成功。可通过 GET 接口查询当前状态。
// @Tags workspace
// @Produce json
// @Success 200 {object} utils.Response{data=string} "data 为 OK"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/workspace [post]
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
