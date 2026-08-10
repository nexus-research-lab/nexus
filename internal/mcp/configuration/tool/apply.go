// INPUT: 已预检的配置请求、幂等 request_id、expected_revision 与宿主人工批准。
// OUTPUT: 领域服务写入、变更后核对、revision 和审计闭环。
// POS: nexus_config 唯一写工具。
package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

func apply(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "apply_nexus_configuration_change",
		Description: "应用已经 plan 的 Nexus 配置变更。强制 request_id 幂等、plan_digest 意图绑定、expected_revision 乐观锁；高风险写入必须经过 Nexus“允许本次”人工批准，模型参数不能代替确认；并执行脱敏审计与写后核对。" +
			"禁止跳过 plan，禁止为了绕过 revision 冲突重复使用旧快照，禁止在回复中复述 secret/token/password。",
		SearchHint:  "Nexus configuration apply update create delete settings",
		InputSchema: applySchema(), Annotations: &sdktool.ToolAnnotations{Destructive: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			request, err := changeRequest(args, true)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := svc.ApplyChange(ctx, sctx.Actor(), request)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
