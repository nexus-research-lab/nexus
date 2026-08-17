// INPUT: Agent 通过 round-scoped Nexus CLI 表达的 Automation 查询或变更意图。
// OUTPUT: 可严格解码、可预检、可做 revision/digest 栅栏的命令协议。
// POS: Automation Skill、CLI broker 与 service command 层共用的线格式真相。
package types

const (
	AutomationCommandActionContract = "contract"
	AutomationCommandActionInspect  = "inspect"
	AutomationCommandActionPlan     = "plan"
	AutomationCommandActionApply    = "apply"
	AutomationCommandActionReplay   = "replay"
)

const (
	AutomationCommandOperationList          = "list"
	AutomationCommandOperationGet           = "get"
	AutomationCommandOperationRuns          = "runs"
	AutomationCommandOperationEvents        = "events"
	AutomationCommandOperationReport        = "report"
	AutomationCommandOperationHeartbeat     = "heartbeat"
	AutomationCommandOperationCreate        = "create"
	AutomationCommandOperationUpdate        = "update"
	AutomationCommandOperationDelete        = "delete"
	AutomationCommandOperationRun           = "run"
	AutomationCommandOperationRetryDelivery = "retry_delivery"
	AutomationCommandOperationSetHeartbeat  = "set_heartbeat"
	AutomationCommandOperationWake          = "wake"
)

// AutomationCommandSchedule 是面向对话和 UI 的调度形状；service 会翻译成持久 Schedule。
type AutomationCommandSchedule struct {
	Kind          string   `json:"kind"`
	RunAt         string   `json:"run_at,omitempty"`
	DailyTime     string   `json:"daily_time,omitempty"`
	Weekdays      []string `json:"weekdays,omitempty"`
	IntervalValue int      `json:"interval_value,omitempty"`
	IntervalUnit  string   `json:"interval_unit,omitempty"`
	Expr          string   `json:"expr,omitempty"`
	Timezone      string   `json:"timezone,omitempty"`
}

// AutomationCommandInput 是所有操作的封闭字段并集。operation 决定允许和必需的子集。
// owner、当前 Agent、Room、Session、来源、DeliveryGrant、job/run runtime 身份不在此结构中。
type AutomationCommandInput struct {
	JobID          string `json:"job_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	Query          string `json:"query,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	Name           string `json:"name,omitempty"`
	Instruction    string `json:"instruction,omitempty"`
	InstructionAdd string `json:"instruction_append,omitempty"`

	Schedule        *AutomationCommandSchedule `json:"schedule,omitempty"`
	ExecutionKind   string                     `json:"execution_kind,omitempty"`
	PermissionMode  string                     `json:"permission_mode,omitempty"`
	ContextMode     string                     `json:"context_mode,omitempty"`
	DeliverResult   *bool                      `json:"deliver_result,omitempty"`
	OverlapPolicy   string                     `json:"overlap_policy,omitempty"`
	ExpiresAt       string                     `json:"expires_at,omitempty"`
	ClearExpiresAt  bool                       `json:"clear_expires_at,omitempty"`
	Enabled         *bool                      `json:"enabled,omitempty"`
	CancelActiveRun bool                       `json:"cancel_active_run,omitempty"`

	// 下面字段只允许 owner main 在自己的可信 Nexus 私有 DM 使用。
	ExecutionMode           string `json:"execution_mode,omitempty"`
	ReplyMode               string `json:"reply_mode,omitempty"`
	SelectedSessionKey      string `json:"selected_session_key,omitempty"`
	NamedSessionKey         string `json:"named_session_key,omitempty"`
	SelectedReplySessionKey string `json:"selected_reply_session_key,omitempty"`
	ReplySessionKey         string `json:"reply_session_key,omitempty"`

	IncludeActive  *bool  `json:"include_active,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	RunLimit       int    `json:"run_limit,omitempty"`
	EventLimit     int    `json:"event_limit,omitempty"`
	Date           string `json:"date,omitempty"`
	Timezone       string `json:"timezone,omitempty"`

	EverySeconds int    `json:"every_seconds,omitempty"`
	TargetMode   string `json:"target_mode,omitempty"`
	AckMaxChars  *int   `json:"ack_max_chars,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Text         string `json:"text,omitempty"`
}

// AutomationCommandRequest 是 nexus CLI 到宿主 broker 的请求。
type AutomationCommandRequest struct {
	Action           string                 `json:"action"`
	Operation        string                 `json:"operation,omitempty"`
	Input            AutomationCommandInput `json:"input,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	ExpectedRevision string                 `json:"expected_revision,omitempty"`
	PlanDigest       string                 `json:"plan_digest,omitempty"`
}

// AutomationCommandContract 是按可信 Actor 返回的按需字段目录。
type AutomationCommandContract struct {
	QueryOperations    []string                                      `json:"query_operations"`
	MutationOperations []string                                      `json:"mutation_operations"`
	MutationAllowed    bool                                          `json:"mutation_allowed"`
	CrossAgentAllowed  bool                                          `json:"cross_agent_allowed"`
	Operations         map[string]AutomationCommandOperationContract `json:"operations"`
}

type AutomationCommandOperationContract struct {
	Kind     string   `json:"kind"`
	Required []string `json:"required,omitempty"`
	Optional []string `json:"optional,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

// AutomationCommandPlan 是不写入的确定性变更计划。
type AutomationCommandPlan struct {
	Operation            string                 `json:"operation"`
	Target               string                 `json:"target"`
	Summary              string                 `json:"summary"`
	Risk                 string                 `json:"risk"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
	CurrentRevision      string                 `json:"current_revision"`
	PlanDigest           string                 `json:"plan_digest"`
	Input                AutomationCommandInput `json:"input"`
}

// AutomationCommandApplyResult 是写入后的稳定结果 envelope。
type AutomationCommandApplyResult struct {
	Operation string `json:"operation"`
	Outcome   string `json:"outcome"`
	Data      any    `json:"data,omitempty"`
}

type AutomationCommandReplayResult struct {
	Found  bool                          `json:"found"`
	Result *AutomationCommandApplyResult `json:"result,omitempty"`
}
