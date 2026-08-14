// INPUT: Orchestration service 已授权、带 expected version 与稳定 command ID 的 mutation。
// OUTPUT: Repository 各事务入口的有语义 command。
// POS: protocol domain object 与 SQL mutation 之间的命令契约。
package orchestration

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CommandMeta 是所有 mutation 共用的幂等与审计身份。
type CommandMeta struct {
	CommandID      string
	EventID        string
	ActorKind      protocol.ExecutionActorKind
	ActorID        string
	RootRoundID    string
	RuntimeRoundID string
	AgentRoundID   string
	Payload        map[string]any
	CreatedAt      time.Time
}

// CreateCommand 创建 Execution aggregate root。
type CreateCommand struct {
	Execution protocol.Execution
	Meta      CommandMeta
}

// CreateWithPlanCommand 原子创建 Execution 与首个 active Plan。
type CreateWithPlanCommand struct {
	Execution protocol.Execution
	Plan      WritePlanCommand
	Meta      CommandMeta
}

// ReplaceWithPlanCommand 原子 supersede 当前 transient Execution，并创建带首个
// active Plan 的 successor。Successor.ReplacesExecutionID 必须指向旧 Execution。
type ReplaceWithPlanCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	Successor                protocol.Execution
	Plan                     WritePlanCommand
	Reason                   string
	Meta                     CommandMeta
	SuccessorMeta            CommandMeta
}

// AbandonCommand 原子取消当前 transient Execution 及其未完成执行链，不创建 successor。
type AbandonCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	Reason                   string
	Meta                     CommandMeta
}

// SupersedeGoalRevisionCommand closes the current WorkGraph of one Goal
// objective revision while reserving the already-minted successor identity.
// A graph that is already terminal keeps its exact status and history; only the
// deterministic successor reservation event is appended. Unlike transient
// replacement this command does not create the successor Plan in the same
// transaction; the durable Goal transition drives that later step.
type SupersedeGoalRevisionCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	ExpectedOwnerUserID      string
	GoalID                   string
	OldGoalObjectiveRevision int64
	NewGoalObjectiveRevision int64
	SuccessorExecutionID     string
	Reason                   string
	Meta                     CommandMeta
}

// FenceGoalExecutionIdentityCommand permanently rejects a Goal-bound
// Execution identity that was reserved in Goal metadata but never
// materialized. CreateWithPlan competes for the same durable identity claim.
type FenceGoalExecutionIdentityCommand struct {
	ExecutionID           string
	ExpectedOwnerUserID   string
	GoalID                string
	GoalObjectiveRevision int64
	SuccessorExecutionID  string
	Meta                  CommandMeta
}

// PlanWorkItem 聚合一个 Plan membership 使用的 stable identity、immutable spec 和 state fence。
type PlanWorkItem struct {
	WorkItem             protocol.WorkItem
	Spec                 protocol.WorkItemSpec
	State                protocol.WorkItemState
	Item                 protocol.ExecutionPlanItem
	OutputClaims         []protocol.ExecutionPlanOutputClaim
	ExpectedStateVersion int64
}

// WritePlanCommand 原子写入或激活一个 immutable Plan revision。
//
// 新 Plan 必须携带完整 WorkItems 和 Dependencies；激活已存在的 proposed Plan 时
// 重传相同 immutable graph，并用 ExpectedPlanVersion/ExpectedStateVersion 保护 lifecycle fence。
type WritePlanCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	ExpectedPlanVersion      int64
	Plan                     protocol.ExecutionPlanRevision
	WorkItems                []PlanWorkItem
	Dependencies             []protocol.ExecutionPlanDependency
	// SupersedeActiveWork permits this transaction to release current Assignment
	// chains belonging to the prior active Plan. RevisionReason remains mandatory.
	SupersedeActiveWork bool
	Meta                CommandMeta
}

// AssignCommand 创建 current Assignment，并可同事务创建 Dispatch 与 root Attempt。
type AssignCommand struct {
	ExpectedExecutionVersion int64
	Assignment               protocol.WorkAssignment
	Dispatch                 *protocol.ExecutionDispatch
	RootAttempt              *protocol.WorkAttempt
	Meta                     CommandMeta
}

// StartAttemptCommand 创建一个 running Attempt，或把已有 pending Attempt 原子激活。
type StartAttemptCommand struct {
	ExpectedExecutionVersion  int64
	ExpectedAssignmentVersion int64
	ExpectedAttemptVersion    int64
	Attempt                   protocol.WorkAttempt
	Meta                      CommandMeta
}

// FinishAttemptCommand 把 pending/running Attempt 变为终态。
type FinishAttemptCommand struct {
	ExpectedExecutionVersion int64
	ExpectedAttemptVersion   int64
	Attempt                  protocol.WorkAttempt
	Meta                     CommandMeta
}

// ScheduleSubagentReconciliationCommand 在 parent round 已退出但 child 尚未终结时，
// 持久化 grace deadline，供重启后的后台恢复器继续收口。
type ScheduleSubagentReconciliationCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	ExpectedAttemptVersion   int64
	AttemptID                string
	ParentRoundExitedAt      time.Time
	ReconcileAfter           time.Time
	Meta                     CommandMeta
}

// SubmitCommand 创建 immutable Submission，并用 Assignment version 拒绝 takeover 后迟到提交。
type SubmitCommand struct {
	ExpectedExecutionVersion  int64
	ExpectedAssignmentVersion int64
	Submission                protocol.WorkSubmission
	ReviewDispatch            *protocol.ExecutionReviewDispatch
	Meta                      CommandMeta
}

// ReviewCommand 追加唯一 Acceptance decision，并完成或释放 Assignment。
type ReviewCommand struct {
	ExpectedExecutionVersion  int64
	ExpectedAssignmentVersion int64
	Acceptance                protocol.WorkAcceptance
	Meta                      CommandMeta
}

// BlockCommand 把 stable Work Item state 置为 waiting_input，并收束旧执行链。
type BlockCommand struct {
	ExpectedExecutionVersion int64
	ExpectedStateVersion     int64
	State                    protocol.WorkItemState
	Meta                     CommandMeta
}

// ResumeCommand 以 resolution/evidence 把 waiting_input 当前 spec 重新置为 open。
type ResumeCommand struct {
	ExpectedExecutionVersion int64
	ExpectedStateVersion     int64
	State                    protocol.WorkItemState
	Resolution               string
	Evidence                 []string
	Meta                     CommandMeta
}

// TakeoverCommand 原子释放旧 Assignment、终止其当前执行，并创建 replacement。
type TakeoverCommand struct {
	ExpectedExecutionVersion         int64
	ExpectedCurrentAssignmentVersion int64
	CurrentAssignmentID              string
	Replacement                      protocol.WorkAssignment
	Dispatch                         *protocol.ExecutionDispatch
	RootAttempt                      *protocol.WorkAttempt
	Meta                             CommandMeta
}

// BindGoalCommand 无损绑定 Goal persistence identity。
type BindGoalCommand struct {
	ExpectedExecutionVersion int64
	Execution                protocol.Execution
	Meta                     CommandMeta
}

// RecordEvidenceCommand 由 runtime/scheduler 写入可验证的持续性边界证据。
type RecordEvidenceCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	MetadataKey              string
	Meta                     CommandMeta
}

// CompleteCommand 在 completion blockers 为空时完成 Execution。
type CompleteCommand struct {
	ExecutionID              string
	ExpectedExecutionVersion int64
	CompletedAt              time.Time
	Meta                     CommandMeta
}
