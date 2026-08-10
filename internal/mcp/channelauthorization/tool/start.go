// INPUT: Channel type and optional exact account binding.
// OUTPUT: opaque flow metadata; QR/device payload is sent only to the native human UI.
// POS: owner-main Channel authorization start tool.
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
)

func start(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "start_nexus_channel_authorization",
		Description: "在主智能体自己的私有 DM 中启动 Channel 扫码授权。" +
			"二维码/verification URL 只显示在 Nexus 原生授权卡，不会返回给模型；" +
			"授权完成使用启动时 Channel version CAS，候选 runtime 失败会恢复旧配置。",
		SearchHint:  "Nexus Channel QR login authorize connect start",
		InputSchema: startSchema(),
		Annotations: &sdktool.ToolAnnotations{Destructive: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.Start(ctx, sctx.Actor(), authorizationsvc.StartInput{
				ChannelType: stringArg(args, "channel_type"),
				AccountID:   stringArg(args, "account_id"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
