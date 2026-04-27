package room

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomMaxHistoryMessages = 80
	roomMaxHistoryChars    = 12_000

	roomHistoryTruncatedSuffix = "\n...（已截断）"
)

type roomVisibleContextInput struct {
	PublicHistory []protocol.Message
	LatestTrigger roomTrigger
	AgentNameByID map[string]string
	TargetAgentID string
}

func buildRoomVisibleContext(input roomVisibleContextInput) string {
	lines := buildHistoryLines(input.PublicHistory, input.AgentNameByID)
	if len(lines) == 0 {
		lines = []string{"（暂无公共历史）"}
	}

	memberNames := make([]string, 0, len(input.AgentNameByID))
	for _, name := range input.AgentNameByID {
		if strings.TrimSpace(name) != "" {
			memberNames = append(memberNames, name)
		}
	}
	sort.Strings(memberNames)
	targetName := firstNonEmpty(input.AgentNameByID[input.TargetAgentID], input.TargetAgentID)

	return fmt.Sprintf(
		"你正在 Nexus 的多人协作 Room 中，以成员 %s 的身份响应新消息。\n"+
			"以下上下文只包含 Room 公共区：public_feed 是所有成员可见的公共历史，latest_trigger 是这次唤醒你的直接原因。\n"+
			"规则：\n"+
			"1. 只把 <public_feed> 里的内容当作权威公共历史。\n"+
			"2. 不要把未完成、被取消或报错的回复当作事实。\n"+
			"3. 正常公开交流直接用最终 assistant 回复，不要为公区消息调用工具或 CLI。\n"+
			"4. @ 是执行触发，不是普通提及；公开回复里的 @成员名 会在当前 round 结束后唤醒对方。\n"+
			"5. 只有明确转交任务、请求对方行动或要求对方公开回复时才 @；回报结果、确认收到、总结状态时不要 @ 发起者。\n"+
			"6. 区分真实唤醒和流程提及：已经轮到对方马上行动时才 @；只是描述后续流程、计划、顺序或未来会轮到某成员时，用成员名但不要加 @。\n"+
			"7. 候选邀请不要多 @：遇到“谁先来、谁来、任选一个、想要成员、你们可以让成员来”等场景，先选定一个下一位成员，只 @ 这一个人；如果暂时不需要立刻唤醒任何人，就不用 @。\n"+
			"8. 如果 latest_trigger 是 public_mention 且 metadata 显示多个目标，只有来源明确要求“分别、各自、同时、都回答”时才并行回答；若语义是候选抢答或选一个人，排在第一位的目标回答，其他目标输出 <nexus_room_no_reply/>。\n"+
			"9. 多轮任务要自己维护轻量进度：目标轮数、当前轮次、下一位成员、停止条件；达到目标后直接总结并停止，最终总结不要 @ 任何成员。\n"+
			"10. 回复前先判断 latest_trigger 是否要求你行动；如果没有轮到你处理，最终回复只能输出 <nexus_room_no_reply/>，不要输出其他文字。\n"+
			"Room 成员：%s\n\n"+
			"<room_member_directory>\n%s\n</room_member_directory>\n\n"+
			"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		targetName,
		firstNonEmpty(strings.Join(memberNames, "、"), "未知成员"),
		formatMemberDirectory(input.AgentNameByID, input.TargetAgentID),
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
}

func buildHistoryLines(history []protocol.Message, agentNameByID map[string]string) []string {
	if len(history) == 0 {
		return nil
	}

	start := 0
	if len(history) > roomMaxHistoryMessages {
		start = len(history) - roomMaxHistoryMessages
	}

	formatted := make([]string, 0, len(history)-start)
	for _, message := range history[start:] {
		line := formatHistoryLine(message, agentNameByID)
		if line != "" {
			formatted = append(formatted, line)
		}
	}

	lines := make([]string, 0, len(formatted))
	totalChars := 0
	for index := len(formatted) - 1; index >= 0; index-- {
		line := formatted[index]
		nextChars := totalChars + len(line)
		if totalChars > 0 {
			nextChars++
		}
		if nextChars > roomMaxHistoryChars {
			if len(lines) == 0 {
				truncated := truncateHistoryText(line, roomMaxHistoryChars)
				if truncated != "" {
					lines = append(lines, truncated)
				}
			}
			break
		}
		lines = append(lines, line)
		totalChars = nextChars
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines
}

func formatMemberDirectory(agentNameByID map[string]string, targetAgentID string) string {
	if len(agentNameByID) == 0 {
		return "（暂无成员目录）"
	}
	type memberLine struct {
		agentID string
		name    string
	}
	members := make([]memberLine, 0, len(agentNameByID))
	for agentID, name := range agentNameByID {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		members = append(members, memberLine{
			agentID: normalizedAgentID,
			name:    firstNonEmpty(strings.TrimSpace(name), normalizedAgentID),
		})
	}
	sort.Slice(members, func(i int, j int) bool {
		if members[i].name != members[j].name {
			return members[i].name < members[j].name
		}
		return members[i].agentID < members[j].agentID
	})
	lines := make([]string, 0, len(members)+1)
	if strings.TrimSpace(targetAgentID) != "" {
		lines = append(lines, fmt.Sprintf("当前成员 agent_id=%s name=%s", strings.TrimSpace(targetAgentID), firstNonEmpty(agentNameByID[targetAgentID], targetAgentID)))
	}
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- name=%s agent_id=%s", member.name, member.agentID))
	}
	return strings.Join(lines, "\n")
}

func formatRoomTrigger(trigger roomTrigger, agentNameByID map[string]string) string {
	if strings.TrimSpace(trigger.TriggerType) == "" && strings.TrimSpace(trigger.Content) == "" {
		return "（无触发消息）"
	}
	return mustJSON(map[string]any{
		"trigger_type": trigger.TriggerType,
		"content":      trigger.Content,
		"message_id":   trigger.MessageID,
		"source_agent": firstNonEmpty(agentNameByID[trigger.SourceAgentID], trigger.SourceAgentID),
		"target_agent": firstNonEmpty(agentNameByID[trigger.TargetAgentID], trigger.TargetAgentID),
		"metadata":     trigger.Metadata,
	})
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func formatHistoryLine(message protocol.Message, agentNameByID map[string]string) string {
	role := strings.TrimSpace(normalizeAnyString(message["role"]))
	var content string
	switch role {
	case "user":
		content = extractHistoryText(message)
	case "assistant":
		if isComplete, ok := message["is_complete"].(bool); ok && !isComplete {
			return ""
		}
		content = extractAssistantResultText(message)
	case "result":
		content = strings.TrimSpace(normalizeAnyString(message["result"]))
	default:
		return ""
	}
	if content == "" {
		return ""
	}

	switch role {
	case "user":
		return "User: " + content
	case "assistant", "result":
		agentID := normalizeAnyString(message["agent_id"])
		return fmt.Sprintf("Assistant(%s): %s", firstNonEmpty(agentNameByID[agentID], agentID, "Assistant"), content)
	default:
		return ""
	}
}

func extractAssistantResultText(message protocol.Message) string {
	if summary, ok := message["result_summary"].(map[string]any); ok {
		if text := extractHistoryText(message); text != "" {
			return text
		}
		return strings.TrimSpace(normalizeAnyString(summary["result"]))
	}
	return ""
}

func extractHistoryText(message protocol.Message) string {
	if raw, ok := message["content"].(string); ok {
		return strings.TrimSpace(raw)
	}

	items := normalizeHistoryContentBlocks(message["content"])
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, payload := range items {
		if text := strings.TrimSpace(normalizeAnyString(payload["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func normalizeHistoryContentBlocks(content any) []map[string]any {
	switch typed := content.(type) {
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if payload, ok := item.(map[string]any); ok {
				items = append(items, payload)
			}
		}
		return items
	case []map[string]any:
		return append([]map[string]any(nil), typed...)
	default:
		return nil
	}
}

func truncateHistoryText(value string, maxBytes int) string {
	trimmed := strings.TrimSpace(value)
	if maxBytes <= 0 || len(trimmed) <= maxBytes {
		return trimmed
	}
	if maxBytes <= len(roomHistoryTruncatedSuffix) {
		return trimStringByBytes(trimmed, maxBytes)
	}
	body := trimStringByBytes(trimmed, maxBytes-len(roomHistoryTruncatedSuffix))
	if body == "" {
		return trimStringByBytes(trimmed, maxBytes)
	}
	return strings.TrimSpace(body) + roomHistoryTruncatedSuffix
}

func trimStringByBytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return strings.TrimSpace(value)
	}
	end := 0
	for index, currentRune := range value {
		width := utf8.RuneLen(currentRune)
		if width <= 0 {
			width = 1
		}
		if index+width > maxBytes {
			break
		}
		end = index + width
	}
	return strings.TrimSpace(value[:end])
}

func normalizeAnyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}
