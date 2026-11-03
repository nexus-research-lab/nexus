// INPUT: Explicit owner progress at a meaningful resume boundary.
// OUTPUT: Immutable checkpoint receipt; normal work and acceptance remain separate.
// POS: Model-facing checkpoint schema and domain adapter.
package operation

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func recordCheckpoint(svc contract.Service, sctx contract.Context) command.Operation {
	const name = "record_checkpoint"
	properties := workReferenceProperties()
	properties["summary"] = stringProperty("Concrete progress observed so far; never a completion claim.")
	properties["next_step"] = stringProperty("Next step after reconciling these artifacts and receipts.")
	properties["artifact_refs"] = map[string]any{"type": "array", "maxItems": 32, "items": stringProperty("Existing artifact reference.")}
	properties["evidence"] = map[string]any{"type": "array", "maxItems": 32, "items": stringProperty("Concrete progress evidence.")}
	properties["effects"] = map[string]any{"type": "array", "maxItems": 32, "items": objectSchema(map[string]any{"operation": stringProperty("External effect needing reconciliation."), "outcome": enumProperty("Unknown outcomes must be reconciled before retry.", "confirmed", "unknown"), "receipt_ref": stringProperty("Durable receipt reference, required for confirmed effects.")}, "operation", "outcome")}
	return command.Operation{Name: name, Description: "Record a meaningful progress checkpoint for your current root Work Item, at most 32 per Attempt. It does not submit, accept, unlock dependencies, or automatically retry anything. Record external effects as unknown when confirmation is missing, and reconcile artifacts/receipts before resuming. The latest checkpoint of the current spec survives takeover as read-only context.", SearchHint: "checkpoint resume progress evidence", InputSchema: objectSchema(properties, "summary", "next_step"), Annotations: &command.OperationAnnotations{IdempotentHint: true}, ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
		var parsed struct {
			ExecutionID string `json:"execution_id"`
			WorkItemID  string `json:"work_item_id"`
			LogicalKey  string `json:"logical_key"`
			protocol.WorkCheckpointData
		}
		if err := decodeInput(input, &parsed); err != nil {
			return transportErrorResult(err), nil
		}
		actor := sctx.Actor()
		snapshot, id, rejected := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, name, input, call)
		if rejected != nil {
			return *rejected, nil
		}
		provider, ok := svc.(interface {
			RecordCheckpoint(context.Context, orchestration.ActorContext, orchestration.RecordCheckpointInput) (orchestration.MutationResult, error)
		})
		if !ok {
			return transportErrorResult(errors.New("checkpoint service unavailable")), nil
		}
		result, err := provider.RecordCheckpoint(ctx, actor, orchestration.RecordCheckpointInput{ExecutionID: snapshot.Execution.ID, SnapshotRevision: snapshot.Execution.Version, CommandID: id, WorkItemID: parsed.WorkItemID, LogicalKey: parsed.LogicalKey, Data: parsed.WorkCheckpointData})
		return serviceMutation(ctx, svc, actor, result, err), nil
	}}
}
