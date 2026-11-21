package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/v1/handler"
)

func sessionRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/session")

	group.GET("/stream", api.Stream)
}
