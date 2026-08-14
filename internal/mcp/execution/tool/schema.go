// INPUT: 模型语义字段、execution domain enums 与稳定的条件 locator 契约。
// OUTPUT: Plan 两阶段工具暴露 document/reference scalar 及显式 Goal binding enum，work/review 工具静态表达 binding 默认，其余工具保留有界集合；全部隐藏 identity/fencing/idempotency。
// POS: nexus_execution 工具的模型调用协议。
package tool

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

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
			"Goal boundary intent. For operation: create, use current only to bind the exact Goal authority granted to this round, or none for a Goal-free WorkGraph. Omit to use current only when this round already has exact Goal id+revision authority; otherwise omission means none. For operation: replan or replace, use inherit or omit because those operations preserve the current Execution boundary.",
			string(orchestration.PlanGoalBindingNone),
			string(orchestration.PlanGoalBindingCurrent),
			string(orchestration.PlanGoalBindingInherit),
		),
	}, "plan_document")
}

func planDocumentSchemaDescription() string {
	contract := orchestration.ExecutionPlanDocumentSchemaContract()
	return fmt.Sprintf(
		"Complete strict Nexus Plan Document v%d as one YAML string; nexus_plan is 1. Parser-required root keys: %s. Allowed root keys only: %s. Every item requires: %s. Allowed item keys only: %s. Item kind is produce, review, verify, or integrate. Operation requirements: create: %s. replan: %s. replace: %s. Exact field corrections: dependencies is invalid; use %s. description is invalid; use %s. acceptance is invalid; use %s. scopes is invalid; use %s. Dependencies are logical-key string sequences. Output scopes use file:<path>, dir:<path>, or semantic:<key>. Output scope requirements: %s. Minimal valid create example (replace the generic text with the actual plan):\n%s\nWhen a new Goal is required, finish create_goal before this call, then set goal_binding to current; never launch them in parallel. Use none for a Goal-free create. Never send JSON objects, placeholders, fragments, aliases, or multiple documents.",
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
			"Opaque sealed proposal id returned by prepare_plan_execution.",
		),
		"proposal_digest": nonEmptyStringProperty(
			"Exact digest returned with the same proposal_id. It binds the document and trusted target fence.",
		),
	}, "proposal_id", "proposal_digest")
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
		"Aggregate result derived from every current Execution completion criterion.",
		"aligned",
		"not_aligned",
		"inconclusive",
	)
	properties["criteria_results"] = map[string]any{
		"type":        "array",
		"description": "One result for every authoritative completion criterion, copied exactly. This check is optional and does not choose the next workflow step.",
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
