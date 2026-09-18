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
	RoomID        string `json:"room_id,omitempty"`
	InviteeUserID string `json:"invitee_user_id,omitempty"`
	InvitedAt     string `json:"invited_at,omitempty"`
	Type          string `json:"type"`
	Text          string `json:"text"`
}

// MessageContent 是 Relay 保存的共享正文。
type MessageContent struct {
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	Version     int                 `json:"version"`
	Blocks      []ContentBlock      `json:"blocks"`
	Execution   *ExecutionMetadata  `json:"execution,omitempty"`
}

// MessageAttachment 仅引用已持久化的群文件，不接受本机路径或远程 URL。
type MessageAttachment struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// ExecutionMetadata 只共享回复统计，不包含私人记忆、路径、工具输入和运行凭据。
type ExecutionMetadata struct {
	Model         string                  `json:"model,omitempty"`
	ResultSummary *ExecutionResultSummary `json:"result_summary,omitempty"`
}

type ExecutionResultSummary struct {
	DurationMS    float64         `json:"duration_ms"`
	DurationAPIMS float64         `json:"duration_api_ms"`
	NumTurns      int64           `json:"num_turns"`
	TotalCostUSD  *float64        `json:"total_cost_usd,omitempty"`
	Usage         *ExecutionUsage `json:"usage,omitempty"`
}

type ExecutionUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
}

// MessageMention 是消息中经过 Relay 校验的结构化 Agent 目标。
type MessageMention struct {
	MemberType string `json:"member_type"`
	MemberID   string `json:"member_id"`
}

