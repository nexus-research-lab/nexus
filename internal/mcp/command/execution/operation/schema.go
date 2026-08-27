// INPUT: 模型语义字段、execution domain enums 与稳定的条件 locator 契约。
// OUTPUT: Plan 两阶段工具严格区分外层 Goal binding 与内层 document scalar，work/review 工具静态表达 binding 默认，其余工具保留有界集合；全部隐藏 identity/fencing/idempotency。
// POS: Execution command 的模型调用协议。
package operation

import (
	"fmt"
	"strings"

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

type extractWorkflowPreviewInput struct {
	SourceExecutionID string `json:"source_execution_id"`
	OutputLanguage    string `json:"output_language"`
}

type getWorkflowPreviewInput struct {
	PreviewID string `json:"preview_id"`
}

type selectWorkflowPreviewRevisionInput struct {
	PreviewID        string `json:"preview_id"`
	Revision         int64  `json:"revision"`
	SelectedRevision int64  `json:"selected_revision"`
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

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
		"required":             append([]string{}, required...),
	}
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func booleanProperty(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func enumProperty(description string, values ...string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        values,
	}
}

func stringArrayProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"maxItems":    protocol.ExecutionProjectionCollectionLimit,
		"items":       map[string]any{"type": "string"},
	}
}

func nonEmptyStringProperty(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"pattern":     `\S`,
	}
}

func executionReferenceProperties() map[string]any {
	return map[string]any{
		"execution_id": stringProperty("Optional opaque Execution id. Omit to use the current Execution in this scope."),
	}
}

func workReferenceProperties() map[string]any {
	properties := executionReferenceProperties()
	properties["work_item_id"] = stringProperty("Optional opaque Work Item id. When supplied with logical_key, both must identify the same active-Plan Work Item.")
	properties["logical_key"] = stringProperty("Optional stable Work Item logical key from the active Plan. When supplied with work_item_id, both must identify the same Work Item.")
	return properties
}

func trustedWorkReferenceProperties() map[string]any {
	properties := workReferenceProperties()
	properties["work_item_id"] = stringProperty("Conditional Work Item locator. Only an exact trusted WorkBinding permits omission and supplies its Work Item; assigned_work/current_actor projections do not establish that binding. An explicit value must match. In DM coordination or any unbound call, provide work_item_id or logical_key.")
	properties["logical_key"] = stringProperty("Conditional stable Work Item locator. Only an exact trusted WorkBinding permits omission and supplies its Work Item; assigned_work/current_actor projections do not establish that binding. An explicit value must match. In DM coordination or any unbound call, provide logical_key when work_item_id is absent.")
	return properties
}

func getExecutionSchema() map[string]any {
	return objectSchema(executionReferenceProperties())
}

func preparePlanExecutionSchema() map[string]any {
	return objectSchema(map[string]any{
		"plan_document": nonEmptyStringProperty(planDocumentSchemaDescription()),
		"goal_binding": enumProperty(
			"Outer command input beside plan_document; never put goal_binding inside the Plan Document YAML. For operation: create, use current only to bind the exact Goal authority granted to this round, or none for a Goal-free WorkGraph. Omit to use current only when this round already has exact Goal id+revision authority; otherwise omission means none. For operation: replan or replace, use inherit or omit because those operations preserve the current Execution boundary.",
			string(orchestration.PlanGoalBindingNone),
			string(orchestration.PlanGoalBindingCurrent),
			string(orchestration.PlanGoalBindingInherit),
		),
	}, "plan_document")
}

func planDocumentSchemaDescription() string {
	contract := orchestration.ExecutionPlanDocumentSchemaContract()
	return fmt.Sprintf(
		"Complete strict Nexus Plan Document v%d as one YAML string; nexus_plan is 1. goal_binding is not a YAML field: it is the sibling outer command input beside plan_document. Select operation only from execution inspect: no current Execution means create even after Goal reset/retarget; replan requires the returned current Execution and preserves its objective boundary; replace requires a current transient Goal-free Execution. Parser-required root keys: %s. Allowed root keys only: %s. Every item requires: %s. Allowed item keys only: %s. Item kind is produce, review, verify, or integrate. Operation requirements: create: %s. replan: %s. replace: %s. Exact field corrections: dependencies is invalid; use %s. description is invalid; use %s. acceptance is invalid; use %s. scopes is invalid; use %s. Dependencies are logical-key string sequences. Output scopes use file:<path>, dir:<path>, or semantic:<key>. Output scope requirements: %s. Minimal valid create example (replace the generic text with the actual plan):\n%s\nWhen a new Goal is required, finish create_goal before this call, then set the outer goal_binding input to current; never launch them in parallel. Use outer goal_binding none for a Goal-free create. Never send JSON objects, placeholders, fragments, aliases, or multiple documents.",
		contract.Version,
		strings.Join(contract.ParserRequiredRootFields, ", "),
		strings.Join(contract.AllowedRootFields, ", "),
		strings.Join(contract.RequiredItemFields, ", "),
		strings.Join(contract.AllowedItemFields, ", "),
		contract.OperationRequirements["create"],
		contract.OperationRequirements["replan"],
		contract.OperationRequirements["replace"],
		contract.CommonAliasCorrections["dependencies"],
		contract.CommonAliasCorrections["description"],
		contract.CommonAliasCorrections["acceptance"],
		contract.CommonAliasCorrections["scopes"],
		contract.OutputScopeRequirements,
		contract.MinimalValidCreateExample,
	)
}

