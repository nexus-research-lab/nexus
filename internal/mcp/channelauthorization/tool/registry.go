// INPUT: 授权服务、模型 action 与 server 固化的 owner-main DM 上下文。
// OUTPUT: 允许上下文中的单一 Channel 授权工具。
// POS: capability 可见性边界；service 每次调用仍动态重验。
package tool

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

const (
	ToolName                      = "channel_authorization"
	actionStart                   = "start"
	actionStatus                  = "status"
	actionCancel                  = "cancel"
	actionRequestVerificationCode = "request_verification_code"
)

func BuildAll(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	if svc == nil ||
		!sctx.IsMainAgent ||
		strings.ToLower(strings.TrimSpace(sctx.ContextKind)) != configurationsvc.ContextKindAgent ||
		strings.TrimSpace(sctx.ContextID) != strings.TrimSpace(sctx.CurrentAgentID) {
		return nil
	}
	return []sdktool.Tool{authorization(svc, sctx)}
}

func authorization(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: ToolName,
		Description: "管理 Nexus Channel 授权。" +
			"action=start 启动扫码授权；status 读取脱密状态；cancel 取消未完成流程；" +
			"request_verification_code 重新展示原生安全输入卡。" +
			"二维码、验证码、token 和凭据不会进入模型参数或工具结果。",
		SearchHint:  "Nexus Channel QR login authorization start status cancel verification code",
		InputSchema: authorizationSchema(),
		Annotations: &sdktool.ToolAnnotations{Destructive: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			var (
				result any
				err    error
			)
			switch stringArg(args, "action") {
			case actionStart:
				result, err = svc.Start(ctx, sctx.Actor(), authorizationsvc.StartInput{
					ChannelType: stringArg(args, "channel_type"),
					AccountID:   stringArg(args, "account_id"),
				})
			case actionStatus:
				result, err = svc.Status(ctx, sctx.Actor(), stringArg(args, "flow_id"))
			case actionCancel:
				result, err = svc.Cancel(ctx, sctx.Actor(), stringArg(args, "flow_id"))
			case actionRequestVerificationCode:
				result, err = svc.RequestVerificationCode(
					ctx, sctx.Actor(), stringArg(args, "flow_id"),
				)
			default:
				return errorResult(errors.New("未知 Channel authorization action")), nil
			}
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
