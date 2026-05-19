package runtime

import (
	"regexp"
	"strings"
)

const redactedSecretValue = "[redacted]"

var sensitiveLogPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(NEXUS_ROOM_INTERNAL_TOKEN\s*=\s*)[^\s"'\\]+`),
	regexp.MustCompile(`(?i)("NEXUS_ROOM_INTERNAL_TOKEN"\s*:\s*")[^"]+(")`),
	regexp.MustCompile(`(?i)(X-Nexus-Internal-Token\s*[:=]\s*)[^\s"'\\]+`),
	regexp.MustCompile(`(?i)("X-Nexus-Internal-Token"\s*:\s*")[^"]+(")`),
}

// RedactSensitiveText 脱敏运行时日志中可能泄露的内部控制面密钥。
func RedactSensitiveText(value string) string {
	result := strings.TrimSpace(value)
	if result == "" {
		return ""
	}
	for _, pattern := range sensitiveLogPatterns {
		result = pattern.ReplaceAllString(result, "${1}"+redactedSecretValue+"${2}")
	}
	return result
}

func redactSDKLogFields(fields []any) []any {
	if len(fields) == 0 {
		return fields
	}
	redacted := make([]any, len(fields))
	for index, field := range fields {
		if text, ok := field.(string); ok {
			redacted[index] = RedactSensitiveText(text)
			continue
		}
		redacted[index] = field
	}
	return redacted
}
