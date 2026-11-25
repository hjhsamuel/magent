package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/v1/handler"
)

func workspaceRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/workspace")

	group.GET("", api.CheckWorkspaceStatus)
	group.POST("", api.CreateWorkspace)
}
