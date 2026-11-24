package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
	"github.com/hjhsamuel/magent/internal/store/schema"
)

// GetToolList 获取工具列表。
// @Summary 获取工具列表
// @Tags tool
// @Produce json
// @Param kind query int false "工具类型，0 为 MCP，1 为 Skill" Enums(0,1)
// @Param offset query string false "分页游标"
// @Param limit query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} utils.Response{data=entities.GetToolListRsp}
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/tool/list [get]
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

// AddTool 添加工具。
// @Summary 添加工具
// @Tags tool
// @Accept json
// @Produce json
// @Param body body entities.AddToolReq true "工具配置，name 和 url 必填"
// @Success 200 {object} utils.Response{data=string}
// @Failure 400 {object} utils.Response "请求参数错误"
// @Failure 401 {object} utils.Response "身份验证失败"
// @Failure 500 {object} utils.Response "服务内部错误"
// @Router /magent/v1/tool [post]
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
