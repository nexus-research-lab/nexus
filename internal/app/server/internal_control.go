package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func newInternalControlToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("生成内部控制面 token 失败: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func internalControlBaseURL(cfg config.Config) string {
	prefix := strings.TrimSpace(cfg.APIPrefix)
	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Port, strings.TrimRight(prefix, "/"))
}
