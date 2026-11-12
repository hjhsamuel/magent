package token

import "github.com/golang-jwt/jwt/v5"

func GenerateJWT(claims jwt.MapClaims, secretKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func GenerateJWTWithoutExp(claims jwt.MapClaims, secretKey string) (string, error) {
	newClaims := jwt.MapClaims{}
	for k, v := range claims {
		newClaims[k] = v
	}

	delete(newClaims, "exp")
	return GenerateJWT(newClaims, secretKey)
}
