// INPUT: 单 Agent、子智能体、Room 与 Goal continuation 的结构化执行状态。
// OUTPUT: Execution、Plan/Work/Assignment/Attempt、Work/Review/Cancellation dispatch、Submission/Acceptance 与事件协议。
// POS: 跨 MCP/runtime/Room/Goal 边界共享的 Execution Orchestration 真相模型。
package protocol

import (
	"strings"
	"time"
)

// ExecutionStatus 表示一次实际执行的生命周期。
type ExecutionStatus string

const (
	ExecutionStatusActive     ExecutionStatus = "active"
	ExecutionStatusWaiting    ExecutionStatus = "waiting"
	ExecutionStatusPaused     ExecutionStatus = "paused"
	ExecutionStatusCompleted  ExecutionStatus = "completed"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusCancelled  ExecutionStatus = "cancelled"
	ExecutionStatusSuperseded ExecutionStatus = "superseded"
)

// ExecutionScopeKind 表示 Execution 的用户可见协作作用域。
type ExecutionScopeKind string

const (
	ExecutionScopeDM   ExecutionScopeKind = "dm"
	ExecutionScopeRoom ExecutionScopeKind = "room"
)

// ExecutionOrigin 表示 Execution 的触发来源。
type ExecutionOrigin string

const (
	ExecutionOriginUserRequest      ExecutionOrigin = "user_request"
	ExecutionOriginGoalContinuation ExecutionOrigin = "goal_continuation"
	ExecutionOriginRecovery         ExecutionOrigin = "recovery"
	ExecutionOriginSystem           ExecutionOrigin = "system"
)

// GoalActivationOrigin 表示 transient Execution 何以绑定 Goal。
type GoalActivationOrigin string

const (
	GoalActivationOriginUserExplicit     GoalActivationOrigin = "user_explicit"
	GoalActivationOriginAdaptiveInitial  GoalActivationOrigin = "adaptive_initial"
	GoalActivationOriginAdaptivePromoted GoalActivationOrigin = "adaptive_promoted"
)

// GoalActivationReason 表示 Goal 持续性边界的确定原因。
type GoalActivationReason string

const (
	GoalActivationReasonPersistenceRequested  GoalActivationReason = "persistence_requested"
	GoalActivationReasonObservedBoundary      GoalActivationReason = "observed_boundary"
	GoalActivationReasonRoomDependencyChain   GoalActivationReason = "room_dependency_chain"
	GoalActivationReasonExternalWait          GoalActivationReason = "external_wait"
	GoalActivationReasonScheduledRetry        GoalActivationReason = "scheduled_retry"
	GoalActivationReasonContextBoundary       GoalActivationReason = "context_boundary"
	GoalActivationReasonRecoveryRequired      GoalActivationReason = "recovery_required"
	GoalActivationReasonSubstantialComplexity GoalActivationReason = "substantial_complexity"
)

