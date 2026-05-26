package protocol

import (
	"errors"
	"strings"
	"time"
)

const (
	// ScheduleKindEvery 表示固定间隔调度。
	ScheduleKindEvery = "every"
	// ScheduleKindCron 表示 cron 表达式调度。
	ScheduleKindCron = "cron"
	// ScheduleKindAt 表示单次定时。
	ScheduleKindAt = "at"

	// SessionTargetIsolated 表示每次运行都创建新会话。
	SessionTargetIsolated = "isolated"
	// SessionTargetMain 表示写入主自动化会话。
	SessionTargetMain = "main"
	// SessionTargetBound 表示绑定到现有结构化会话。
	SessionTargetBound = "bound"
	// SessionTargetNamed 表示绑定到命名自动化会话。
	SessionTargetNamed = "named"

	// WakeModeNow 表示立即唤醒。
	WakeModeNow = "now"
	// WakeModeNextHeartbeat 表示在下一次 heartbeat 时消费。
	WakeModeNextHeartbeat = "next-heartbeat"

	// DeliveryModeNone 表示不做外部投递。
	DeliveryModeNone = "none"
	// DeliveryModeLast 表示投递到最近通道。
	DeliveryModeLast = "last"
	// DeliveryModeExplicit 表示投递到显式目标。
	DeliveryModeExplicit = "explicit"

	// DeliveryStatusNotRequired 表示该 run 不需要额外投递。
	DeliveryStatusNotRequired = "not_required"
	// DeliveryStatusSkipped 表示无需重复投递或没有可投递内容。
	DeliveryStatusSkipped = "skipped"
	// DeliveryStatusSucceeded 表示投递成功。
	DeliveryStatusSucceeded = "succeeded"
	// DeliveryStatusFailed 表示投递失败。
	DeliveryStatusFailed = "failed"
	// DeliveryStatusNotAttempted 表示 run 在投递前失败或被取消。
	DeliveryStatusNotAttempted = "not_attempted"
	// DeliveryStatusPending 表示 run 尚未结束，投递状态未定。
	DeliveryStatusPending = "pending"

	// SourceKindUserPage 表示来自页面创建。
	SourceKindUserPage = "user_page"
	// SourceKindAgent 表示来自 Agent 创建。
	SourceKindAgent = "agent"
	// SourceKindCLI 表示来自 CLI 创建。
	SourceKindCLI = "cli"
	// SourceKindSystem 表示来自系统创建。
	SourceKindSystem = "system"

	// RunStatusPending 表示已登记但未开始执行。
	RunStatusPending = "pending"
	// RunStatusRunning 表示执行中。
	RunStatusRunning = "running"
	// RunStatusSucceeded 表示执行成功。
	RunStatusSucceeded = "succeeded"
	// RunStatusFailed 表示执行失败。
	RunStatusFailed = "failed"
	// RunStatusCancelled 表示执行取消。
	RunStatusCancelled = "cancelled"
	// RunStatusQueuedToMain 表示已排入主会话队列。
	RunStatusQueuedToMain = "queued_to_main_session"
	// RunStatusSkipped 表示因重叠策略跳过本次触发。
	RunStatusSkipped = "skipped"

	// OverlapPolicySkip 表示已有执行时跳过新触发。
	OverlapPolicySkip = "skip"
	// OverlapPolicyAllow 表示允许同一任务并发执行。
	OverlapPolicyAllow = "allow"

	// ExecutionKindAgent 表示由 Agent 会话执行任务。
	ExecutionKindAgent = "agent"
	// ExecutionKindScript 表示直接在 workspace 中执行脚本任务。
	ExecutionKindScript = "script"

	// TaskEventActionCreate 表示创建定时任务。
	TaskEventActionCreate = "create"
	// TaskEventActionUpdate 表示修改定时任务。
	TaskEventActionUpdate = "update"
	// TaskEventActionEnable 表示启用定时任务。
	TaskEventActionEnable = "enable"
	// TaskEventActionDisable 表示停用定时任务。
	TaskEventActionDisable = "disable"
	// TaskEventActionDelete 表示删除定时任务。
	TaskEventActionDelete = "delete"
	// TaskEventActionRunNow 表示手动立即运行。
	TaskEventActionRunNow = "run_now"
	// TaskEventActionRecover 表示手动恢复卡住运行。
	TaskEventActionRecover = "recover"
	// TaskEventActionRetryDelivery 表示手动重试投递。
	TaskEventActionRetryDelivery = "retry_delivery"
	// TaskEventActionAutoRetryDelivery 表示系统自动重试投递。
	TaskEventActionAutoRetryDelivery = "auto_retry_delivery"

	// HeartbeatTargetNone 表示不投递。
	HeartbeatTargetNone = "none"
	// HeartbeatTargetLast 表示投递到最近通道。
	HeartbeatTargetLast = "last"
	// HeartbeatTargetExplicit 表示投递到显式目标。
	HeartbeatTargetExplicit = "explicit"
)

