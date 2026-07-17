package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
)

const sendDirectedMessageDescription = "发送 Room 私域消息。recipients 决定谁可见，wake_targets 决定唤醒谁；正文不进入 public feed。" +
	"reply_route 决定被唤醒成员的 final reply 投递位置。" +
	"如果要私下回给主持人并让主持人随后自然公开推进，用 reply_route={mode:private,recipients:[host],wake_policy:immediate,next_reply_route:{mode:public}}。"

func sendDirectedMessage(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "send_directed_message",
		Description: sendDirectedMessageDescription,
		SearchHint:  "Room 私聊 私信 小范围讨论 directed message hidden private reply_route wake_policy",
		InputSchema: sendDirectedMessageSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errRoomServiceMissing), nil
			}
			sourceAgentID, roomID, conversationID, err := requireRoomScope(sctx)
			if err != nil {
				return errorResult(err), nil
			}
			request := protocol.CreateRoomDirectedMessageRequest{
				SourceAgentID: sourceAgentID,
				RootRoundID:   sctx.CurrentRoundID,
				Recipients:    stringListArg(args, "recipients"),
				WakeTargets:   stringListArg(args, "wake_targets"),
				Content:       stringArg(args, "content"),
				WakePolicy:    protocol.RoomWakePolicy(stringArg(args, "wake_policy")),
				ReplyRoute:    roomReplyRouteArg(objectArg(args, "reply_route")),
				DelaySeconds:  intArg(args, "delay_seconds"),
				CorrelationID: stringArg(args, "correlation_id"),
			}
			item, err := svc.HandleDirectedMessage(scopedToolContext(ctx, sctx), roomID, conversationID, request)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(directedMessageOutput(item)), nil
		},
	}
}

func roomReplyRouteArg(raw map[string]any) protocol.RoomReplyRoute {
	mode := protocol.RoomReplyRouteMode(stringArg(raw, "mode"))
	route := protocol.RoomReplyRoute{
		Mode:       mode,
		Recipients: stringListArg(raw, "recipients"),
		WakePolicy: protocol.RoomWakePolicy(stringArg(raw, "wake_policy")),
	}
	if next := objectArg(raw, "next_reply_route"); next != nil {
		nextRoute := roomReplyRouteArg(next)
		route.NextReplyRoute = &nextRoute
	}
	return route
}

func directedMessageOutput(message *protocol.RoomDirectedMessageRecord) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	status := "recorded"
	if message.WakePolicy == protocol.RoomWakePolicyImmediate {
		status = "queued"
	} else if message.WakePolicy == protocol.RoomWakePolicyDelayed {
		status = "scheduled"
	}
	return map[string]any{"message_id": message.MessageID, "status": status}
}
