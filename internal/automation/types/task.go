// INPUT: Automation task/run 的持久字段与 service 投影。
// OUTPUT: HTTP、CLI 与内部编排共享的任务、运行、历史和即时执行结果。
// POS: Automation read model；client request 只用于 owner-scoped 人工运行对账。
package types

import "time"

// ScheduledTask 表示对外暴露的定时任务视图。
type ScheduledTask struct {
	JobID                      string               `json:"job_id"`
	OwnerUserID                string               `json:"-"`
	Name                       string               `json:"name"`
	AgentID                    string               `json:"agent_id"`
	Schedule                   Schedule             `json:"schedule"`
	Instruction                string               `json:"instruction"`
	ExecutionKind              string               `json:"execution_kind,omitempty"`
	PermissionMode             string               `json:"permission_mode"`
	SessionTarget              SessionTarget        `json:"session_target"`
	Delivery                   DeliveryTarget       `json:"delivery"`
	SessionBindingState        string               `json:"session_binding_state"`
	SessionBindingIssues       []string             `json:"session_binding_issues,omitempty"`
	InvalidatedSessionKeys     []string             `json:"-"`
	Source                     Source               `json:"source"`
	DeliveryGrant              Source               `json:"-"`
	OverlapPolicy              string               `json:"overlap_policy,omitempty"`
	ExpiresAt                  *time.Time           `json:"expires_at,omitempty"`
	Enabled                    bool                 `json:"enabled"`
	NextRunAt                  *time.Time           `json:"next_run_at,omitempty"`
	Running                    bool                 `json:"running"`
	RunningRunID               string               `json:"running_run_id,omitempty"`
	RunningStartedAt           *time.Time           `json:"running_started_at,omitempty"`
	LastRunAt                  *time.Time           `json:"last_run_at,omitempty"`
	LastRunStatus              string               `json:"last_run_status,omitempty"`
	FailureStreak              int                  `json:"failure_streak,omitempty"`
	LastError                  *string              `json:"last_error,omitempty"`
	LastDeliveryStatus         string               `json:"last_delivery_status,omitempty"`
	ConfigurationVersion       int64                `json:"configuration_version"`
	DeletionState              string               `json:"deletion_state,omitempty"`
	DeletionToken              string               `json:"-"`
	DeletionClaimedAt          *time.Time           `json:"-"`
	PermissionPolicy           TaskPermissionPolicy `json:"permission_policy"`
	PermissionState            string               `json:"permission_state"`
	PendingPermissionRequestID string               `json:"pending_permission_request_id,omitempty"`
}

const (
	// TaskCreateRequestStatusNotFound 表示服务端没有受理该创建意图。
	TaskCreateRequestStatusNotFound = "not_found"
	// TaskCreateRequestStatusCommitted 表示创建意图已提交且任务仍存在。
	TaskCreateRequestStatusCommitted = "committed"
	// TaskCreateRequestStatusGone 表示创建意图已提交，但任务后来被删除。
	TaskCreateRequestStatusGone = "gone"
)

// ScheduledTaskCreateRequestStatus 是页面刷新后核对创建结果的 owner-scoped 回执。
// 它不暴露原始正文或意图摘要。
type ScheduledTaskCreateRequestStatus struct {
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Task      *ScheduledTask `json:"task,omitempty"`
}

// ScheduledTaskRun 表示 run ledger 条目。
type ScheduledTaskRun struct {
	RunID           string  `json:"run_id"`
	JobID           string  `json:"job_id"`
	OwnerUserID     string  `json:"-"`
	ClientRequestID string  `json:"client_request_id,omitempty"`
	Status          string  `json:"status"`
	TriggerKind     string  `json:"trigger_kind,omitempty"`
	SessionKey      string  `json:"session_key,omitempty"`
	RoundID         string  `json:"round_id,omitempty"`
	SessionID       *string `json:"session_id,omitempty"`
	MessageCount    int     `json:"message_count,omitempty"`
	DeliveryMode    string  `json:"delivery_mode,omitempty"`
	DeliveryTo      string  `json:"delivery_to,omitempty"`
	// DeliveryTarget 是本次 run 开始时冻结的逻辑投递目标；DeliveryTo 记录实际解析结果摘要。
	DeliveryTarget           *DeliveryTarget `json:"delivery_target,omitempty"`
	DeliveryStatus           string          `json:"delivery_status,omitempty"`
	DeliveryError            *string         `json:"delivery_error,omitempty"`
	DeliveredAt              *time.Time      `json:"delivered_at,omitempty"`
	DeliveryAttempts         int             `json:"delivery_attempts,omitempty"`
	DeliveryNextAttemptAt    *time.Time      `json:"delivery_next_attempt_at,omitempty"`
	DeliveryDeadLetterAt     *time.Time      `json:"delivery_dead_letter_at,omitempty"`
	ScheduledFor             *time.Time      `json:"scheduled_for,omitempty"`
	StartedAt                *time.Time      `json:"started_at,omitempty"`
	FinishedAt               *time.Time      `json:"finished_at,omitempty"`
	Attempts                 int             `json:"attempts"`
	ErrorMessage             *string         `json:"error_message,omitempty"`
	ResultSummary            *string         `json:"result_summary,omitempty"`
	AssistantText            *string         `json:"assistant_text,omitempty"`
	ResultText               *string         `json:"result_text,omitempty"`
	ArtifactPath             *string         `json:"artifact_path,omitempty"`
	PermissionPolicyRevision int             `json:"permission_policy_revision,omitempty"`
	BlockState               string          `json:"block_state,omitempty"`
	BlockedRequestID         string          `json:"blocked_request_id,omitempty"`
	EffectStarted            bool            `json:"effect_started,omitempty"`
	CreatedAt                time.Time       `json:"created_at,omitempty"`
	UpdatedAt                time.Time       `json:"updated_at,omitempty"`
}

