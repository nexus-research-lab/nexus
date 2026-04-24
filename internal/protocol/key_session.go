package protocol

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// SessionChannelWebSocketSegment 表示 session_key 中的 WebSocket 通道段。
	SessionChannelWebSocketSegment = "ws"
	// SessionChannelDiscordSegment 表示 session_key 中的 Discord 通道段。
	SessionChannelDiscordSegment = "dg"
	// SessionChannelTelegramSegment 表示 session_key 中的 Telegram 通道段。
	SessionChannelTelegramSegment = "tg"
	// SessionChannelInternalSegment 表示 session_key 中的内部通道段。
	SessionChannelInternalSegment = "internal"

	// SessionChannelWebSocket 表示持久化后的 WebSocket 通道类型。
	SessionChannelWebSocket = "websocket"
	// SessionChannelDiscord 表示持久化后的 Discord 通道类型。
	SessionChannelDiscord = "discord"
	// SessionChannelTelegram 表示持久化后的 Telegram 通道类型。
	SessionChannelTelegram = "telegram"
)

// SessionKeyKind 表示协议族。
type SessionKeyKind string

const (
	// SessionKeyKindAgent 表示 agent 私有运行时。
	SessionKeyKindAgent SessionKeyKind = "agent"
	// SessionKeyKindRoom 表示共享 room 流。
	SessionKeyKindRoom SessionKeyKind = "room"
	// SessionKeyKindUnknown 表示无法识别。
	SessionKeyKindUnknown SessionKeyKind = "unknown"

	roomSharedChatType = "group"
	topicSegment       = "topic"
)

