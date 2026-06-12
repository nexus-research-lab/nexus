package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func channelAccountIDFromSecret(prefix string, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(secret))
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(sum[:8])
}
