package types

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
	// DeliveryStatusRetrying 表示一次重投递已被唯一领取，但最终结果尚未得到持久确认。
	DeliveryStatusRetrying = "retrying"
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

	// TriggerKindScheduled 表示由 Scheduler 到点触发。
	TriggerKindScheduled = "scheduled"
	// TriggerKindMisfire 表示接管后处理错过的调度窗口。
	TriggerKindMisfire = "misfire"
	// TriggerKindManual 表示用户立即运行。
	TriggerKindManual = "manual"

	// OverlapPolicySkip 表示已有执行时跳过新触发。
	OverlapPolicySkip = "skip"
	// OverlapPolicyAllow 表示允许同一任务并发执行。
	OverlapPolicyAllow = "allow"

	// ExecutionKindAgent 表示由 Agent 会话执行任务。
	ExecutionKindAgent = "agent"
	// ExecutionKindScript 表示直接在 workspace 中执行脚本任务。
	ExecutionKindScript = "script"

	// TaskDeletionStateDeleting 表示任务已停止接受新操作，正在完成幂等清理。
	TaskDeletionStateDeleting = "deleting"
	// TaskDeletionStateReviewRequired 表示删除已 claim，但活跃外部执行的持有者无法确认，禁止自动收尾。
	TaskDeletionStateReviewRequired = "review_required"

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
	// TaskEventActionExpire 表示任务到达显式过期时间后自动停用。
	TaskEventActionExpire = "expire"
	// TaskEventActionAutoRetryDelivery 表示系统自动重试投递。
	TaskEventActionAutoRetryDelivery = "auto_retry_delivery"
	// TaskEventActionPermissionRequested 表示后台 run 已持久化用户交互请求。
	TaskEventActionPermissionRequested = "permission_requested"
	// TaskEventActionPermissionApproved 表示 owner 已批准能力并允许继续。
	TaskEventActionPermissionApproved = "permission_approved"
	// TaskEventActionPermissionDenied 表示 owner 已拒绝能力请求。
	TaskEventActionPermissionDenied = "permission_denied"
	// TaskEventActionPermissionRetry 表示 connector 恢复或 owner 确认后重试被阻塞 run。
	TaskEventActionPermissionRetry = "permission_retry"
	// TaskEventActionSessionBindingInvalidated 表示 Session 删除后任务已停用并等待重绑。
	TaskEventActionSessionBindingInvalidated = "session_binding_invalidated"

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
	// ErrRunRecoveryConflict 表示恢复提交时任务已不再指向计划释放的 run。
	ErrRunRecoveryConflict = errors.New("scheduled task running state changed before recovery")
	// ErrRunCompletionConflict 表示 terminal observation 已过期或不再拥有 exact run。
	ErrRunCompletionConflict = errors.New("scheduled task run changed before terminal completion")
	// ErrConfigurationVersionConflict 表示配置已被其他写入推进。
	ErrConfigurationVersionConflict = errors.New("automation configuration version conflict")
	// ErrTaskDeleting 表示任务已进入不可逆的删除清理阶段。
	ErrTaskDeleting = errors.New("scheduled task deletion is in progress")
	// ErrTaskDeletionReviewConflict 表示显式停止确认不再对应当前 review_required 删除快照。
	ErrTaskDeletionReviewConflict = errors.New("scheduled task deletion is not waiting for execution stop confirmation")
	// ErrCreateRequestConflict 表示同一个创建幂等键被用于不同意图。
	ErrCreateRequestConflict = errors.New("scheduled task create request conflicts with an existing intent")
	// ErrCreateRequestResultGone 表示创建意图已经提交，但对应任务后来被删除。
	ErrCreateRequestResultGone = errors.New("scheduled task create request committed but its task was later deleted")
	// ErrRuntimeCommandConflict 表示同一个 runtime command request_id 被复用于不同意图。
	ErrRuntimeCommandConflict = errors.New("automation runtime command request conflicts with an existing intent")
	// ErrRuntimeCommandUncertain 表示命令已经开始，但无法安全证明是否完成，禁止自动重放。
	ErrRuntimeCommandUncertain = errors.New("automation runtime command outcome is uncertain; inspect authoritative state before issuing a new command")
	// ErrDeliveryRetryConflict 表示该 run 的投递状态、尝试次数或任务配置已在领取前变化。
	ErrDeliveryRetryConflict = errors.New("automation delivery retry state changed before the attempt was claimed")
	// ErrDeliveryRetryCompletionUnconfirmed 表示外投已发生，但 exact attempt 的最终状态未能确认。
	ErrDeliveryRetryCompletionUnconfirmed = errors.New("automation delivery retry occurred but its durable completion is unconfirmed")
	// ErrDeliveryRetryUnverified 表示上一次外投结果未确认，只能在用户核对后显式重投。
	ErrDeliveryRetryUnverified = errors.New("automation delivery retry outcome is unverified; inspect delivery history before explicitly retrying")
	// ErrPermissionRequestNotFound 表示审批请求不存在或不属于当前 owner。
	ErrPermissionRequestNotFound = errors.New("automation permission request not found")
	// ErrPermissionRequestResolved 表示审批请求已由其他决策处理。
	ErrPermissionRequestResolved = errors.New("automation permission request already resolved")
	// ErrPermissionRequestStale 表示任务修订已使审批请求失效。
	ErrPermissionRequestStale = errors.New("automation permission request is stale")
	// ErrPermissionDecisionInvalid 表示决策不适用于当前请求类型。
	ErrPermissionDecisionInvalid = errors.New("automation permission decision is invalid")
	// ErrPermissionRunNotResumable 表示 run 不在可显式重试状态。
	ErrPermissionRunNotResumable = errors.New("automation permission run is not resumable")
	// ErrPermissionConnectorNotReady 表示 connector 尚未恢复可用连接。
	ErrPermissionConnectorNotReady = errors.New("automation permission connector is not ready")
	// ErrHeartbeatConfigInvalid 表示 heartbeat 配置非法。
	ErrHeartbeatConfigInvalid = errors.New("heartbeat config is invalid")
	// ErrDailyReportDateInvalid 表示日报日期不符合查询契约。
	ErrDailyReportDateInvalid = errors.New("daily report date is invalid")
	// ErrDailyReportTimezoneInvalid 表示日报时区无法解析。
	ErrDailyReportTimezoneInvalid = errors.New("daily report timezone is invalid")
	// ErrHeartbeatWakeRequestConflict 表示同 owner 的 durable wake request_id 被复用于不同意图。
	ErrHeartbeatWakeRequestConflict = errors.New("heartbeat wake request conflicts with an existing intent")
)
