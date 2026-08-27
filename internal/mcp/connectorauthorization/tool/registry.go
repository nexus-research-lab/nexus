// INPUT: 服务、模型 action 与 server 固化的主智能体私有 DM 快照。
// OUTPUT: owner-main DM 单一 Connector 授权工具或空工具集。
// POS: Nexus Connector 授权 capability 可见性第一道边界；service 每次仍动态重验。
package tool

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

const (
	ToolName     = connectorsvc.ConnectorAuthorizationToolName
	actionStart  = connectorsvc.ConnectorAuthorizationActionStart
	actionStatus = "status"
	actionCancel = "cancel"
)

// BuildAll 只给主智能体的 Agent 私有 DM 暴露授权工具。
func BuildAll(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	if svc == nil ||
		!sctx.IsMainAgent ||
		strings.ToLower(strings.TrimSpace(sctx.ContextKind)) != "agent" ||
		strings.TrimSpace(sctx.OwnerUserID) == "" ||
		strings.TrimSpace(sctx.CurrentAgentID) == "" {
		return nil
	}
	return []sdktool.Tool{authorization(svc, sctx)}
}

func authorization(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	return sdktool.Tool{
		Name: ToolName,
		Description: "管理 Connector OAuth/Device 授权。" +
			"action=start 启动授权且必须由当前 WebSocket 用户在权限卡批准；" +
			"status 读取并推进脱密状态；cancel 取消未完成流程并擦除临时凭据。" +
			"绝不要索取、解析或回显 state、device_code、auth_code 或 token。",
		SearchHint:  "connect connector oauth device authorization start status cancel login",
		AlwaysLoad:  true,
		InputSchema: authorizationSchema(),
		Annotations: &sdktool.ToolAnnotations{Destructive: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			var (
				result any
				err    error
			)
			switch stringArg(args, "action") {
			case actionStart:
				result, err = svc.Start(ctx, sctx.Actor(), startRequest(args))
			case actionStatus:
				result, err = svc.Status(ctx, sctx.Actor(), flowRef(args))
			case actionCancel:
				result, err = svc.Cancel(ctx, sctx.Actor(), flowRef(args))
			default:
				return errorResult(errors.New("未知 Connector authorization action")), nil
			}
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
