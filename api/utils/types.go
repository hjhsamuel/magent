package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hjhsamuel/magent/pkg/token"
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
	ID    int
	Name  string
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

func ParseAuth(raw string) (*Auth, error) {
	t, claims, err := token.ParseJWT(raw, GetJwtSalt())
	if err != nil {
		return nil, err
	}

	id, _ := claims["id"].(float64)
	name, _ := claims["name"].(string)
	if int(id) == 0 || name == "" {
		return nil, errors.New("missing user identity")
	}
	return &Auth{
		ID:    int(id),
		Name:  name,
		Raw:   claims,
		Token: t.Raw,
	}, nil
}

// ------------------------------------------ Request --------------------------------------- //

type Response struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}
