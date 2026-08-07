// INPUT: domain/operation/target/input 配置意图。
// OUTPUT: 不写入真相源的 revision、风险、确认要求和生效语义。
// POS: nexus_config 所有写入前的强制预检工具。
package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

func plan(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "plan_nexus_configuration_change",
		Description: "预检一项 Nexus 配置变更，不执行写入。返回 scope、current_revision、plan_digest、风险、确认要求与运行时生效时机。" +
			"必须把完全相同的 domain/operation/target/input、current_revision 和 plan_digest 交给 apply；需确认的操作先向用户说明影响并取得明确确认。",
		SearchHint:  "Nexus configuration plan change validate dry run settings",
		InputSchema: planSchema(), Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			request, err := changeRequest(args, false)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := svc.PlanChange(ctx, sctx.Actor(), request)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