func planExecutionSchema() map[string]any {
	return objectSchema(map[string]any{
		"proposal_id": nonEmptyStringProperty(
			"Deprecated compatibility field. Omit it; the host resolves the exact durable proposal binding. If present, it must match that binding and proposal_digest must also be present.",
		),
		"proposal_digest": nonEmptyStringProperty(
			"Deprecated compatibility field. Omit it; the host verifies the bound proposal digest. If present, it must match the active binding and proposal_id must also be present.",
		),
	})
}

func distillWorkflowSchema() map[string]any {
	return objectSchema(map[string]any{
		"preview_id": nonEmptyStringProperty(
			"用户已确认的 WorkGraph 草图所对应的精确不透明 preview ID。必须原样保存该预览，不得猜测、重建或替换。",
		),
	}, "preview_id")
}

func extractWorkflowPreviewSchema() map[string]any {
	return objectSchema(map[string]any{
		"source_execution_id": nonEmptyStringProperty("当前 Session 的 completed WorkGraph execution_id；先查询 WorkGraph library，不得从文字猜测。"),
		"output_language":     enumProperty("草图面向用户字段的语言。", "zh", "en"),
	}, "source_execution_id", "output_language")
}

func getWorkflowPreviewSchema() map[string]any {
	return objectSchema(map[string]any{
		"preview_id": nonEmptyStringProperty("当前 Session WorkGraph Draft 目录中的 exact preview_id。"),
	}, "preview_id")
}

func reviseWorkflowDraftSchema() map[string]any {
	schema := reviseWorkflowPreviewSchema()
	properties := schema["properties"].(map[string]any)
	properties["preview_id"] = nonEmptyStringProperty("当前 Session WorkGraph Draft 的 exact preview_id。")
	required := schema["required"].([]string)
	schema["required"] = append([]string{"preview_id"}, required...)
	return schema
}

func selectWorkflowPreviewRevisionSchema() map[string]any {
	return objectSchema(map[string]any{
		"preview_id": nonEmptyStringProperty("当前 Session WorkGraph Draft 的 exact preview_id。"),
		"revision": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "Draft 当前 head_revision，用作并发 CAS。",
		},
		"selected_revision": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "用户明确选中的既有不可变版本。",
		},
	}, "preview_id", "revision", "selected_revision")
}

func selectBoundWorkflowPreviewRevisionSchema() map[string]any {
	return objectSchema(map[string]any{
		"revision": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "当前隐藏编辑 Session 的 head_revision。",
		},
		"selected_revision": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "用户明确选中的既有不可变版本。",
		},
	}, "revision", "selected_revision")
}

