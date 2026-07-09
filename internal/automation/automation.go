package automation

import (
	"errors"
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
