package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
)

const publishPublicMessageDescription = "主动发布一条 Room public feed 消息。普通公区发言不要用这个工具，直接 final reply 即可。" +
	"仅当当前轮次是私域/tool-driven 流程，且需要额外主动广播公开事实时使用；公开正文中的非代码 @成员 会走统一公区唤醒规则。"

func publishPublicMessage(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "publish_public_message",
		Description: publishPublicMessageDescription,
		SearchHint:  "Room public feed publish broadcast 公开 广播",
		InputSchema: publishPublicMessageSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errRoomServiceMissing), nil
			}
			sourceAgentID, roomID, conversationID, err := requireRoomScope(sctx)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := svc.HandlePublicMessage(scopedToolContext(ctx, sctx), roomID, conversationID, protocol.CreateRoomPublicMessageRequest{
				SourceAgentID: sourceAgentID,
				RootRoundID:   sctx.CurrentRoundID,
				Content:       stringArg(args, "content"),
				CorrelationID: stringArg(args, "correlation_id"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(publicMessageOutput(item)), nil
		},
	}
}

func publicMessageOutput(message protocol.Message) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	return map[string]any{"message_id": message["message_id"], "status": "published"}
}
