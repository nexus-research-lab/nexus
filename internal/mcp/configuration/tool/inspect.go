// INPUT: 可选配置域列表与本地 verify 标志。
// OUTPUT: 当前 scope 的脱敏值、状态版本、每域 revision、授权操作目录与健康检查。
// POS: nexus_config 的首选发现和变更后核对工具。
package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

func inspect(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "inspect_nexus_configuration",
		Description: "读取 Nexus 配置控制面。返回各域真相源、管理工具、允许操作、脱敏当前值、revision 与健康检查。" +
			"任何配置修改前先调用本工具或 plan；不要向用户回显 input 中的凭据、内部提示词或其他秘密。",
		SearchHint: "Nexus configuration settings providers agents channels connectors skills sessions host inspect verify",
		AlwaysLoad: true, InputSchema: inspectSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.Inspect(ctx, sctx.Actor(), stringSliceArg(args, "domains"), boolArg(args, "verify"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
