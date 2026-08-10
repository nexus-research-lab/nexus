// INPUT: owner-main 的可选 agent_id、session_key 与列表上限。
// OUTPUT: 不含 DB/SDK session ID 和 options 的统一 session 视图。
// POS: nexus_manager session 查询工具。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func listSessions(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_nexus_sessions",
		Description: "列出当前 owner 的统一 session 目录，可按 agent_id 过滤；不会返回内部 DB/SDK session ID 或 options。",
		SearchHint:  "Nexus list sessions conversations runtime activity",
		InputSchema: listSessionsSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.ListSessions(
				ctx, sctx.Actor(), stringArg(args, "agent_id"), intArg(args, "limit"),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func getSession(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_nexus_session",
		Description: "读取当前 owner 下一个非 Room session 的脱敏状态；Room 运行上下文请使用 get_nexus_conversation。",
		SearchHint:  "Nexus get session runtime activity status",
		InputSchema: idSchema("session_key", "结构化 Agent session_key"),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.GetSession(ctx, sctx.Actor(), stringArg(args, "session_key"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
