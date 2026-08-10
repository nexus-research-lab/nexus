// INPUT: owner-main 私有 DM 的可选 agent_id。
// OUTPUT: 不含 workspace、options、MCP 配置或凭据的 Agent 目录。
// POS: nexus_manager Agent 查询工具；写配置仍归 nexus_config。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func listAgents(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_nexus_agents",
		Description: "列出当前 owner 的脱敏 Agent 目录。不会返回 workspace 绝对路径、运行时 options、MCP server 配置或秘密。",
		SearchHint:  "Nexus list agents directory identity status",
		InputSchema: emptySchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, _ map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.ListAgents(ctx, sctx.Actor())
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func getAgent(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_nexus_agent",
		Description: "读取当前 owner 下一个 Agent 的脱敏身份与状态。Agent 创建和配置变更必须改用 nexus_config。",
		SearchHint:  "Nexus get agent inspect identity status",
		InputSchema: idSchema("agent_id", "当前 owner 下的 agent_id"),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.GetAgent(ctx, sctx.Actor(), stringArg(args, "agent_id"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
