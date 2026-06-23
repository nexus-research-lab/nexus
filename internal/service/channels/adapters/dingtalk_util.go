package adapters

import (
	"net/url"
	"strings"
)

func normalizeDingTalkConversationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "single", "private", "private_chat", "dm":
		return "dm"
	default:
		return "group"
	}
}

func normalizeDingTalkBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "https://api.dingtalk.com"
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return strings.TrimRight(value, "/")
	}
	return value
}