var (
	// ErrJobNotFound 表示任务不存在。
	ErrJobNotFound = errors.New("scheduled task not found")
	// ErrRunNotFound 表示任务运行记录不存在。
	ErrRunNotFound = errors.New("scheduled task run not found")
	// ErrHeartbeatConfigInvalid 表示 heartbeat 配置非法。
	ErrHeartbeatConfigInvalid = errors.New("heartbeat config is invalid")
)

// Schedule 表示结构化调度定义。
type Schedule struct {
	Kind            string  `json:"kind"`
	RunAt           *string `json:"run_at,omitempty"`
	IntervalSeconds *int    `json:"interval_seconds,omitempty"`
	CronExpression  *string `json:"cron_expression,omitempty"`
	Timezone        string  `json:"timezone,omitempty"`
}

// Validate 校验调度形状。
func (s Schedule) Validate() error {
	kind := strings.TrimSpace(s.Kind)
	timezoneName := strings.TrimSpace(s.Timezone)
	if timezoneName == "" {
		timezoneName = "Asia/Shanghai"
	}
	switch kind {
	case ScheduleKindEvery:
		if s.IntervalSeconds == nil || *s.IntervalSeconds <= 0 {
			return errors.New("interval_seconds must be greater than 0 when kind is every")
		}
		if s.RunAt != nil || s.CronExpression != nil {
			return errors.New("run_at and cron_expression must be empty when kind is every")
		}
	case ScheduleKindAt:
		if s.RunAt == nil || strings.TrimSpace(*s.RunAt) == "" {
			return errors.New("run_at is required when kind is at")
		}
		if s.IntervalSeconds != nil || s.CronExpression != nil {
			return errors.New("interval_seconds and cron_expression must be empty when kind is at")
		}
	case ScheduleKindCron:
		if s.CronExpression == nil || strings.TrimSpace(*s.CronExpression) == "" {
			return errors.New("cron_expression is required when kind is cron")
		}
		if s.RunAt != nil || s.IntervalSeconds != nil {
			return errors.New("run_at and interval_seconds must be empty when kind is cron")
		}
	default:
		return errors.New("schedule.kind must be one of every, cron, at")
	}
	if strings.TrimSpace(timezoneName) == "" {
		return errors.New("timezone is required")
	}
	return nil
}

// Normalized 返回带默认值的调度副本。
func (s Schedule) Normalized() Schedule {
	result := s
	result.Kind = strings.TrimSpace(result.Kind)
	if strings.TrimSpace(result.Timezone) == "" {
		result.Timezone = "Asia/Shanghai"
	}
	if result.RunAt != nil {
		value := strings.TrimSpace(*result.RunAt)
		result.RunAt = &value
	}
	if result.CronExpression != nil {
		value := strings.TrimSpace(*result.CronExpression)
		result.CronExpression = &value
	}
	return result
}

// SessionTarget 表示执行目标会话。
type SessionTarget struct {
	Kind            string `json:"kind"`
	BoundSessionKey string `json:"bound_session_key,omitempty"`
	NamedSessionKey string `json:"named_session_key,omitempty"`
	WakeMode        string `json:"wake_mode,omitempty"`
}

