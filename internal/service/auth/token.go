package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func hashSessionToken(sessionToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionToken)))
	return hex.EncodeToString(sum[:])
}
