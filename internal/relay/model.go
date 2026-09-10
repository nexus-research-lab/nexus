// INPUT: Nexus Relay `/api/relay/v1` JSON envelope 与分页字段。
// OUTPUT: Nexus 不依赖 Relay 仓库即可消费的稳定 Relay wire DTO。
// POS: Relay 客户端、同步服务与本地投影共用的独立跨仓协议合同。
package relay

import (
	"fmt"
	"time"
)

const (
	// ContentVersionV1 是首版 Markdown block 正文版本。
	ContentVersionV1 = 1
	// BlockTypeMarkdown 是 Relay M1 唯一正文 block。
	BlockTypeMarkdown = "markdown"
	// AuthorTypeUser 是 Relay M1 唯一消息作者类型。
	AuthorTypeUser = "user"
)

// ContentBlock 是一段版本化消息正文。
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MessageContent 是 Relay 保存的共享正文。
type MessageContent struct {
	Version int            `json:"version"`
	Blocks  []ContentBlock `json:"blocks"`
}

// Room 是显式创建的在线协作空间。
type Room struct {
	ID                     string    `json:"id"`
	TeamID                 string    `json:"team_id,omitempty"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Avatar                 string    `json:"avatar"`
	CoordinatorAgentID     string    `json:"coordinator_agent_id,omitempty"`
	HostAutoReplyEnabled   bool      `json:"host_auto_reply_enabled"`
	PrivateMessagesEnabled bool      `json:"private_messages_enabled"`
	SkillNames             []string  `json:"skill_names"`
	ConfigurationVersion   int64     `json:"configuration_version"`
	MembershipVersion      int64     `json:"membership_version"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Conversation 是 Relay 消息排序与同步边界。
type Conversation struct {
	ID                    string     `json:"id"`
	RoomID                string     `json:"room_id"`
	Type                  string     `json:"type"`
	HighWaterMessageSeq   int64      `json:"high_water_message_seq"`
	LastActivityAt        *time.Time `json:"last_activity_at"`
	SyncStreamID          string     `json:"sync_stream_id"`
	StreamEpoch           string     `json:"stream_epoch"`
	HighWaterSyncEventSeq int64      `json:"high_water_sync_event_seq"`
}

// RoomView 是当前成员可见的 Room、主 Conversation 和自身角色。
type RoomView struct {
	Room            Room         `json:"room"`
	Conversation    Conversation `json:"conversation"`
	CurrentUserRole string       `json:"current_user_role"`
}

// RoomList 是当前真人的在线 Room 目录。
type RoomList struct {
	Rooms []RoomView `json:"rooms"`
}

// CreateRoomInput 是显式建群请求。
type CreateRoomInput struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	Avatar                 string   `json:"avatar,omitempty"`
	CoordinatorAgentID     string   `json:"coordinator_agent_id,omitempty"`
	HostAutoReplyEnabled   bool     `json:"host_auto_reply_enabled,omitempty"`
	PrivateMessagesEnabled bool     `json:"private_messages_enabled,omitempty"`
	SkillNames             []string `json:"skill_names,omitempty"`
	AgentIDs               []string `json:"agent_ids,omitempty"`
	MemberUserIDs          []string `json:"member_user_ids,omitempty"`
}

// Message 是 Conversation 中只追加一次的真人消息。
type Message struct {
	ID                string         `json:"id"`
	ConversationID    string         `json:"conversation_id"`
	MessageSeq        int64          `json:"message_seq"`
	AuthorType        string         `json:"author_type"`
	AuthorUserID      string         `json:"author_user_id"`
	AuthorUsername    string         `json:"author_username"`
	AuthorDisplayName string         `json:"author_display_name"`
	ClientMessageID   string         `json:"client_message_id"`
	Content           MessageContent `json:"content"`
	CreatedAt         time.Time      `json:"created_at"`
}

// CreateMessageInput 是 Relay M1 消息写入意图。
type CreateMessageInput struct {
	Content MessageContent `json:"content"`
}

// MessageCommit 是 Message 与同步水位的原子提交结果。
type MessageCommit struct {
	Message      Message `json:"message"`
	StreamID     string  `json:"stream_id"`
	StreamEpoch  string  `json:"stream_epoch"`
	EventSeq     int64   `json:"event_seq"`
	HighWaterSeq int64   `json:"high_water_seq"`
	Replayed     bool    `json:"replayed"`
}

// Snapshot 是固定消息上界与同步水位的一页一致性快照。
type Snapshot struct {
	ConversationID    string    `json:"conversation_id"`
	StreamID          string    `json:"stream_id"`
	StreamEpoch       string    `json:"stream_epoch"`
	SnapshotSeq       int64     `json:"snapshot_seq"`
	ThroughMessageSeq int64     `json:"through_message_seq"`
	AfterMessageSeq   int64     `json:"after_message_seq"`
	NextMessageSeq    int64     `json:"next_message_seq"`
	HasMore           bool      `json:"has_more"`
	Messages          []Message `json:"messages"`
}

// SyncEvent 是可由 current-state Message 重建的同步索引事件。
type SyncEvent struct {
	EventSeq int64   `json:"event_seq"`
	Type     string  `json:"type"`
	Message  Message `json:"message"`
}

// Difference 是 Conversation stream 的连续增量。
type Difference struct {
	StreamID       string      `json:"stream_id"`
	StreamEpoch    string      `json:"stream_epoch"`
	AfterSeq       int64       `json:"after_seq"`
	NextSeq        int64       `json:"next_seq"`
	HighWaterSeq   int64       `json:"high_water_seq"`
	MinRetainedSeq int64       `json:"min_retained_seq"`
	HasMore        bool        `json:"has_more"`
	Events         []SyncEvent `json:"events"`
}

// StreamUpdated 是 Relay WSS 发出的已提交水位提示；消息恢复仍走 Difference。
type StreamUpdated struct {
	Type         string `json:"type"`
	StreamID     string `json:"stream_id"`
	StreamEpoch  string `json:"stream_epoch"`
	HighWaterSeq int64  `json:"high_water_seq"`
}

// SnapshotOptions 描述一页固定上界的快照请求。
type SnapshotOptions struct {
	AfterMessageSeq   int64
	Limit             int
	ThroughMessageSeq *int64
	SnapshotSeq       *int64
	StreamEpoch       string
}

// DifferenceOptions 描述一页增量同步请求。
type DifferenceOptions struct {
	AfterSeq    int64
	Limit       int
	StreamEpoch string
}

// RemoteError 是 Relay 返回的结构化失败。
type RemoteError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *RemoteError) Error() string {
	if e == nil {
		return "Relay 请求失败"
	}
	return fmt.Sprintf("Relay 请求失败: %s (%s)", e.Message, e.Code)
}
