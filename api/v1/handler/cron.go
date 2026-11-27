package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
)

// GetCronList 获取定时任务列表。
// @Summary 获取定时任务列表
// @Tags cron
// @Produce json
// @Param offset query string false "分页游标"
// @Param limit query int true "每页数量" minimum(1) maximum(50)
// @Param status query int false "状态，0 为在线，1 为离线" Enums(0,1)
// @Success 200 {object} utils.Response{data=entities.GetCronListRsp}
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/cron [get]
func (a *Api) GetCronList(ctx *gin.Context) {
	var req entities.GetCronListReq
	_ = ctx.ShouldBindQuery(&req)

	if req.Limit <= 0 || req.Limit > 50 {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: limit out of range, expect (0, 50]",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	objs, err := a.srv.GetCronList(auth, req.Offset, req.Limit, req.Status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	rsp := &entities.GetCronListRsp{
		Offset: req.Offset,
		Data:   make([]*entities.CronInfo, 0, len(objs)),
	}
	for _, item := range objs {
		rsp.Offset = item.ID.Hex()
		rsp.Data = append(rsp.Data, &entities.CronInfo{
			ID:        item.ID.Hex(),
			Name:      item.Name,
			Schedule:  item.Schedule,
			Content:   item.Content,
			Status:    int(item.Status),
			Creator:   item.Creator,
			CreatedAt: item.CreatedAt.Unix(),
		})
	}
	ctx.JSON(http.StatusOK, utils.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

// DeleteCronJob 删除定时任务。
// @Summary 删除定时任务
// @Tags cron
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} utils.Response{data=string} "data 为 OK"
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/cron/{id} [delete]
func (a *Api) DeleteCronJob(ctx *gin.Context) {
	var req entities.DeleteCronJobReq
	_ = ctx.ShouldBindUri(&req)

	if req.ID == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: id is required",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	err := a.srv.DeleteCronJob(auth, req.ID)
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

// AddCronJob 添加定时任务。
// @Summary 添加定时任务
// @Tags cron
// @Accept json
// @Produce json
// @Param body body entities.AddCronJobReq true "任务配置，name、spec 和 content 必填"
// @Success 200 {object} utils.Response{data=string} "data 为任务 ID"
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/cron [post]
func (a *Api) AddCronJob(ctx *gin.Context) {
	var req entities.AddCronJobReq
	_ = ctx.ShouldBindJSON(&req)

	if req.Name == "" || req.Spec == "" || req.Content == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: name, spec and content are required",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	id, err := a.srv.AddCronJob(auth, req.Name, req.Spec, req.Content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	} else {
		ctx.JSON(http.StatusOK, utils.Response{
			Code: http.StatusOK,
			Data: id.Hex(),
		})
	}
}

// UpdateCronJob 更新定时任务。
// @Summary 更新定时任务
// @Tags cron
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Param body body entities.UpdateCronJobReq true "至少提供 name、spec、content、status 中的一项"
// @Success 200 {object} utils.Response{data=string} "data 为 OK"
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/cron/{id} [put]
func (a *Api) UpdateCronJob(ctx *gin.Context) {
	var req entities.UpdateCronJobReq
	_ = ctx.ShouldBindUri(&req)
	_ = ctx.ShouldBindJSON(&req)

	if req.ID == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: id is required",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	err := a.srv.UpdateCronJob(auth, req.ID, req.Name, req.Spec, req.Content, req.Status)
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