// Validate 校验会话目标。
func (t SessionTarget) Validate() error {
	kind := strings.TrimSpace(t.Kind)
	wakeMode := strings.TrimSpace(t.WakeMode)
	if wakeMode == "" {
		wakeMode = WakeModeNextHeartbeat
	}
	switch wakeMode {
	case WakeModeNow, WakeModeNextHeartbeat:
	default:
		return errors.New("wake_mode must be one of now, next-heartbeat")
	}

	switch kind {
	case SessionTargetIsolated, SessionTargetMain:
		if strings.TrimSpace(t.BoundSessionKey) != "" || strings.TrimSpace(t.NamedSessionKey) != "" {
			return errors.New("bound_session_key and named_session_key must be empty for isolated/main target")
		}
	case SessionTargetBound:
		if strings.TrimSpace(t.BoundSessionKey) == "" {
			return errors.New("bound_session_key is required when session_target.kind is bound")
		}
		if _, err := RequireStructuredSessionKey(t.BoundSessionKey); err != nil {
			return err
		}
	case SessionTargetNamed:
		if strings.TrimSpace(t.NamedSessionKey) == "" {
			return errors.New("named_session_key is required when session_target.kind is named")
		}
		if strings.EqualFold(strings.TrimSpace(t.NamedSessionKey), "main") {
			return errors.New("named_session_key 'main' is reserved")
		}
	default:
		return errors.New("session_target.kind must be one of isolated, main, bound, named")
	}
	return nil
}

// Normalized 返回带默认值的会话目标副本。
func (t SessionTarget) Normalized() SessionTarget {
	result := t
	result.Kind = strings.TrimSpace(result.Kind)
	if result.Kind == "" {
		result.Kind = SessionTargetIsolated
	}
	result.BoundSessionKey = strings.TrimSpace(result.BoundSessionKey)
	result.NamedSessionKey = strings.TrimSpace(result.NamedSessionKey)
	result.WakeMode = strings.TrimSpace(result.WakeMode)
	if result.WakeMode == "" {
		result.WakeMode = WakeModeNextHeartbeat
	}
	return result
}

