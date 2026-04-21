package agent

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var nameAllowedPattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9 _-]+$`)

const (
	nameMinLength = 2
	nameMaxLength = 40
)

// NormalizeName 标准化 Agent 名称。
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

// BuildWorkspaceDirName 生成安全目录名。
func BuildWorkspaceDirName(agentName string) string {
	normalized := strings.ReplaceAll(NormalizeName(agentName), " ", "_")
	var builder strings.Builder
	lastUnderscore := false
	for _, value := range normalized {
		switch {
		case unicode.IsLetter(value), unicode.IsDigit(value), value == '_', value == '-':
			builder.WriteRune(value)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteRune('_')
			}
			lastUnderscore = true
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "agent"
	}
	return result
}

// ValidateName 校验名称格式。
func ValidateName(name string) string {
	normalized := NormalizeName(name)
	switch {
	case normalized == "":
		return "名称不能为空"
	case len([]rune(normalized)) < nameMinLength:
		return fmt.Sprintf("名称至少 %d 个字符", nameMinLength)
	case len([]rune(normalized)) > nameMaxLength:
		return fmt.Sprintf("名称不能超过 %d 个字符", nameMaxLength)
	case !nameAllowedPattern.MatchString(normalized):
		return "仅支持中文、英文、数字、空格、下划线和连字符"
	default:
		return ""
	}
}

// NewAgentID 生成新的 agent_id。
func NewAgentID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())[:12]
}

func buildStableID(prefix string, raw string) string {
	digest := sha1.Sum([]byte(raw))
	return prefix + "_" + hex.EncodeToString(digest[:])[:20]
}
