package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
)

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
