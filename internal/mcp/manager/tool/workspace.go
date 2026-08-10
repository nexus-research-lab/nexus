// INPUT: owner-main 固定 owner Agent 目标，或普通 Agent 服务端固定的自身身份。
// OUTPUT: 有界 workspace 文件目录与内容。
// POS: nexus_manager 唯一 Agent-self 能力；严格只读。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func listWorkspace(svc contract.Service, sctx contract.ServerContext, withAgentID bool) sdktool.Tool {
	description := "列出当前 Agent 自身 workspace 的有界文件目录；目标由服务端固定，不能读取其他 Agent。"
	if withAgentID {
		description = "列出当前 owner 下指定 Agent workspace 的有界文件目录。"
	}
	return sdktool.Tool{
		Name: "list_nexus_workspace", Description: description,
		SearchHint:  "Nexus workspace list files inspect own agent",
		InputSchema: listSchema(withAgentID),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			agentID := ""
			if withAgentID {
				agentID = stringArg(args, "agent_id")
			}
			result, err := svc.ListWorkspaceFiles(
				ctx, sctx.Actor(), agentID, intArg(args, "limit"),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func readWorkspaceFile(svc contract.Service, sctx contract.ServerContext, withAgentID bool) sdktool.Tool {
	description := "读取当前 Agent 自身 workspace 的一个文件；目标由服务端固定，内容有大小上限。"
	if withAgentID {
		description = "读取当前 owner 下指定 Agent workspace 的一个文件；内容有大小上限。"
	}
	return sdktool.Tool{
		Name: "read_nexus_workspace_file", Description: description,
		SearchHint:  "Nexus workspace read file inspect own agent",
		InputSchema: readFileSchema(withAgentID),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, MaxResultSizeChars: 140000},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			agentID := ""
			if withAgentID {
				agentID = stringArg(args, "agent_id")
			}
			result, err := svc.ReadWorkspaceFile(
				ctx, sctx.Actor(), agentID, stringArg(args, "path"), intArg(args, "max_bytes"),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
