// INPUT: 隔离内部 Session 中宿主绑定的 exact preview_id，或隐藏编辑 Session 提交的完整草图 revision，以及 trusted owner/session identity。
// OUTPUT: owner-scoped 可复用命名 WorkGraph，或经 exact-preview/CAS/DAG 校验的新草图 revision。
// POS: execution-orchestrator Skill 到隐藏编辑/后台沉淀 service 的受限 CLI 入口；普通会话 authoring 由 workflow_authoring.go 暴露。
package operation

import (
	"context"
	"errors"
	"strings"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildWorkGraphEditor 只为 exact 隐藏编辑 Session 暴露草图修改和版本选择 operation。
func BuildWorkGraphEditor(
	svc contract.WorkflowEditorService,
	sctx contract.Context,
) []runtimecommand.Operation {
	if svc == nil {
		return nil
	}
	return []runtimecommand.Operation{
		reviseWorkGraphPreview(svc, sctx),
		selectBoundWorkGraphPreviewRevision(svc, sctx),
	}
}

// BuildWorkGraphDistillation 只为宿主绑定 exact preview 的隔离内部 Session 暴露保存 operation。
func BuildWorkGraphDistillation(
	svc contract.WorkflowService,
	sctx contract.Context,
) []runtimecommand.Operation {
	if svc == nil || strings.TrimSpace(sctx.WorkGraphPreviewID) == "" {
		return nil
	}
	return []runtimecommand.Operation{distillWorkGraphWorkflow(svc, sctx)}
}

func reviseWorkGraphPreview(
	svc contract.WorkflowEditorService,
	sctx contract.Context,
) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        "revise_workgraph_preview",
		Description: "提交当前隐藏编辑 Session 的完整 WorkGraph 草图 revision。服务端以 revision CAS 校验命令名、节点、父子结构、DAG、关键路径和最终交付。",
		SearchHint:  "修改 WorkGraph 草图 命令 标题 节点 结构 依赖",
		InputSchema: reviseWorkflowPreviewSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var request protocol.ReviseWorkGraphWorkflowPreviewRequest
			if err := decodeInput(input, &request); err != nil {
				return transportErrorResult(err), nil
			}
			result, err := svc.ReviseEditorPreview(
				ctx,
				strings.TrimSpace(sctx.OwnerUserID),
				strings.TrimSpace(sctx.RuntimeSessionKey),
				request,
			)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome":          "applied",
				"revision":         result.Revision,
				"title":            result.Preview.Title,
				"node_count":       len(result.Preview.Nodes),
				"dependency_count": len(result.Preview.Dependencies),
				"changed":          []string{"workgraph_preview:" + result.EditorID},
				"message":          "工作图草图已更新。",
			}), nil
		},
	}
}

func selectBoundWorkGraphPreviewRevision(
	svc contract.WorkflowEditorService,
	sctx contract.Context,
) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        "select_workgraph_preview_revision",
		Description: "选择当前隐藏编辑 Session 的既有不可变版本作为后续修改基线；不删除其他版本。",
		SearchHint:  "选择 回退 WorkGraph 草图 版本",
		InputSchema: selectBoundWorkflowPreviewRevisionSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed selectWorkflowPreviewRevisionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			result, err := svc.SelectEditorVersionBySession(
				ctx, strings.TrimSpace(sctx.OwnerUserID), strings.TrimSpace(sctx.RuntimeSessionKey),
				parsed.Revision, parsed.SelectedRevision,
			)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "selected", "revision": result.Revision,
				"selected_revision": result.SelectedRevision,
				"title":             result.Preview.Title,
				"changed":           []string{"workgraph_preview:" + result.EditorID},
				"message":           "已选择指定 WorkGraph 草图版本作为后续修改基线。",
			}), nil
		},
	}
}

func distillWorkGraphWorkflow(
	svc contract.WorkflowService,
	sctx contract.Context,
) runtimecommand.Operation {
	const operationName = "distill_workgraph"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "原样持久化默认对话模型已生成、用户已确认且尚未过期的 WorkGraph 草图。" +
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
			if strings.TrimSpace(parsed.PreviewID) != strings.TrimSpace(sctx.WorkGraphPreviewID) {
				return transportErrorResult(errors.New("preview_id does not match the host-bound WorkGraph save request")), nil
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
