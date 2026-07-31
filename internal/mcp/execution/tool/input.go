// INPUT: 十个 execution tool 的模型可见 JSON 参数；Plan WorkGraph 以单个 JSON 字符串跨 Provider 传输。
// OUTPUT: 严格解码且不含 command_id/snapshot_revision/runtime identity 的 typed semantic intent。
// POS: MCP schema 与 service command 之间的无权限输入层，隔离 Provider 的深层对象数组兼容差异。
package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type getExecutionInput struct {
	ExecutionID string `json:"execution_id,omitempty"`
}

type planExecutionInput struct {
	ExecutionID             string   `json:"execution_id,omitempty"`
	Objective               string   `json:"objective,omitempty"`
	CompletionCriteria      []string `json:"completion_criteria,omitempty"`
	RevisionReason          string   `json:"revision_reason,omitempty"`
	SupersedeActiveWork     bool     `json:"supersede_active_work,omitempty"`
	ReplaceCurrentExecution bool     `json:"replace_current_execution,omitempty"`
	ReplacementReason       string   `json:"replacement_reason,omitempty"`
	WorkGraphJSON           string   `json:"work_graph_json,omitempty"`
	// Items 只为 work_graph_json 成为模型协议前的进程内调用方保留解码兼容；
	// 模型 schema 不再暴露这条 Provider 易损路径。
	Items []planItemInput `json:"items,omitempty"`
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
	decodedItems := input.Items
	workGraphJSON := strings.TrimSpace(input.WorkGraphJSON)
	if workGraphJSON != "" {
		if len(input.Items) > 0 {
			return orchestration.PlanDraft{}, fmt.Errorf(
				"work_graph_json and legacy items cannot both be provided",
			)
		}
		decoder := json.NewDecoder(bytes.NewBufferString(workGraphJSON))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decodedItems); err != nil {
			return orchestration.PlanDraft{}, fmt.Errorf(
				"work_graph_json must be a JSON array of Work Item objects: %w",
				err,
			)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				return orchestration.PlanDraft{}, fmt.Errorf(
					"work_graph_json must contain exactly one JSON array",
				)
			}
			return orchestration.PlanDraft{}, fmt.Errorf(
				"work_graph_json contains trailing invalid JSON: %w",
				err,
			)
		}
	}

	items := make([]orchestration.PlanWorkItemDraft, 0, len(decodedItems))
	for _, item := range decodedItems {
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
