package token

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

func ParseJWT(tokenString string, secretKey string) (*jwt.Token, jwt.MapClaims, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(secretKey), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("JWT 校验失败: %w", err)
	}

	if !token.Valid {
		return nil, nil, fmt.Errorf("JWT 无效")
	}

	return token, claims, nil
}
