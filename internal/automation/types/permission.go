// INPUT: 定时任务运行时工具授权请求、任务授权快照与用户决策。
// OUTPUT: 可持久化的 capability/grant/request 以及任务和 run 的授权状态。
// POS: automation 权限协议真相源；session 只记录路由上下文，不承担授权所有权。
package types

import "time"

const (
	// TaskPermissionStateUninitialized 表示历史任务尚未生成兼容授权快照。
	TaskPermissionStateUninitialized = "uninitialized"
	// TaskPermissionStateReady 表示任务可按计划执行。
	TaskPermissionStateReady = "ready"
	// TaskPermissionStateAwaitingApproval 表示任务正在等待 owner 审批工具或脚本能力。
	TaskPermissionStateAwaitingApproval = "awaiting_approval"
	// TaskPermissionStateAwaitingReauth 表示任务已获执行授权，但 connector 连接需要恢复。
	TaskPermissionStateAwaitingReauth = "awaiting_reauth"
	// TaskPermissionStateAwaitingInput 表示任务缺少只能由用户补充的执行输入。
	TaskPermissionStateAwaitingInput = "awaiting_input"
	// TaskPermissionStateReadyToRetry 表示授权已完成，但已有副作用使自动重放不安全。
	TaskPermissionStateReadyToRetry = "ready_to_retry"
	// TaskPermissionStateDenied 表示 owner 明确拒绝了当前授权请求。
	TaskPermissionStateDenied = "denied"

	// RunBlockStateNone 表示 run 没有用户交互阻塞。
	RunBlockStateNone = ""
	// RunBlockStateAwaitingApproval 表示 run 等待能力审批。
	RunBlockStateAwaitingApproval = "awaiting_approval"
	// RunBlockStateAwaitingReauth 表示 run 等待 connector 重新连接。
	RunBlockStateAwaitingReauth = "awaiting_reauth"
	// RunBlockStateAwaitingInput 表示 run 等待任务配置补充输入。
	RunBlockStateAwaitingInput = "awaiting_input"
	// RunBlockStateReadyToRetry 表示 run 需要 owner 明确确认后才能重放。
	RunBlockStateReadyToRetry = "ready_to_retry"

	// PermissionRequestKindTool 表示运行时工具能力审批。
	PermissionRequestKindTool = "tool"
	// PermissionRequestKindScript 表示 workspace host script 执行审批。
	PermissionRequestKindScript = "script"
	// PermissionRequestKindConnectorReauth 表示 connector 认证恢复请求。
	PermissionRequestKindConnectorReauth = "connector_reauth"
	// PermissionRequestKindHumanInput 表示任务配置缺少用户输入。
	PermissionRequestKindHumanInput = "human_input"

	// PermissionRequestStatusPending 表示请求仍待处理。
	PermissionRequestStatusPending = "pending"
	// PermissionRequestStatusApproved 表示请求已获准。
	PermissionRequestStatusApproved = "approved"
	// PermissionRequestStatusDenied 表示请求被 owner 拒绝。
	PermissionRequestStatusDenied = "denied"
	// PermissionRequestStatusSuperseded 表示任务修订已使请求失效。
	PermissionRequestStatusSuperseded = "superseded"
	// PermissionRequestStatusCancelled 表示任务/run 生命周期结束并取消请求。
	PermissionRequestStatusCancelled = "cancelled"

	// PermissionDecisionAllowOnce 只对同一 logical run 的同一输入指纹授权。
	PermissionDecisionAllowOnce = "allow_once"
	// PermissionDecisionAllowTask 把 capability grant 写入当前任务。
	PermissionDecisionAllowTask = "allow_task"
	// PermissionDecisionDeny 拒绝当前请求并结束被阻塞的 run。
	PermissionDecisionDeny = "deny"
	// PermissionDecisionRetry 在 connector 恢复后重新检查并继续 run。
	PermissionDecisionRetry = "retry"

	// PermissionEffectRead 表示只读能力。
	PermissionEffectRead = "read"
	// PermissionEffectWrite 表示会修改外部或 workspace 状态的能力。
	PermissionEffectWrite = "write"
	// PermissionEffectExecute 表示执行代码、进程或无法安全归类的能力。
	PermissionEffectExecute = "execute"

	// PermissionGrantSourceAgentSnapshot 表示任务创建/迁移时从 Agent AllowedTools 固化。
	PermissionGrantSourceAgentSnapshot = "agent_snapshot"
	// PermissionGrantSourceUserApproval 表示 owner 从任务审批卡显式授权。
	PermissionGrantSourceUserApproval = "user_approval"
	// PermissionGrantSourceDirectUser 表示用户直接创建/修改脚本时的显式确认。
	PermissionGrantSourceDirectUser = "direct_user"
	// PermissionGrantSourceLegacyCompat 表示为旧任务保留既有行为的迁移授权。
	PermissionGrantSourceLegacyCompat = "legacy_compat"
)