// DeliveryTarget 表示自动化外部投递目标。
type DeliveryTarget struct {
	Mode      string `json:"mode"`
	Channel   string `json:"channel,omitempty"`
	To        string `json:"to,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
}

// Source 表示任务来源元数据。
type Source struct {
	Kind           string `json:"kind"`
	CreatorAgentID string `json:"creator_agent_id,omitempty"`
	ContextType    string `json:"context_type,omitempty"`
	ContextID      string `json:"context_id,omitempty"`
	ContextLabel   string `json:"context_label,omitempty"`
	SessionKey     string `json:"session_key,omitempty"`
	SessionLabel   string `json:"session_label,omitempty"`
}

// Validate 校验投递目标。
func (d DeliveryTarget) Validate() error {
	switch strings.TrimSpace(d.Mode) {
	case "", DeliveryModeNone, DeliveryModeLast, DeliveryModeExplicit:
		return nil
	default:
		return errors.New("delivery.mode must be one of none, last, explicit")
	}
}

// Normalized 返回带默认值的投递目标副本。
func (d DeliveryTarget) Normalized() DeliveryTarget {
	result := d
	result.Mode = strings.TrimSpace(result.Mode)
	if result.Mode == "" {
		result.Mode = DeliveryModeNone
	}
	result.Channel = strings.TrimSpace(result.Channel)
	result.To = strings.TrimSpace(result.To)
	result.AccountID = strings.TrimSpace(result.AccountID)
	result.ThreadID = strings.TrimSpace(result.ThreadID)
	return result
}

// Validate 校验任务来源。
func (s Source) Validate() error {
	contextID := strings.TrimSpace(s.ContextID)
	contextLabel := strings.TrimSpace(s.ContextLabel)
	switch strings.TrimSpace(s.Kind) {
	case "", SourceKindUserPage, SourceKindAgent, SourceKindCLI, SourceKindSystem:
	default:
		return errors.New("source.kind must be one of user_page, agent, cli, system")
	}
	contextType := strings.TrimSpace(s.ContextType)
	switch contextType {
	case "", "agent", "room":
	default:
		return errors.New("source.context_type must be one of agent, room")
	}
	if contextType == "" {
		if contextID != "" || contextLabel != "" {
			return errors.New("context_type is required when context_id or context_label is provided")
		}
	} else if contextID == "" {
		return errors.New("context_id is required when context_type is provided")
	}
	if strings.TrimSpace(s.SessionKey) != "" {
		if _, err := RequireStructuredSessionKey(s.SessionKey); err != nil {
			return err
		}
	}
	return nil
}

// Normalized 返回带默认值的来源副本。
func (s Source) Normalized() Source {
	result := s
	result.Kind = strings.TrimSpace(result.Kind)
	if result.Kind == "" {
		result.Kind = SourceKindSystem
	}
	result.CreatorAgentID = strings.TrimSpace(result.CreatorAgentID)
	result.ContextType = strings.TrimSpace(result.ContextType)
	result.ContextID = strings.TrimSpace(result.ContextID)
	result.ContextLabel = strings.TrimSpace(result.ContextLabel)
	result.SessionKey = strings.TrimSpace(result.SessionKey)
	result.SessionLabel = strings.TrimSpace(result.SessionLabel)
	return result
}

// CronJob 表示对外暴露的定时任务视图。
type CronJob struct {
	JobID              string         `json:"job_id"`
	OwnerUserID        string         `json:"-"`
	Name               string         `json:"name"`
	AgentID            string         `json:"agent_id"`
	Schedule           Schedule       `json:"schedule"`
	Instruction        string         `json:"instruction"`
	ExecutionKind      string         `json:"execution_kind,omitempty"`
	SessionTarget      SessionTarget  `json:"session_target"`
	Delivery           DeliveryTarget `json:"delivery"`
	Source             Source         `json:"source"`
	OverlapPolicy      string         `json:"overlap_policy,omitempty"`
	Enabled            bool           `json:"enabled"`
	NextRunAt          *time.Time     `json:"next_run_at,omitempty"`
	Running            bool           `json:"running"`
	RunningRunID       string         `json:"running_run_id,omitempty"`
	RunningStartedAt   *time.Time     `json:"running_started_at,omitempty"`
	LastRunAt          *time.Time     `json:"last_run_at,omitempty"`
	LastRunStatus      string         `json:"last_run_status,omitempty"`
	FailureStreak      int            `json:"failure_streak,omitempty"`
	LastError          *string        `json:"last_error,omitempty"`
	LastDeliveryStatus string         `json:"last_delivery_status,omitempty"`
}

// CronRun 表示 run ledger 条目。
type CronRun struct {
	RunID                 string     `json:"run_id"`
	JobID                 string     `json:"job_id"`
	OwnerUserID           string     `json:"-"`
	Status                string     `json:"status"`
	TriggerKind           string     `json:"trigger_kind,omitempty"`
	SessionKey            string     `json:"session_key,omitempty"`
	RoundID               string     `json:"round_id,omitempty"`
	SessionID             *string    `json:"session_id,omitempty"`
	MessageCount          int        `json:"message_count,omitempty"`
	DeliveryMode          string     `json:"delivery_mode,omitempty"`
	DeliveryTo            string     `json:"delivery_to,omitempty"`
	DeliveryStatus        string     `json:"delivery_status,omitempty"`
	DeliveryError         *string    `json:"delivery_error,omitempty"`
	DeliveredAt           *time.Time `json:"delivered_at,omitempty"`
	DeliveryAttempts      int        `json:"delivery_attempts,omitempty"`
	DeliveryNextAttemptAt *time.Time `json:"delivery_next_attempt_at,omitempty"`
	DeliveryDeadLetterAt  *time.Time `json:"delivery_dead_letter_at,omitempty"`
	ScheduledFor          *time.Time `json:"scheduled_for,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	FinishedAt            *time.Time `json:"finished_at,omitempty"`
	Attempts              int        `json:"attempts"`
	ErrorMessage          *string    `json:"error_message,omitempty"`
	ResultSummary         *string    `json:"result_summary,omitempty"`
	AssistantText         *string    `json:"assistant_text,omitempty"`
	ResultText            *string    `json:"result_text,omitempty"`
	ArtifactPath          *string    `json:"artifact_path,omitempty"`
	CreatedAt             time.Time  `json:"created_at,omitempty"`
	UpdatedAt             time.Time  `json:"updated_at,omitempty"`
}

