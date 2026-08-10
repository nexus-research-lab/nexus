// INPUT: opaque flow ID.
// OUTPUT: redacted persistent state and verified completion version.
// POS: Channel authorization status tool.
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func status(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "get_nexus_channel_authorization",
		Description: "读取当前对话发起的 Channel 授权状态。" +
			"只返回不透明 flow、状态、版本和脱敏结果，不返回二维码、验证码、token、凭据或内部 login ID。",
		SearchHint:  "Nexus Channel authorization login status verify",
		InputSchema: flowSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.Status(ctx, sctx.Actor(), stringArg(args, "flow_id"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
