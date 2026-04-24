package room

import "github.com/nexus-research-lab/nexus/internal/protocol"

const (
	// RoomTypeDM 表示单成员直聊房间。
	RoomTypeDM = protocol.RoomTypeDM
	// RoomTypeGroup 表示多人协作房间。
	RoomTypeGroup = protocol.RoomTypeGroup
	// ConversationTypeDM 表示 DM 主对话。
	ConversationTypeDM = protocol.ConversationTypeDM
	// ConversationTypeMain 表示 Room 主对话。
	ConversationTypeMain = protocol.ConversationTypeMain
	// ConversationTypeTopic 表示 Room 话题对话。
	ConversationTypeTopic = protocol.ConversationTypeTopic
	// MemberTypeUser 表示用户成员。
	MemberTypeUser = protocol.MemberTypeUser
	// MemberTypeAgent 表示 Agent 成员。
	MemberTypeAgent = protocol.MemberTypeAgent
)

// MemberRecord 表示房间成员记录。
type MemberRecord = protocol.MemberRecord

// RoomRecord 表示房间记录。
type RoomRecord = protocol.RoomRecord

// RoomAggregate 表示房间聚合。
type RoomAggregate = protocol.RoomAggregate

// ConversationRecord 表示房间对话记录。
type ConversationRecord = protocol.ConversationRecord

// SessionRecord 表示房间内的运行时会话索引。
type SessionRecord = protocol.SessionRecord

// ConversationContextAggregate 表示房间对话上下文聚合。
type ConversationContextAggregate = protocol.ConversationContextAggregate

// AgentRuntimeRef 表示为房间创建会话时所需的 Agent 运行时信息。
type AgentRuntimeRef = protocol.AgentRuntimeRef

// CreateRoomBundle 表示创建房间时一次性写入的数据。
type CreateRoomBundle = protocol.CreateRoomBundle

// CreateConversationBundle 表示创建话题时一次性写入的数据。
type CreateConversationBundle = protocol.CreateConversationBundle
