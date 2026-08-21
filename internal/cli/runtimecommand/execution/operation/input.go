// INPUT: Execution operation 的模型可见 JSON 参数；Plan 通过 document string/Goal intent 与零输入 host-bound commit 传输。
// OUTPUT: 严格解码且不含 command_id/snapshot_revision/runtime identity 的 typed semantic intent。
// POS: command schema 与 service command 之间的无权限输入层；legacy proposal reference 只能匹配宿主 binding，不能选择对象。
package operation

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type getExecutionInput struct {
	ExecutionID string `json:"execution_id,omitempty"`
}

type preparePlanExecutionInput struct {
	PlanDocument string                              `json:"plan_document"`
	GoalBinding  orchestration.PlanGoalBindingIntent `json:"goal_binding,omitempty"`
}

type planExecutionInput struct {
	ProposalID     string `json:"proposal_id,omitempty"`
	ProposalDigest string `json:"proposal_digest,omitempty"`
}

type distillWorkflowInput struct {
	PreviewID string `json:"preview_id"`
}

type abandonExecutionInput struct {
	ExecutionID string `json:"execution_id"`
	Reason      string `json:"reason"`
}

type assignWorkInput struct {
	ExecutionID     string                         `json:"execution_id,omitempty"`
	WorkItemID      string                         `json:"work_item_id,omitempty"`
	LogicalKey      string                         `json:"logical_key,omitempty"`
	TargetAgentID   string                         `json:"target_agent_id"`
	ReturnToAgentID string                         `json:"return_to_agent_id,omitempty"`
	Strategy        protocol.AssignmentStrategy    `json:"strategy,omitempty"`
	Reason          string                         `json:"reason,omitempty"`
	Instruction     string                         `json:"instruction,omitempty"`
	DispatchKind    protocol.ExecutionDispatchKind `json:"dispatch_kind,omitempty"`
}

type submitWorkInput struct {
	ExecutionID   string   `json:"execution_id,omitempty"`
	WorkItemID    string   `json:"work_item_id,omitempty"`
	LogicalKey    string   `json:"logical_key,omitempty"`
	AssignmentID  string   `json:"assignment_id,omitempty"`
	ResultSummary string   `json:"result_summary"`
	ResultRefs    []string `json:"result_refs,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

type reviewWorkInput struct {
	ExecutionID     string                                   `json:"execution_id,omitempty"`
	SubmissionID    string                                   `json:"submission_id,omitempty"`
	WorkItemID      string                                   `json:"work_item_id,omitempty"`
	LogicalKey      string                                   `json:"logical_key,omitempty"`
	Decision        protocol.WorkAcceptanceDecision          `json:"decision"`
	CriteriaResults []protocol.WorkAcceptanceCriterionResult `json:"criteria_results,omitempty"`
	Feedback        string                                   `json:"feedback,omitempty"`
}

type blockWorkInput struct {
	ExecutionID string `json:"execution_id,omitempty"`
	WorkItemID  string `json:"work_item_id,omitempty"`
	LogicalKey  string `json:"logical_key,omitempty"`
	Reason      string `json:"reason"`
	NeededInput string `json:"needed_input"`
}

type resumeWorkInput struct {
	ExecutionID string   `json:"execution_id,omitempty"`
	WorkItemID  string   `json:"work_item_id,omitempty"`
	LogicalKey  string   `json:"logical_key,omitempty"`
	Resolution  string   `json:"resolution"`
	Evidence    []string `json:"evidence"`
}

type takeOverWorkInput struct {
	ExecutionID     string                         `json:"execution_id,omitempty"`
	WorkItemID      string                         `json:"work_item_id,omitempty"`
	LogicalKey      string                         `json:"logical_key,omitempty"`
	TargetAgentID   string                         `json:"target_agent_id"`
	ReturnToAgentID string                         `json:"return_to_agent_id,omitempty"`
	Strategy        protocol.AssignmentStrategy    `json:"strategy,omitempty"`
	Reason          string                         `json:"reason"`
	Instruction     string                         `json:"instruction,omitempty"`
	DispatchKind    protocol.ExecutionDispatchKind `json:"dispatch_kind,omitempty"`
}

type promoteExecutionInput struct {
	ExecutionID       string                        `json:"execution_id,omitempty"`
	ObjectiveProposal string                        `json:"objective_proposal,omitempty"`
	ActivationReason  protocol.GoalActivationReason `json:"activation_reason"`
}

type auditExecutionAlignmentInput struct {
	ExecutionID     string                                       `json:"execution_id,omitempty"`
	Decision        protocol.ObjectiveAlignmentDecision          `json:"decision"`
	CriteriaResults []protocol.ObjectiveAlignmentCriterionResult `json:"criteria_results"`
	Summary         string                                       `json:"summary"`
}

func (input auditExecutionAlignmentInput) report() protocol.ObjectiveAlignmentReport {
	return protocol.ObjectiveAlignmentReport{
		Decision:        input.Decision,
		CriteriaResults: input.CriteriaResults,
		Summary:         input.Summary,
	}
}
