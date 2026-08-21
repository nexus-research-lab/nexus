// INPUT: 用户已确认的 exact 草图 preview_id 与 trusted owner/session/round identity。
// OUTPUT: owner-scoped 可复用命名 WorkGraph，以及只含简体中文叙述的 contract 与保存回执。
// POS: execution-orchestrator Skill 到命名 WorkGraph service 的唯一持久化入口。
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
	const operationName = "distill_workgraph"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "原样持久化默认后台模型已生成、用户已确认且尚未过期的 WorkGraph 草图。" +
			"宿主负责绑定所有者和来源会话；命令只接收 preview_id，不得重新选择节点或改写草图。",
		SearchHint:  "保存 已确认 可复用 WorkGraph 草图 preview Slash 命令",
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
			created, err := svc.SavePreview(ctx, strings.TrimSpace(sctx.OwnerUserID), protocol.SaveWorkGraphWorkflowRequest{
				CommandID: commandID, SourceSessionKey: strings.TrimSpace(sctx.ScopeSessionKey),
				PreviewID: strings.TrimSpace(parsed.PreviewID),
			})
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "applied", "workflow_id": created.ID,
				"slash_name": created.SlashName, "command": "/" + created.SlashName,
				"node_count": len(created.Nodes), "dependency_count": len(created.Dependencies),
				"changed": []string{"workgraph_workflow:" + created.ID},
				"message": "WorkGraph 命令已保存，可以在其他会话中复用。",
			}), nil
		},
	}
}
