package protocol

import sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"

// 会话键的权威定义已迁移至 internal/model/session/session_key.go。
// 这里仅保留 re-export shim，避免一次性改 100+ 处调用。新代码请直接 import
// `github.com/nexus-research-lab/nexus/internal/model/session` 使用。

const (
	SessionChannelWebSocketSegment = sessionmodel.SessionChannelWebSocketSegment
	SessionChannelDiscordSegment   = sessionmodel.SessionChannelDiscordSegment
	SessionChannelTelegramSegment  = sessionmodel.SessionChannelTelegramSegment
	SessionChannelInternalSegment  = sessionmodel.SessionChannelInternalSegment

	SessionChannelWebSocket = sessionmodel.SessionChannelWebSocket
	SessionChannelDiscord   = sessionmodel.SessionChannelDiscord
	SessionChannelTelegram  = sessionmodel.SessionChannelTelegram
)

type (
	SessionKeyKind            = sessionmodel.SessionKeyKind
	SessionKey                = sessionmodel.SessionKey
	StructuredSessionKeyError = sessionmodel.StructuredSessionKeyError
)

const (
	SessionKeyKindAgent   = sessionmodel.SessionKeyKindAgent
	SessionKeyKindRoom    = sessionmodel.SessionKeyKindRoom
	SessionKeyKindUnknown = sessionmodel.SessionKeyKindUnknown
)

var ErrInvalidSessionKey = sessionmodel.ErrInvalidSessionKey

var (
	GetSessionKeyValidationError      = sessionmodel.GetSessionKeyValidationError
	IsStructuredSessionKey            = sessionmodel.IsStructuredSessionKey
	RequireStructuredSessionKey       = sessionmodel.RequireStructuredSessionKey
	ParseSessionKey                   = sessionmodel.ParseSessionKey
	BuildAgentSessionKey              = sessionmodel.BuildAgentSessionKey
	BuildRoomSharedSessionKey         = sessionmodel.BuildRoomSharedSessionKey
	BuildRoomSessionKey               = sessionmodel.BuildRoomSessionKey
	BuildRoomAgentSessionKey          = sessionmodel.BuildRoomAgentSessionKey
	IsRoomSharedSessionKey            = sessionmodel.IsRoomSharedSessionKey
	ParseRoomConversationID           = sessionmodel.ParseRoomConversationID
	NormalizeSessionKeyChannelSegment = sessionmodel.NormalizeSessionKeyChannelSegment
	NormalizeStoredChannelType        = sessionmodel.NormalizeStoredChannelType
	NormalizeSessionChatType          = sessionmodel.NormalizeSessionChatType
)
