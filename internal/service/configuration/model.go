// INPUT: 可信 runtime 身份、Agent/Room 资源 scope 与用户声明的配置变更。
// OUTPUT: 含 authority、access、版本、plan digest、reload 状态与审计的控制面协议。
// POS: configuration 控制面的跨层协议模型。
package configuration

import (
	"encoding/json"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
)

const (
	DomainPreferences = "preferences"
	DomainProviders   = "providers"
	DomainAgents      = "agents"
	DomainEmotion     = "emotion"
	DomainChannels    = "channels"
	DomainConnectors  = "connectors"
	DomainSkills      = "skills"
	DomainHost        = "host"
	DomainAutomation  = "automation"
	DomainSessions    = "sessions"
	DomainRooms       = "rooms"
	DomainWorkspaces  = "workspaces"
	DomainGoals       = "goals"

	ContextKindAgent = "agent"
	ContextKindRoom  = "room"

	AuthorityOwnerMain  = "owner_main"
	AuthorityAgentSelf  = "agent_self"
	AuthorityRoomHost   = "room_host"
	AuthorityRoomMember = "room_member"

	ScopeKindOwner = "owner"
	ScopeKindAgent = "agent"
	ScopeKindRoom  = "room"
)

// Actor 表示一次配置控制调用的可信 runtime 身份。
type Actor struct {
	OwnerUserID string `json:"owner_user_id"`
	AgentID     string `json:"agent_id"`
	SessionKey  string `json:"session_key,omitempty"`
	RoundID     string `json:"round_id,omitempty"`
	// LeaseSessionKey/LeaseRoundID 是 runtime Manager 实际登记的执行槽位。
	// Room 中 SessionKey/RoundID 保持为共享业务会话/root round，二者不得混用。
	LeaseSessionKey string `json:"-"`
	LeaseRoundID    string `json:"-"`
	IsMainAgent     bool   `json:"is_main_agent"`
	ContextKind     string `json:"context_kind,omitempty"`
	ContextID       string `json:"context_id,omitempty"`
	RoomID          string `json:"room_id,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
	SourceContext   string `json:"source_context,omitempty"`
	// PrincipalRole/AuthMethod/LocalSingleUser 由 server transport 固化，
	// 不能来自模型工具参数。它们只决定宿主管理面和公共资源权限，
	// 不改变 Agent/Room 资源所有权。
	PrincipalRole string `json:"-"`
	AuthMethod    string `json:"-"`
	// AuthSessionID 仅由认证 transport 注入。需要真人在场的专用授权能力
	// 必须把它固定到 durable flow；Bearer/runtime principal 不得伪造。
	AuthSessionID   string `json:"-"`
	LocalSingleUser bool   `json:"-"`
	// RoundLeaseRequired 仅由 in-process transport 设置；配置 MCP 的每次调用
	// 都必须仍属于创建该 server 的真实 DM round 或 Room Agent slot。
	RoundLeaseRequired bool `json:"-"`
}

// ScopeRef 表示一次读取或写入绑定的配置资源。
type ScopeRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

// Access 描述当前可信 Actor 对一个配置域的实际权限。
type Access struct {
	Authority         string   `json:"authority"`
	CanRead           bool     `json:"can_read"`
	AllowedOperations []string `json:"allowed_operations"`
	Reason            string   `json:"reason,omitempty"`
}

// OperationDefinition 描述一个配置域可执行的操作。
type OperationDefinition struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	TargetDescription    string   `json:"target_description,omitempty"`
	InputShape           any      `json:"input_shape,omitempty"`
	RequiredInputFields  []string `json:"required_input_fields,omitempty"`
	RequiresConfirmation bool     `json:"requires_confirmation,omitempty"`
	RuntimeEffect        string   `json:"runtime_effect"`
}

// DomainDefinition 描述配置域的真相源、写入入口与能力边界。
type DomainDefinition struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Source      string                `json:"source"`
	ManagedBy   string                `json:"managed_by"`
	Mutable     bool                  `json:"mutable"`
	Operations  []OperationDefinition `json:"operations"`
}

// Check 表示不会泄漏凭据的配置健康检查结果。
type Check struct {
	Code     string `json:"code"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Domain   string `json:"domain"`
	Target   string `json:"target,omitempty"`
	Remedy   string `json:"remedy,omitempty"`
	Verified bool   `json:"verified"`
}

// DomainSnapshot 表示一个配置域的脱敏当前值与乐观并发版本。
type DomainSnapshot struct {
	Definition   DomainDefinition `json:"definition"`
	Scope        ScopeRef         `json:"scope"`
	Access       Access           `json:"access"`
	Revision     string           `json:"revision"`
	StateVersion int64            `json:"state_version,omitempty"`
	Values       any              `json:"values,omitempty"`
	Checks       []Check          `json:"checks"`
}

