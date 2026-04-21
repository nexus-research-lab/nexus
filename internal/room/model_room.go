package room

import roommodel "github.com/nexus-research-lab/nexus/internal/model/room"

const (
	// RoomTypeDM 表示单成员直聊房间。
	RoomTypeDM = roommodel.RoomTypeDM
	// RoomTypeGroup 表示多人协作房间。
	RoomTypeGroup = roommodel.RoomTypeGroup
	// ConversationTypeDM 表示 DM 主对话。
	ConversationTypeDM = roommodel.ConversationTypeDM
	// ConversationTypeMain 表示 Room 主对话。
	ConversationTypeMain = roommodel.ConversationTypeMain
	// ConversationTypeTopic 表示 Room 话题对话。
	ConversationTypeTopic = roommodel.ConversationTypeTopic
	// MemberTypeUser 表示用户成员。
	MemberTypeUser = roommodel.MemberTypeUser
	// MemberTypeAgent 表示 Agent 成员。
	MemberTypeAgent = roommodel.MemberTypeAgent
)

// MemberRecord 表示房间成员记录。
type MemberRecord = roommodel.MemberRecord

// RoomRecord 表示房间记录。
type RoomRecord = roommodel.RoomRecord

// RoomAggregate 表示房间聚合。
type RoomAggregate = roommodel.RoomAggregate

// ConversationRecord 表示房间对话记录。
type ConversationRecord = roommodel.ConversationRecord

// SessionRecord 表示房间内的运行时会话索引。
type SessionRecord = roommodel.SessionRecord

// ConversationContextAggregate 表示房间对话上下文聚合。
type ConversationContextAggregate = roommodel.ConversationContextAggregate

// AgentRuntimeRef 表示为房间创建会话时所需的 Agent 运行时信息。
type AgentRuntimeRef = roommodel.AgentRuntimeRef

// CreateRoomBundle 表示创建房间时一次性写入的数据。
type CreateRoomBundle = roommodel.CreateRoomBundle

// CreateConversationBundle 表示创建话题时一次性写入的数据。
type CreateConversationBundle = roommodel.CreateConversationBundle
