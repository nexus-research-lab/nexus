// INPUT: trusted Session、completed Execution、Draft preview 与隔离保存 identity。
// OUTPUT: WorkGraph library、可恢复 Draft、不可变版本与 owner-scoped 命名图。
// POS: execution-orchestrator Skill 到 WorkGraph authoring/edit/save service 的统一入口。
package operation

import (
	"context"
	"errors"
	"strings"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildWorkGraphAuthoring 为可信普通 Session 暴露草图查询、提取、修改、版本选择与保存。
func BuildWorkGraphAuthoring(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) []command.Operation {
	if svc == nil || strings.TrimSpace(sctx.OwnerUserID) == "" || strings.TrimSpace(sctx.ScopeSessionKey) == "" {
		return nil
	}
	return []command.Operation{
		inspectWorkGraphLibrary(svc, sctx),
		extractWorkGraphPreview(svc, sctx),
		getWorkGraphPreview(svc, sctx),
		reviseWorkGraphDraft(svc, sctx),
		selectWorkGraphPreviewRevision(svc, sctx),
		saveWorkGraphPreview(svc, sctx),
	}
}

func inspectWorkGraphLibrary(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "inspect_workgraph_library",
		Description: "列出当前 Session 的 completed WorkGraph 来源、已提取 Draft 及 owner 已保存命名图。一个会话可能有多张图；不要靠聊天记忆统计。",
		SearchHint:  "查询 列出 历史 WorkGraph 草图 Draft 已保存 命名图",
		InputSchema: objectSchema(nil),
		Annotations: &command.OperationAnnotations{ReadOnly: true, ReadOnlyHint: true},
		ContextHandler: func(ctx context.Context, _ map[string]any, _ *command.CallContext) (command.Result, error) {
			library, err := svc.InspectLibrary(ctx, sctx.OwnerUserID, sctx.ScopeSessionKey)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(workGraphLibraryPayload(library)), nil
		},
	}
}

func extractWorkGraphPreview(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "extract_workgraph_preview",
		Description: "从当前 Session 的 exact completed WorkGraph 提取或复用可恢复 Draft。相同 source Execution 已有 Draft 时不会重复模型提取。",
		SearchHint:  "提取 沉淀 WorkGraph 草图 Draft completed execution",
		InputSchema: extractWorkflowPreviewSchema(),
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			var parsed extractWorkflowPreviewInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			preview, err := svc.PreviewFromExecution(ctx, sctx.OwnerUserID, protocol.PreviewWorkGraphWorkflowRequest{
				SourceSessionKey: sctx.ScopeSessionKey, SourceExecutionID: parsed.SourceExecutionID,
				OutputLanguage: parsed.OutputLanguage,
			})
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "ready", "preview": preview,
				"changed": []string{"workgraph_preview:" + preview.PreviewID},
				"message": "WorkGraph 草图已就绪；保存前请让用户确认当前版本。",
			}), nil
		},
	}
}

func getWorkGraphPreview(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "get_workgraph_preview",
		Description: "读取当前 Session 一张 Draft 的 selected preview、head revision 与不可变版本目录。",
		SearchHint:  "读取 WorkGraph Draft 版本 preview",
		InputSchema: getWorkflowPreviewSchema(),
		Annotations: &command.OperationAnnotations{ReadOnly: true, ReadOnlyHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			var parsed getWorkflowPreviewInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			draft, err := svc.GetDraft(ctx, sctx.OwnerUserID, sctx.ScopeSessionKey, parsed.PreviewID)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(workGraphDraftPayload(draft)), nil
		},
	}
}

func reviseWorkGraphDraft(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "revise_workgraph_preview",
		Description: "按当前 selected version 提交完整 WorkGraph Draft 新版本。revision 必须使用当前 head_revision；服务端校验命名、父子结构、DAG、关键路径与最终交付。",
		SearchHint:  "修改 编辑 WorkGraph 草图 节点 依赖 版本",
		InputSchema: reviseWorkflowDraftSchema(),
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			var request protocol.ReviseWorkGraphWorkflowDraftRequest
			if err := decodeInput(input, &request); err != nil {
				return transportErrorResult(err), nil
			}
			draft, err := svc.ReviseDraftPreview(ctx, sctx.OwnerUserID, sctx.ScopeSessionKey, request)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "applied", "draft": workGraphDraftPayload(draft),
				"changed": []string{"workgraph_preview:" + draft.PreviewID},
				"message": "WorkGraph 草图新版本已创建。",
			}), nil
		},
	}
}

func selectWorkGraphPreviewRevision(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "select_workgraph_preview_revision",
		Description: "选择一条既有不可变版本作为当前偏好基线；不删除较新的版本。后续修改会从所选版本继续。",
		SearchHint:  "选择 回退 比较 WorkGraph 草图 版本",
		InputSchema: selectWorkflowPreviewRevisionSchema(),
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			var parsed selectWorkflowPreviewRevisionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			draft, err := svc.SelectDraftRevision(
				ctx, sctx.OwnerUserID, sctx.ScopeSessionKey, parsed.PreviewID,
				parsed.Revision, parsed.SelectedRevision,
			)
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "selected", "draft": workGraphDraftPayload(draft),
				"changed": []string{"workgraph_preview:" + draft.PreviewID},
				"message": "已选择指定 WorkGraph 草图版本作为后续编辑基线。",
			}), nil
		},
	}
}