// PermissionCapability 是权限匹配的最小稳定单元。
// ToolName 是运行时真名；ConnectorID/Effect/ResourceScope 用于收紧任务级授权；
// InputFingerprint 只用于 run 级一次授权，不写入任务级宽授权。
type PermissionCapability struct {
	ToolName         string `json:"tool_name"`
	ConnectorID      string `json:"connector_id,omitempty"`
	Effect           string `json:"effect"`
	ResourceScope    string `json:"resource_scope,omitempty"`
	InputFingerprint string `json:"input_fingerprint,omitempty"`
}

// TaskPermissionGrant 是任务拥有的持久 capability grant。
type TaskPermissionGrant struct {
	GrantID    string               `json:"grant_id"`
	Capability PermissionCapability `json:"capability"`
	Source     string               `json:"source"`
	ApprovedAt *time.Time           `json:"approved_at,omitempty"`
}

// TaskPermissionPolicy 是任务级授权快照。Revision 同时持久化为独立列用于 CAS。
type TaskPermissionPolicy struct {
	Version     int                   `json:"version"`
	Revision    int                   `json:"revision"`
	Grants      []TaskPermissionGrant `json:"grants"`
	DeniedTools []string              `json:"denied_tools"`
}

// AutomationPermissionRequest 是后台任务需要用户交互时的持久事实。
type AutomationPermissionRequest struct {
	RequestID          string               `json:"request_id"`
	OwnerUserID        string               `json:"-"`
	JobID              string               `json:"job_id"`
	RunID              string               `json:"run_id,omitempty"`
	PolicyRevision     int                  `json:"policy_revision"`
	Kind               string               `json:"kind"`
	Status             string               `json:"status"`
	Decision           string               `json:"decision,omitempty"`
	Capability         PermissionCapability `json:"capability"`
	InputSummary       map[string]any       `json:"input_summary,omitempty"`
	Title              string               `json:"title,omitempty"`
	Description        string               `json:"description,omitempty"`
	Reason             string               `json:"reason,omitempty"`
	SessionKey         string               `json:"session_key,omitempty"`
	DeliverySessionKey string               `json:"delivery_session_key,omitempty"`
	RoundID            string               `json:"round_id,omitempty"`
	ToolUseID          string               `json:"tool_use_id,omitempty"`
	ResumeSafe         bool                 `json:"resume_safe"`
	ResolvedByUserID   string               `json:"resolved_by_user_id,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	ResolvedAt         *time.Time           `json:"resolved_at,omitempty"`
}

// PermissionDecisionInput 是任务审批卡提交的决策与其看到的授权边界快照。
// JobID/RunID/PolicyRevision 必须与 RequestID 对应的持久请求完全一致，
// 防止旧页面上的动作落到已经变化的任务或 logical run。
type PermissionDecisionInput struct {
	Decision       string `json:"decision"`
	JobID          string `json:"job_id"`
	RunID          string `json:"run_id"`
	PolicyRevision int    `json:"policy_revision"`
}

// PermissionResumeInput 是显式确认重试时提交的已批准请求快照。
type PermissionResumeInput struct {
	RequestID      string `json:"request_id"`
	PolicyRevision int    `json:"policy_revision"`
}

// PermissionDecisionResult 返回审批后的权威请求、任务和 run 快照。
type PermissionDecisionResult struct {
	Request       *AutomationPermissionRequest `json:"request,omitempty"`
	Task          ScheduledTask                `json:"task"`
	Run           *ScheduledTaskRun            `json:"run,omitempty"`
	ResumeStarted bool                         `json:"resume_started"`
}
