// INPUT: Assignment、Submission、Acceptance、Block 与 Takeover 的模型语义 intent 及宿主签发的 exact responsibility binding。
// OUTPUT: 稳定工具契约与使用最新 snapshot revision、稳定 command id 的统一 MutationResult。
// POS: Work Item 协作生命周期的六个模型入口；Attempt bookkeeping 由服务自动完成。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func assignWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "assign_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Create and dispatch one Assignment for a ready Work Item to exactly one responsible Agent. " +
			"Use strategy=room_member for a tracked Room handoff and strategy=self for the current Agent. This records ownership; a later subagent remains internal to that owner. " +
			"Assigning sibling Work Items to the same Agent creates a serial queue, not another concurrent Agent slot. Use different Room members for independent managed parallel work, or let one owner's current Work Item use native subagents for local parallelism.",
		SearchHint:  "assign work room handoff responsibility agent",
		InputSchema: assignWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed assignWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.AssignWork(ctx, actor, orchestration.AssignWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        command,
				WorkItemID:       parsed.WorkItemID,
				LogicalKey:       parsed.LogicalKey,
				TargetAgentID:    parsed.TargetAgentID,
				ReturnToAgentID:  parsed.ReturnToAgentID,
				Strategy:         parsed.Strategy,
				Reason:           parsed.Reason,
				Instruction:      parsed.Instruction,
				DispatchKind:     parsed.DispatchKind,
			})
			if err == nil && applyMutationResponsibilityAuthority(sctx, response) {
				actor = sctx.Actor()
			}
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func submitWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "submit_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Append the current Assignment owner's concrete result and evidence as an immutable Submission for the selected reviewer. " +
			"Tool availability, assigned_work, and current_actor are state projections, not proof that this call carries a trusted WorkBinding. " +
			"Only an exact host-issued WorkBinding permits omitting work_item_id, logical_key, and assignment_id; explicit values must match it. In DM coordination or any unbound round, provide work_item_id or logical_key; assignment_id remains optional. " +
			"The backend correlates the Attempt and routes review; downstream hard dependencies remain locked until Acceptance.",
		SearchHint:  "submit work deliverable evidence assignment",
		InputSchema: submitWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed submitWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			sdkSessionID := ""
			toolUseID := ""
			if callContext != nil {
				sdkSessionID = callContext.SessionID
				toolUseID = callContext.ToolUseID
			}
			response, err := svc.SubmitWork(ctx, actor, orchestration.SubmitWorkInput{
				ExecutionID:       snapshot.Execution.ID,
				SnapshotRevision:  snapshot.Execution.Version,
				CommandID:         command,
				WorkItemID:        parsed.WorkItemID,
				LogicalKey:        parsed.LogicalKey,
				AssignmentID:      parsed.AssignmentID,
				ResultSummary:     parsed.ResultSummary,
				ResultRefs:        parsed.ResultRefs,
				Evidence:          parsed.Evidence,
				RuntimeSessionKey: sctx.RuntimeSessionKey,
				RoomSessionID:     sctx.RoomSessionID,
				SDKSessionID:      sdkSessionID,
				ToolUseID:         toolUseID,
			})
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func reviewWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "review_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Append the Assignment-selected reviewer's immutable decision for one Submission. " +
			"Tool availability, assigned_work, and current_actor are state projections, not proof that this call carries a trusted ReviewBinding or WorkBinding. " +
			"Only an exact host-issued ReviewBinding, or a permitted self-review exact WorkBinding, permits omitting submission_id, work_item_id, and logical_key; explicit values must match the bound target. In DM coordination or any unbound round, provide at least one of submission_id, work_item_id, or logical_key. " +
			"Accepted requires a passing result for every acceptance criterion and is the only decision that unlocks downstream hard dependencies.",
		SearchHint:  "review accept reject changes requested criteria",
		InputSchema: reviewWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed reviewWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.ReviewWork(ctx, actor, orchestration.ReviewWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        command,
				SubmissionID:     parsed.SubmissionID,
				WorkItemID:       parsed.WorkItemID,
				LogicalKey:       parsed.LogicalKey,
				Decision:         parsed.Decision,
				CriteriaResults:  parsed.CriteriaResults,
				Feedback:         parsed.Feedback,
			})
			if err == nil && applyMutationResponsibilityAuthority(sctx, response) {
				actor = sctx.Actor()
			}
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func blockWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "block_work"
	return sdktool.Tool{
		Name:        toolName,
		Description: "Put one Work Item into waiting_input because a specific external input or authority is missing. Inside an exact trusted WorkBinding, omit work_item_id and logical_key; explicit values must match it. Ordinary Plan dependencies are derived automatically and are not blockers.",
		SearchHint:  "block work external input authority dependency",
		InputSchema: blockWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed blockWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.BlockWork(ctx, actor, orchestration.BlockWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        command,
				WorkItemID:       parsed.WorkItemID,
				LogicalKey:       parsed.LogicalKey,
				Reason:           parsed.Reason,
				NeededInput:      parsed.NeededInput,
			})
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func resumeWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "resume_work"
	return sdktool.Tool{
		Name:        toolName,
		Description: "Reopen one waiting_input Work Item after its exact external blocker is resolved. Inside an exact trusted WorkBinding, omit work_item_id and logical_key; explicit values must match it. Provide resolution evidence; this creates no Assignment and never revives an old Attempt.",
		SearchHint:  "resume unblock work resolved input evidence",
		InputSchema: resumeWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed resumeWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.ResumeWork(ctx, actor, orchestration.ResumeWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        command,
				WorkItemID:       parsed.WorkItemID,
				LogicalKey:       parsed.LogicalKey,
				Resolution:       parsed.Resolution,
				Evidence:         parsed.Evidence,
			})
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func takeOverWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "take_over_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Coordinator-only atomic replacement of the current responsible Agent after a concrete failure, timeout, conflict, or explicit reassignment need. " +
			"It releases the old Assignment and creates one replacement; never create parallel owners for the same deliverable.",
		SearchHint:  "take over reassign work owner failure timeout",
		InputSchema: takeOverWorkSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed takeOverWorkInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.TakeOverWork(ctx, actor, orchestration.TakeOverWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        command,
				WorkItemID:       parsed.WorkItemID,
				LogicalKey:       parsed.LogicalKey,
				TargetAgentID:    parsed.TargetAgentID,
				ReturnToAgentID:  parsed.ReturnToAgentID,
				Strategy:         parsed.Strategy,
				Reason:           parsed.Reason,
				Instruction:      parsed.Instruction,
				DispatchKind:     parsed.DispatchKind,
			})
			if err == nil && applyMutationResponsibilityAuthority(sctx, response) {
				actor = sctx.Actor()
			}
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func bindMutationWorkBinding(
	sctx contract.ServerContext,
	result orchestration.MutationResult,
) bool {
	if result.Outcome != orchestration.MutationApplied &&
		result.Outcome != orchestration.MutationNoOp ||
		sctx.ScopeKind != protocol.ExecutionScopeRoom ||
		result.WorkBinding == nil ||
		result.WorkBinding.Clear ||
		result.WorkBinding.Binding == nil {
		return false
	}
	if sctx.ResponsibilityAuthority != nil {
		return sctx.ResponsibilityAuthority.BindWork(result.WorkBinding.Binding)
	}
	if sctx.WorkBindingState == nil {
		return false
	}
	return sctx.WorkBindingState.Bind(result.WorkBinding.Binding)
}

