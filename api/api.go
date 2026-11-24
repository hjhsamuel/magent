package api

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/hjhsamuel/magent/api/v1"
	_ "github.com/hjhsamuel/magent/docs"
	"github.com/hjhsamuel/magent/internal/service"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Route(srv *service.Service, e *gin.Engine) {
	group := e.Group("/magent")

	swaggerRoute(group)

	v1.Route(srv, group)
}

func swaggerRoute(e *gin.RouterGroup) {
	e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
