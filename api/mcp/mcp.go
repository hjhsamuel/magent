package mcp

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/mcp/handler"
	"github.com/hjhsamuel/magent/internal/service"
)

func Route(srv *service.Service, g *gin.RouterGroup) {
	group := g.Group("/mcp")

	h := handler.NewHandler(srv)

	group.Any("", gin.WrapH(h))
}
