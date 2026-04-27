package room

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// NoReplyMarker 是 Room 成员显式表示本轮无需公开输出的标记。
const NoReplyMarker = "<nexus_room_no_reply/>"

// IsNoReplyAssistantMessage 判断 assistant 终态消息是否为无回复标记。
func IsNoReplyAssistantMessage(message protocol.Message) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	return strings.TrimSpace(extractHistoryText(message)) == NoReplyMarker
}

// IsNoReplyCandidateStreamEvent 判断流式事件是否仍可能只是无回复标记。
func IsNoReplyCandidateStreamEvent(event protocol.EventMessage) bool {
	eventType := strings.TrimSpace(normalizeAnyString(event.Data["type"]))
	switch eventType {
	case "message_start", "message_delta", "message_stop":
		return true
	case "content_block_stop":
		return true
	case "content_block_start", "content_block_delta":
		block, _ := event.Data["content_block"].(map[string]any)
		if strings.TrimSpace(normalizeAnyString(block["type"])) != "text" {
			return false
		}
		text := strings.TrimSpace(normalizeAnyString(block["text"]))
		return text == "" || strings.HasPrefix(NoReplyMarker, text)
	default:
		return false
	}
}
