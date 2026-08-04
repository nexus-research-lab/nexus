// INPUT: 完整 WorkGraph intent、首次 Execution 顶层完成标准、可选现有 Execution id 与本次 tool call identity。
// OUTPUT: 首次 Execution+Plan 原子创建、same-objective replan 或完整 objective replacement。
// POS: 模型唯一的 Plan 批量写入口；Execution boundary 变化必须显式且事务化。
package tool

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func planExecution(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "plan_execution"
	return sdktool.Tool{
		Name: toolName,
		Description: "Create one complete immutable Plan revision when coordinated delivery is useful. " +
			"Plan expresses how work unfolds; each Work Item expresses one deliverable, declared dependencies prevent premature work, and optional output scopes can prevent duplicate production. " +
			"Pass the complete WorkGraph once through the native items array. Populate the actual argument object in this call; never send {} as a placeholder or merely announce a later call. The schema uses the common function-calling subset shared across Providers, and the backend validates the entire graph before one atomic write. " +
			"Every item needs logical_key, kind, subject, objective, and deliverable. Required/terminal flags, acceptance criteria, dependencies, integration, verification, and output scopes are Agent-selected contract details: include them when they improve the task rather than to satisfy a fixed workflow. " +
			"If no Execution exists, objective and at least one nonblank top-level completion_criteria entry are mandatory; the backend atomically creates the Execution and first active Plan. " +
			"An existing same-objective replan may omit both because a Plan revision never rewrites the Execution boundary. " +
			"Ordinary replan is monotonic: resubmit every existing node unchanged, then append new nodes and edges whose target is new. An old node may be an upstream dependency of a new node; a new edge may not target an old node because that would change its readiness contract. Removing or changing an existing node or its incoming dependencies requires supersede_active_work=true and a non-empty revision_reason. Activate any successor revision only at a quiescent boundary with no current Assignment or unreviewed Submission. " +
			"When the user explicitly changes a transient objective that still needs a managed WorkGraph, set replace_current_execution=true and provide the current execution_id, replacement_reason, new objective, new completion criteria, and complete successor graph. " +
			"Never use replacement for a Goal-bound Execution or a direct atomic task.",
		SearchHint:  "plan execution work graph completion criteria dependencies duplicate work deliverables",
		InputSchema: planExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{
			IdempotentHint: true,
		},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed planExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			draft, draftErr := parsed.draft()
			if draftErr != nil {
				return rejectedResult(
					draftErr.Error(),
					orchestration.NextAction{
						Tool:   toolName,
						Reason: "send items as one non-empty native array containing every complete Work Item object",
					},
				), nil
			}
			if parsed.ReplaceCurrentExecution && strings.TrimSpace(parsed.ExecutionID) == "" {
				return rejectedResult(
					"replace_current_execution requires the explicit current execution_id from nexus_execution_context",
					orchestration.NextAction{
						Tool:   "get_execution",
						Reason: "refresh the current Execution identity before replacement",
					},
				), nil
			}
			actor := sctx.Actor()
			lookupActor := actor
			if strings.TrimSpace(parsed.ExecutionID) == "" {
				// A Goal retarget can supersede the Execution captured when this
				// MCP server was built. Planning without an explicit ID must
				// therefore resolve the current session state, not pin the call
				// to that now-terminal predecessor.
				lookupActor.ExecutionID = ""
			}
			snapshot, err := loadSnapshot(ctx, svc, lookupActor, parsed.ExecutionID)
			if err != nil {
				return transportErrorResult(err), nil
			}
			if snapshot == nil {
				if strings.TrimSpace(parsed.ExecutionID) != "" {
					return rejectedResult("explicit execution was not found"), nil
				}
			}
			snapshotRevision := int64(0)
			executionID := ""
			if snapshot != nil {
				snapshotRevision = snapshot.Execution.Version
				executionID = snapshot.Execution.ID
			}
			commandRevision := snapshotRevision
			if parsed.ReplaceCurrentExecution {
				// Replacement command identity must survive the old Execution's
				// terminal version bump when a tool response is retried.
				commandRevision = 0
			}
			planCommandID, commandErr := commandID(
				sctx,
				callContext,
				toolName,
				input,
				commandRevision,
			)
			if commandErr != nil {
				return transportErrorResult(commandErr), nil
			}
			result, serviceErr := svc.PlanExecution(ctx, actor, orchestration.PlanExecutionInput{
				ExecutionID:             executionID,
				SnapshotRevision:        snapshotRevision,
				CommandID:               planCommandID,
				Objective:               parsed.Objective,
				CompletionCriteria:      parsed.CompletionCriteria,
				ReplaceCurrentExecution: parsed.ReplaceCurrentExecution,
				ReplacementReason:       parsed.ReplacementReason,
				SupersedeActiveWork:     parsed.SupersedeActiveWork,
				Draft:                   draft,
			})
			if serviceErr != nil {
				return transportErrorResult(serviceErr), nil
			}
			contextActor := actor
			if result.Snapshot != nil &&
				(snapshot == nil || result.Snapshot.Execution.ID != snapshot.Execution.ID) {
				contextActor.ExecutionID = ""
				contextActor.WorkBinding = nil
				contextActor.ReviewBinding = nil
			}
			return mutationResult(withFreshExecutionContext(ctx, svc, contextActor, result)), nil
		},
	}
}
