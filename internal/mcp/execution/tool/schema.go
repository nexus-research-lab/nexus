// INPUT: 模型语义字段和 execution domain enums。
// OUTPUT: Plan 使用可移植嵌套 schema，其余工具保留有界集合；全部隐藏 fencing/idempotency。
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

// portableStringArrayProperty stays inside the common function-calling JSON
// Schema subset. Cardinality and nonblank checks remain authoritative in the
// service layer instead of relying on Provider-specific schema keywords.
func portableStringArrayProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
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
	properties["objective"] = stringProperty("Execution objective. Required and nonblank when no current Execution exists; omit during replan because it cannot rewrite the existing objective.")
	properties["completion_criteria"] = portableStringArrayProperty("Execution-level completion criteria. When no current Execution exists, provide at least one nonblank top-level criterion. Existing Execution replans may omit this field and never rewrite it.")
	properties["revision_reason"] = stringProperty("Why this complete immutable Plan revision is needed.")
	properties["supersede_active_work"] = booleanProperty("Authorize a non-monotonic replan that changes existing nodes or incoming dependencies. Requires revision_reason and an allowed quiescent boundary; omit for append-only extension.")
	properties["replace_current_execution"] = booleanProperty("Replace the referenced current transient Execution with a successor because the user changed to a different objective. Requires explicit execution_id, replacement_reason, new objective, new completion_criteria, and the complete successor WorkGraph. Never use for a same-objective replan or Goal-bound Execution.")
	properties["replacement_reason"] = stringProperty("Why the current transient objective is being replaced. Required and nonblank only with replace_current_execution.")
	properties["items"] = map[string]any{
		"type":        "array",
		"description": "The complete WorkGraph as native Work Item objects. Submit every item in this one call; never send an empty placeholder call.",
		"items":       planItemSchema(),
	}
	return objectSchema(properties, "items")
}

func planItemSchema() map[string]any {
	return objectSchema(map[string]any{
		"logical_key":           stringProperty("Stable readable key used by dependencies and later tool calls."),
		"existing_work_item_id": stringProperty("Optional opaque id when intentionally carrying a stable Work Item into a new Plan revision."),
		"kind":                  enumProperty("produce creates a deliverable; review checks it; verify validates outcomes; integrate assembles the final result.", "produce", "review", "verify", "integrate"),
		"subject":               stringProperty("Short Work Item subject."),
		"objective":             stringProperty("What this Work Item must accomplish."),
		"deliverable":           stringProperty("Concrete output the owner must submit."),
		"acceptance_criteria":   portableStringArrayProperty("Optional observable criteria used by review_work when explicit checks help."),
		"required":              booleanProperty("Whether Execution completion requires Acceptance for this item."),
		"terminal":              booleanProperty("Whether this item should act as a completion gate. It need not be integrate or verify."),
		"parent_logical_key":    stringProperty("Optional hierarchy parent within this Plan."),
		"depends_on": map[string]any{
			"type":        "array",
			"description": "Dependencies within this Plan. Hard edges require upstream Acceptance.",
			"items": objectSchema(map[string]any{
				"logical_key": stringProperty("Upstream logical key."),
				"kind":        enumProperty("Dependency kind.", "hard", "soft"),
			}, "logical_key"),
		},
		"input_refs": portableStringArrayProperty("Known inputs, artifacts, URLs, or identifiers."),
		"output_scopes": map[string]any{
			"type":        "array",
			"description": "Optional typed canonical output areas used when duplicate or overlapping production must be prevented. Use file:<workspace-relative-posix-path>, dir:<workspace-relative-posix-path>, or semantic:<nonempty-key>.",
			"items": objectSchema(map[string]any{
				"scope": stringProperty("Typed scope. File and directory scopes are workspace-relative; semantic scopes are exact keys."),
				"mode":  enumProperty("exclusive rejects every overlapping scope; overlap is allowed only when both declarations are shared. Defaults to exclusive.", "exclusive", "shared"),
			}, "scope"),
		},
	}, "logical_key", "kind", "subject", "objective", "deliverable")
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
		"Why the Agent chooses to preserve this objective across execution boundaries. The backend validates authority, user configuration, conflicts, and current state; the reason is an Agent strategy choice.",
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
