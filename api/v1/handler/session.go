package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/v1/entities"
)

func (a *Api) Stream(ctx *gin.Context) {
	var req entities.StreamReq
	_ = ctx.ShouldBindQuery(&req)

	auth, _ := utils.GetAuth(ctx)
	err := a.srv.Listen(ctx, auth, req.ID, req.Seq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}
}

func (a *Api) Chat(ctx *gin.Context) {
	var req entities.ChatReq
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
	err := a.srv.Chat(auth, req.ID, req.Content)
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

func (a *Api) Approval(ctx *gin.Context) {
	var req entities.ApprovalReq
	_ = ctx.ShouldBindUri(&req)
	_ = ctx.ShouldBindJSON(&req)

	if req.ID == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: id is required",
		})
		return
	}

	err := a.srv.Approve(req.ID, req.Info.ID, req.Accept, req.Info.Method, req.Info.Params)
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

func (a *Api) GetSessionList(ctx *gin.Context) {
	var req entities.GetSessionListReq
	_ = ctx.ShouldBindQuery(&req)

	if req.Limit <= 0 || req.Limit > 50 {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: limit is out of range, expect (0, 50]",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	objs, err := a.srv.GetSessionList(auth, req.Offset, req.Limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	rsp := &entities.GetSessionListRsp{
		Offset: req.Offset,
		Data:   make([]*entities.SessionBase, 0, len(objs)),
	}
	if len(objs) == 0 {
		ctx.JSON(http.StatusOK, utils.Response{
			Code: http.StatusOK,
			Data: &entities.GetSessionListRsp{},
		})
		return
	}

	rsp.Offset = objs[0].ID.Hex()
	for _, item := range objs {
		rsp.Data = append(rsp.Data, &entities.SessionBase{
			ID:        item.ID.Hex(),
			Name:      item.Name,
			Status:    int(item.Status),
			CreatedAt: item.CreatedAt.Unix(),
			UpdatedAt: item.UpdatedAt.Unix(),
		})
	}
	ctx.JSON(http.StatusOK, utils.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) GetTurns(ctx *gin.Context) {
	var req entities.GetTurnsReq
	_ = ctx.ShouldBindUri(&req)
	_ = ctx.ShouldBindQuery(&req)

	if req.ID == "" {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: id is required",
		})
		return
	}
	if req.Limit <= 0 || req.Limit > 10 {
		ctx.JSON(http.StatusBadRequest, utils.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid params: limit is out of range, expect (0, 10]",
		})
		return
	}

	auth, _ := utils.GetAuth(ctx)
	objs, err := a.srv.GetTurns(auth, req.ID, req.Offset, req.Limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	rsp := &entities.GetTurnsRsp{
		Offset: req.Offset,
		Data:   make([]*entities.TurnBase, 0, len(objs)),
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
		info := &entities.TurnBase{
			ID:       item.ID.Hex(),
			Messages: make([]*entities.Message, 0, len(item.Messages)),
		}
		for _, elem := range item.Messages {
			info.Messages = append(info.Messages, &entities.Message{
				Role:      string(elem.Role),
				Content:   elem.Content,
				CreatedAt: elem.CreatedAt.Unix(),
			})
		}
		rsp.Data = append(rsp.Data, info)
	}
	ctx.JSON(http.StatusOK, utils.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}
