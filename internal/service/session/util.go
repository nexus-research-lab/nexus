package session

import (
	"slices"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) requireSessionKey(raw string) (string, protocol.SessionKey, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(raw)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	return sessionKey, protocol.ParseSessionKey(sessionKey), nil
}

func closePersistedSessionMeta(item protocol.Session) protocol.Session {
	item.Status = "closed"
	item.IsActive = false
	return item
}

func mergeSessions(fileSessions []protocol.Session, roomSessions []protocol.Session) []protocol.Session {
	merged := make(map[string]protocol.Session, len(fileSessions)+len(roomSessions))
	roomByKey := make(map[string]protocol.Session, len(roomSessions))
	for _, item := range roomSessions {
		normalized := normalizeSession(item)
		roomByKey[normalized.SessionKey] = normalized
		merged[normalized.SessionKey] = normalized
	}
	for _, item := range fileSessions {
		normalized := normalizeSession(item)
		if protocol.IsRoomSharedSessionKey(normalized.SessionKey) {
			continue
		}
		if roomSession, ok := roomByKey[normalized.SessionKey]; ok {
			// SQL owns Room identity/title/configuration. The workspace projection owns
			// runtime progress such as message_count and transcript lineage.
			merged[normalized.SessionKey] = normalizeSession(
				dmdomain.MergeRoomBackedSession(normalized, roomSession),
			)
			continue
		}
		if normalized.RoomSessionID != nil && strings.TrimSpace(*normalized.RoomSessionID) != "" {
			// A deleted Room may leave a recoverable workspace artifact. It must not
			// reappear in the public directory without its authoritative SQL row.
			continue
		}
		merged[normalized.SessionKey] = normalized
	}

	result := make([]protocol.Session, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	// 同秒级时间戳的成员 session 很常见（同事务写入）；排序必须确定，
	// 否则“每个 room 的第一条 session”这类消费口径会在刷新间随机漂移。
	slices.SortFunc(result, func(left protocol.Session, right protocol.Session) int {
		if compared := right.LastActivity.Compare(left.LastActivity); compared != 0 {
			return compared
		}
		return strings.Compare(left.SessionKey, right.SessionKey)
	})
	return result
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func shouldHideWorkspaceSession(item protocol.Session) bool {
	if protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return true
	}
	if protocol.SessionIsHiddenFromDirectory(item) {
		return true
	}
	return item.RoomSessionID != nil && strings.TrimSpace(*item.RoomSessionID) != ""
}

func normalizeSession(item protocol.Session) protocol.Session {
	if item.Options == nil {
		item.Options = map[string]any{}
	}
	if item.Title == "" {
		item.Title = "New Chat"
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.ChannelType == "" {
		item.ChannelType = "websocket"
	}
	if item.ChatType == "" {
		item.ChatType = "dm"
	}
	if item.LastActivity.IsZero() {
		item.LastActivity = item.CreatedAt
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.LastActivity = item.LastActivity.UTC()
	item.IsActive = item.Status == "active"
	if item.ConfigurationVersion < 1 {
		item.ConfigurationVersion = 1
	}
	return item
}
