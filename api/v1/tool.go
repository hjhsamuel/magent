package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/v1/handler"
)

func toolRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/tool")

	group.GET("/list", api.GetToolList)
	group.POST("", api.AddTool)
	group.DELETE("/:id", api.DelTool)
}
