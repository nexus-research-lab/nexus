// INPUT: exact 历史 Execution、显式 Work Item 选择、命名 Slash 与 trusted owner/session/round identity。
// OUTPUT: owner-scoped 可复用 Workflow；只保存责任契约、协作角色与依赖。
// POS: execution-orchestrator Skill 到 WorkGraph Workflow service 的唯一模型写入口。
package operation

import (
	"context"
	"strings"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func distillWorkGraphWorkflow(
	svc contract.WorkflowService,
	sctx contract.Context,
) runtimecommand.Operation {
	const operationName = "distill_workgraph_workflow"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Distill selected semantic responsibility and collaboration nodes from one exact historical Execution into an owner-scoped reusable Slash command. " +
			"Inspect the source Execution first, choose only Work Items, and label each retained node key or collaboration. " +
			"The service preserves selected node contracts and dependencies but never copies Tool calls, runtime identities, Assignments, Attempts, results, Artifacts, Submissions, Reviews, or Acceptances.",
		SearchHint:  "save distill reusable workflow slash command historical workgraph key collaboration nodes",
		InputSchema: distillWorkflowSchema(),
		Idempotent:  true,
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			call *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed distillWorkflowInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			commandID, err := commandID(sctx, call, operationName, input, 0)
			if err != nil {
				return transportErrorResult(err), nil
			}
			created, err := svc.CreateFromExecution(ctx, strings.TrimSpace(sctx.OwnerUserID), protocol.CreateWorkGraphWorkflowRequest{
				CommandID: commandID, SourceSessionKey: strings.TrimSpace(sctx.ScopeSessionKey),
				SourceExecutionID: parsed.ExecutionID, SlashName: parsed.SlashName,
				Title: parsed.Title, Description: parsed.Description, Nodes: parsed.Nodes,
			})
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "applied", "workflow_id": created.ID,
				"slash_name": created.SlashName, "command": "/" + created.SlashName,
				"node_count": len(created.Nodes), "dependency_count": len(created.Dependencies),
				"changed": []string{"workgraph_workflow:" + created.ID},
				"message": "Workflow command is ready for reuse in other sessions.",
			}), nil
		},
	}
}
