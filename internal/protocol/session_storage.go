// INPUT: 结构化或 legacy session_key。
// OUTPUT: 与现有 workspace `.agents/sessions` 目录完全一致的兼容物理身份。
// POS: legacy 路径、删除 tombstone 和 runtime admission 共用的碰撞边界；不得用于显示。
package protocol

import (
	"strconv"
	"strings"
)

// LegacySessionDirectoryIdentity 返回当前已落盘 session 目录的兼容身份。
//
// 历史编码不是单射，因此调用方必须把相同结果视为同一物理安全边界，并继续
// 用 meta.session_key 做精确身份核对。
func LegacySessionDirectoryIdentity(value string) string {
	parsed := ParseSessionKey(value)
	switch parsed.Kind {
	case SessionKeyKindRoom:
		return joinLegacySessionPathSegments("room", escapeLegacySessionPathAtom(parsed.ConversationID))
	case SessionKeyKindAgent:
		switch strings.TrimSpace(parsed.ChatType) {
		case "dm":
			parts := []string{"dm"}
			if channel := escapeLegacySessionPathAtom(parsed.Channel); channel != "" {
				parts = append(parts, channel)
			}
			if accountID := escapeLegacySessionPathAtom(parsed.AccountID); accountID != "" {
				parts = append(parts, "acct", accountID)
			}
			if ref := escapeLegacySessionPathAtom(parsed.Ref); ref != "" {
				parts = append(parts, ref)
			}
			if threadID := escapeLegacySessionPathAtom(parsed.ThreadID); threadID != "" {
				parts = append(parts, "topic", threadID)
			}
			return joinLegacySessionPathSegments(parts...)
		case "group":
			parts := []string{"room"}
			if channel := strings.TrimSpace(parsed.Channel); channel != "" && channel != SessionChannelWebSocketSegment {
				parts = append(parts, escapeLegacySessionPathAtom(channel))
			}
			if accountID := escapeLegacySessionPathAtom(parsed.AccountID); accountID != "" {
				parts = append(parts, "acct", accountID)
			}
			if ref := escapeLegacySessionPathAtom(parsed.Ref); ref != "" {
				parts = append(parts, ref)
			}
			if threadID := escapeLegacySessionPathAtom(parsed.ThreadID); threadID != "" {
				parts = append(parts, "topic", threadID)
			}
			return joinLegacySessionPathSegments(parts...)
		default:
			return joinLegacySessionPathSegments(
				"session",
				escapeLegacySessionPathAtom(parsed.Channel),
				escapeLegacySessionPathAtom(parsed.AccountID),
				escapeLegacySessionPathAtom(parsed.Ref),
				escapeLegacySessionPathAtom(parsed.ThreadID),
			)
		}
	default:
		return joinLegacySessionPathSegments("session", escapeLegacySessionPathAtom(value))
	}
}

func joinLegacySessionPathSegments(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "-")
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "-")
}

func escapeLegacySessionPathAtom(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, character := range value {
		isLetter := character >= 'a' && character <= 'z'
		isUpper := character >= 'A' && character <= 'Z'
		isDigit := character >= '0' && character <= '9'
		switch {
		case isLetter || isUpper || isDigit:
			builder.WriteRune(character)
		case character == '-' || character == '_' || character == '.':
			builder.WriteRune(character)
		default:
			builder.WriteString("_")
			builder.WriteString(strconv.FormatInt(int64(character), 16))
		}
	}
	return strings.Trim(builder.String(), "-")
}