// ScheduledTaskEvent 表示定时任务管理动作审计记录。
type ScheduledTaskEvent struct {
	EventID      string         `json:"event_id"`
	JobID        string         `json:"job_id"`
	OwnerUserID  string         `json:"-"`
	AgentID      string         `json:"agent_id"`
	Action       string         `json:"action"`
	ActorUserID  string         `json:"actor_user_id,omitempty"`
	ActorAgentID string         `json:"actor_agent_id,omitempty"`
	RunID        string         `json:"run_id,omitempty"`
	Detail       map[string]any `json:"detail,omitempty"`
	CreatedAt    time.Time      `json:"created_at,omitempty"`
}

// ScheduledTaskHistorySearchInput 表示按自然语言线索定位当前或历史任务的查询。
type ScheduledTaskHistorySearchInput struct {
	Query          string
	AgentID        string
	IncludeActive  bool
	IncludeDeleted bool
	Limit          int
}

// ScheduledTaskHistoryItem 表示可供 Agent 继续管理或追溯的任务候选。
type ScheduledTaskHistoryItem struct {
	JobID              string     `json:"job_id"`
	Name               string     `json:"name,omitempty"`
	AgentID            string     `json:"agent_id,omitempty"`
	Deleted            bool       `json:"deleted"`
	Enabled            *bool      `json:"enabled,omitempty"`
	Running            bool       `json:"running,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
	LastRunAt          *time.Time `json:"last_run_at,omitempty"`
	LastRunStatus      string     `json:"last_run_status,omitempty"`
	LastDeliveryStatus string     `json:"last_delivery_status,omitempty"`
	LatestAction       string     `json:"latest_action,omitempty"`
	LatestEventAt      *time.Time `json:"latest_event_at,omitempty"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
	RunCount           int        `json:"run_count,omitempty"`
}

// ScheduledTaskHealth 表示单个定时任务的可操作健康摘要。
type ScheduledTaskHealth struct {
	State                      string   `json:"state"`
	Signals                    []string `json:"signals,omitempty"`
	SuggestedTools             []string `json:"suggested_tools,omitempty"`
	RecoveryAvailable          bool     `json:"recovery_available"`
	RecoveryRunID              string   `json:"recovery_run_id,omitempty"`
	ManualRedeliveryAvailable  bool     `json:"manual_redelivery_available"`
	ManualRedeliveryRunIDs     []string `json:"manual_redelivery_run_ids,omitempty"`
	DeliveryFailedRunCount     int      `json:"delivery_failed_run_count,omitempty"`
	DeliveryUnverifiedRunCount int      `json:"delivery_unverified_run_count,omitempty"`
	DeliveryUnverifiedRunIDs   []string `json:"delivery_unverified_run_ids,omitempty"`
	DeliveryPendingRunCount    int      `json:"delivery_pending_run_count,omitempty"`
	DeliveryPendingRunIDs      []string `json:"delivery_pending_run_ids,omitempty"`
	DeliverySkippedRunCount    int      `json:"delivery_skipped_run_count,omitempty"`
	DeliverySkippedRunIDs      []string `json:"delivery_skipped_run_ids,omitempty"`
	DeliveryDeadLetterCount    int      `json:"delivery_dead_letter_count,omitempty"`
	DeliveryDeadLetterRunIDs   []string `json:"delivery_dead_letter_run_ids,omitempty"`
	FailedRunCount             int      `json:"failed_run_count,omitempty"`
	ExecutionFailedRunIDs      []string `json:"execution_failed_run_ids,omitempty"`
	LatestExecutionError       *string  `json:"latest_execution_error,omitempty"`
	LatestDeliveryError        *string  `json:"latest_delivery_error,omitempty"`
	RunningForSeconds          int64    `json:"running_for_seconds,omitempty"`
}

// ScheduledTaskStatus 表示单个任务的配置、健康摘要与最近观测记录。
type ScheduledTaskStatus struct {
	Job          ScheduledTask        `json:"job"`
	Health       ScheduledTaskHealth  `json:"health"`
	RecentRuns   []ScheduledTaskRun   `json:"recent_runs"`
	RecentEvents []ScheduledTaskEvent `json:"recent_events"`
}

// DeleteJobResult 表示删除定时任务后的可解释结果。
type DeleteJobResult struct {
	JobID              string `json:"job_id"`
	AgentID            string `json:"agent_id,omitempty"`
	Deleted            bool   `json:"deleted"`
	ActiveRunID        string `json:"active_run_id,omitempty"`
	CancelledRunID     string `json:"cancelled_run_id,omitempty"`
	CancelledActiveRun bool   `json:"cancelled_active_run,omitempty"`
}

// ExecutionResult 表示一次手动触发或后台触发的返回体。
type ExecutionResult struct {
	JobID        string     `json:"job_id"`
	RunID        *string    `json:"run_id,omitempty"`
	Status       string     `json:"status"`
	SessionKey   string     `json:"session_key"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	RoundID      *string    `json:"round_id,omitempty"`
	SessionID    *string    `json:"session_id,omitempty"`
	MessageCount int        `json:"message_count"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	Replayed     bool       `json:"-"`
}
