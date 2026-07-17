package room

import (
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// NoReplyMarker 是 Room 成员显式表示本轮无需公开输出的标记。
const NoReplyMarker = "<nexus_room_no_reply/>"

// noReplyMarkerPattern 容忍标签内空白（模型流式拆块后 join 可能插入空白）。
// ponytail: 只容忍标签内空白，不追任意位置的拆分；模型基本整 token 输出。
var noReplyMarkerPattern = regexp.MustCompile(`<nexus_room_no_reply\s*/>`)

// stripNoReplyMarker 移除文本中的无回复标记并去除首尾空白。
func stripNoReplyMarker(text string) string {
	return strings.TrimSpace(noReplyMarkerPattern.ReplaceAllString(text, ""))
}

// IsNoReplyOutputMessage 判断 assistant/result 输出剥离标记后是否仅剩空文本。
func IsNoReplyOutputMessage(message protocol.Message) bool {
	var text string
	switch protocol.MessageRole(message) {
	case "assistant":
		text = extractHistoryText(message)
	case "result":
		if message["is_error"] == true || strings.TrimSpace(normalizeAnyString(message["subtype"])) == "error" {
			return false
		}
		text = normalizeAnyString(message["result"])
	default:
		return false
	}
	text = strings.TrimSpace(text)
	return text != "" && stripNoReplyMarker(text) == ""
}

// IsNoReplyAssistantMessage 判断 assistant 终态消息是否仅包含无回复标记。
func IsNoReplyAssistantMessage(message protocol.Message) bool {
	return protocol.MessageRole(message) == "assistant" && IsNoReplyOutputMessage(message)
}

// StripNoReplyMarker 返回剥离了无回复标记的消息副本（content 字符串/块与
// result 摘要），让混合内容里的标记永不进入存储或公区。
func StripNoReplyMarker(message protocol.Message) protocol.Message {
	cleaned := protocol.Clone(message)

	switch content := cleaned["content"].(type) {
	case string:
		cleaned["content"] = stripNoReplyMarker(content)
	default:
		blocks := normalizeHistoryContentBlocks(cleaned["content"])
		if len(blocks) > 0 {
			next := make([]map[string]any, 0, len(blocks))
			for _, block := range blocks {
				if strings.TrimSpace(normalizeAnyString(block["type"])) == "text" {
					stripped := stripNoReplyMarker(normalizeAnyString(block["text"]))
					if stripped == "" {
						continue
					}
					block["text"] = stripped
				}
				next = append(next, block)
			}
			cleaned["content"] = next
		}
	}

	if raw, ok := cleaned["result"].(string); ok {
		if stripped := stripNoReplyMarker(raw); stripped == "" {
			delete(cleaned, "result")
		} else {
			cleaned["result"] = stripped
		}
	}

	return cleaned
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