// Room 是显式创建的在线协作空间。
type Room struct {
	DirectUserID           string    `json:"direct_user_id,omitempty"`
	ID                     string    `json:"id"`
	OrganizationID         string    `json:"organization_id"`
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

// RoomMember 是真人和 Agent 共用的在线 Room 成员记录。
type RoomMember struct {
	RoomID           string     `json:"room_id"`
	Type             string     `json:"member_type"`
	ID               string     `json:"member_id"`
	Role             string     `json:"role"`
	State            string     `json:"state"`
	AgentOwnerUserID string     `json:"agent_owner_user_id,omitempty"`
	AgentPaused      bool       `json:"agent_paused,omitempty"`
	InvitedByUserID  string     `json:"invited_by_user_id"`
	JoinedAt         *time.Time `json:"joined_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// RoomDetails 是在线 Room 管理使用的成员快照。
type RoomDetails struct {
	RoomView
	Members    []RoomMember     `json:"members"`
	Deliveries []DeliveryStatus `json:"deliveries"`
}

// DeliveryStatus 是所有群成员可见的最小进度，不包含本机执行信息。
type DeliveryStatus struct {
	ID          string `json:"id"`
	MessageID   string `json:"message_id"`
	AgentID     string `json:"agent_id"`
	State       string `json:"state"`
	FailureCode string `json:"failure_code,omitempty"`
}

// RoomInvitation 是当前真人尚未处理的在线 Room 邀请。
type RoomInvitation struct {
	Room            Room      `json:"room"`
	InvitedByUserID string    `json:"invited_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoomInvitationList 是当前真人的待处理邀请。
type RoomInvitationList struct {
	RecoveryRooms []RoomRecovery   `json:"recovery_rooms"`
	Invitations   []RoomInvitation `json:"invitations"`
}

// RoomRecovery 只提供待接管群的治理信息，不授予消息读取权限。
type RoomRecovery struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	MembershipVersion int64  `json:"membership_version"`
}

// RoomMembershipMutation 是成员治理命令的确定结果。
type RoomMembershipMutation struct {
	RoomID            string `json:"room_id"`
	MembershipVersion int64  `json:"membership_version"`
	Replayed          bool   `json:"replayed"`
}

// RoomConfigurationMutation 是 Room 配置命令的确定结果。
type RoomConfigurationMutation struct {
	RoomID               string `json:"room_id"`
	ConfigurationVersion int64  `json:"configuration_version"`
	Replayed             bool   `json:"replayed"`
}

// CreateRoomInput 是显式建群请求。
type CreateRoomInput struct {
	DirectUserID           string   `json:"direct_user_id,omitempty"`
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	Avatar                 string   `json:"avatar,omitempty"`
	PrivateMessagesEnabled bool     `json:"private_messages_enabled,omitempty"`
	SkillNames             []string `json:"skill_names,omitempty"`
	MemberUserIDs          []string `json:"member_user_ids,omitempty"`
	AgentIDs               []string `json:"agent_ids,omitempty"`
	CoordinatorAgentID     string   `json:"coordinator_agent_id,omitempty"`
}

// AddRoomAgentInput 将当前真人拥有的 Control Agent 加入 Room。
type AddRoomAgentInput struct {
	AgentID                   string `json:"agent_id"`
	ExpectedMembershipVersion int64  `json:"expected_membership_version"`
}

// RemoveRoomAgentInput 将 Agent 成员移出 Room。
type RemoveRoomAgentInput struct {
	ExpectedMembershipVersion int64 `json:"expected_membership_version"`
}

// UpdateRoomAgentInput 暂停或恢复当前真人拥有的 Agent。
type UpdateRoomAgentInput struct {
	Paused                    bool  `json:"paused"`
	ExpectedMembershipVersion int64 `json:"expected_membership_version"`
}

// UpdateRoomInput 更新群资料、主持 Agent，或显式解散群。
type UpdateRoomInput struct {
	HideDirect                   bool    `json:"hide_direct,omitempty"`
	Dissolve                     bool    `json:"dissolve,omitempty"`
	CoordinatorAgentID           *string `json:"coordinator_agent_id,omitempty"`
	Name                         *string `json:"name,omitempty"`
	Avatar                       *string `json:"avatar,omitempty"`
	ExpectedConfigurationVersion int64   `json:"expected_configuration_version"`
}

// InviteRoomMemberInput 邀请一名 Organization 真人。
type InviteRoomMemberInput struct {
	UserID                    string `json:"user_id"`
	ExpectedMembershipVersion int64  `json:"expected_membership_version"`
}

// ResolveRoomInvitationInput 接受、拒绝或撤销一条邀请。
type ResolveRoomInvitationInput struct {
	ExpectedMembershipVersion int64 `json:"expected_membership_version"`
}

// UpdateRoomMemberInput 修改真人成员角色或移除成员。
type UpdateRoomMemberInput struct {
	Role                      string `json:"role,omitempty"`
	Remove                    bool   `json:"remove,omitempty"`
	ExpectedMembershipVersion int64  `json:"expected_membership_version"`
}

// TransferRoomOwnershipInput 移交唯一真人群主。
type TransferRoomOwnershipInput struct {
	Takeover                  bool   `json:"takeover,omitempty"`
	NewOwnerUserID            string `json:"new_owner_user_id"`
	ExpectedMembershipVersion int64  `json:"expected_membership_version"`
}

// Message 是 Conversation 中只追加一次的真人消息或完整 Agent 回复。
type Message struct {
	ID                string           `json:"id"`
	ConversationID    string           `json:"conversation_id"`
	MessageSeq        int64            `json:"message_seq"`
	AuthorType        string           `json:"author_type"`
	AuthorUserID      string           `json:"author_user_id"`
	AuthorAgentID     string           `json:"author_agent_id,omitempty"`
	DeliveryID        string           `json:"delivery_id,omitempty"`
	OutputKind        string           `json:"output_kind,omitempty"`
	AuthorUsername    string           `json:"author_username"`
	AuthorDisplayName string           `json:"author_display_name"`
	ClientMessageID   string           `json:"client_message_id"`
	Content           MessageContent   `json:"content"`
	Mentions          []MessageMention `json:"mentions"`
	CreatedAt         time.Time        `json:"created_at"`
}

// CreateMessageInput 是 Relay M1 消息写入意图。
type CreateMessageInput struct {
	Content                   MessageContent   `json:"content"`
	Mentions                  []MessageMention `json:"mentions,omitempty"`
	ExpectedMembershipVersion int64            `json:"expected_membership_version,omitempty"`
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
