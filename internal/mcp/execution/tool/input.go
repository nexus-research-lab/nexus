// INPUT: 十一个 execution tool 的模型可见 JSON 参数；Plan WorkGraph 与 alignment report 使用原生 typed arrays。
// OUTPUT: 严格解码且不含 command_id/snapshot_revision/runtime identity 的 typed semantic intent。
// POS: MCP schema 与 service command 之间的无权限输入层；跨 Provider 传输后由领域层复核完整图。
package tool

import (
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type getExecutionInput struct {
	ExecutionID string `json:"execution_id,omitempty"`
}

type planExecutionInput struct {
	ExecutionID             string          `json:"execution_id,omitempty"`
	Objective               string          `json:"objective,omitempty"`
	CompletionCriteria      []string        `json:"completion_criteria,omitempty"`
	RevisionReason          string          `json:"revision_reason,omitempty"`
	SupersedeActiveWork     bool            `json:"supersede_active_work,omitempty"`
	ReplaceCurrentExecution bool            `json:"replace_current_execution,omitempty"`
	ReplacementReason       string          `json:"replacement_reason,omitempty"`
	Items                   []planItemInput `json:"items"`
}

type abandonExecutionInput struct {
	ExecutionID string `json:"execution_id"`
	Reason      string `json:"reason"`
}

type planItemInput struct {
	LogicalKey         string                `json:"logical_key"`
	ExistingWorkItemID string                `json:"existing_work_item_id,omitempty"`
	Kind               protocol.WorkItemKind `json:"kind"`
	Subject            string                `json:"subject"`
	Objective          string                `json:"objective"`
	Deliverable        string                `json:"deliverable"`
	AcceptanceCriteria []string              `json:"acceptance_criteria"`
	Required           bool                  `json:"required"`
	Terminal           bool                  `json:"terminal"`
	ParentLogicalKey   string                `json:"parent_logical_key,omitempty"`
	DependsOn          []planDependencyInput `json:"depends_on,omitempty"`
	InputRefs          []string              `json:"input_refs,omitempty"`
	OutputScopes       []outputScopeInput    `json:"output_scopes,omitempty"`
}

type planDependencyInput struct {
	LogicalKey string                      `json:"logical_key"`
	Kind       protocol.WorkDependencyKind `json:"kind,omitempty"`
}

type outputScopeInput struct {
	Scope string                       `json:"scope"`
	Mode  protocol.WorkOutputScopeMode `json:"mode,omitempty"`
}

func (input planExecutionInput) draft() (orchestration.PlanDraft, error) {
	if len(input.Items) == 0 {
		return orchestration.PlanDraft{}, fmt.Errorf(
			"items is required and must contain at least one complete Work Item object",
		)
	}

	items := make([]orchestration.PlanWorkItemDraft, 0, len(input.Items))
	for _, item := range input.Items {
		dependencies := make([]orchestration.PlanDependencyDraft, 0, len(item.DependsOn))
		for _, dependency := range item.DependsOn {
			dependencies = append(dependencies, orchestration.PlanDependencyDraft{
				LogicalKey: dependency.LogicalKey,
				Kind:       dependency.Kind,
			})
		}
		scopes := make([]protocol.WorkOutputScope, 0, len(item.OutputScopes))
		for _, scope := range item.OutputScopes {
			scopes = append(scopes, protocol.WorkOutputScope{
				Scope: scope.Scope,
				Mode:  scope.Mode,
			})
		}
		items = append(items, orchestration.PlanWorkItemDraft{
			LogicalKey:         item.LogicalKey,
			ExistingWorkItemID: item.ExistingWorkItemID,
			Kind:               item.Kind,
			Subject:            item.Subject,
			Objective:          item.Objective,
			Deliverable:        item.Deliverable,
			AcceptanceCriteria: item.AcceptanceCriteria,
			Required:           item.Required,
			Terminal:           item.Terminal,
			ParentLogicalKey:   item.ParentLogicalKey,
			DependsOn:          dependencies,
			InputRefs:          item.InputRefs,
			OutputScopes:       scopes,
		})
	}
	return orchestration.PlanDraft{
		RevisionReason: input.RevisionReason,
		Items:          items,
	}, nil
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
