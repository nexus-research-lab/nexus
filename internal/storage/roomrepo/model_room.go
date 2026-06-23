package roomrepo

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
)

// AgentRuntimeRef 表示为房间创建会话时所需的 Agent 运行时信息。
type AgentRuntimeRef = roomdomain.AgentRuntimeRef

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

// UpdateRoomPatch 表示房间资料的可选更新字段。
type UpdateRoomPatch struct {
	Name                   *string
	Description            *string
	Title                  *string
	Avatar                 *string
	SkillNames             *[]string
	HostAgentID            *string
	HostAutoReplyEnabled   *bool
	PrivateMessagesEnabled *bool
}

// RoomColumnUpdate 表示一列安全白名单内的 room 表更新。
type RoomColumnUpdate struct {
	Column string
	Value  any
}

func (patch UpdateRoomPatch) RoomColumnUpdates() []RoomColumnUpdate {
	updates := make([]RoomColumnUpdate, 0, 7)
	if patch.Name != nil {
		updates = append(updates, RoomColumnUpdate{Column: "name", Value: NullIfEmpty(*patch.Name)})
	}
	if patch.Description != nil {
		updates = append(updates, RoomColumnUpdate{Column: "description", Value: *patch.Description})
	}
	if patch.Avatar != nil {
		updates = append(updates, RoomColumnUpdate{Column: "avatar", Value: NullIfEmpty(*patch.Avatar)})
	}
	if patch.SkillNames != nil {
		updates = append(updates, RoomColumnUpdate{Column: "skill_names", Value: jsoncodec.MarshalStringSlice(*patch.SkillNames)})
	}
	if patch.HostAgentID != nil {
		updates = append(updates, RoomColumnUpdate{Column: "host_agent_id", Value: NullIfEmpty(*patch.HostAgentID)})
	}
	if patch.HostAutoReplyEnabled != nil {
		updates = append(updates, RoomColumnUpdate{Column: "host_auto_reply_enabled", Value: *patch.HostAutoReplyEnabled})
	}
	if patch.PrivateMessagesEnabled != nil {
		updates = append(updates, RoomColumnUpdate{Column: "private_messages_enabled", Value: *patch.PrivateMessagesEnabled})
	}
	return updates
}

func NullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// NewEntityID 生成 room 相关实体 ID。
func NewEntityID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
