package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/v1/handler"
)

func cronRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/cron")

	group.GET("", api.GetCronList)
	group.DELETE("/:id", api.DeleteCronJob)
	group.POST("", api.AddCronJob)
	group.PUT("/:id", api.UpdateCronJob)
}
