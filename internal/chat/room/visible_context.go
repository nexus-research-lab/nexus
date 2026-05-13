package room

import (
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

// VisibleContextInput 描述一次 Room 成员被唤醒时可见的公共上下文。
type VisibleContextInput struct {
	PublicMessages []protocol.Message
	RoomActions    []protocol.RoomActionRecord
	LatestTrigger  Trigger
	AgentNameByID  map[string]string
	TargetAgentID  string
}

// PublicCursor 描述目标成员上次消费到的公区位置。
type PublicCursor struct {
	LastMessageID string
	LastTimestamp int64
}

// PublicInputBatchInput 描述公区消息批次选择输入。
type PublicInputBatchInput struct {
	PublicHistory []protocol.Message
	Cursor        PublicCursor
	AgentNameByID map[string]string
	TargetAgentID string
}

// PublicInputBatch 是一次要投递给目标成员的公区消息批次。
type PublicInputBatch struct {
	Messages      []protocol.Message
	LastMessageID string
	LastTimestamp int64
}

// Trigger 描述 Room round 里唤醒单个成员的直接原因。
type Trigger struct {
	TriggerType   string
	Content       string
	MessageID     string
	SourceAgentID string
	TargetAgentID string
}

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt() string {
	return `# Nexus Room 公区协作规则

你正在 Nexus 的多人协作 Room 中参与公开协作。
Room 运行时会在系统提示词中提供成员目录，并在每轮用户消息里提供 public_feed 和 latest_trigger；public_feed 是你上次处理之后的新公区消息，latest_trigger 是这次唤醒你的直接原因。
规则：
1. 只把 <public_feed> 里的内容当作权威公共历史，输出不需要携带<public_feed>标签。
2. 不要把未完成、被取消或报错的回复当作事实。
3. 正常公开交流直接用最终 assistant 回复，不要为公区消息调用工具或 CLI。
4. @ 是执行触发，不是普通提及；公开回复里的 @成员名 会在当前 round 结束后唤醒对方。
5. 只有明确转交任务、请求对方行动或要求对方公开回复时才 @；回报结果、确认收到、总结状态时不要 @ 发起者。
6. 区分真实唤醒和流程提及：已经轮到对方马上行动时才 @；只是描述后续流程、计划、顺序或未来会轮到某成员时，用成员名但不要加 @。
7. 候选邀请不要多 @：遇到“谁先来、谁来、任选一个、想要成员、你们可以让成员来”等场景，先选定一个下一位成员，只 @ 这一个人；如果暂时不需要立刻唤醒任何人，就不用 @。
8. 如果 latest_trigger 这一行同时 @ 多个成员，只有来源明确要求“分别、各自、同时、都回答”时才并行回答；若语义是候选抢答或选一个人，只由第一个被 @ 的目标回答，其余目标输出 <nexus_room_no_reply/>。
9. 多轮任务要自己维护轻量进度：目标轮数、当前轮次、下一位成员、停止条件；达到目标后直接总结并停止，最终总结不要 @ 任何成员。
10. 遇到私下提醒、只给某成员、自己记录、暗号、密码、密钥、后续让成员复述或核对这类不应进入公区的内容，直接创建 Room action，不要调用 Skill 工具，不要写文件，不要调用 MCP；公区只输出非敏感确认，不要泄露正文。
11. 创建 Room action 时使用 cd "$NEXUS_PROJECT_ROOT" && go run ./cmd/nexusctl --json room action ...；Room runtime 已注入 room、conversation、source agent、内部控制面地址/token 和用户作用域，不要手写这些字段，不要打印、查询或复述 NEXUS_ROOM_INTERNAL_TOKEN。
12. private-message 用于给指定成员私域投递，--target-agent-id 必须使用成员目录里的真实 agent_id，不是成员名；暗号、密码、密钥如果后续要让某成员复述、核对或使用，也用 private-message 发给该成员。
13. private-note 只写给你自己，适合记录后续需要记住但不该公开的上下文；marker --visibility public|private 用于协作标记。
14. 需要投影给指定受众集合时使用 --reply-target audience 并为每个受众追加 --audience-agent-id；只想落盘记录、不让任何成员后续看到正文时使用 --reply-target none。
15. 回复前先判断 latest_trigger 是否要求你行动；如果没有轮到你处理，最终回复只能输出 <nexus_room_no_reply/>，不要输出其他文字。`
}

// BuildMemberDirectoryPrompt 构建 Room 级稳定成员目录提示词。
func BuildMemberDirectoryPrompt(agentNameByID map[string]string) string {
	return fmt.Sprintf(
		"# Nexus Room 成员目录\n\n"+
			"<room_member_directory>\n%s\n</room_member_directory>",
		formatMemberDirectory(agentNameByID),
	)
}

// BuildVisibleContext 构建 Room 成员本轮动态输入。
func BuildVisibleContext(input VisibleContextInput) string {
	lines := buildHistoryLines(contextPublicMessages(input.PublicMessages, input.LatestTrigger), input.AgentNameByID)
	if len(lines) == 0 {
		lines = []string{"（本次没有新的公区消息）"}
	}

	contextValue := fmt.Sprintf(
		"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
	if actionContext := buildRoomActionContext(input.RoomActions, input.AgentNameByID, input.TargetAgentID); actionContext != "" {
		contextValue += "\n\n" + actionContext
	}
	return contextValue
}

// BuildPublicInputBatch 根据目标成员 cursor 选择本次公区输入批次。
func BuildPublicInputBatch(input PublicInputBatchInput) PublicInputBatch {
	candidates := publicMessagesAfterCursor(input.PublicHistory, input.Cursor)
	if len(candidates) > roomMaxHistoryMessages {
		candidates = candidates[len(candidates)-roomMaxHistoryMessages:]
	}
	candidates = trimPublicBatchByChars(candidates, input.AgentNameByID)

	messages := make([]protocol.Message, 0, len(candidates))
	for _, message := range candidates {
		if !isVisiblePublicInputMessage(message, input.TargetAgentID) {
			continue
		}
		messages = append(messages, message)
	}

	batch := PublicInputBatch{Messages: messages}
	if len(candidates) > 0 {
		boundary := candidates[len(candidates)-1]
		batch.LastMessageID = normalizeAnyString(boundary["message_id"])
		batch.LastTimestamp = normalizeInt64(boundary["timestamp"])
	}
	return batch
}

// BuildGuidedPublicInputContext 构造运行中 round 的公区增量引导文本。
func BuildGuidedPublicInputContext(input VisibleContextInput) string {
	lines := buildHistoryLines(contextPublicMessages(input.PublicMessages, input.LatestTrigger), input.AgentNameByID)
	if len(lines) == 0 {
		if strings.TrimSpace(input.LatestTrigger.TriggerType) == "" && strings.TrimSpace(input.LatestTrigger.Content) == "" {
			return ""
		}
		lines = []string{"（本次没有新的公区消息）"}
	}
	return fmt.Sprintf(
		"Room 公区在你当前运行期间出现了新的消息。把这些消息当作已经进入公区的事实；如果需要调整当前任务，请结合它们继续。\n\n"+
			"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
}

func buildRoomActionContext(
	actions []protocol.RoomActionRecord,
	agentNameByID map[string]string,
	targetAgentID string,
) string {
	if len(actions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		content := strings.TrimSpace(action.Content)
		if content == "" {
			continue
		}
		sourceName := displayAgentName(action.SourceAgentID, agentNameByID)
		targetName := displayAgentName(action.TargetAgentID, agentNameByID)
		switch action.ActionType {
		case protocol.RoomActionTypePrivateMessage:
			lines = append(lines, fmt.Sprintf("[private_message] %s -> %s: %s", sourceName, targetName, content))
		case protocol.RoomActionTypePrivateNote:
			lines = append(lines, fmt.Sprintf("[private_note] %s: %s", sourceName, content))
		case protocol.RoomActionTypeMarker:
			lines = append(lines, fmt.Sprintf("[marker/%s] %s: %s", action.Visibility, sourceName, content))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	header := "以下是投影给你的 Room action，不属于公区 feed；只有需要时才在回复中显式公开。"
	if strings.TrimSpace(targetAgentID) != "" {
		header = fmt.Sprintf("以下是投影给 %s 的 Room action，不属于公区 feed；只有需要时才在回复中显式公开。", displayAgentName(targetAgentID, agentNameByID))
	}
	return fmt.Sprintf(
		"%s\n\n<room_actions>\n%s\n</room_actions>",
		header,
		strings.Join(lines, "\n"),
	)
}

func displayAgentName(agentID string, agentNameByID map[string]string) string {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return "unknown"
	}
	if name := strings.TrimSpace(agentNameByID[normalizedAgentID]); name != "" {
		return name
	}
	return normalizedAgentID
}

func contextPublicMessages(messages []protocol.Message, trigger Trigger) []protocol.Message {
	triggerMessageID := strings.TrimSpace(trigger.MessageID)
	if triggerMessageID == "" || len(messages) == 0 {
		return messages
	}
	filtered := make([]protocol.Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(normalizeAnyString(message["message_id"])) == triggerMessageID {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func publicMessagesAfterCursor(history []protocol.Message, cursor PublicCursor) []protocol.Message {
	if len(history) == 0 {
		return nil
	}
	lastMessageID := strings.TrimSpace(cursor.LastMessageID)
	if lastMessageID != "" {
		for index, message := range history {
			if strings.TrimSpace(normalizeAnyString(message["message_id"])) == lastMessageID {
				return append([]protocol.Message(nil), history[index+1:]...)
			}
		}
	}
	if cursor.LastTimestamp > 0 {
		for index, message := range history {
			if normalizeInt64(message["timestamp"]) > cursor.LastTimestamp {
				return append([]protocol.Message(nil), history[index:]...)
			}
		}
		return nil
	}
	return append([]protocol.Message(nil), history...)
}

func trimPublicBatchByChars(messages []protocol.Message, agentNameByID map[string]string) []protocol.Message {
	if len(messages) == 0 {
		return nil
	}
	totalChars := 0
	start := len(messages)
	for index := len(messages) - 1; index >= 0; index-- {
		line := formatHistoryLine(messages[index], agentNameByID)
		lineChars := len(line)
		nextChars := totalChars
		if lineChars > 0 {
			nextChars += lineChars
			if totalChars > 0 {
				nextChars++
			}
		}
		if nextChars > roomMaxHistoryChars && start < len(messages) {
			break
		}
		start = index
		totalChars = nextChars
		if nextChars > roomMaxHistoryChars {
			break
		}
	}
	return append([]protocol.Message(nil), messages[start:]...)
}

func isVisiblePublicInputMessage(message protocol.Message, targetAgentID string) bool {
	role := strings.TrimSpace(normalizeAnyString(message["role"]))
	switch role {
	case "user":
		return extractHistoryText(message) != ""
	case "assistant", "result":
		if strings.TrimSpace(normalizeAnyString(message["agent_id"])) == strings.TrimSpace(targetAgentID) {
			return false
		}
		return formatHistoryLine(message, nil) != ""
	default:
		return false
	}
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

func formatMemberDirectory(agentNameByID map[string]string) string {
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
	lines := make([]string, 0, len(members))
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- name=%s agent_id=%s", member.name, member.agentID))
	}
	return strings.Join(lines, "\n")
}

func formatRoomTrigger(trigger Trigger, agentNameByID map[string]string) string {
	if strings.TrimSpace(trigger.TriggerType) == "" && strings.TrimSpace(trigger.Content) == "" {
		return "（无触发消息）"
	}
	sourceName := firstNonEmpty(agentNameByID[trigger.SourceAgentID], trigger.SourceAgentID)
	if sourceName == "" {
		sourceName = "User"
	}
	if content := strings.TrimSpace(trigger.Content); content != "" {
		return sourceName + ": " + content
	}
	return sourceName + ": （无内容）"
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
	if message["is_complete"] == true {
		return extractHistoryText(message)
	}
	return ""
}

// ExtractAssistantResultText 返回 assistant 终态摘要中的公开文本。
func ExtractAssistantResultText(message protocol.Message) string {
	return extractAssistantResultText(message)
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

// ExtractHistoryText 返回消息 content 中可进入 Room 公区上下文的文本。
func ExtractHistoryText(message protocol.Message) string {
	return extractHistoryText(message)
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

func normalizeInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
