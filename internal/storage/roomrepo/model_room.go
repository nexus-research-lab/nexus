package roomrepo

import "github.com/nexus-research-lab/nexus/internal/protocol"

// AgentRuntimeRef 表示为房间创建会话时所需的 Agent 运行时信息。
type AgentRuntimeRef struct {
	AgentID     string
	Name        string
	DisplayName string
	RuntimeID   string
	Status      string
}

// CreateRoomBundle 表示创建房间时一次性写入的数据。
type CreateRoomBundle struct {
	Room         protocol.RoomRecord
	Members      []protocol.MemberRecord
	Conversation protocol.ConversationRecord
	Sessions     []protocol.SessionRecord
}

// CreateConversationBundle 表示创建话题时一次性写入的数据。
type CreateConversationBundle struct {
	RoomID       string
	Conversation protocol.ConversationRecord
	Sessions     []protocol.SessionRecord
}
