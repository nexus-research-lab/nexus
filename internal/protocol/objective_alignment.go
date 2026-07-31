// INPUT: 模型或校验节点提交的逐 criterion 目标对齐结论、证据与缺口。
// OUTPUT: Goal completion 与未来 loop guard 共用的三态 Objective Alignment 协议。
// POS: 跨 MCP、Goal、Execution 与持久审计事件的共享语义模型；不承载任何一方生命周期。
package protocol

import "time"

const ObjectiveAlignmentCollectionLimit = 32

// ObjectiveAlignmentDecision 是整份目标对齐审计的三态结论。
type ObjectiveAlignmentDecision string

const (
	ObjectiveAlignmentAligned      ObjectiveAlignmentDecision = "aligned"
	ObjectiveAlignmentNotAligned   ObjectiveAlignmentDecision = "not_aligned"
	ObjectiveAlignmentInconclusive ObjectiveAlignmentDecision = "inconclusive"
)

// ObjectiveAlignmentCriterionStatus 是单条完成标准的证据判定。
type ObjectiveAlignmentCriterionStatus string

const (
	ObjectiveAlignmentCriterionSatisfied    ObjectiveAlignmentCriterionStatus = "satisfied"
	ObjectiveAlignmentCriterionUnsatisfied  ObjectiveAlignmentCriterionStatus = "unsatisfied"
	ObjectiveAlignmentCriterionInconclusive ObjectiveAlignmentCriterionStatus = "inconclusive"
)

// ObjectiveAlignmentEvidence 指向一个可复查的权威当前状态来源。
type ObjectiveAlignmentEvidence struct {
	Ref   string `json:"ref"`
	Claim string `json:"claim"`
}

// ObjectiveAlignmentCriterionResult 对一条服务端给定 criterion 给出结论。
type ObjectiveAlignmentCriterionResult struct {
	Criterion string                            `json:"criterion"`
	Status    ObjectiveAlignmentCriterionStatus `json:"status"`
	Evidence  []ObjectiveAlignmentEvidence      `json:"evidence,omitempty"`
	Gap       string                            `json:"gap,omitempty"`
}

// ObjectiveAlignmentReport 是与 Goal/loop 生命周期无关的审计结果。
type ObjectiveAlignmentReport struct {
	Decision        ObjectiveAlignmentDecision          `json:"decision"`
	CriteriaResults []ObjectiveAlignmentCriterionResult `json:"criteria_results"`
	Summary         string                              `json:"summary"`
}

// GoalObjectiveAlignmentRecord 是 Goal 对共享报告的持久引用包装。
// loop guard 将使用自己的 NodeRun 包装，不能复用这个 Goal identity。
type GoalObjectiveAlignmentRecord struct {
	ID                string                   `json:"id"`
	ObjectiveRevision int64                    `json:"objective_revision"`
	TargetFingerprint string                   `json:"target_fingerprint"`
	RoundID           string                   `json:"round_id"`
	AgentID           string                   `json:"agent_id,omitempty"`
	Report            ObjectiveAlignmentReport `json:"report"`
	AuditedAt         time.Time                `json:"audited_at"`
}

// AuditGoalObjectiveAlignmentRequest 只携带模型判定；目标与 criteria 由服务端读取。
type AuditGoalObjectiveAlignmentRequest struct {
	Report                    ObjectiveAlignmentReport `json:"report"`
	RoundID                   string                   `json:"round_id,omitempty"`
	AgentID                   string                   `json:"-"`
	ExpectedObjectiveRevision int64                    `json:"-"`
}