func reviseWorkflowPreviewSchema() map[string]any {
	nodeSchema := objectSchema(map[string]any{
		"logical_key":         nonEmptyStringProperty("稳定英文标识；新增节点必须创建新的 logical_key。"),
		"role":                enumProperty("节点在草图中的责任角色。", "key", "collaboration"),
		"kind":                enumProperty("节点交付类型。", "produce", "review", "verify", "integrate"),
		"subject":             nonEmptyStringProperty("面向用户的节点标题。"),
		"objective":           nonEmptyStringProperty("节点目的。"),
		"deliverable":         nonEmptyStringProperty("可验证交付物。"),
		"acceptance_criteria": stringArrayProperty("节点验收标准。"),
		"required":            booleanProperty("是否为必需节点。"),
		"terminal":            booleanProperty("是否为最终交付节点。"),
		"parent_logical_key":  stringProperty("可选父节点 logical_key。"),
		"position":            map[string]any{"type": "integer", "minimum": 0},
	}, "logical_key", "role", "kind", "subject", "objective", "deliverable", "required", "terminal")
	edgeSchema := objectSchema(map[string]any{
		"logical_key":            nonEmptyStringProperty("下游节点 logical_key。"),
		"depends_on_logical_key": nonEmptyStringProperty("上游节点 logical_key。"),
		"kind":                   enumProperty("依赖强度。", "hard", "soft"),
	}, "logical_key", "depends_on_logical_key", "kind")
	return objectSchema(map[string]any{
		"revision":            map[string]any{"type": "integer", "minimum": 1},
		"slash_name":          nonEmptyStringProperty("英文 kebab-case 命令名，不含斜杠。"),
		"title":               nonEmptyStringProperty("草图标题。"),
		"description":         nonEmptyStringProperty("草图用途说明。"),
		"objective":           nonEmptyStringProperty("复用时发送给模型的内部执行目标。"),
		"completion_criteria": stringArrayProperty("整张草图的完成标准。"),
		"nodes": map[string]any{
			"type": "array", "minItems": 1, "maxItems": 64, "items": nodeSchema,
		},
		"dependencies": map[string]any{
			"type": "array", "maxItems": 256, "items": edgeSchema,
		},
	}, "revision", "slash_name", "title", "description", "objective", "nodes", "dependencies")
}

func abandonExecutionSchema() map[string]any {
	return objectSchema(map[string]any{
		"execution_id": nonEmptyStringProperty("Opaque current transient Execution id from nexus_execution_context."),
		"reason":       nonEmptyStringProperty("Concrete user-directed reason to stop this objective without creating a successor Execution."),
	}, "execution_id", "reason")
}

func assignWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["target_agent_id"] = stringProperty("The responsible Agent id. Human display names are not stable assignment keys.")
	properties["return_to_agent_id"] = stringProperty("Agent selected to review the completed handoff; defaults to the coordinator. It may be the owner for self-review, the Lead, or another authorized Room member.")
	properties["strategy"] = enumProperty("self for the current Agent, including a Room coordinator's own work; room_member for a structured Room handoff.", "self", "room_member")
	properties["reason"] = stringProperty("Why this Agent owns this Work Item.")
	properties["instruction"] = stringProperty("Optional handoff instruction. The service supplies the immutable deliverable and criteria.")
	properties["dispatch_kind"] = enumProperty("Room delivery route for a tracked room_member Assignment.", "room_directed", "room_public")
	return objectSchema(properties, "target_agent_id")
}

func submitWorkSchema() map[string]any {
	properties := trustedWorkReferenceProperties()
	properties["assignment_id"] = stringProperty("Optional current Assignment id; it never replaces the required Work Item locator in an unbound call. Omit inside an exact trusted WorkBinding to use its Assignment. Otherwise the backend selects the current Assignment for the explicit Work Item; any explicit value must match.")
	properties["result_summary"] = stringProperty("Concise description of the delivered result.")
	properties["result_refs"] = stringArrayProperty("Artifact, file, URL, commit, or message references.")
	properties["evidence"] = stringArrayProperty("Evidence that the immutable acceptance criteria are met.")
	return objectSchema(properties, "result_summary")
}

func reviewWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["work_item_id"] = stringProperty("Conditional review locator. Only an exact trusted ReviewBinding, or a permitted self-review exact trusted WorkBinding, permits omission and supplies its Work Item; assigned_work/current_actor projections do not establish either binding. An explicit value must match. In DM coordination or any unbound call, provide at least one of submission_id, work_item_id, or logical_key, and all supplied locators must identify the same Work Item.")
	properties["logical_key"] = stringProperty("Conditional stable review locator. Only an exact trusted ReviewBinding, or a permitted self-review exact trusted WorkBinding, permits omission and supplies its Work Item; assigned_work/current_actor projections do not establish either binding. An explicit value must match. In DM coordination or any unbound call, provide at least one of submission_id, work_item_id, or logical_key, and all supplied locators must identify the same Work Item.")
	properties["submission_id"] = stringProperty("Conditional immutable Submission locator. An exact trusted ReviewBinding permits omission and supplies its Submission; a permitted self-review exact trusted WorkBinding selects the current unreviewed Submission for its Work Item. assigned_work/current_actor projections do not establish either binding. Explicit values must match the bound target. In DM coordination or any unbound call, provide at least one of submission_id, work_item_id, or logical_key; a Work Item locator selects its current unreviewed Submission.")
	properties["decision"] = enumProperty("Append-only review decision.", "accepted", "rejected", "changes_requested")
	properties["criteria_results"] = map[string]any{
		"type":        "array",
		"description": "For accepted decisions, include a passing result for every immutable acceptance criterion.",
		"maxItems":    protocol.ExecutionProjectionCollectionLimit,
		"items": objectSchema(map[string]any{
			"criterion": stringProperty("Criterion copied exactly from the Work Item spec."),
			"passed":    booleanProperty("Whether the criterion passed."),
			"evidence":  stringArrayProperty("Evidence supporting this judgment."),
			"note":      stringProperty("Optional reviewer note."),
		}, "criterion", "passed"),
	}
	properties["feedback"] = stringProperty("Review feedback, especially for rejection or requested changes.")
	return objectSchema(properties, "decision")
}

