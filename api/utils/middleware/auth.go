package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/pkg/token"
)

func AuthMiddleware(ctx *gin.Context) {
	content, err := ctx.Cookie(utils.GetCookieKey())
	if err != nil || content == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &utils.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	t, claim, err := token.ParseJWT(content, utils.GetJwtSalt())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &utils.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	auth := &utils.Auth{
		Raw:   claim,
		Token: t.Raw,
	}
	if v, ok := claim["id"].(int); ok {
		auth.ID = v
	}
	if v, ok := claim["name"].(string); ok {
		auth.Name = v
	}
	if auth.ID == 0 || auth.Name == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &utils.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	ctx.Set(utils.AuthKey, auth)
	ctx.Next()
}