func saveWorkGraphPreview(
	svc contract.WorkflowAuthoringService,
	sctx contract.Context,
) command.Operation {
	const operationName = "save_workgraph_preview"
	return command.Operation{
		Name:        operationName,
		Description: "在用户于当前对话中明确确认后，原样保存 exact selected WorkGraph Draft 为命名图。不得在未确认时调用。",
		SearchHint:  "确认 保存 沉淀 WorkGraph Draft Slash 命名图",
		InputSchema: getWorkflowPreviewSchema(),
		Idempotent:  true,
		ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
			var parsed getWorkflowPreviewInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			commandID, err := commandID(sctx, call, operationName, input, 0)
			if err != nil {
				return transportErrorResult(err), nil
			}
			created, err := svc.SavePreview(ctx, sctx.OwnerUserID, protocol.SaveWorkGraphWorkflowRequest{
				CommandID: commandID, SourceSessionKey: sctx.ScopeSessionKey, PreviewID: parsed.PreviewID,
			})
			if err != nil {
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome": "applied", "workflow_id": created.ID,
				"slash_name": created.SlashName, "command": "/" + created.SlashName,
				"workflow": created,
				"changed":  []string{"workgraph_workflow:" + created.ID},
				"message":  "WorkGraph 命名图已保存。",
			}), nil
		},
	}
}

func workGraphLibraryPayload(library *protocol.WorkGraphWorkflowLibrary) map[string]any {
	if library == nil {
		return map[string]any{"sources": []any{}, "drafts": []any{}, "workflows": []any{}}
	}
	workflows := make([]map[string]any, 0, len(library.Workflows))
	for _, workflow := range library.Workflows {
		workflows = append(workflows, map[string]any{
			"workflow_id": workflow.ID, "slash_name": workflow.SlashName,
			"title": workflow.Title, "source_execution_id": workflow.SourceExecutionID,
			"version": workflow.Version, "node_count": len(workflow.Nodes),
			"dependency_count": len(workflow.Dependencies), "updated_at": workflow.UpdatedAt,
		})
	}
	return map[string]any{"sources": library.Sources, "drafts": library.Drafts, "workflows": workflows}
}

func workGraphDraftPayload(draft *protocol.WorkGraphWorkflowDraft) map[string]any {
	if draft == nil {
		return map[string]any{}
	}
	return map[string]any{
		"preview_id": draft.PreviewID, "source_execution_id": draft.SourceExecutionID,
		"head_revision": draft.HeadRevision, "selected_revision": draft.SelectedRevision,
		"preview":        draft.Preview,
		"versions":       previewVersionSummariesForCommand(draft.Versions, draft.SelectedRevision),
		"save_scheduled": draft.SaveScheduled, "saved_workflow_id": draft.SavedWorkflowID,
	}
}

func previewVersionSummariesForCommand(
	versions []protocol.WorkGraphWorkflowPreviewVersion,
	selectedRevision int64,
) []protocol.WorkGraphWorkflowPreviewVersionSummary {
	result := make([]protocol.WorkGraphWorkflowPreviewVersionSummary, 0, len(versions))
	for _, version := range versions {
		result = append(result, protocol.WorkGraphWorkflowPreviewVersionSummary{
			Revision: version.Revision, SlashName: version.Preview.SlashName,
			Title: version.Preview.Title, NodeCount: len(version.Preview.Nodes),
			DependencyCount: len(version.Preview.Dependencies),
			Selected:        version.Revision == selectedRevision, CreatedAt: version.CreatedAt,
		})
	}
	return result
}

// BuildWorkGraphEditor 只为 exact 隐藏编辑 Session 暴露草图修改和版本选择 operation。
func BuildWorkGraphEditor(
	svc contract.WorkflowEditorService,
	sctx contract.Context,
) []command.Operation {
	if svc == nil {
		return nil
	}
	return []command.Operation{
		reviseWorkGraphPreview(svc, sctx),
		selectBoundWorkGraphPreviewRevision(svc, sctx),
	}
}

// BuildWorkGraphDistillation 只为宿主绑定 exact preview 的隔离内部 Session 暴露保存 operation。
func BuildWorkGraphDistillation(
	svc contract.WorkflowService,
	sctx contract.Context,
) []command.Operation {
	if svc == nil || strings.TrimSpace(sctx.WorkGraphPreviewID) == "" {
		return nil
	}
	return []command.Operation{distillWorkGraphWorkflow(svc, sctx)}
}

func reviseWorkGraphPreview(
	svc contract.WorkflowEditorService,
	sctx contract.Context,
) command.Operation {
	return command.Operation{
		Name:        "revise_workgraph_preview",
		Description: "提交当前隐藏编辑 Session 的完整 WorkGraph 草图 revision。服务端以 revision CAS 校验命令名、节点、父子结构、DAG、关键路径和最终交付。",
		SearchHint:  "修改 WorkGraph 草图 命令 标题 节点 结构 依赖",
		InputSchema: reviseWorkflowPreviewSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *command.CallContext,
		) (command.Result, error) {
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
) command.Operation {
	return command.Operation{
		Name:        "select_workgraph_preview_revision",
		Description: "选择当前隐藏编辑 Session 的既有不可变版本作为后续修改基线；不删除其他版本。",
		SearchHint:  "选择 回退 WorkGraph 草图 版本",
		InputSchema: selectBoundWorkflowPreviewRevisionSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *command.CallContext,
		) (command.Result, error) {
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
) command.Operation {
	const operationName = "distill_workgraph"
	return command.Operation{
		Name: operationName,
		Description: "原样持久化默认对话模型已生成、用户已确认且尚未过期的 WorkGraph 草图。" +
			"宿主负责绑定所有者和来源会话；命令只接收 preview_id，不得重新选择节点或改写草图。",
		SearchHint:  "保存 已确认 可复用 WorkGraph 草图 preview Slash 命令",
		InputSchema: distillWorkflowSchema(),
		Idempotent:  true,
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			call *command.CallContext,
		) (command.Result, error) {
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
