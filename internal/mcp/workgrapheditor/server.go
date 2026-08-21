// INPUT: 宿主固化的 owner/editor Session 与模型提交的完整 WorkGraph 草图。
// OUTPUT: 经过 service DAG/交付校验的 preview revision 工具结果。
// POS: 只挂载于隐藏 WorkGraph 编辑 Session 的进程内 MCP transport。
package workgrapheditor

import (
	"context"
	"encoding/json"
	"errors"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const ServerName = "nexus_workgraph_editor"

type Service interface {
	ReviseEditorPreview(context.Context, string, string, protocol.ReviseWorkGraphWorkflowPreviewRequest) (*protocol.WorkGraphWorkflowEditorSession, error)
}

// NewServer 创建只绑定 exact owner/session 的草图修改工具。
func NewServer(svc Service, ownerUserID string, sessionKey string) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(ServerName, "1.0.0", []sdktool.Tool{{
		Name:        "revise_workgraph_preview",
		Description: "提交修改后的完整 WorkGraph 草图。每次必须携带当前 revision 和所有节点、父子关系、依赖；服务端会校验 logical key、节点类型、DAG、关键路径与最终交付。",
		AlwaysLoad:  true,
		InputSchema: revisionSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errors.New("WorkGraph editor service is unavailable")), nil
			}
			payload, err := json.Marshal(args)
			if err != nil {
				return errorResult(err), nil
			}
			var request protocol.ReviseWorkGraphWorkflowPreviewRequest
			if err = json.Unmarshal(payload, &request); err != nil {
				return errorResult(err), nil
			}
			result, err := svc.ReviseEditorPreview(ctx, ownerUserID, sessionKey, request)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(map[string]any{
				"status":     "updated",
				"revision":   result.Revision,
				"title":      result.Preview.Title,
				"node_count": len(result.Preview.Nodes),
			}), nil
		},
	}})
}

func revisionSchema() map[string]any {
	stringField := func(description string) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "description": description}
	}
	nodeSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"logical_key":         stringField("稳定英文标识；新增节点必须创建新的 logical_key"),
			"role":                map[string]any{"type": "string", "enum": []string{"key", "collaboration"}},
			"kind":                map[string]any{"type": "string", "enum": []string{"produce", "review", "verify", "integrate"}},
			"subject":             stringField("面向用户的节点标题"),
			"objective":           stringField("节点目的"),
			"deliverable":         stringField("可验证交付物"),
			"acceptance_criteria": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
			"required":            map[string]any{"type": "boolean"},
			"terminal":            map[string]any{"type": "boolean"},
			"parent_logical_key":  map[string]any{"type": "string"},
			"position":            map[string]any{"type": "integer", "minimum": 0},
		},
		"required": []string{"logical_key", "role", "kind", "subject", "objective", "deliverable", "required", "terminal"},
	}
	edgeSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"logical_key":            stringField("下游节点 logical_key"),
			"depends_on_logical_key": stringField("上游节点 logical_key"),
			"kind":                   map[string]any{"type": "string", "enum": []string{"hard", "soft"}},
		},
		"required": []string{"logical_key", "depends_on_logical_key", "kind"},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"revision":            map[string]any{"type": "integer", "minimum": 1},
			"slash_name":          stringField("英文 kebab-case 命令名，不含斜杠"),
			"title":               stringField("草图标题"),
			"description":         stringField("草图用途说明"),
			"objective":           stringField("供复用时发送给模型的内部执行目标"),
			"completion_criteria": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
			"nodes":               map[string]any{"type": "array", "minItems": 1, "items": nodeSchema},
			"dependencies":        map[string]any{"type": "array", "items": edgeSchema},
		},
		"required": []string{"revision", "slash_name", "title", "description", "objective", "nodes", "dependencies"},
	}
}

func jsonResult(value any) sdktool.ToolResult {
	payload, err := json.Marshal(value)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{Content: []map[string]any{{"type": "text", "text": string(payload)}}}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
