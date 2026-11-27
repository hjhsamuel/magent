package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
)

func AuthMiddleware(ctx *gin.Context) {
	content, err := ctx.Cookie(utils.GetCookieKey())
	if err != nil || content == "" {
		bearer := ctx.GetHeader("Authorization")
		content = strings.TrimPrefix(bearer, "Bearer ")
	}
	if content == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &utils.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	auth, err := utils.ParseAuth(content)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &utils.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	ctx.Set(utils.AuthKey, auth)
	ctx.Next()
}
