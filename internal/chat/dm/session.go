package dm

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MergeRoomBackedSession 合并 Room 索引会话与本地 overlay 会话。
func MergeRoomBackedSession(current protocol.Session, roomSession protocol.Session) protocol.Session {
	merged := roomSession
	if strings.TrimSpace(StringPointerValue(merged.SessionID)) == "" && current.SessionID != nil {
		merged.SessionID = current.SessionID
	}
	return merged
}

// SessionsEqual 判断两个 session 的关键持久字段是否一致。
func SessionsEqual(left protocol.Session, right protocol.Session) bool {
	return left.SessionKey == right.SessionKey &&
		left.AgentID == right.AgentID &&
		StringPointerValue(left.SessionID) == StringPointerValue(right.SessionID) &&
		StringPointerValue(left.RoomSessionID) == StringPointerValue(right.RoomSessionID) &&
		StringPointerValue(left.RoomID) == StringPointerValue(right.RoomID) &&
		StringPointerValue(left.ConversationID) == StringPointerValue(right.ConversationID) &&
		left.ChannelType == right.ChannelType &&
		left.ChatType == right.ChatType &&
		left.Status == right.Status &&
		left.Title == right.Title
}

// StringPointerValue 返回字符串指针的去空白值。
func StringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// PreferSessionID 在 next 非空时优先使用 next。
func PreferSessionID(current *string, next string) *string {
	if strings.TrimSpace(next) != "" {
		return &next
	}
	return current
}

// NormalizeString 返回 any 中的字符串值。
func NormalizeString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

// FirstNonEmpty 返回首个非空字符串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