// Inspection 是主智能体读取配置控制面的统一结果。
type Inspection struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Authority   string                    `json:"authority"`
	Context     ScopeRef                  `json:"context"`
	Domains     map[string]DomainSnapshot `json:"domains"`
}

// ChangeRequest 表示一项可预检、可审计的配置变更。
type ChangeRequest struct {
	RequestID        string          `json:"request_id,omitempty"`
	Domain           string          `json:"domain"`
	Operation        string          `json:"operation"`
	Target           string          `json:"target,omitempty"`
	Input            json.RawMessage `json:"input,omitempty"`
	ExpectedRevision string          `json:"expected_revision,omitempty"`
	PlanDigest       string          `json:"plan_digest,omitempty"`
}

// ChangePlan 是写入前的确定性预检结果。
type ChangePlan struct {
	Domain               string   `json:"domain"`
	Operation            string   `json:"operation"`
	Target               string   `json:"target,omitempty"`
	Scope                ScopeRef `json:"scope"`
	CurrentRevision      string   `json:"current_revision"`
	StateVersion         int64    `json:"state_version,omitempty"`
	PlanDigest           string   `json:"plan_digest"`
	Risk                 string   `json:"risk"`
	RuntimeEffect        string   `json:"runtime_effect"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Summary              string   `json:"summary"`
	SanitizedInput       any      `json:"sanitized_input,omitempty"`
	// SecretSlots 只描述需要由当前真人通过受信入口填写的路径和 opaque ID。
	// 值从不进入模型结果、tool input、审计或持久 transcript。
	SecretSlots []secretinput.Slot `json:"secret_slots,omitempty"`
}

// ApplyResult 表示变更与变更后核对的完整结果。
type ApplyResult struct {
	RequestID        string       `json:"request_id"`
	Applied          bool         `json:"applied"`
	IdempotentReplay bool         `json:"idempotent_replay,omitempty"`
	Domain           string       `json:"domain"`
	Operation        string       `json:"operation"`
	Target           string       `json:"target,omitempty"`
	Scope            ScopeRef     `json:"scope"`
	RevisionBefore   string       `json:"revision_before"`
	RevisionAfter    string       `json:"revision_after"`
	RuntimeEffect    string       `json:"runtime_effect"`
	Reload           ReloadStatus `json:"reload"`
	Result           any          `json:"result,omitempty"`
	Checks           []Check      `json:"checks"`
}

// ReloadStatus 把“已持久化”和“运行态何时采用”分开报告。
type ReloadStatus struct {
	Mode                 string `json:"mode"`
	State                string `json:"state"`
	CurrentRoundAffected bool   `json:"current_round_affected"`
	Message              string `json:"message"`
}

// AuditRecord 表示一条永不含明文凭据的配置变更审计。
type AuditRecord struct {
	RequestID              string          `json:"request_id"`
	OwnerUserID            string          `json:"owner_user_id"`
	ActorAgentID           string          `json:"actor_agent_id"`
	SessionKey             string          `json:"session_key,omitempty"`
	RoundID                string          `json:"round_id,omitempty"`
	LeaseSessionKey        string          `json:"lease_session_key,omitempty"`
	LeaseRoundID           string          `json:"lease_round_id,omitempty"`
	ContextKind            string          `json:"context_kind,omitempty"`
	ContextID              string          `json:"context_id,omitempty"`
	ScopeKind              string          `json:"scope_kind"`
	ScopeID                string          `json:"scope_id,omitempty"`
	Authority              string          `json:"authority"`
	IntentDigest           string          `json:"intent_digest"`
	HumanApprovalRequestID string          `json:"human_approval_request_id,omitempty"`
	HumanPrincipalUserID   string          `json:"human_principal_user_id,omitempty"`
	HumanPrincipalRole     string          `json:"human_principal_role,omitempty"`
	HumanAuthMethod        string          `json:"human_auth_method,omitempty"`
	HumanApprovedAt        *time.Time      `json:"human_approved_at,omitempty"`
	Domain                 string          `json:"domain"`
	Operation              string          `json:"operation"`
	Target                 string          `json:"target,omitempty"`
	Request                json.RawMessage `json:"request"`
	Result                 json.RawMessage `json:"result"`
	RevisionBefore         string          `json:"revision_before"`
	RevisionAfter          string          `json:"revision_after,omitempty"`
	Status                 string          `json:"status"`
	ErrorMessage           string          `json:"error_message,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}
