package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (s *Service) accessTokenEnabled() bool {
	return strings.TrimSpace(s.config.AccessToken) != ""
}

func extractBearerToken(rawAuthorization string) string {
	header := strings.TrimSpace(rawAuthorization)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func hashSessionToken(sessionToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionToken)))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func newAuthID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UTC().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}
