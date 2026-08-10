// INPUT: opaque flow_id 与固定 connector_id。
// OUTPUT: 脱密流程状态；到时可在服务端推进 Device poll/CAS 完成。
// POS: Connector 授权跨 round 检查工具。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func status(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	return sdktool.Tool{
		Name: "get_connector_authorization_status",
		Description: "读取并推进当前主智能体私有 DM 已批准的 Connector 授权。" +
			"Device Flow 的 provider 轮询和最终 CAS 只在服务端进行；结果永不包含秘密。",
		SearchHint:  "connector oauth device authorization status poll",
		InputSchema: flowRefSchema(),
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(
			ctx context.Context,
			args map[string]any,
		) (sdktool.ToolResult, error) {
			result, err := svc.Status(ctx, sctx.Actor(), flowRef(args))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
