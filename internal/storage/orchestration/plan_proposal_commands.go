// INPUT: sealed proposal、exact owner/session/scope/coordinator binding/access 与 expected proposal version。
// OUTPUT: Create-or-bind、materialization/confirmation receipt CAS 和 recoverable query command。
// POS: 非权威 ExecutionPlanProposal aggregate 与唯一 active binding 的持久化命令契约；不复用 Execution event command。
package orchestration

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// PlanProposalAccess 是每次 proposal 读取与 mutation 必须重新提供的权限 fence。
// Round identity 只属于 proposal 创建审计，不能成为跨 round 使用 proposal 的 bearer capability。
type PlanProposalAccess struct {
	ProposalID         string
	OwnerUserID        string
	SessionKey         string
	ScopeKind          protocol.ExecutionScopeKind
	RoomID             string
	ConversationID     string
	CoordinatorAgentID string
}

// PlanProposalBindingAccess 定位一个 trusted scope 当前显式绑定的 proposal。
// 它不接受 proposal id；id 只能由 prepare 写入的宿主持久 binding 解析。
type PlanProposalBindingAccess struct {
	OwnerUserID        string
	SessionKey         string
	ScopeKind          protocol.ExecutionScopeKind
	RoomID             string
	ConversationID     string
	CoordinatorAgentID string
}

// CreateOrGetPlanProposalCommand 按调用方给出的 deterministic proposal ID
// 原子创建 sealed immutable proposal，或返回完全相同的既有 aggregate。
type CreateOrGetPlanProposalCommand struct {
	Proposal protocol.ExecutionPlanProposal
}

// GetPlanProposalQuery 按 exact scope/coordinator 权限读取 proposal。
type GetPlanProposalQuery struct {
	Access PlanProposalAccess
}

// GetBoundPlanProposalQuery 按 exact scope/coordinator 读取 prepare 最后明确绑定的 proposal。
type GetBoundPlanProposalQuery struct {
	Access PlanProposalBindingAccess
}

// MarkPlanProposalMaterializingCommand 持久化 authoritative mutation 前的
// stable command/Execution identity 与 sealed exact Goal binding。
type MarkPlanProposalMaterializingCommand struct {
	Access                   PlanProposalAccess
	ExpectedVersion          int64
	ReservedExecutionID      string
	MaterializationCommandID string
	GoalID                   string
	GoalObjectiveRevision    int64
	GoalActivationOrigin     protocol.GoalActivationOrigin
	GoalActivationReason     protocol.GoalActivationReason
	ReplacesExecutionID      string
	NextAttemptAt            *time.Time
}

// ClaimPlanProposalMaterializingCommand 在执行权威 command 前原子领取一段
// 有界 lease，避免前台重放、后台 reconciler 与多实例同时进入 materializer。
type ClaimPlanProposalMaterializingCommand struct {
	Access          PlanProposalAccess
	ExpectedVersion int64
	ClaimAt         time.Time
	LeaseUntil      time.Time
}

// MarkPlanProposalMaterializedCommand 记录现有 materializer 返回的权威 receipt。
type MarkPlanProposalMaterializedCommand struct {
	Access                  PlanProposalAccess
	ExpectedVersion         int64
	MaterializedExecutionID string
	MaterializedPlanID      string
	NextAttemptAt           *time.Time
}

// SchedulePlanProposalRetryCommand 记录 materializing 阶段一次 transient failure
// 与下一次允许恢复的时间，避免 reconciler 每个 tick 盲重试。
type SchedulePlanProposalRetryCommand struct {
	Access          PlanProposalAccess
	ExpectedVersion int64
	LastError       string
	NextAttemptAt   *time.Time
}

// MarkPlanProposalConfirmationCommand 记录 Goal confirmation 的一次失败重试或成功。
type MarkPlanProposalConfirmationCommand struct {
	Access            PlanProposalAccess
	ExpectedVersion   int64
	ConfirmationState protocol.ExecutionPlanProposalConfirmationState
	LastError         string
	NextAttemptAt     *time.Time
}

// MarkPlanProposalBlockedCommand 将确定性 materialization failure 固定为人工可见状态。
type MarkPlanProposalBlockedCommand struct {
	Access          PlanProposalAccess
	ExpectedVersion int64
	LastError       string
}

// ListRecoverablePlanProposalsQuery 是 trusted background reconciler 的有界扫描。
type ListRecoverablePlanProposalsQuery struct {
	Now   time.Time
	Limit int
}
