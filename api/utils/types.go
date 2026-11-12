package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ----------------------------------------- Auth ---------------------------------------- //

var (
	CookieKey string
	JwtSalt   string
)

const AuthKey = "AUTH"

func SetCookieKey(key string) {
	CookieKey = key
}

func GetCookieKey() string {
	return CookieKey
}

func SetJwtSalt(salt string) {
	JwtSalt = salt
}

func GetJwtSalt() string {
	return JwtSalt
}

type Auth struct {
	Raw   jwt.MapClaims
	Token string
}

func GetAuth(ctx *gin.Context) (*Auth, error) {
	val, exists := ctx.Get(AuthKey)
	if !exists {
		return nil, errors.New("not exists")
	}
	return val.(*Auth), nil
}

// ------------------------------------------ Request --------------------------------------- //

type Response struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}