func blockWorkSchema() map[string]any {
	properties := trustedWorkReferenceProperties()
	properties["reason"] = stringProperty("Known reason progress cannot continue.")
	properties["needed_input"] = stringProperty("Specific external input or authority needed to resume.")
	return objectSchema(properties, "reason", "needed_input")
}

func resumeWorkSchema() map[string]any {
	properties := trustedWorkReferenceProperties()
	properties["resolution"] = stringProperty("How the exact external blocker was resolved.")
	properties["evidence"] = stringArrayProperty("At least one concrete reference or observation proving the required input or authority is now available.")
	return objectSchema(properties, "resolution", "evidence")
}

func takeOverWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["target_agent_id"] = stringProperty("Replacement responsible Agent id.")
	properties["return_to_agent_id"] = stringProperty("Agent that receives and reviews the replacement handoff; defaults to the coordinator. The Room coordinator may self-review coordinator-owned work.")
	properties["strategy"] = enumProperty("Replacement responsibility strategy.", "self", "room_member")
	properties["reason"] = stringProperty("Concrete reason the current Assignment must be replaced.")
	properties["instruction"] = stringProperty("Optional replacement handoff instruction.")
	properties["dispatch_kind"] = enumProperty("Room delivery route for room_member.", "room_directed", "room_public")
	return objectSchema(properties, "target_agent_id", "reason")
}

func promoteExecutionSchema() map[string]any {
	properties := executionReferenceProperties()
	properties["objective_proposal"] = stringProperty("Optional clearer objective proposal. This cannot grant authority or prove persistence need.")
	properties["activation_reason"] = enumProperty(
		"Why this existing WorkGraph should gain durable Goal persistence. Use persistence_requested only for explicit user/system Goal intent; the remaining values are adaptive Agent choices. The backend validates authority, configuration, conflicts, and current state.",
		"persistence_requested",
		"observed_boundary",
		"room_dependency_chain",
		"external_wait",
		"scheduled_retry",
		"context_boundary",
		"recovery_required",
		"substantial_complexity",
	)
	return objectSchema(properties, "activation_reason")
}

func auditExecutionAlignmentSchema() map[string]any {
	properties := executionReferenceProperties()
	properties["decision"] = enumProperty(
		"Aggregate result derived from every current Execution completion criterion. This optional current-Execution Gate is not Goal completion evidence.",
		"aligned",
		"not_aligned",
		"inconclusive",
	)
	properties["criteria_results"] = map[string]any{
		"type":        "array",
		"description": "One result for every authoritative completion criterion, copied exactly. This check is optional, requires a current non-terminal Execution, and does not choose the next workflow step. Goal+WorkGraph closure uses Goal audit_objective_alignment after Execution completion instead.",
		"items": objectSchema(map[string]any{
			"criterion": stringProperty("Current Execution completion criterion copied exactly."),
			"status": enumProperty(
				"Evidence result for this criterion.",
				"satisfied",
				"unsatisfied",
				"inconclusive",
			),
			"evidence": map[string]any{
				"type":        "array",
				"description": "Reviewable evidence for a satisfied result, or useful evidence available for another result.",
				"items": objectSchema(map[string]any{
					"ref":   stringProperty("Artifact, file, URL, command result, message, or other reviewable reference."),
					"claim": stringProperty("What the reference establishes."),
				}, "ref", "claim"),
			},
			"gap": stringProperty("For unsatisfied or inconclusive status, the concrete missing outcome or evidence."),
		}, "criterion", "status"),
	}
	properties["summary"] = stringProperty("Concise explanation of the aggregate alignment result.")
	return objectSchema(properties, "decision", "criteria_results", "summary")
}
