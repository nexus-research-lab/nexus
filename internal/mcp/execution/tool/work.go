// INPUT: Assignment、Submission、Acceptance、Block 与 Takeover 的模型语义 intent。
// OUTPUT: 使用最新 snapshot revision 和稳定 command id 的统一 MutationResult。
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
		Description: "Assign one ready Work Item to exactly one responsible Agent. " +
			"In a Room, use strategy=room_member for a structured, visible handoff; the backend dispatches it, so do not duplicate the assignment with a hand-written @ message. " +
			"Assignment expresses responsibility. A responsible Agent may later use a subagent internally without transferring ownership.",
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
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func submitWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "submit_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Submit the current Assignment owner's concrete result and evidence for review. " +
			"In a Room, the backend durably routes the immutable Submission to its configured reviewer; do not depend on a hand-written @ message for review return. " +
			"Do not call start_work or report machine state: the backend records the Attempt automatically. Submission does not unlock downstream hard dependencies until accepted.",
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
		Description: "Append the coordinator's acceptance decision for an immutable Submission. " +
			"Accepted requires a passing result for every criterion and is the only decision that unlocks downstream hard dependencies. Review is intentional verification, not duplicate production.",
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
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func blockWork(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "block_work"
	return sdktool.Tool{
		Name: toolName,
		Description: "Record that a Work Item cannot continue without one specific external input or authority. " +
			"Do not use this for ordinary Plan dependencies; those are derived automatically.",
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
		Name: toolName,
		Description: "Reopen one waiting_input Work Item only after its exact external blocker is resolved. " +
			"Provide concrete resolution evidence; this creates no Assignment and never revives an old Attempt.",
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
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
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
		result := transportErrorResult(err)
		return nil, "", &result
	}
	if snapshot == nil {
		result := rejectedResult("no current Execution exists; use plan_execution with an objective and completion criteria first")
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
