// INPUT: opaque flow_id 与固定 connector_id。
// OUTPUT: canceled 终态并擦除 provider 临时秘密。
// POS: Connector 授权主动撤销工具。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func cancel(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	return sdktool.Tool{
		Name: "cancel_connector_authorization",
		Description: "取消尚未完成的 Connector 授权并立即擦除加密临时凭据。" +
			"不能借此断开已经连接的 Connector。",
		InputSchema: flowRefSchema(),
		Annotations: &sdktool.ToolAnnotations{Destructive: true},
		Handler: func(
			ctx context.Context,
			args map[string]any,
		) (sdktool.ToolResult, error) {
			result, err := svc.Cancel(ctx, sctx.Actor(), flowRef(args))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
