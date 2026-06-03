package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/room/contract"
)

const sendDirectedMessageDescription = "发送 Room 私域 directed message。用于单人私聊、小范围讨论、私域记录和隐藏信息收集。" +
	"recipients 填 Room 成员 agent_id；content 不进入 public feed。" +
	"wake_policy=none 只记录，immediate 立即唤醒 recipients，delayed 需要 delay_seconds。" +
	"reply_route 决定被唤醒成员本轮 final reply 投到 public、private recipients，或不投递。" +
	"如果要私下回给主持人并让主持人随后自然公开推进，用 reply_route={mode:private,recipients:[host],wake_policy:immediate,next_reply_route:{mode:public}}。"

func sendDirectedMessage(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "send_directed_message",
		Description: sendDirectedMessageDescription,
		SearchHint:  "Room 私聊 私信 小范围讨论 directed message hidden private reply_route wake_policy",
		AlwaysLoad:  true,
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
				Recipients:    stringListArg(args, "recipients"),
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
			return jsonResult(map[string]any{
				"domain": "room",
				"action": "send_directed_message",
				"item":   directedMessageOutput(item),
			}), nil
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
	return map[string]any{
		"message_id":      message.MessageID,
		"room_id":         message.RoomID,
		"conversation_id": message.ConversationID,
		"source_agent_id": message.SourceAgentID,
		"recipients":      message.Recipients,
		"wake_policy":     message.WakePolicy,
		"reply_route":     message.ReplyRoute,
		"delay_seconds":   message.DelaySeconds,
		"correlation_id":  message.CorrelationID,
		"timestamp":       message.Timestamp,
		"content_chars":   len([]rune(message.Content)),
	}
}
