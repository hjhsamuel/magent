package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils/middleware"
	"github.com/hjhsamuel/magent/api/v1/handler"
	"github.com/hjhsamuel/magent/internal/service"
)

func Route(srv *service.Service, g *gin.RouterGroup) {
	api := handler.NewApi(srv)
	group := g.Group("/v1")
	group.Use(middleware.AuthMiddleware)

	sessionRoute(api, group)
	toolRoute(api, group)
	workspaceRoute(api, group)
}
