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
		Description: "Atomically create one complete immutable Plan revision from the native items array. " +
			"Send the actual non-empty Work Item objects in this call; never send {} or a placeholder. " +
			"A first Plan requires objective and at least one nonblank completion criterion; a same-objective replan may omit both and cannot rewrite that boundary. " +
			"Append-only revisions preserve existing nodes and incoming edges. Non-monotonic changes require supersede_active_work with revision_reason at an allowed quiescent boundary. " +
			"Replacing a different transient objective requires the explicit replacement fields and complete successor graph; Goal-bound Executions cannot be replaced here.",
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
