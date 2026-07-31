// INPUT: 模型语义字段和 execution domain enums。
// OUTPUT: additionalProperties=false、统一 maxItems 且不暴露 fencing/idempotency 的 JSON schemas。
// POS: nexus_execution 工具的模型调用协议。
package tool

import "github.com/nexus-research-lab/nexus/internal/protocol"

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

func nonEmptyStringArrayProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"minItems":    1,
		"maxItems":    protocol.ExecutionProjectionCollectionLimit,
		"items": map[string]any{
			"type":    "string",
			"pattern": `\S`,
		},
	}
}

func executionReferenceProperties() map[string]any {
	return map[string]any{
		"execution_id": stringProperty("Optional opaque Execution id. Omit to use the current Execution in this scope."),
	}
}

func workReferenceProperties() map[string]any {
	properties := executionReferenceProperties()
	properties["work_item_id"] = stringProperty("Optional opaque Work Item id. Prefer logical_key when it is unambiguous in the active Plan.")
	properties["logical_key"] = stringProperty("Stable human-readable Work Item logical key from the active Plan.")
	return properties
}

func getExecutionSchema() map[string]any {
	return objectSchema(executionReferenceProperties())
}

func planExecutionSchema() map[string]any {
	properties := executionReferenceProperties()
	properties["objective"] = nonEmptyStringProperty("Execution objective. Required when no current Execution exists; omit during replan because it cannot rewrite the existing objective.")
	properties["completion_criteria"] = nonEmptyStringArrayProperty("Execution-level completion criteria. When no current Execution exists, provide at least one nonblank top-level criterion. Existing Execution replans may omit this field and never rewrite it.")
	properties["revision_reason"] = stringProperty("Why this complete immutable Plan revision is needed.")
	properties["supersede_active_work"] = booleanProperty("Explicitly release current Assignments, interrupt live Attempts, and cancel pending Dispatches from the prior Plan before activating this revision. Use only for an intentional replan, provide revision_reason, and never use while an unreviewed Submission exists.")
	properties["replace_current_execution"] = booleanProperty("Replace the referenced current transient Execution with a successor because the user changed to a different objective. Requires explicit execution_id, replacement_reason, new objective, new completion_criteria, and the complete successor WorkGraph. Never use for a same-objective replan or Goal-bound Execution.")
	properties["replacement_reason"] = nonEmptyStringProperty("Why the current transient objective is being replaced. Required only with replace_current_execution.")
	properties["work_graph_json"] = nonEmptyStringProperty(
		"The complete WorkGraph encoded as one JSON array string. This string transport avoids provider loss of nested array objects while the backend still decodes and validates the graph atomically. " +
			"Each object must include logical_key, kind (produce/review/verify/integrate), subject, objective, deliverable, acceptance_criteria (non-empty string array), required, and terminal. " +
			"Optional fields are existing_work_item_id, parent_logical_key, depends_on [{logical_key,kind}], input_refs, and output_scopes [{scope,mode}]. " +
			"Use at most 32 Work Items; every produce item needs a typed output scope and the graph needs a required terminal integrate or verify item. " +
			`A minimal valid string value is [{"logical_key":"verify","kind":"verify","subject":"Verify","objective":"Verify the outcome","deliverable":"Verification report","acceptance_criteria":["Outcome verified"],"required":true,"terminal":true}].`,
	)
	return objectSchema(properties, "work_graph_json")
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
	properties["return_to_agent_id"] = stringProperty("Agent that receives the completed handoff; defaults to the coordinator.")
	properties["strategy"] = enumProperty("self for the current Agent in a DM Execution; room_member for a structured Room handoff. Room self Assignment is rejected until an independent reviewer is bound.", "self", "room_member")
	properties["reason"] = stringProperty("Why this Agent owns this Work Item.")
	properties["instruction"] = stringProperty("Optional handoff instruction. The service supplies the immutable deliverable and criteria.")
	properties["dispatch_kind"] = enumProperty("Room delivery route for room_member. Structured assignment is primary; do not hand-write @ messages.", "room_directed", "room_public")
	return objectSchema(properties, "target_agent_id")
}

func submitWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["assignment_id"] = stringProperty("Optional current Assignment id when the Work Item has assignment history.")
	properties["result_summary"] = stringProperty("Concise description of the delivered result.")
	properties["result_refs"] = stringArrayProperty("Artifact, file, URL, commit, or message references.")
	properties["evidence"] = stringArrayProperty("Evidence that the immutable acceptance criteria are met.")
	return objectSchema(properties, "result_summary")
}

func reviewWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["submission_id"] = stringProperty("Optional opaque Submission id. Omit to review the current unreviewed Submission for the Work Item.")
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
	properties := workReferenceProperties()
	properties["reason"] = stringProperty("Known reason progress cannot continue.")
	properties["needed_input"] = stringProperty("Specific external input or authority needed to resume.")
	return objectSchema(properties, "reason", "needed_input")
}

func resumeWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["resolution"] = stringProperty("How the exact external blocker was resolved.")
	properties["evidence"] = stringArrayProperty("At least one concrete reference or observation proving the required input or authority is now available.")
	return objectSchema(properties, "resolution", "evidence")
}

func takeOverWorkSchema() map[string]any {
	properties := workReferenceProperties()
	properties["target_agent_id"] = stringProperty("Replacement responsible Agent id.")
	properties["return_to_agent_id"] = stringProperty("Agent that receives the completed handoff; defaults to the coordinator.")
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
		"Durable-boundary reason being proposed. The backend independently verifies all hard gates and evidence; task complexity alone never qualifies.",
		"observed_boundary",
		"room_dependency_chain",
		"external_wait",
		"scheduled_retry",
		"context_boundary",
		"recovery_required",
	)
	return objectSchema(properties, "activation_reason")
}
