// INPUT: opaque flow ID only.
// OUTPUT: native human verification-code card request; code never enters MCP input.
// POS: safe conversational bridge for platform verification-code submission.
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func submitCode(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "submit_nexus_channel_authorization_code",
		Description: "请求 Nexus 原生安全输入卡，让当前用户直接提交平台验证码。" +
			"本工具故意不接受 code 参数；不要让用户把验证码发到聊天中，验证码不会进入模型、工具日志或审计。",
		SearchHint:  "Nexus Channel verification code secure submit",
		InputSchema: flowSchema(),
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.RequestVerificationCode(
				ctx,
				sctx.Actor(),
				stringArg(args, "flow_id"),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
