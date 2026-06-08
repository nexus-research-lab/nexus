package tool

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/room/contract"
)

const publishPublicMessageDescription = "主动发布一条 Room public feed 消息。普通公区发言不要用这个工具，直接 final reply 即可。" +
	"仅当当前轮次是私域/tool-driven 流程，且需要额外主动广播公开事实时使用；公开正文中的非代码 @成员 会走统一公区唤醒规则。"

func publishPublicMessage(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "publish_public_message",
		Description: publishPublicMessageDescription,
		SearchHint:  "Room public feed publish broadcast 公开 广播",
		AlwaysLoad:  true,
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
				Content:       stringArg(args, "content"),
				CorrelationID: stringArg(args, "correlation_id"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(map[string]any{
				"domain": "room",
				"action": "publish_public_message",
				"item":   publicMessageOutput(item),
			}), nil
		},
	}
}

func publicMessageOutput(message protocol.Message) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	content := strings.TrimSpace(publicMessageText(message))
	return map[string]any{
		"message_id":      message["message_id"],
		"room_id":         message["room_id"],
		"conversation_id": message["conversation_id"],
		"source_agent_id": message["agent_id"],
		"timestamp":       message["timestamp"],
		"correlation_id":  message["correlation_id"],
		"content_chars":   len([]rune(content)),
	}
}

func publicMessageText(message protocol.Message) string {
	content, ok := message["content"].([]map[string]any)
	if ok {
		return textBlocks(content)
	}
	raw, rawOK := message["content"].([]any)
	if !rawOK {
		return strings.TrimSpace(stringValue(message["content"]))
	}
	blocks := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if block, blockOK := item.(map[string]any); blockOK {
			blocks = append(blocks, block)
		}
	}
	return textBlocks(blocks)
}

func textBlocks(blocks []map[string]any) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(stringValue(block["type"])) != "text" {
			continue
		}
		if text := strings.TrimSpace(stringValue(block["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
