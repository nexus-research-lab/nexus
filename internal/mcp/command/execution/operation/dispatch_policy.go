// INPUT: Coordinator's full explicit dispatch policy for the current Plan.
// OUTPUT: A durable configuration receipt; future dependency transitions need no model command.
// POS: Parser-owned scheduling operation, separate from immediate ready assignment.
package operation

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func configureDispatch(svc contract.Service, sctx contract.Context) command.Operation {
	const name = "configure_dispatch"
	item := objectSchema(map[string]any{"work_item_id": stringProperty("Exact current Work Item, or use logical_key."), "logical_key": stringProperty("Current Plan logical key."), "target_agent_id": stringProperty("Explicit owner Agent."), "return_to_agent_id": stringProperty("Explicit reviewer Agent."), "instruction": stringProperty("Optional execution instruction."), "dispatch_kind": enumProperty("Room routing policy.", "room_directed", "room_public")}, "target_agent_id", "return_to_agent_id")
	return command.Operation{Name: name, Description: "Replace pending automatic dispatch policies for the current Plan. Each policy names an unassigned Work Item, owner and reviewer. The host assigns it once after hard dependencies are accepted, honoring queue/Goal pauses and rechecking membership. Empty policies cancel pending policies; already assigned work is unchanged. A changed Plan invalidates its old policies. No automatic retry, takeover or acceptance.", SearchHint: "automatic dependency dispatch schedule policy", InputSchema: objectSchema(map[string]any{"execution_id": stringProperty("Current Execution."), "policies": map[string]any{"type": "array", "maxItems": 32, "items": item}}, "policies"), Annotations: &command.OperationAnnotations{IdempotentHint: true}, ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
		var parsed struct {
			ExecutionID string                              `json:"execution_id"`
			Policies    []orchestration.DispatchPolicyInput `json:"policies"`
		}
		if err := decodeInput(input, &parsed); err != nil {
			return transportErrorResult(err), nil
		}
		if _, present := input["policies"]; !present {
			return transportErrorResult(errors.New("policies must be explicitly supplied; use [] to cancel pending policies")), nil
		}
		actor := sctx.Actor()
		snapshot, id, rejected := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, name, input, call)
		if rejected != nil {
			return *rejected, nil
		}
		provider, ok := svc.(interface {
			ConfigureDispatch(context.Context, orchestration.ActorContext, orchestration.ConfigureDispatchInput) (orchestration.MutationResult, error)
		})
		if !ok {
			return transportErrorResult(errors.New("dispatch policy service unavailable")), nil
		}
		result, err := provider.ConfigureDispatch(ctx, actor, orchestration.ConfigureDispatchInput{ExecutionID: snapshot.Execution.ID, SnapshotRevision: snapshot.Execution.Version, CommandID: id, Policies: parsed.Policies})
		return serviceMutation(ctx, svc, actor, result, err), nil
	}}
}