func applyMutationWorkBindingTransition(
	sctx contract.ServerContext,
	result orchestration.MutationResult,
) bool {
	if bindMutationWorkBinding(sctx, result) {
		return true
	}
	if result.Outcome != orchestration.MutationApplied &&
		result.Outcome != orchestration.MutationNoOp ||
		result.WorkBinding == nil ||
		!result.WorkBinding.Clear {
		return false
	}
	if sctx.ResponsibilityAuthority != nil {
		return sctx.ResponsibilityAuthority.BindCoordination(result.ExecutionID)
	}
	if sctx.WorkBindingState == nil {
		return false
	}
	sctx.WorkBindingState.Clear()
	return true
}

// applyMutationResponsibilityAuthority consumes only host-issued, in-process
// receipts. It advances the complete lane in one generation so the next tool
// call cannot observe a new ExecutionID beside an old ReviewBinding.
func applyMutationResponsibilityAuthority(
	sctx contract.ServerContext,
	result orchestration.MutationResult,
) bool {
	if result.Outcome != orchestration.MutationApplied &&
		result.Outcome != orchestration.MutationNoOp &&
		result.Outcome != orchestration.MutationSuperseded {
		return false
	}
	changed := false
	if sctx.ResponsibilityAuthority != nil && result.ResponsibilityAuthority != nil {
		changed = sctx.ResponsibilityAuthority.BindCoordination(
			result.ResponsibilityAuthority.ExecutionID,
		) || changed
	}
	if result.WorkBinding != nil {
		changed = applyMutationWorkBindingTransition(sctx, result) || changed
	}
	if sctx.ResponsibilityAuthority != nil && result.Snapshot != nil {
		switch result.Snapshot.Execution.Status {
		case protocol.ExecutionStatusCompleted,
			protocol.ExecutionStatusFailed,
			protocol.ExecutionStatusCancelled,
			protocol.ExecutionStatusSuperseded:
			changed = sctx.ResponsibilityAuthority.RevokeExecution(
				result.Snapshot.Execution.ID,
			) || changed
		}
	}
	return changed
}

func mutationEnvelope(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	actor orchestration.ActorContext,
	executionID string,
	toolName string,
	input map[string]any,
	callContext *sdktool.CallContext,
) (*orchestrationSnapshot, string, *sdktool.ToolResult) {
	snapshot, err := loadSnapshot(ctx, svc, actor, executionID)
	if err != nil {
		if result, ok := recoverableMutationRejection(err); ok {
			return nil, "", &result
		}
		result := transportErrorResult(err)
		return nil, "", &result
	}
	if snapshot == nil {
		result := rejectedResult("no current Execution exists; use prepare_plan_execution with one complete Nexus Plan Document, then commit its sealed proposal")
		return nil, "", &result
	}
	command, err := commandID(sctx, callContext, toolName, input, snapshot.Execution.Version)
	if err != nil {
		result := transportErrorResult(err)
		return nil, "", &result
	}
	return (*orchestrationSnapshot)(snapshot), command, nil
}

// orchestrationSnapshot is a local alias that keeps the helper signature
// readable without introducing another model-facing type.
type orchestrationSnapshot = protocol.ExecutionSnapshot

func serviceMutation(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	result orchestration.MutationResult,
	err error,
) sdktool.ToolResult {
	if err != nil {
		return transportErrorResult(err)
	}
	return mutationResult(withFreshExecutionContext(ctx, svc, actor, result))
}
