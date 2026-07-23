// INPUT: 配置读取范围、当前主智能体身份与用户声明的变更请求。
// OUTPUT: 可脱敏展示的配置快照、预检计划、应用结果与审计记录。
// POS: configuration 控制面的跨层协议模型。
package configuration

import (
	"encoding/json"
	"time"
)

const (
	DomainPreferences = "preferences"
	DomainProviders   = "providers"
	DomainAgents      = "agents"
	DomainChannels    = "channels"
	DomainConnectors  = "connectors"
	DomainSkills      = "skills"
	DomainHost        = "host"
	DomainAutomation  = "automation"
	DomainRooms       = "rooms"
	DomainWorkspaces  = "workspaces"
	DomainGoals       = "goals"
)

// Actor 表示一次配置控制调用的可信 runtime 身份。
type Actor struct {
	OwnerUserID   string `json:"owner_user_id"`
	AgentID       string `json:"agent_id"`
	SessionKey    string `json:"session_key,omitempty"`
	IsMainAgent   bool   `json:"is_main_agent"`
	SourceContext string `json:"source_context,omitempty"`
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
	Definition DomainDefinition `json:"definition"`
	Revision   string           `json:"revision"`
	Values     any              `json:"values,omitempty"`
	Checks     []Check          `json:"checks"`
}

// Inspection 是主智能体读取配置控制面的统一结果。
type Inspection struct {
	GeneratedAt time.Time                 `json:"generated_at"`
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
	Confirm          bool            `json:"confirm,omitempty"`
}

// ChangePlan 是写入前的确定性预检结果。
type ChangePlan struct {
	Domain               string `json:"domain"`
	Operation            string `json:"operation"`
	Target               string `json:"target,omitempty"`
	CurrentRevision      string `json:"current_revision"`
	Risk                 string `json:"risk"`
	RuntimeEffect        string `json:"runtime_effect"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Summary              string `json:"summary"`
	SanitizedInput       any    `json:"sanitized_input,omitempty"`
}

// ApplyResult 表示变更与变更后核对的完整结果。
type ApplyResult struct {
	RequestID        string  `json:"request_id"`
	Applied          bool    `json:"applied"`
	IdempotentReplay bool    `json:"idempotent_replay,omitempty"`
	Domain           string  `json:"domain"`
	Operation        string  `json:"operation"`
	Target           string  `json:"target,omitempty"`
	RevisionBefore   string  `json:"revision_before"`
	RevisionAfter    string  `json:"revision_after"`
	RuntimeEffect    string  `json:"runtime_effect"`
	Result           any     `json:"result,omitempty"`
	Checks           []Check `json:"checks"`
}

// AuditRecord 表示一条永不含明文凭据的配置变更审计。
type AuditRecord struct {
	RequestID      string          `json:"request_id"`
	OwnerUserID    string          `json:"owner_user_id"`
	ActorAgentID   string          `json:"actor_agent_id"`
	SessionKey     string          `json:"session_key,omitempty"`
	Domain         string          `json:"domain"`
	Operation      string          `json:"operation"`
	Target         string          `json:"target,omitempty"`
	Request        json.RawMessage `json:"request"`
	Result         json.RawMessage `json:"result"`
	RevisionBefore string          `json:"revision_before"`
	RevisionAfter  string          `json:"revision_after,omitempty"`
	Status         string          `json:"status"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
