package cryptox

import (
	"crypto/sha256"
	"encoding/hex"
)

func TokenSha256(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