// SessionKey 表示结构化会话键。
type SessionKey struct {
	Raw            string         `json:"raw"`
	Kind           SessionKeyKind `json:"kind"`
	IsStructured   bool           `json:"is_structured"`
	IsShared       bool           `json:"is_shared"`
	AgentID        string         `json:"agent_id,omitempty"`
	Channel        string         `json:"channel,omitempty"`
	ChatType       string         `json:"chat_type,omitempty"`
	Ref            string         `json:"ref,omitempty"`
	ThreadID       string         `json:"thread_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	RoomRef        string         `json:"room_ref,omitempty"`
}

// ErrInvalidSessionKey 表示 session_key 不符合结构化协议。
var ErrInvalidSessionKey = errors.New("invalid structured session_key")

// StructuredSessionKeyError 对齐前端网关的 422 校验错误。
type StructuredSessionKeyError struct {
	Message string
}

func (e StructuredSessionKeyError) Error() string {
	return e.Message
}

func findTopicIndex(parts []string) int {
	for index, value := range parts {
		if value == topicSegment && index >= 4 {
			return index
		}
	}
	return -1
}

// GetSessionKeyValidationError 返回结构化 session_key 校验错误。
func GetSessionKeyValidationError(raw string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "session_key is required"
	}

	if strings.HasPrefix(normalized, string(SessionKeyKindAgent)+":") {
		parts := strings.Split(normalized, ":")
		if len(parts) < 5 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" || strings.TrimSpace(parts[3]) == "" {
			return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"
		}

		topicIndex := findTopicIndex(parts)
		if topicIndex >= 0 {
			ref := strings.TrimSpace(strings.Join(parts[4:topicIndex], ":"))
			threadID := strings.TrimSpace(strings.Join(parts[topicIndex+1:], ":"))
			if ref == "" || threadID == "" {
				return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"
			}
			return ""
		}

		if strings.TrimSpace(strings.Join(parts[4:], ":")) == "" {
			return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"
		}
		return ""
	}

	if strings.HasPrefix(normalized, string(SessionKeyKindRoom)+":") {
		parts := strings.Split(normalized, ":")
		conversationID := ""
		if len(parts) > 2 {
			conversationID = strings.TrimSpace(strings.Join(parts[2:], ":"))
		}
		if len(parts) < 3 || parts[1] != roomSharedChatType || conversationID == "" {
			return "session_key must match room:group:<conversation_id>"
		}
		return ""
	}

	return "session_key must use structured gateway format"
}

// IsStructuredSessionKey 判断是否合法。
func IsStructuredSessionKey(raw string) bool {
	return GetSessionKeyValidationError(raw) == ""
}

// RequireStructuredSessionKey 要求必须是结构化 session_key。
func RequireStructuredSessionKey(raw string) (string, error) {
	if message := GetSessionKeyValidationError(raw); message != "" {
		return "", StructuredSessionKeyError{Message: message}
	}
	return strings.TrimSpace(raw), nil
}

// ParseSessionKey 解析 session_key。
func ParseSessionKey(raw string) SessionKey {
	normalized := strings.TrimSpace(raw)
	validationError := GetSessionKeyValidationError(normalized)
	result := SessionKey{
		Raw:          normalized,
		Kind:         SessionKeyKindUnknown,
		IsStructured: validationError == "",
	}

	if strings.HasPrefix(normalized, string(SessionKeyKindAgent)+":") {
		parts := strings.Split(normalized, ":")
		result.Kind = SessionKeyKindAgent
		if len(parts) > 1 {
			result.AgentID = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			result.Channel = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
			result.ChatType = strings.TrimSpace(parts[3])
		} else {
			result.ChatType = "dm"
		}

		// `:topic:` 是保留边界，ref 允许带冒号，但不能跨过这个边界。
		topicIndex := findTopicIndex(parts)
		if topicIndex >= 0 {
			result.Ref = strings.TrimSpace(strings.Join(parts[4:topicIndex], ":"))
			result.ThreadID = strings.TrimSpace(strings.Join(parts[topicIndex+1:], ":"))
			return result
		}

		if len(parts) > 4 {
			result.Ref = strings.TrimSpace(strings.Join(parts[4:], ":"))
		}
		return result
	}

	if strings.HasPrefix(normalized, string(SessionKeyKindRoom)+":") {
		parts := strings.Split(normalized, ":")
		conversationID := ""
		if len(parts) > 2 {
			conversationID = strings.TrimSpace(strings.Join(parts[2:], ":"))
		}
		result.Kind = SessionKeyKindRoom
		result.IsShared = validationError == ""
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			result.ChatType = strings.TrimSpace(parts[1])
		} else {
			result.ChatType = roomSharedChatType
		}
		result.Ref = conversationID
		result.RoomRef = conversationID
		result.ConversationID = conversationID
		return result
	}

	return result
}

// BuildAgentSessionKey 构建 agent 作用域 key。
func BuildAgentSessionKey(agentID string, channel string, chatType string, ref string, threadID string) string {
	base := fmt.Sprintf(
		"agent:%s:%s:%s:%s",
		agentID,
		NormalizeSessionKeyChannelSegment(channel),
		NormalizeSessionChatType(chatType),
		ref,
	)
	if threadID == "" {
		return base
	}
	return base + ":" + topicSegment + ":" + threadID
}

// BuildRoomSharedSessionKey 构建共享 room key。
func BuildRoomSharedSessionKey(conversationID string) string {
	return "room:" + roomSharedChatType + ":" + strings.TrimSpace(conversationID)
}

// BuildRoomAgentSessionKey 构建 Room 成员侧的 agent session_key。
func BuildRoomAgentSessionKey(conversationID string, agentID string, roomType string) string {
	chatType := "group"
	if strings.TrimSpace(roomType) == "dm" {
		chatType = "dm"
	}
	return BuildAgentSessionKey(strings.TrimSpace(agentID), SessionChannelWebSocketSegment, chatType, strings.TrimSpace(conversationID), "")
}

// IsRoomSharedSessionKey 判断是否为 Room 共享消息流 key。
func IsRoomSharedSessionKey(raw string) bool {
	parsed := ParseSessionKey(raw)
	return parsed.Kind == SessionKeyKindRoom && parsed.IsStructured && parsed.ConversationID != ""
}

// ParseRoomConversationID 读取 Room 共享流里的 conversation_id。
func ParseRoomConversationID(raw string) string {
	parsed := ParseSessionKey(raw)
	if parsed.Kind != SessionKeyKindRoom {
		return ""
	}
	return parsed.ConversationID
}

// NormalizeSessionKeyChannelSegment 把外部输入统一成 session_key 使用的 channel 段。
func NormalizeSessionKeyChannelSegment(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case SessionChannelWebSocketSegment, SessionChannelWebSocket:
		return SessionChannelWebSocketSegment
	case SessionChannelDiscordSegment, SessionChannelDiscord:
		return SessionChannelDiscordSegment
	case SessionChannelTelegramSegment, SessionChannelTelegram:
		return SessionChannelTelegramSegment
	case SessionChannelInternalSegment:
		return SessionChannelInternalSegment
	default:
		return strings.TrimSpace(channel)
	}
}

// NormalizeStoredChannelType 把 channel 归一成持久化和运行时使用的名称。
func NormalizeStoredChannelType(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case SessionChannelWebSocketSegment, SessionChannelWebSocket:
		return SessionChannelWebSocket
	case SessionChannelDiscordSegment, SessionChannelDiscord:
		return SessionChannelDiscord
	case SessionChannelTelegramSegment, SessionChannelTelegram:
		return SessionChannelTelegram
	case SessionChannelInternalSegment:
		return SessionChannelInternalSegment
	default:
		return strings.TrimSpace(channel)
	}
}

// NormalizeSessionChatType 统一 chat_type 的默认值和枚举。
func NormalizeSessionChatType(chatType string) string {
	switch strings.ToLower(strings.TrimSpace(chatType)) {
	case "", "dm":
		return "dm"
	case "group":
		return "group"
	default:
		return strings.TrimSpace(chatType)
	}
}
