package auth

import (
	"errors"
	"strings"
)

func normalizeAvatar(avatar string) (string, error) {
	normalized := strings.TrimSpace(avatar)
	if len(normalized) > 255 {
		return "", errors.New("头像标识不能超过 255 个字符")
	}
	return normalized, nil
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}
