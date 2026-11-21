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
