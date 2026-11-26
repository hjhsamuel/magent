package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/v1/handler"
)

func sessionRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/session")

	group.GET("/stream", api.Stream)
	group.POST("/:id/chat", api.Chat)
	group.POST("/:id/approval", api.Approval)

	group.GET("/list", api.GetSessionList)
	group.GET("/:id/turns", api.GetTurns)
	group.DELETE("/:id", api.DelSession)
}
