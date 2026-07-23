// INPUT: 可选配置域与审计条数。
// OUTPUT: 当前 owner 的脱敏变更审计。
// POS: nexus_config 的追溯与幂等状态查询工具。
package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

func history(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "list_nexus_configuration_changes",
		Description: "查询当前用户的 Nexus 配置变更审计，包含 actor、操作、revision、状态和脱敏请求/结果。" +
			"用于核对成功、失败、幂等重放与配置漂移，不返回明文凭据。",
		SearchHint:  "Nexus configuration history audit changes revision status",
		InputSchema: historySchema(), Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.ListChanges(ctx, sctx.Actor(), stringArg(args, "domain"), intArg(args, "limit"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
