package room

import (
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FanoutMarker 是旧版多目标路由使用的隐藏控制标记。
// 当前每个有效 @ 都会触发 handoff；常量只用于剥离历史或旧 runtime 输出。
const FanoutMarker = "<nexus_room_fanout/>"

var fanoutMarkerPattern = regexp.MustCompile(`(?i)<nexus_room_fanout\s*/>`)

// StripFanoutMarker 从消息中移除旧版隐藏控制标记，避免它进入正文或时间线。
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
