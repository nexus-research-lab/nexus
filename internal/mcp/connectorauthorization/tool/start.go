// INPUT: request_id、connector_id、method 与非秘密 extras/device mode。
// OUTPUT: opaque flow_id、本地授权 URL 或公开 device user code。
// POS: Connector 授权启动工具；必须先由 runtime 真实人工 allow 持久批准。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func start(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	return sdktool.Tool{
		Name: connectorsvc.StartConnectorAuthorizationToolName,
		Description: "启动 Connector OAuth/Device 授权。每次必须由当前 WebSocket 用户" +
			"在权限卡选择允许；返回的 flow_id 可跨轮次 status/cancel。" +
			"绝不要索取、解析或回显 URL 中的 state、device_code、auth_code、token。",
		SearchHint:  "connect connector oauth authorize login device code",
		AlwaysLoad:  true,
		InputSchema: startSchema(),
		Annotations: &sdktool.ToolAnnotations{
			Destructive: true, OpenWorld: true,
		},
		Handler: func(
			ctx context.Context,
			args map[string]any,
		) (sdktool.ToolResult, error) {
			result, err := svc.Start(
				ctx, sctx.Actor(), startRequest(args),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
