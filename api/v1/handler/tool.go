package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
	"github.com/hjhsamuel/magent/internal/store/schema"
)

func (a *Api) GetToolList(ctx *gin.Context) {
	var req entities.GetToolListReq
	_ = ctx.ShouldBindQuery(&req)

	if req.Limit <= 0 || req.Limit > 50 {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: limit out of range, expect (0, 50]",
		})
		return
	}

	objs, err := a.srv.GetToolList(req.Kind, req.Offset, req.Limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	rsp := &entities.GetToolListRsp{
		Offset: req.Offset,
		Data:   make([]*entities.ToolBase, 0, len(objs)),
	}
	if len(objs) == 0 {
		ctx.JSON(http.StatusOK, utils.Response{
			Code: http.StatusOK,
			Data: rsp,
		})
		return
	}

	rsp.Offset = objs[0].ID.Hex()
	for _, item := range objs {
		rsp.Data = append(rsp.Data, &entities.ToolBase{
			ID:          item.ID.Hex(),
			Name:        item.Name,
			Description: item.Description,
			Status:      int(item.Status),
			Creator:     item.Creator,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
		})
	}
	ctx.JSON(http.StatusOK, utils.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) AddTool(ctx *gin.Context) {
	var req entities.AddToolReq
	_ = ctx.ShouldBindJSON(&req)

	if req.Name == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: name is required",
		})
		return
	}
	if req.Url == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: url is required",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	var (
		id  string
		err error
	)
	switch schema.ToolKind(req.Kind) {
	case schema.ToolMcp:
		id, err = a.srv.AddMcp(auth, &schema.Tool{
			Name:        req.Name,
			Description: req.Description,
			Url:         req.Url,
			Kind:        schema.ToolMcp,
			Token:       req.Token,
		})
	case schema.ToolSkill:
		id, err = a.srv.AddSkill(auth, &schema.Tool{
			Name:        req.Name,
			Description: req.Description,
			Url:         req.Url,
			Kind:        schema.ToolSkill,
			Token:       req.Token,
		})
	default:
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: kind",
		})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	} else {
		ctx.JSON(http.StatusOK, utils.Response{
			Code: http.StatusOK,
			Data: id,
		})
	}
}
