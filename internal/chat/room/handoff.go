package room

import (
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FanoutMarker 是 Agent 明确要求同时唤醒多个 @ 目标时使用的隐藏控制标记。
// 普通正文中的多个 @ 只把第一个目标视为 handoff，其余仍作为可点击的展示 mention。
const FanoutMarker = "<nexus_room_fanout/>"

var fanoutMarkerPattern = regexp.MustCompile(`(?i)<nexus_room_fanout\s*/>`)

// HasFanoutMarker 判断消息任意可见文本中是否声明了显式 fanout。
func HasFanoutMarker(message protocol.Message) bool {
	if message == nil {
		return false
	}
	if containsFanoutMarker(message["content"]) || containsFanoutMarker(message["result"]) {
		return true
	}
	if summary, ok := message["result_summary"].(map[string]any); ok {
		return containsFanoutMarker(summary["result"])
	}
	return false
}

// StripFanoutMarker 从持久化消息中移除隐藏控制标记，避免它进入公区正文或时间线。
func StripFanoutMarker(message protocol.Message) protocol.Message {
	cleaned := protocol.Clone(message)
	if _, ok := cleaned["content"]; ok {
		cleaned["content"] = stripFanoutContent(cleaned["content"])
	}
	if text, ok := cleaned["result"].(string); ok {
		cleaned["result"] = StripFanoutMarkerText(text)
	}
	if summary, ok := cleaned["result_summary"].(map[string]any); ok {
		copySummary := make(map[string]any, len(summary))
		for key, value := range summary {
			copySummary[key] = value
		}
		if text, ok := copySummary["result"].(string); ok {
			copySummary["result"] = StripFanoutMarkerText(text)
		}
		cleaned["result_summary"] = copySummary
	}
	return cleaned
}

// StripFanoutMarkerText 移除正文中的 fanout 控制标记并规范首尾空白。
func StripFanoutMarkerText(text string) string {
	return strings.TrimSpace(fanoutMarkerPattern.ReplaceAllString(text, ""))
}

func containsFanoutMarker(value any) bool {
	switch typed := value.(type) {
	case string:
		return fanoutMarkerPattern.MatchString(typed)
	case []map[string]any:
		for _, block := range typed {
			if containsFanoutMarker(block["text"]) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if block, ok := item.(map[string]any); ok && containsFanoutMarker(block["text"]) {
				return true
			}
		}
	}
	return false
}

func stripFanoutContent(value any) any {
	switch typed := value.(type) {
	case string:
		return StripFanoutMarkerText(typed)
	case []map[string]any:
		blocks := make([]map[string]any, 0, len(typed))
		for _, block := range typed {
			copyBlock := make(map[string]any, len(block))
			for key, item := range block {
				copyBlock[key] = item
			}
			if text, ok := copyBlock["text"].(string); ok {
				copyBlock["text"] = StripFanoutMarkerText(text)
			}
			if strings.TrimSpace(anyString(copyBlock["text"])) == "" && strings.TrimSpace(anyString(copyBlock["type"])) == "text" {
				continue
			}
			blocks = append(blocks, copyBlock)
		}
		return blocks
	case []any:
		blocks := make([]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				blocks = append(blocks, item)
				continue
			}
			copyBlock := make(map[string]any, len(block))
			for key, value := range block {
				copyBlock[key] = value
			}
			if text, ok := copyBlock["text"].(string); ok {
				copyBlock["text"] = StripFanoutMarkerText(text)
			}
			if strings.TrimSpace(anyString(copyBlock["text"])) == "" && strings.TrimSpace(anyString(copyBlock["type"])) == "text" {
				continue
			}
			blocks = append(blocks, copyBlock)
		}
		return blocks
	default:
		return value
	}
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}
