package protocol

import (
	"time"
)

const (
	// RoomTypeDM 表示单成员直聊房间。
	RoomTypeDM = "dm"
	// RoomTypeGroup 表示多人协作房间。
	RoomTypeGroup = "room"
	// ConversationTypeDM 表示 DM 主对话。
	ConversationTypeDM = "dm"
	// ConversationTypeMain 表示 Room 主对话。
	ConversationTypeMain = "room_main"
	// ConversationTypeTopic 表示 Room 话题对话。
	ConversationTypeTopic = "topic"
	// MemberTypeUser 表示用户成员。
	MemberTypeUser = "user"
	// MemberTypeAgent 表示 Agent 成员。
	MemberTypeAgent = "agent"
)

// MemberRecord 表示房间成员记录。
type MemberRecord struct {
	ID            string    `json:"id"`
	RoomID        string    `json:"room_id"`
	MemberType    string    `json:"member_type"`
	MemberUserID  string    `json:"member_user_id,omitempty"`
	MemberAgentID string    `json:"member_agent_id,omitempty"`
	JoinedAt      time.Time `json:"joined_at,omitempty"`
}

// RoomRecord 表示房间记录。
type RoomRecord struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"-"`
	RoomType    string    `json:"room_type"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description"`
	Avatar      string    `json:"avatar,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// RoomAggregate 表示房间聚合。
type RoomAggregate struct {
	Room    RoomRecord     `json:"room"`
	Members []MemberRecord `json:"members"`
}

// ConversationRecord 表示房间对话记录。
type ConversationRecord struct {
	ID               string    `json:"id"`
	RoomID           string    `json:"room_id"`
	ConversationType string    `json:"conversation_type"`
	Title            string    `json:"title,omitempty"`
	MessageCount     int       `json:"message_count"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// SessionRecord 表示房间内的运行时会话索引。
type SessionRecord struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	AgentID        string    `json:"agent_id"`
	RuntimeID      string    `json:"runtime_id"`
	VersionNo      int       `json:"version_no"`
	BranchKey      string    `json:"branch_key"`
	IsPrimary      bool      `json:"is_primary"`
	SDKSessionID   string    `json:"sdk_session_id,omitempty"`
	Status         string    `json:"status"`
	LastActivityAt time.Time `json:"last_activity_at,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ConversationContextAggregate 表示房间对话上下文聚合。
type ConversationContextAggregate struct {
	Room         RoomRecord         `json:"room"`
	Members      []MemberRecord     `json:"members"`
	MemberAgents []Agent            `json:"member_agents,omitempty"`
	Conversation ConversationRecord `json:"conversation"`
	Sessions     []SessionRecord    `json:"sessions"`
}
