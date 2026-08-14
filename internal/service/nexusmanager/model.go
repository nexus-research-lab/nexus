// INPUT: runtime 注入的可信身份与 Nexus 资源查询/创建参数。
// OUTPUT: 不含 workspace 绝对路径、runtime/SDK ID、Agent options 或凭据的稳定模型。
// POS: nexus_manager 服务的跨 transport 数据契约。
package nexusmanager

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	ContextKindAgent = "agent"
	ContextKindRoom  = "room"

	AuthorityOwnerMain  = "owner_main"
	AuthorityAgentSelf  = "agent_self"
	AuthorityRoomMember = "room_member"
)

// Actor 是 server 根据当前 runtime 固化的调用身份；字段不能来自模型工具参数。
type Actor struct {
	OwnerUserID     string
	AgentID         string
	SessionKey      string
	RoundID         string
	LeaseSessionKey string
	LeaseRoundID    string
	ContextKind     string
	ContextID       string
	RoomID          string
	ConversationID  string
	// GoalCollaborationBinding 由可信宿主动态读取当前 exact Goal/revision。
	// communication 只把返回值用于协作归因，绝不把它传播为目标 round capability。
	GoalCollaborationBinding func() *protocol.GoalCollaborationBinding
}

// CapabilitySnapshot 明确当前上下文实际获得的能力和被排除的高风险边界。
type CapabilitySnapshot struct {
	Authority      string   `json:"authority"`
	ContextKind    string   `json:"context_kind"`
	ContextID      string   `json:"context_id"`
	ReadOperations []string `json:"read_operations"`
	Excluded       []string `json:"excluded"`
}

// AgentView 是 Agent 的脱敏目录投影。
type AgentView struct {
	AgentID        string    `json:"agent_id"`
	Name           string    `json:"name"`
	IsMain         bool      `json:"is_main"`
	DisplayName    string    `json:"display_name,omitempty"`
	Headline       string    `json:"headline,omitempty"`
	Status         string    `json:"status"`
	Avatar         string    `json:"avatar,omitempty"`
	Description    string    `json:"description,omitempty"`
	VibeTags       []string  `json:"vibe_tags,omitempty"`
	SkillsCount    int       `json:"skills_count"`
	RuntimeVersion int64     `json:"runtime_version"`
	CreatedAt      time.Time `json:"created_at"`
}

// RoomView 是 Room 及其 Agent 成员的脱敏投影。
type RoomView struct {
	ID                     string    `json:"id"`
	RoomType               string    `json:"room_type"`
	Name                   string    `json:"name,omitempty"`
	Description            string    `json:"description,omitempty"`
	Avatar                 string    `json:"avatar,omitempty"`
	SkillNames             []string  `json:"skill_names,omitempty"`
	HostAgentID            string    `json:"host_agent_id,omitempty"`
	HostAutoReplyEnabled   bool      `json:"host_auto_reply_enabled"`
	PrivateMessagesEnabled bool      `json:"private_messages_enabled"`
	ConfigurationVersion   int64     `json:"configuration_version"`
	AuthorityEpoch         int64     `json:"authority_epoch"`
	MemberAgentIDs         []string  `json:"member_agent_ids"`
	CreatedAt              time.Time `json:"created_at,omitempty"`
	UpdatedAt              time.Time `json:"updated_at,omitempty"`
}

// ConversationView 是 Room conversation 的公开业务状态。
type ConversationView struct {
	ID               string    `json:"id"`
	RoomID           string    `json:"room_id"`
	ConversationType string    `json:"conversation_type"`
	Title            string    `json:"title,omitempty"`
	IsDraft          bool      `json:"is_draft"`
	MessageCount     int       `json:"message_count"`
	LastActivityAt   time.Time `json:"last_activity_at,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// RoomRuntimeView 只保留定位当前成员执行状态所需的信息。
type RoomRuntimeView struct {
	AgentID        string    `json:"agent_id"`
	VersionNo      int       `json:"version_no"`
	IsPrimary      bool      `json:"is_primary"`
	Status         string    `json:"status"`
	LastActivityAt time.Time `json:"last_activity_at,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ConversationContextView 是 conversation、成员和运行状态的脱敏聚合。
type ConversationContextView struct {
	Room         RoomView          `json:"room"`
	Conversation ConversationView  `json:"conversation"`
	MemberAgents []AgentView       `json:"member_agents"`
	Sessions     []RoomRuntimeView `json:"sessions"`
}

// SessionView 是统一 session 目录投影；不返回内部 DB/SDK session ID 或 options。
type SessionView struct {
	SessionKey     string    `json:"session_key"`
	AgentID        string    `json:"agent_id"`
	RoomID         string    `json:"room_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	ChannelType    string    `json:"channel_type"`
	ChatType       string    `json:"chat_type"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivity   time.Time `json:"last_activity"`
	Title          string    `json:"title,omitempty"`
	MessageCount   int       `json:"message_count"`
	IsActive       bool      `json:"is_active"`
}

// WorkspaceListing 是有界的 workspace 文件目录。
type WorkspaceListing struct {
	AgentID   string           `json:"agent_id"`
	Items     []WorkspaceEntry `json:"items"`
	Truncated bool             `json:"truncated"`
}

// WorkspaceEntry 是 workspace 文件浏览器条目的稳定投影。
type WorkspaceEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       *int64 `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at"`
	Depth      int    `json:"depth"`
}

// WorkspaceFileView 是有界文件内容；truncated=true 时必须继续通过原生文件工具读取。
type WorkspaceFileView struct {
	AgentID    string `json:"agent_id"`
	Path       string `json:"path"`
	Content    string `json:"content"`
	TotalBytes int    `json:"total_bytes"`
	Truncated  bool   `json:"truncated"`
}