// CronTaskEvent 表示定时任务管理动作审计记录。
type CronTaskEvent struct {
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

// CronTaskHistorySearchInput 表示按自然语言线索定位当前或历史任务的查询。
type CronTaskHistorySearchInput struct {
	Query          string
	AgentID        string
	IncludeActive  bool
	IncludeDeleted bool
	Limit          int
}

// CronTaskHistoryItem 表示可供 Agent 继续管理或追溯的任务候选。
type CronTaskHistoryItem struct {
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

// CronTaskHealth 表示单个定时任务的可操作健康摘要。
type CronTaskHealth struct {
	State                     string   `json:"state"`
	Signals                   []string `json:"signals,omitempty"`
	SuggestedTools            []string `json:"suggested_tools,omitempty"`
	RecoveryAvailable         bool     `json:"recovery_available"`
	RecoveryRunID             string   `json:"recovery_run_id,omitempty"`
	ManualRedeliveryAvailable bool     `json:"manual_redelivery_available"`
	ManualRedeliveryRunIDs    []string `json:"manual_redelivery_run_ids,omitempty"`
	DeliveryFailedRunCount    int      `json:"delivery_failed_run_count,omitempty"`
	DeliveryPendingRunCount   int      `json:"delivery_pending_run_count,omitempty"`
	DeliveryPendingRunIDs     []string `json:"delivery_pending_run_ids,omitempty"`
	DeliverySkippedRunCount   int      `json:"delivery_skipped_run_count,omitempty"`
	DeliverySkippedRunIDs     []string `json:"delivery_skipped_run_ids,omitempty"`
	DeliveryDeadLetterCount   int      `json:"delivery_dead_letter_count,omitempty"`
	DeliveryDeadLetterRunIDs  []string `json:"delivery_dead_letter_run_ids,omitempty"`
	FailedRunCount            int      `json:"failed_run_count,omitempty"`
	ExecutionFailedRunIDs     []string `json:"execution_failed_run_ids,omitempty"`
	LatestExecutionError      *string  `json:"latest_execution_error,omitempty"`
	LatestDeliveryError       *string  `json:"latest_delivery_error,omitempty"`
	RunningForSeconds         int64    `json:"running_for_seconds,omitempty"`
}

// CronTaskStatus 表示单个任务的配置、健康摘要与最近观测记录。
type CronTaskStatus struct {
	Job          CronJob         `json:"job"`
	Health       CronTaskHealth  `json:"health"`
	RecentRuns   []CronRun       `json:"recent_runs"`
	RecentEvents []CronTaskEvent `json:"recent_events"`
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

// CronDailyReportTotals 表示定时任务日报聚合计数。
type CronDailyReportTotals struct {
	TaskCount                  int `json:"task_count"`
	EnabledTaskCount           int `json:"enabled_task_count"`
	RunningTaskCount           int `json:"running_task_count"`
	RunCount                   int `json:"run_count"`
	SucceededRunCount          int `json:"succeeded_run_count"`
	FailedRunCount             int `json:"failed_run_count"`
	CancelledRunCount          int `json:"cancelled_run_count"`
	SkippedRunCount            int `json:"skipped_run_count"`
	DeliveredRunCount          int `json:"delivered_run_count"`
	DeliveryFailedRunCount     int `json:"delivery_failed_run_count"`
	DeliveryPendingRunCount    int `json:"delivery_pending_run_count"`
	DeliverySkippedRunCount    int `json:"delivery_skipped_run_count"`
	DeliveryDeadLetterRunCount int `json:"delivery_dead_letter_run_count"`
	DeliveryNotNeededCount     int `json:"delivery_not_needed_count"`
	DeliveryNotAttemptedCount  int `json:"delivery_not_attempted_count"`
}

// CronDailyReportTask 表示日报里单个任务的运行与投递情况。
type CronDailyReportTask struct {
	JobID                    string                `json:"job_id"`
	Name                     string                `json:"name"`
	AgentID                  string                `json:"agent_id"`
	Deleted                  bool                  `json:"deleted,omitempty"`
	Enabled                  bool                  `json:"enabled"`
	Running                  bool                  `json:"running"`
	RunningRunID             string                `json:"running_run_id,omitempty"`
	RecoveryRunID            string                `json:"recovery_run_id,omitempty"`
	NextRunAt                *time.Time            `json:"next_run_at,omitempty"`
	LastRunAt                *time.Time            `json:"last_run_at,omitempty"`
	LastRunStatus            string                `json:"last_run_status,omitempty"`
	LastDeliveryStatus       string                `json:"last_delivery_status,omitempty"`
	FailureStreak            int                   `json:"failure_streak,omitempty"`
	LastError                *string               `json:"last_error,omitempty"`
	LatestExecutionError     *string               `json:"latest_execution_error,omitempty"`
	LatestDeliveryError      *string               `json:"latest_delivery_error,omitempty"`
	Signals                  []string              `json:"signals,omitempty"`
	SuggestedTools           []string              `json:"suggested_tools,omitempty"`
	ExecutionFailedRunIDs    []string              `json:"execution_failed_run_ids,omitempty"`
	ManualRedeliveryRunIDs   []string              `json:"manual_redelivery_run_ids,omitempty"`
	DeliveryPendingRunIDs    []string              `json:"delivery_pending_run_ids,omitempty"`
	DeliverySkippedRunIDs    []string              `json:"delivery_skipped_run_ids,omitempty"`
	DeliveryDeadLetterRunIDs []string              `json:"delivery_dead_letter_run_ids,omitempty"`
	Runs                     []CronRun             `json:"runs"`
	Totals                   CronDailyReportTotals `json:"totals"`
}

// CronDailyReport 表示指定日期的任务运行和投递日报。
type CronDailyReport struct {
	Date     string                `json:"date"`
	Timezone string                `json:"timezone"`
	AgentID  string                `json:"agent_id,omitempty"`
	JobID    string                `json:"job_id,omitempty"`
	StartAt  time.Time             `json:"start_at"`
	EndAt    time.Time             `json:"end_at"`
	Totals   CronDailyReportTotals `json:"totals"`
	Tasks    []CronDailyReportTask `json:"tasks"`
}

// CronDailyReportInput 表示日报查询输入。
type CronDailyReportInput struct {
	Date     string
	Timezone string
	AgentID  string
	JobID    string
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
}

// CreateJobInput 表示创建请求。
type CreateJobInput struct {
	Name          string         `json:"name"`
	AgentID       string         `json:"agent_id"`
	Schedule      Schedule       `json:"schedule"`
	Instruction   string         `json:"instruction"`
	ExecutionKind string         `json:"execution_kind,omitempty"`
	SessionTarget SessionTarget  `json:"session_target"`
	Delivery      DeliveryTarget `json:"delivery"`
	Source        Source         `json:"source"`
	OverlapPolicy string         `json:"overlap_policy,omitempty"`
	Enabled       bool           `json:"enabled"`
}

// Validate 校验创建请求。
func (i CreateJobInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(i.AgentID) == "" {
		return errors.New("agent_id is required")
	}
	if strings.TrimSpace(i.Instruction) == "" {
		return errors.New("instruction is required")
	}
	if err := validateExecutionKind(i.ExecutionKind); err != nil {
		return err
	}
	if err := i.Schedule.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.SessionTarget.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.Delivery.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.Source.Normalized().Validate(); err != nil {
		return err
	}
	if err := validateOverlapPolicy(i.OverlapPolicy); err != nil {
		return err
	}
	return nil
}

// Normalized 返回标准化副本。
func (i CreateJobInput) Normalized() CreateJobInput {
	result := i
	result.Name = strings.TrimSpace(result.Name)
	result.AgentID = strings.TrimSpace(result.AgentID)
	result.Instruction = strings.TrimSpace(result.Instruction)
	result.ExecutionKind = NormalizeExecutionKind(result.ExecutionKind)
	result.Schedule = result.Schedule.Normalized()
	result.SessionTarget = result.SessionTarget.Normalized()
	result.Delivery = result.Delivery.Normalized()
	result.Source = result.Source.Normalized()
	result.OverlapPolicy = NormalizeOverlapPolicy(result.OverlapPolicy)
	return result
}

// NormalizeOverlapPolicy 返回重叠触发策略的默认值。
func NormalizeOverlapPolicy(policy string) string {
	normalized := strings.TrimSpace(policy)
	if normalized == "" {
		return OverlapPolicySkip
	}
	return normalized
}

func validateOverlapPolicy(policy string) error {
	switch NormalizeOverlapPolicy(policy) {
	case OverlapPolicySkip, OverlapPolicyAllow:
		return nil
	default:
		return errors.New("overlap_policy must be one of skip, allow")
	}
}

// NormalizeExecutionKind 返回执行体类型的默认值。
func NormalizeExecutionKind(kind string) string {
	normalized := strings.TrimSpace(kind)
	if normalized == "" {
		return ExecutionKindAgent
	}
	return normalized
}

func validateExecutionKind(kind string) error {
	switch NormalizeExecutionKind(kind) {
	case ExecutionKindAgent, ExecutionKindScript:
		return nil
	default:
		return errors.New("execution_kind must be one of agent, script")
	}
}

// UpdateJobInput 表示更新请求。
type UpdateJobInput struct {
	Name          *string         `json:"name,omitempty"`
	Schedule      *Schedule       `json:"schedule,omitempty"`
	Instruction   *string         `json:"instruction,omitempty"`
	ExecutionKind *string         `json:"execution_kind,omitempty"`
	SessionTarget *SessionTarget  `json:"session_target,omitempty"`
	Delivery      *DeliveryTarget `json:"delivery,omitempty"`
	Source        *Source         `json:"source,omitempty"`
	OverlapPolicy *string         `json:"overlap_policy,omitempty"`
	Enabled       *bool           `json:"enabled,omitempty"`
}

// HeartbeatConfig 表示持久化 heartbeat 配置。
type HeartbeatConfig struct {
	AgentID      string `json:"agent_id"`
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

// Validate 校验配置。
func (c HeartbeatConfig) Validate() error {
	if strings.TrimSpace(c.AgentID) == "" {
		return errors.New("agent_id is required")
	}
	if c.EverySeconds <= 0 {
		return errors.New("every_seconds must be greater than 0")
	}
	if c.AckMaxChars < 0 {
		return errors.New("ack_max_chars must be greater than or equal to 0")
	}
	switch strings.TrimSpace(c.TargetMode) {
	case "", HeartbeatTargetNone, HeartbeatTargetLast, HeartbeatTargetExplicit:
		return nil
	default:
		return ErrHeartbeatConfigInvalid
	}
}

// Normalized 返回带默认值的配置副本。
func (c HeartbeatConfig) Normalized() HeartbeatConfig {
	result := c
	result.AgentID = strings.TrimSpace(result.AgentID)
	if result.EverySeconds <= 0 {
		result.EverySeconds = 1800
	}
	if result.AckMaxChars < 0 {
		result.AckMaxChars = 300
	}
	result.TargetMode = strings.TrimSpace(result.TargetMode)
	if result.TargetMode == "" {
		result.TargetMode = HeartbeatTargetNone
	}
	return result
}

// DefaultHeartbeatConfig 返回默认 heartbeat 配置。
func DefaultHeartbeatConfig(agentID string) HeartbeatConfig {
	return HeartbeatConfig{
		AgentID:      strings.TrimSpace(agentID),
		Enabled:      false,
		EverySeconds: 1800,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}
}

// HeartbeatStatus 表示运行态和配置快照。
type HeartbeatStatus struct {
	AgentID         string     `json:"agent_id"`
	Enabled         bool       `json:"enabled"`
	EverySeconds    int        `json:"every_seconds"`
	TargetMode      string     `json:"target_mode"`
	AckMaxChars     int        `json:"ack_max_chars"`
	Running         bool       `json:"running"`
	PendingWake     bool       `json:"pending_wake"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LastAckAt       *time.Time `json:"last_ack_at,omitempty"`
	DeliveryError   *string    `json:"delivery_error,omitempty"`
}

// HeartbeatWakeResult 表示手动唤醒返回。
type HeartbeatWakeResult struct {
	AgentID   string `json:"agent_id"`
	Mode      string `json:"mode"`
	Scheduled bool   `json:"scheduled"`
}

// HeartbeatUpdateInput 表示 heartbeat 配置更新请求。
type HeartbeatUpdateInput struct {
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

// HeartbeatWakeRequest 表示唤醒请求。
type HeartbeatWakeRequest struct {
	Mode string  `json:"mode"`
	Text *string `json:"text,omitempty"`
}

// SystemEvent 表示 heartbeat/main-session 消费的系统事件。
type SystemEvent struct {
	EventID    string
	EventType  string
	SourceType string
	SourceID   string
	Payload    string
	Status     string
	CreatedAt  time.Time
}
