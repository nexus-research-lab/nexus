// INPUT: opaque flow ID.
// OUTPUT: scope-bound cancellation or the completion that already won the commit fence.
// POS: Channel authorization cancellation tool.
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func cancel(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "cancel_nexus_channel_authorization",
		Description: "取消当前对话发起且仍有效的 Channel 授权；不会删除已有 Channel 配置或账号。",
		SearchHint:  "Nexus Channel authorization cancel stop",
		InputSchema: flowSchema(),
		Annotations: &sdktool.ToolAnnotations{Destructive: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.Cancel(ctx, sctx.Actor(), stringArg(args, "flow_id"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