// Execution 表示一次可恢复、可选绑定 Goal 的实际推进。
//
// Active Plan 只由 ExecutionPlanRevision.Status 表达；本对象不复制 active plan identity。
type Execution struct {
	ID                    string               `json:"id"`
	OwnerUserID           string               `json:"owner_user_id"`
	SessionKey            string               `json:"session_key"`
	ScopeKind             ExecutionScopeKind   `json:"scope_kind"`
	RoomID                string               `json:"room_id,omitempty"`
	ConversationID        string               `json:"conversation_id,omitempty"`
	CoordinatorAgentID    string               `json:"coordinator_agent_id,omitempty"`
	Origin                ExecutionOrigin      `json:"origin"`
	Objective             string               `json:"objective"`
	CompletionCriteria    []string             `json:"completion_criteria,omitempty"`
	GoalID                string               `json:"goal_id,omitempty"`
	GoalObjectiveRevision int64                `json:"goal_objective_revision,omitempty"`
	GoalActivationOrigin  GoalActivationOrigin `json:"goal_activation_origin,omitempty"`
	GoalActivationReason  GoalActivationReason `json:"goal_activation_reason,omitempty"`
	RecoveryOfExecutionID string               `json:"recovery_of_execution_id,omitempty"`
	ReplacesExecutionID   string               `json:"replaces_execution_id,omitempty"`
	RootRoundID           string               `json:"root_round_id,omitempty"`
	TriggerMessageID      string               `json:"trigger_message_id,omitempty"`
	Status                ExecutionStatus      `json:"status"`
	Version               int64                `json:"version"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
	CompletedAt           *time.Time           `json:"completed_at,omitempty"`
	Metadata              map[string]any       `json:"metadata,omitempty"`
}

// PlanRevisionStatus 表示 immutable Plan revision 的生命周期。
//
// Plan 内容一经创建不可修改；这里只允许 proposed -> active/cancelled 与
// active -> superseded 的生命周期迁移。
type PlanRevisionStatus string

const (
	PlanRevisionStatusProposed   PlanRevisionStatus = "proposed"
	PlanRevisionStatusActive     PlanRevisionStatus = "active"
	PlanRevisionStatusSuperseded PlanRevisionStatus = "superseded"
	PlanRevisionStatusCancelled  PlanRevisionStatus = "cancelled"
)

// ExecutionPlanRevision 表示 Execution 的一个不可变工作图版本。
type ExecutionPlanRevision struct {
	ID               string             `json:"id"`
	ExecutionID      string             `json:"execution_id"`
	Revision         int64              `json:"revision"`
	Status           PlanRevisionStatus `json:"status"`
	BasePlanID       string             `json:"base_plan_id,omitempty"`
	CreatedByAgentID string             `json:"created_by_agent_id,omitempty"`
	RevisionReason   string             `json:"revision_reason,omitempty"`
	Version          int64              `json:"version"`
	CreatedAt        time.Time          `json:"created_at"`
	ActivatedAt      *time.Time         `json:"activated_at,omitempty"`
	SupersededAt     *time.Time         `json:"superseded_at,omitempty"`
	Metadata         map[string]any     `json:"metadata,omitempty"`
}

// WorkItemKind 区分生产、复核、验证与最终整合，避免把有意复核误判为重复劳动。
type WorkItemKind string

const (
	WorkItemKindProduce   WorkItemKind = "produce"
	WorkItemKindReview    WorkItemKind = "review"
	WorkItemKindVerify    WorkItemKind = "verify"
	WorkItemKindIntegrate WorkItemKind = "integrate"
)

// WorkItemStatus 只表达 stable Work Item identity 的生命周期。
//
// ready、assigned、running、submitted 与 accepted 均由当前 Plan、Assignment、
// Attempt、Submission 和 Acceptance 派生，不在 lifecycle state 上复制。
type WorkItemStatus string

const (
	WorkItemStatusOpen         WorkItemStatus = "open"
	WorkItemStatusWaitingInput WorkItemStatus = "waiting_input"
	WorkItemStatusCancelled    WorkItemStatus = "cancelled"
	WorkItemStatusSuperseded   WorkItemStatus = "superseded"
)

// WorkItem 表示跨 Plan revision 保持稳定且不可变的交付单元身份。
type WorkItem struct {
	ID          string         `json:"id"`
	ExecutionID string         `json:"execution_id"`
	LogicalKey  string         `json:"logical_key"`
	Kind        WorkItemKind   `json:"kind"`
	CreatedAt   time.Time      `json:"created_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// WorkItemState 是 stable Work Item 唯一的 mutable lifecycle state。
type WorkItemState struct {
	WorkItemID    string         `json:"work_item_id"`
	ExecutionID   string         `json:"execution_id"`
	CurrentSpecID string         `json:"current_spec_id"`
	Status        WorkItemStatus `json:"status"`
	BlockReason   string         `json:"block_reason,omitempty"`
	NeededInput   string         `json:"needed_input,omitempty"`
	Version       int64          `json:"version"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// WorkOutputScopeMode 表示产出范围是否排斥其他生产型 Work Item。
type WorkOutputScopeMode string

const (
	WorkOutputScopeExclusive WorkOutputScopeMode = "exclusive"
	WorkOutputScopeShared    WorkOutputScopeMode = "shared"
)

// WorkOutputScope 描述 Work Item spec 声明的文件、资源或语义产出范围。
type WorkOutputScope struct {
	Scope string              `json:"scope"`
	Mode  WorkOutputScopeMode `json:"mode"`
}

// WorkItemSpec 表示 Work Item 的一个 immutable 交付契约版本。
type WorkItemSpec struct {
	ID                 string         `json:"id"`
	WorkItemID         string         `json:"work_item_id"`
	ExecutionID        string         `json:"execution_id"`
	Version            int64          `json:"version"`
	Subject            string         `json:"subject"`
	Objective          string         `json:"objective"`
	Deliverable        string         `json:"deliverable"`
	AcceptanceCriteria []string       `json:"acceptance_criteria"`
	InputRefs          []string       `json:"input_refs,omitempty"`
	SpecHash           string         `json:"spec_hash"`
	CreatedByAgentID   string         `json:"created_by_agent_id,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// ExecutionPlanItem 把 stable Work Item 的一个 immutable spec 纳入 Plan revision。
type ExecutionPlanItem struct {
	PlanID           string    `json:"plan_id"`
	ExecutionID      string    `json:"execution_id"`
	WorkItemID       string    `json:"work_item_id"`
	SpecID           string    `json:"spec_id"`
	ParentWorkItemID string    `json:"parent_work_item_id,omitempty"`
	Required         bool      `json:"required"`
	Terminal         bool      `json:"terminal,omitempty"`
	Position         int       `json:"position"`
	CreatedAt        time.Time `json:"created_at"`
}

// WorkDependencyKind 表示下游是否必须等待上游 Acceptance。
type WorkDependencyKind string

const (
	WorkDependencyHard WorkDependencyKind = "hard"
	WorkDependencySoft WorkDependencyKind = "soft"
)

// ExecutionPlanDependency 表示同一个 immutable Plan revision 内的依赖边。
type ExecutionPlanDependency struct {
	PlanID              string             `json:"plan_id"`
	ExecutionID         string             `json:"execution_id"`
	WorkItemID          string             `json:"work_item_id"`
	DependsOnWorkItemID string             `json:"depends_on_work_item_id"`
	Kind                WorkDependencyKind `json:"kind"`
	CreatedAt           time.Time          `json:"created_at"`
}

// ExecutionPlanOutputClaim 是 Plan 中用于判定并行产出冲突的规范化资源声明。
type ExecutionPlanOutputClaim struct {
	PlanID      string              `json:"plan_id"`
	ExecutionID string              `json:"execution_id"`
	WorkItemID  string              `json:"work_item_id"`
	SpecID      string              `json:"spec_id"`
	Scope       string              `json:"scope"`
	Mode        WorkOutputScopeMode `json:"mode"`
	CreatedAt   time.Time           `json:"created_at"`
}

// AssignmentStrategy 表示 owner 如何完成自己的 Work Item。
type AssignmentStrategy string

const (
	AssignmentStrategySelf       AssignmentStrategy = "self"
	AssignmentStrategyRoomMember AssignmentStrategy = "room_member"
)

// WorkAssignmentStatus 表示责任归属的生命周期。
type WorkAssignmentStatus string

const (
	WorkAssignmentStatusAssigned  WorkAssignmentStatus = "assigned"
	WorkAssignmentStatusActive    WorkAssignmentStatus = "active"
	WorkAssignmentStatusReleased  WorkAssignmentStatus = "released"
	WorkAssignmentStatusCompleted WorkAssignmentStatus = "completed"
	WorkAssignmentStatusCancelled WorkAssignmentStatus = "cancelled"
	WorkAssignmentStatusRevoked   WorkAssignmentStatus = "revoked"
)

// WorkAssignment 表示当前由哪个 Agent 对 Plan 中的 Work Item spec 负责。
type WorkAssignment struct {
	ID                string               `json:"id"`
	ExecutionID       string               `json:"execution_id"`
	PlanID            string               `json:"plan_id"`
	WorkItemID        string               `json:"work_item_id"`
	SpecID            string               `json:"spec_id"`
	OwnerAgentID      string               `json:"owner_agent_id"`
	AssignedByAgentID string               `json:"assigned_by_agent_id,omitempty"`
	ReturnToAgentID   string               `json:"return_to_agent_id,omitempty"`
	Strategy          AssignmentStrategy   `json:"strategy"`
	Status            WorkAssignmentStatus `json:"status"`
	AssignmentReason  string               `json:"assignment_reason,omitempty"`
	TakeoverReason    string               `json:"takeover_reason,omitempty"`
	Version           int64                `json:"version"`
	AssignedAt        time.Time            `json:"assigned_at"`
	ActivatedAt       *time.Time           `json:"activated_at,omitempty"`
	ReleasedAt        *time.Time           `json:"released_at,omitempty"`
	CompletedAt       *time.Time           `json:"completed_at,omitempty"`
	Metadata          map[string]any       `json:"metadata,omitempty"`
}

// ExecutionDispatchKind 表示 Assignment 如何被交付给实际执行者。
type ExecutionDispatchKind string

const (
	ExecutionDispatchRoomPublic   ExecutionDispatchKind = "room_public"
	ExecutionDispatchRoomDirected ExecutionDispatchKind = "room_directed"
	ExecutionDispatchSubagent     ExecutionDispatchKind = "subagent"
)

// ExecutionDispatchStatus 表示事务性 Assignment outbox 的投递状态。
type ExecutionDispatchStatus string

const (
	ExecutionDispatchStatusPending   ExecutionDispatchStatus = "pending"
	ExecutionDispatchStatusClaimed   ExecutionDispatchStatus = "claimed"
	ExecutionDispatchStatusDelivered ExecutionDispatchStatus = "delivered"
	ExecutionDispatchStatusCancelled ExecutionDispatchStatus = "cancelled"
	ExecutionDispatchStatusFailed    ExecutionDispatchStatus = "failed"
)

// ExecutionDispatch 把 SQL Assignment 与 Room handoff/queue 或 subagent 调用幂等绑定。
type ExecutionDispatch struct {
	ID               string                  `json:"id"`
	ExecutionID      string                  `json:"execution_id"`
	PlanID           string                  `json:"plan_id"`
	WorkItemID       string                  `json:"work_item_id"`
	SpecID           string                  `json:"spec_id"`
	AssignmentID     string                  `json:"assignment_id"`
	CommandID        string                  `json:"command_id"`
	DedupeKey        string                  `json:"dedupe_key"`
	TargetAgentID    string                  `json:"target_agent_id"`
	Kind             ExecutionDispatchKind   `json:"kind"`
	Status           ExecutionDispatchStatus `json:"status"`
	Instruction      string                  `json:"instruction"`
	HandoffID        string                  `json:"handoff_id,omitempty"`
	QueueItemID      string                  `json:"queue_item_id,omitempty"`
	DeliveryAttempts int                     `json:"delivery_attempts"`
	Version          int64                   `json:"version"`
	AvailableAt      time.Time               `json:"available_at"`
	LeaseOwner       string                  `json:"lease_owner,omitempty"`
	LeaseExpiresAt   *time.Time              `json:"lease_expires_at,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	ClaimedAt        *time.Time              `json:"claimed_at,omitempty"`
	DeliveredAt      *time.Time              `json:"delivered_at,omitempty"`
	LastError        string                  `json:"last_error,omitempty"`
	Metadata         map[string]any          `json:"metadata,omitempty"`
}

// ExecutionWorkBinding 是 Dispatch、Room queue/slot 与 runtime context 共用的
// 稳定工作身份。聊天正文只负责让人和模型理解 instruction，不参与关联。
type ExecutionWorkBinding struct {
	ExecutionID  string `json:"execution_id"`
	PlanID       string `json:"plan_id"`
	WorkItemID   string `json:"work_item_id"`
	SpecID       string `json:"spec_id"`
	AssignmentID string `json:"assignment_id"`
	AttemptID    string `json:"attempt_id"`
	DispatchID   string `json:"dispatch_id"`
}

// Normalized 返回去除首尾空白的绑定副本；nil 视为空绑定。binding 的清洗
// 只发生在 ingress（反序列化 / 构造）这一处，下游一律信任已清洗的值。
func (b *ExecutionWorkBinding) Normalized() ExecutionWorkBinding {
	if b == nil {
		return ExecutionWorkBinding{}
	}
	return ExecutionWorkBinding{
		ExecutionID:  strings.TrimSpace(b.ExecutionID),
		PlanID:       strings.TrimSpace(b.PlanID),
		WorkItemID:   strings.TrimSpace(b.WorkItemID),
		SpecID:       strings.TrimSpace(b.SpecID),
		AssignmentID: strings.TrimSpace(b.AssignmentID),
		AttemptID:    strings.TrimSpace(b.AttemptID),
		DispatchID:   strings.TrimSpace(b.DispatchID),
	}
}

// Complete 判断 binding 是否携带全部必需身份字段（Attempt 之后均为必填，
// DispatchID 允许为空）。对任意输入都成立，不必先 Normalized。
func (b *ExecutionWorkBinding) Complete() bool {
	normalized := b.Normalized()
	return normalized.ExecutionID != "" &&
		normalized.PlanID != "" &&
		normalized.WorkItemID != "" &&
		normalized.SpecID != "" &&
		normalized.AssignmentID != "" &&
		normalized.AttemptID != ""
}

// AttemptExecutorKind 区分责任 Agent 自己与其临时子智能体。
type AttemptExecutorKind string

const (
	AttemptExecutorAgent    AttemptExecutorKind = "agent"
	AttemptExecutorSubagent AttemptExecutorKind = "subagent"
)

// WorkAttemptStatus 表示一次真实执行尝试的生命周期。
type WorkAttemptStatus string

const (
	WorkAttemptStatusPending     WorkAttemptStatus = "pending"
	WorkAttemptStatusRunning     WorkAttemptStatus = "running"
	WorkAttemptStatusSucceeded   WorkAttemptStatus = "succeeded"
	WorkAttemptStatusFailed      WorkAttemptStatus = "failed"
	WorkAttemptStatusInterrupted WorkAttemptStatus = "interrupted"
	WorkAttemptStatusCancelled   WorkAttemptStatus = "cancelled"
	WorkAttemptStatusTimedOut    WorkAttemptStatus = "timed_out"
)

// WorkAttempt 表示 Assignment 的一次可恢复执行尝试。
//
// RuntimeSessionKey 是 Nexus runtime manager key；RoomSessionID 是 SQL Room session row；
// SDKSessionID 是 SDK resume identity。RuntimeRoundID 与 RootRoundID 不得互换。
// Runtime round 三元组与 AssignmentID 共同唯一标识 root Attempt；同一物理
// round 可以串行承载不同 Assignment，但不能为同一 Assignment 重复建 root
// Attempt。child Attempt 合法共享该身份，并由 parent_attempt_id + tool_use_id
// 精确区分。
type WorkAttempt struct {
	ID                  string              `json:"id"`
	ExecutionID         string              `json:"execution_id"`
	PlanID              string              `json:"plan_id"`
	WorkItemID          string              `json:"work_item_id"`
	SpecID              string              `json:"spec_id"`
	AssignmentID        string              `json:"assignment_id"`
	DispatchID          string              `json:"dispatch_id,omitempty"`
	ParentAttemptID     string              `json:"parent_attempt_id,omitempty"`
	ExecutorKind        AttemptExecutorKind `json:"executor_kind"`
	ExecutorAgentID     string              `json:"executor_agent_id,omitempty"`
	ParentAgentID       string              `json:"parent_agent_id,omitempty"`
	RuntimeSessionKey   string              `json:"runtime_session_key,omitempty"`
	RoomSessionID       string              `json:"room_session_id,omitempty"`
	SDKSessionID        string              `json:"sdk_session_id,omitempty"`
	RuntimeRoundID      string              `json:"runtime_round_id,omitempty"`
	RootRoundID         string              `json:"root_round_id,omitempty"`
	AgentRoundID        string              `json:"agent_round_id,omitempty"`
	ChildSessionID      string              `json:"child_session_id,omitempty"`
	SDKTaskID           string              `json:"sdk_task_id,omitempty"`
	ToolUseID           string              `json:"tool_use_id,omitempty"`
	Status              WorkAttemptStatus   `json:"status"`
	FailureReason       string              `json:"failure_reason,omitempty"`
	Version             int64               `json:"version"`
	CreatedAt           time.Time           `json:"created_at"`
	StartedAt           *time.Time          `json:"started_at,omitempty"`
	FinishedAt          *time.Time          `json:"finished_at,omitempty"`
	ParentRoundExitedAt *time.Time          `json:"parent_round_exited_at,omitempty"`
	ReconcileAfter      *time.Time          `json:"reconcile_after,omitempty"`
	Metadata            map[string]any      `json:"metadata,omitempty"`
}

// ExecutionCancellationTargetKind 表示 SQL terminal mutation 捕获到的物理
// runtime 目标。not_started/unavailable 是显式能力边界，不能伪装成已中断。
type ExecutionCancellationTargetKind string

const (
	ExecutionCancellationTargetRoomSlot     ExecutionCancellationTargetKind = "room_slot"
	ExecutionCancellationTargetRuntimeRound ExecutionCancellationTargetKind = "runtime_round"
	ExecutionCancellationTargetNotStarted   ExecutionCancellationTargetKind = "not_started"
	ExecutionCancellationTargetUnavailable  ExecutionCancellationTargetKind = "unavailable"
)

// ExecutionCancellationDispatchStatus 表示 Attempt physical cancellation
// outbox 的 lease 与最终投递状态。
type ExecutionCancellationDispatchStatus string

const (
	ExecutionCancellationDispatchPending     ExecutionCancellationDispatchStatus = "pending"
	ExecutionCancellationDispatchClaimed     ExecutionCancellationDispatchStatus = "claimed"
	ExecutionCancellationDispatchDelivered   ExecutionCancellationDispatchStatus = "delivered"
	ExecutionCancellationDispatchNotRequired ExecutionCancellationDispatchStatus = "not_required"
	ExecutionCancellationDispatchUnsupported ExecutionCancellationDispatchStatus = "unsupported"
	ExecutionCancellationDispatchCancelled   ExecutionCancellationDispatchStatus = "cancelled"
	ExecutionCancellationDispatchFailed      ExecutionCancellationDispatchStatus = "failed"
)

// ExecutionCancellationOutcome 区分 provider 已接受 interrupt、只取消 Nexus
// local round、目标已经结束与显式能力限制，禁止把 context cancel 伪报为 provider stop。
type ExecutionCancellationOutcome string

const (
	ExecutionCancellationOutcomeProviderInterrupted ExecutionCancellationOutcome = "provider_interrupted"
	ExecutionCancellationOutcomeLocalRoundCancelled ExecutionCancellationOutcome = "local_round_cancelled"
	ExecutionCancellationOutcomeAlreadyEnded        ExecutionCancellationOutcome = "already_ended"
	ExecutionCancellationOutcomeStaleTarget         ExecutionCancellationOutcome = "stale_target"
	ExecutionCancellationOutcomeNotStarted          ExecutionCancellationOutcome = "not_started"
	ExecutionCancellationOutcomeUnsupported         ExecutionCancellationOutcome = "unsupported"
)

// ExecutionCancellationDispatch 是 SQL Attempt terminalization 与物理 runtime
// interrupt 之间的事务性 outbox。AttemptID 是被终止的逻辑 Attempt；
// RuntimeAttemptID 是真正承载 runtime 的 root Attempt，subagent 两者可以不同。
type ExecutionCancellationDispatch struct {
	ID                string                              `json:"id"`
	ExecutionID       string                              `json:"execution_id"`
	PlanID            string                              `json:"plan_id"`
	WorkItemID        string                              `json:"work_item_id"`
	SpecID            string                              `json:"spec_id"`
	AssignmentID      string                              `json:"assignment_id"`
	AttemptID         string                              `json:"attempt_id"`
	RuntimeAttemptID  string                              `json:"runtime_attempt_id"`
	DispatchID        string                              `json:"dispatch_id,omitempty"`
	CommandID         string                              `json:"command_id"`
	DedupeKey         string                              `json:"dedupe_key"`
	ScopeKind         ExecutionScopeKind                  `json:"scope_kind"`
	ScopeSessionKey   string                              `json:"scope_session_key"`
	RoomID            string                              `json:"room_id,omitempty"`
	ConversationID    string                              `json:"conversation_id,omitempty"`
	ExecutorKind      AttemptExecutorKind                 `json:"executor_kind"`
	TargetKind        ExecutionCancellationTargetKind     `json:"target_kind"`
	TargetAgentID     string                              `json:"target_agent_id,omitempty"`
	RuntimeSessionKey string                              `json:"runtime_session_key,omitempty"`
	RoomSessionID     string                              `json:"room_session_id,omitempty"`
	SDKSessionID      string                              `json:"sdk_session_id,omitempty"`
	RuntimeRoundID    string                              `json:"runtime_round_id,omitempty"`
	RootRoundID       string                              `json:"root_round_id,omitempty"`
	AgentRoundID      string                              `json:"agent_round_id,omitempty"`
	ChildSessionID    string                              `json:"child_session_id,omitempty"`
	SDKTaskID         string                              `json:"sdk_task_id,omitempty"`
	ToolUseID         string                              `json:"tool_use_id,omitempty"`
	Status            ExecutionCancellationDispatchStatus `json:"status"`
	Reason            string                              `json:"reason"`
	LimitationCode    string                              `json:"limitation_code,omitempty"`
	Outcome           ExecutionCancellationOutcome        `json:"outcome,omitempty"`
	Receipt           string                              `json:"receipt,omitempty"`
	DeliveryAttempts  int                                 `json:"delivery_attempts"`
	Version           int64                               `json:"version"`
	AvailableAt       time.Time                           `json:"available_at"`
	LeaseOwner        string                              `json:"lease_owner,omitempty"`
	LeaseExpiresAt    *time.Time                          `json:"lease_expires_at,omitempty"`
	CreatedAt         time.Time                           `json:"created_at"`
	UpdatedAt         time.Time                           `json:"updated_at"`
	ClaimedAt         *time.Time                          `json:"claimed_at,omitempty"`
	DeliveredAt       *time.Time                          `json:"delivered_at,omitempty"`
	LastError         string                              `json:"last_error,omitempty"`
	Metadata          map[string]any                      `json:"metadata,omitempty"`
}

// Normalized 返回去除首尾空白的 dispatch 副本。清洗只发生在构造（outbox 写入）
// 一次；target kind 判定与 consumer 匹配一律基于已清洗值。
func (d ExecutionCancellationDispatch) Normalized() ExecutionCancellationDispatch {
	d.ID = strings.TrimSpace(d.ID)
	d.ExecutionID = strings.TrimSpace(d.ExecutionID)
	d.PlanID = strings.TrimSpace(d.PlanID)
	d.WorkItemID = strings.TrimSpace(d.WorkItemID)
	d.SpecID = strings.TrimSpace(d.SpecID)
	d.AssignmentID = strings.TrimSpace(d.AssignmentID)
	d.AttemptID = strings.TrimSpace(d.AttemptID)
	d.RuntimeAttemptID = strings.TrimSpace(d.RuntimeAttemptID)
	d.DispatchID = strings.TrimSpace(d.DispatchID)
	d.CommandID = strings.TrimSpace(d.CommandID)
	d.DedupeKey = strings.TrimSpace(d.DedupeKey)
	d.ScopeSessionKey = strings.TrimSpace(d.ScopeSessionKey)
	d.RoomID = strings.TrimSpace(d.RoomID)
	d.ConversationID = strings.TrimSpace(d.ConversationID)
	d.TargetAgentID = strings.TrimSpace(d.TargetAgentID)
	d.RuntimeSessionKey = strings.TrimSpace(d.RuntimeSessionKey)
	d.RoomSessionID = strings.TrimSpace(d.RoomSessionID)
	d.SDKSessionID = strings.TrimSpace(d.SDKSessionID)
	d.RuntimeRoundID = strings.TrimSpace(d.RuntimeRoundID)
	d.RootRoundID = strings.TrimSpace(d.RootRoundID)
	d.AgentRoundID = strings.TrimSpace(d.AgentRoundID)
	d.ChildSessionID = strings.TrimSpace(d.ChildSessionID)
	d.SDKTaskID = strings.TrimSpace(d.SDKTaskID)
	d.ToolUseID = strings.TrimSpace(d.ToolUseID)
	d.Reason = strings.TrimSpace(d.Reason)
	d.LimitationCode = strings.TrimSpace(d.LimitationCode)
	d.Receipt = strings.TrimSpace(d.Receipt)
	d.LeaseOwner = strings.TrimSpace(d.LeaseOwner)
	d.LastError = strings.TrimSpace(d.LastError)
	return d
}

// ExecutionCancellationBinding 是 consumer 使用的不可变精确 interrupt identity。
// 它同时保留逻辑 Attempt 与实际 runtime root Attempt，避免 subagent 回收误伤 successor。
type ExecutionCancellationBinding struct {
	ExecutionID       string                          `json:"execution_id"`
	PlanID            string                          `json:"plan_id"`
	WorkItemID        string                          `json:"work_item_id"`
	SpecID            string                          `json:"spec_id"`
	AssignmentID      string                          `json:"assignment_id"`
	AttemptID         string                          `json:"attempt_id"`
	RuntimeAttemptID  string                          `json:"runtime_attempt_id"`
	DispatchID        string                          `json:"dispatch_id,omitempty"`
	TargetKind        ExecutionCancellationTargetKind `json:"target_kind"`
	TargetAgentID     string                          `json:"target_agent_id,omitempty"`
	ScopeSessionKey   string                          `json:"scope_session_key"`
	RoomID            string                          `json:"room_id,omitempty"`
	ConversationID    string                          `json:"conversation_id,omitempty"`
	RuntimeSessionKey string                          `json:"runtime_session_key,omitempty"`
	RuntimeRoundID    string                          `json:"runtime_round_id,omitempty"`
	AgentRoundID      string                          `json:"agent_round_id,omitempty"`
}

// WorkSubmission 表示 worker 对 immutable Work Item spec 的完成声明与证据。
//
// Submission 本身不可修改；review decision 独立写入 WorkAcceptance。
type WorkSubmission struct {
	ID               string         `json:"id"`
	ExecutionID      string         `json:"execution_id"`
	PlanID           string         `json:"plan_id"`
	WorkItemID       string         `json:"work_item_id"`
	SpecID           string         `json:"spec_id"`
	AssignmentID     string         `json:"assignment_id"`
	AttemptID        string         `json:"attempt_id"`
	Sequence         int64          `json:"sequence"`
	SubmitterAgentID string         `json:"submitter_agent_id"`
	ResultSummary    string         `json:"result_summary"`
	ResultRefs       []string       `json:"result_refs,omitempty"`
	Evidence         []string       `json:"evidence,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// ExecutionReviewDispatchStatus 表示 Submission 回交 reviewer 的独立事务性 outbox 状态。
type ExecutionReviewDispatchStatus string

const (
	ExecutionReviewDispatchStatusPending   ExecutionReviewDispatchStatus = "pending"
	ExecutionReviewDispatchStatusClaimed   ExecutionReviewDispatchStatus = "claimed"
	ExecutionReviewDispatchStatusDelivered ExecutionReviewDispatchStatus = "delivered"
	ExecutionReviewDispatchStatusCancelled ExecutionReviewDispatchStatus = "cancelled"
	ExecutionReviewDispatchStatusFailed    ExecutionReviewDispatchStatus = "failed"
)

// ExecutionReviewDispatch 把 immutable Submission 与 Room reviewer 的 durable
// handoff/queue 幂等绑定。它不是 worker Assignment，也不创建 reviewer Attempt。
type ExecutionReviewDispatch struct {
	ID               string                        `json:"id"`
	ExecutionID      string                        `json:"execution_id"`
	PlanID           string                        `json:"plan_id"`
	WorkItemID       string                        `json:"work_item_id"`
	SpecID           string                        `json:"spec_id"`
	AssignmentID     string                        `json:"assignment_id"`
	SubmissionID     string                        `json:"submission_id"`
	CommandID        string                        `json:"command_id"`
	DedupeKey        string                        `json:"dedupe_key"`
	TargetAgentID    string                        `json:"target_agent_id"`
	Status           ExecutionReviewDispatchStatus `json:"status"`
	Instruction      string                        `json:"instruction"`
	HandoffID        string                        `json:"handoff_id,omitempty"`
	QueueItemID      string                        `json:"queue_item_id,omitempty"`
	DeliveryAttempts int                           `json:"delivery_attempts"`
	Version          int64                         `json:"version"`
	AvailableAt      time.Time                     `json:"available_at"`
	LeaseOwner       string                        `json:"lease_owner,omitempty"`
	LeaseExpiresAt   *time.Time                    `json:"lease_expires_at,omitempty"`
	CreatedAt        time.Time                     `json:"created_at"`
	UpdatedAt        time.Time                     `json:"updated_at"`
	ClaimedAt        *time.Time                    `json:"claimed_at,omitempty"`
	DeliveredAt      *time.Time                    `json:"delivered_at,omitempty"`
	LastError        string                        `json:"last_error,omitempty"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

// ExecutionReviewBinding 是 review-return outbox、Room queue/slot 与 reviewer
// runtime 共用的 capability identity；它刻意不包含 worker Attempt/Dispatch。
type ExecutionReviewBinding struct {
	ExecutionID      string `json:"execution_id"`
	PlanID           string `json:"plan_id"`
	WorkItemID       string `json:"work_item_id"`
	SpecID           string `json:"spec_id"`
	AssignmentID     string `json:"assignment_id"`
	SubmissionID     string `json:"submission_id"`
	ReviewDispatchID string `json:"review_dispatch_id"`
	TargetAgentID    string `json:"target_agent_id"`
}

// Normalized 返回去除首尾空白的绑定副本；nil 视为空绑定。清洗只发生在
// ingress（反序列化 / 构造）这一处，下游一律信任已清洗的值。
func (b *ExecutionReviewBinding) Normalized() ExecutionReviewBinding {
	if b == nil {
		return ExecutionReviewBinding{}
	}
	return ExecutionReviewBinding{
		ExecutionID:      strings.TrimSpace(b.ExecutionID),
		PlanID:           strings.TrimSpace(b.PlanID),
		WorkItemID:       strings.TrimSpace(b.WorkItemID),
		SpecID:           strings.TrimSpace(b.SpecID),
		AssignmentID:     strings.TrimSpace(b.AssignmentID),
		SubmissionID:     strings.TrimSpace(b.SubmissionID),
		ReviewDispatchID: strings.TrimSpace(b.ReviewDispatchID),
		TargetAgentID:    strings.TrimSpace(b.TargetAgentID),
	}
}

// Complete 判断 binding 是否携带全部必需身份字段；对任意输入都成立。
func (b *ExecutionReviewBinding) Complete() bool {
	normalized := b.Normalized()
	return normalized.ExecutionID != "" &&
		normalized.PlanID != "" &&
		normalized.WorkItemID != "" &&
		normalized.SpecID != "" &&
		normalized.AssignmentID != "" &&
		normalized.SubmissionID != "" &&
		normalized.ReviewDispatchID != "" &&
		normalized.TargetAgentID != ""
}

// WorkAcceptanceDecision 表示 reviewer 对一个 immutable Submission 的唯一决定。
type WorkAcceptanceDecision string

const (
	WorkAcceptanceAccepted         WorkAcceptanceDecision = "accepted"
	WorkAcceptanceRejected         WorkAcceptanceDecision = "rejected"
	WorkAcceptanceChangesRequested WorkAcceptanceDecision = "changes_requested"
)

// WorkReviewerKind 表示 Acceptance 的授权主体类型。
type WorkReviewerKind string

const (
	WorkReviewerAgent  WorkReviewerKind = "agent"
	WorkReviewerUser   WorkReviewerKind = "user"
	WorkReviewerSystem WorkReviewerKind = "system"
	WorkReviewerPolicy WorkReviewerKind = "policy"
)

// WorkAcceptanceCriterionResult 记录单条验收标准的判断与证据。
type WorkAcceptanceCriterionResult struct {
	Criterion string   `json:"criterion"`
	Passed    bool     `json:"passed"`
	Evidence  []string `json:"evidence,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// WorkAcceptance 是 append-only 的 Submission review decision。
type WorkAcceptance struct {
	ID              string                          `json:"id"`
	ExecutionID     string                          `json:"execution_id"`
	PlanID          string                          `json:"plan_id"`
	WorkItemID      string                          `json:"work_item_id"`
	SpecID          string                          `json:"spec_id"`
	AssignmentID    string                          `json:"assignment_id"`
	SubmissionID    string                          `json:"submission_id"`
	Decision        WorkAcceptanceDecision          `json:"decision"`
	ReviewerKind    WorkReviewerKind                `json:"reviewer_kind"`
	ReviewerID      string                          `json:"reviewer_id"`
	CriteriaResults []WorkAcceptanceCriterionResult `json:"criteria_results,omitempty"`
	Feedback        string                          `json:"feedback,omitempty"`
	DecisionRoundID string                          `json:"decision_round_id,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
	Metadata        map[string]any                  `json:"metadata,omitempty"`
}

// ExecutionEventType 是 Execution Orchestration 的稳定领域事件类型。
type ExecutionEventType string

const (
	ExecutionEventCreated            ExecutionEventType = "execution_created"
	ExecutionEventPromoted           ExecutionEventType = "execution_promoted"
	ExecutionEventCancelled          ExecutionEventType = "execution_cancelled"
	ExecutionEventSuperseded         ExecutionEventType = "execution_superseded"
	ExecutionEventEvidenceRecorded   ExecutionEventType = "persistence_evidence_recorded"
	ExecutionEventStatusChanged      ExecutionEventType = "execution_status_changed"
	ExecutionEventPlanProposed       ExecutionEventType = "plan_proposed"
	ExecutionEventPlanActivated      ExecutionEventType = "plan_activated"
	ExecutionEventPlanSuperseded     ExecutionEventType = "plan_superseded"
	ExecutionEventWorkItemCreated    ExecutionEventType = "work_item_created"
	ExecutionEventWorkAssigned       ExecutionEventType = "work_assigned"
	ExecutionEventAttemptStarted     ExecutionEventType = "attempt_started"
	ExecutionEventAttemptReconcile   ExecutionEventType = "attempt_reconciliation_scheduled"
	ExecutionEventAttemptTerminal    ExecutionEventType = "attempt_terminal"
	ExecutionEventWorkSubmitted      ExecutionEventType = "work_submitted"
	ExecutionEventAcceptanceRecorded ExecutionEventType = "acceptance_recorded"
	ExecutionEventWorkTakenOver      ExecutionEventType = "work_taken_over"
	ExecutionEventCompleted          ExecutionEventType = "execution_completed"
)

// ExecutionEntityType 表示领域事件主要影响的 aggregate。
type ExecutionEntityType string

const (
	ExecutionEntityExecution      ExecutionEntityType = "execution"
	ExecutionEntityPlan           ExecutionEntityType = "plan"
	ExecutionEntityWorkItem       ExecutionEntityType = "work_item"
	ExecutionEntityAssignment     ExecutionEntityType = "assignment"
	ExecutionEntityDispatch       ExecutionEntityType = "dispatch"
	ExecutionEntityAttempt        ExecutionEntityType = "attempt"
	ExecutionEntitySubmission     ExecutionEntityType = "submission"
	ExecutionEntityReviewDispatch ExecutionEntityType = "review_dispatch"
	ExecutionEntityAcceptance     ExecutionEntityType = "acceptance"
)

// ExecutionActorKind 表示发出 mutation command 的授权主体。
type ExecutionActorKind string

const (
	ExecutionActorUser    ExecutionActorKind = "user"
	ExecutionActorAgent   ExecutionActorKind = "agent"
	ExecutionActorRuntime ExecutionActorKind = "runtime"
	ExecutionActorSystem  ExecutionActorKind = "system"
)

// ExecutionEvent 是与 aggregate mutation 同事务写入的 append-only 审计事件。
type ExecutionEvent struct {
	ID               string              `json:"id"`
	ExecutionID      string              `json:"execution_id"`
	Sequence         int64               `json:"sequence"`
	Type             ExecutionEventType  `json:"type"`
	EntityType       ExecutionEntityType `json:"entity_type"`
	EntityID         string              `json:"entity_id"`
	EntityVersion    int64               `json:"entity_version"`
	ActorKind        ExecutionActorKind  `json:"actor_kind"`
	ActorID          string              `json:"actor_id,omitempty"`
	CommandID        string              `json:"command_id"`
	GoalID           string              `json:"goal_id,omitempty"`
	PlanID           string              `json:"plan_id,omitempty"`
	WorkItemID       string              `json:"work_item_id,omitempty"`
	SpecID           string              `json:"spec_id,omitempty"`
	AssignmentID     string              `json:"assignment_id,omitempty"`
	DispatchID       string              `json:"dispatch_id,omitempty"`
	AttemptID        string              `json:"attempt_id,omitempty"`
	SubmissionID     string              `json:"submission_id,omitempty"`
	ReviewDispatchID string              `json:"review_dispatch_id,omitempty"`
	AcceptanceID     string              `json:"acceptance_id,omitempty"`
	RootRoundID      string              `json:"root_round_id,omitempty"`
	RuntimeRoundID   string              `json:"runtime_round_id,omitempty"`
	AgentRoundID     string              `json:"agent_round_id,omitempty"`
	Payload          map[string]any      `json:"payload,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
}

// ExecutionSnapshot 是服务内编排与 HTTP/UI 投影需要的有界当前状态。
//
// 模型 MCP 只接收由它派生的 actor-specific context，不直接接收整份 Snapshot。
// 历史 Attempt、Submission、Acceptance 与 event 使用独立分页接口，不应无限塞入快照。
type ExecutionSnapshot struct {
	Execution              Execution                       `json:"execution"`
	Plan                   *ExecutionPlanRevision          `json:"plan,omitempty"`
	WorkItems              []WorkItem                      `json:"work_items,omitempty"`
	WorkItemStates         []WorkItemState                 `json:"work_item_states,omitempty"`
	WorkItemSpecs          []WorkItemSpec                  `json:"work_item_specs,omitempty"`
	PlanItems              []ExecutionPlanItem             `json:"plan_items,omitempty"`
	Dependencies           []ExecutionPlanDependency       `json:"dependencies,omitempty"`
	OutputClaims           []ExecutionPlanOutputClaim      `json:"output_claims,omitempty"`
	Assignments            []WorkAssignment                `json:"assignments,omitempty"`
	Dispatches             []ExecutionDispatch             `json:"dispatches,omitempty"`
	Attempts               []WorkAttempt                   `json:"attempts,omitempty"`
	CancellationDispatches []ExecutionCancellationDispatch `json:"cancellation_dispatches,omitempty"`
	Submissions            []WorkSubmission                `json:"submissions,omitempty"`
	ReviewDispatches       []ExecutionReviewDispatch       `json:"review_dispatches,omitempty"`
	Acceptances            []WorkAcceptance                `json:"acceptances,omitempty"`
	ReadyWorkItemIDs       []string                        `json:"ready_work_item_ids,omitempty"`
	CompletionBlockers     []string                        `json:"completion_blockers,omitempty"`
}
