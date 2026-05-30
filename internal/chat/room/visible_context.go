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

	roomHistoryTruncatedSuffix = "\n...(truncated)"
)

// VisibleContextInput 描述一次 Room 成员被唤醒时可见的公共上下文。
type VisibleContextInput struct {
	PublicMessages []protocol.Message
	RoomMessages   []protocol.RoomDirectedMessageRecord
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
	ReplyRoute    protocol.RoomReplyRoute
}

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt() string {
	return `# Nexus Room

You are a member in a multi-member Nexus Room. Each user turn includes <public_feed> (new public messages since your last boundary) and <latest_trigger> (why you were activated).

Rules:
1. Only <public_feed> is authoritative public history. Incomplete, cancelled, or errored replies are not facts.
2. For normal public conversation, answer directly. Do not call tools or CLI for ordinary public messages.
3. @ outside inline code or fenced code is an execution trigger. @member wakes that member after the current round completes. Literal @ examples must be written in code spans.
4. Use @ only when handing off work, requesting action, or asking another member to reply. Do not @ the initiator when reporting results, acknowledging, or summarizing status.
5. @ is for "act now", not future plans or process mentions. Use the member name without @ when describing a plan, possible next step, or later handoff.
6. Never @ multiple candidates. For candidate-selection phrases ("who wants to go", "someone handle this", "anyone"), pick one and @ only them. If no wakeup is needed, do not @ anyone.
7. If latest_trigger @mentions multiple members, act in parallel only when the source clearly asks for simultaneous or all-member replies. For candidate selection or first-responder cases, only the first targeted member answers; all others output <nexus_room_no_reply/>.
8. Multi-turn tasks: track target turns, current turn, next member, and stop condition. When done, summarize and stop. Final summaries must not @ anyone.
9. If latest_trigger says "room host default takeover", the user did not @ any member and Room settings require you to handle it. Answer directly or @ exactly one member to delegate.
10. For private reminders, secrets, codes, hidden collection, or anything to be later repeated or verified privately, use the built-in Room communication tool nexus_room.send_directed_message to create a Room directed message. Do not use Bash, nexusctl, Skill tools, or files for Room communication. In public, only acknowledge without leaking private content.
11. Normal public Room speech is your final reply; do not use tools for ordinary public messages. To proactively publish an extra public Room message from a private/tool-driven turn, use nexus_room.publish_public_message with content. Any non-code @member in that public text wakes normally. After publishing from a private turn, output exactly <nexus_room_no_reply/> and nothing else unless your current reply_route also asks for a final reply.
12. Room directed message tool input shape: recipients: string[], content: string, wake_policy: none|immediate|delayed, optional delay_seconds, reply_route: { mode: public|private|none, recipients: string[], wake_policy: none|immediate, next_reply_route: {...} }, optional correlation_id.
13. Runtime injects room, conversation, and source agent. Do not set those fields manually.
14. recipients can be one agent or a small group. Small-group discussion is just a directed message with multiple recipients; the skill must decide who summarizes and where the result goes. When a directed message wakes multiple recipients, every recipient inherits the same reply_route, so only the member the message designates as the responder should produce a final reply; all other recipients must output <nexus_room_no_reply/>, otherwise the route fires once per replier.
15. Wake policy: immediate wakes recipients now. none only records privately. delayed wakes later; set delay_seconds.
16. reply_route decides where your final reply goes when a directed message wakes you: public, private, or none. Use reply_route.mode="private" with explicit reply_route.recipients when the result must return to a host or another member. If that private handback wakes the route recipient and their natural final reply should then go public, include reply_route.next_reply_route={"mode":"public"} on the original directed message.
17. When you receive a directed message, answer in this turn's final reply. Do not create another directed message just to answer. Runtime projects per reply_route. Create a new directed message only when the request explicitly asks you to send a separate private message to a third party.
18. Never restate directed message content, secrets, or internal notes in public unless the task explicitly requires public disclosure.
19. Before replying, decide whether latest_trigger actually asks you to act. If it is not your turn, output exactly <nexus_room_no_reply/> and nothing else.
20. Your final reply may be persisted or projected verbatim according to reply_route. Do not include private analysis, hidden role facts, drafts, tool notes, or "public version below" separators unless you intend that text to be visible to its routed audience.`
}

// BuildMemberDirectoryPrompt 构建 Room 级稳定成员目录提示词。
func BuildMemberDirectoryPrompt(agentNameByID map[string]string) string {
	return fmt.Sprintf(
		"# Nexus Room Member Directory\n\n"+
			"<room_member_directory>\n%s\n</room_member_directory>",
		formatMemberDirectory(agentNameByID),
	)
}

// BuildVisibleContext 构建 Room 成员本轮动态输入。
func BuildVisibleContext(input VisibleContextInput) string {
	lines := buildHistoryLines(contextPublicMessages(input.PublicMessages, input.LatestTrigger), input.AgentNameByID)
	if len(lines) == 0 {
		lines = []string{"(No new public messages this turn.)"}
	}

	contextValue := fmt.Sprintf(
		"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
	if privateContext := buildRoomDirectedMessageContext(input.RoomMessages, input.AgentNameByID, input.TargetAgentID); privateContext != "" {
		contextValue += "\n\n" + privateContext
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
		lines = []string{"(No new public messages this turn.)"}
	}
	return fmt.Sprintf(
		"New public Room messages arrived while you were running. Treat them as public facts already in the Room. If they affect your current work, incorporate them and continue.\n\n"+
			"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
}

func buildRoomDirectedMessageContext(
	messages []protocol.RoomDirectedMessageRecord,
	agentNameByID map[string]string,
	targetAgentID string,
) string {
	if len(messages) == 0 {
		return ""
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		sourceName := displayAgentName(message.SourceAgentID, agentNameByID)
		recipients := formatReplyRecipients(message.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		lines = append(lines, fmt.Sprintf(
			"[directed_message recipients=%s reply_route=%s] %s: %s",
			recipients,
			formatReplyRoute(message.ReplyRoute, message.SourceAgentID, agentNameByID),
			sourceName,
			content,
		))
	}
	if len(lines) == 0 {
		return ""
	}
	header := "These Room directed messages are projected to you and are not part of public_feed. Reveal them only when the task explicitly requires it."
	if strings.TrimSpace(targetAgentID) != "" {
		header = fmt.Sprintf("These Room directed messages are projected to %s and are not part of public_feed. Reveal them only when the task explicitly requires it.", displayAgentName(targetAgentID, agentNameByID))
	}
	return fmt.Sprintf(
		"%s\n\n<room_directed_messages>\n%s\n</room_directed_messages>",
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
	case "assistant":
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
		return "(No room members listed.)"
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
		return "(No trigger message.)"
	}
	sourceName := firstNonEmpty(agentNameByID[trigger.SourceAgentID], trigger.SourceAgentID)
	if sourceName == "" {
		sourceName = "User"
	}
	var line string
	if content := strings.TrimSpace(trigger.Content); content != "" {
		line = sourceName + ": " + content
	} else {
		line = sourceName + ": (No content.)"
	}
	if strings.TrimSpace(trigger.TriggerType) == "room_host_default" {
		line += "\nroom host default takeover: the user did not @ any member, and Room settings require you as host to handle this turn. You may answer directly or @ exactly one member to delegate."
	}
	if projection := formatRoomReplyProjection(trigger, agentNameByID); projection != "" {
		line += "\n" + projection
	}
	return line
}

func formatRoomReplyProjection(trigger Trigger, agentNameByID map[string]string) string {
	if trigger.ReplyRoute.Mode == "" {
		return ""
	}
	return fmt.Sprintf("reply_route=%s", formatReplyRoute(trigger.ReplyRoute, trigger.SourceAgentID, agentNameByID))
}

func formatReplyRecipients(agentIDs []string, agentNameByID map[string]string) string {
	if len(agentIDs) == 0 {
		return ""
	}
	items := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		items = append(items, fmt.Sprintf("%s(%s)", displayAgentName(normalizedAgentID, agentNameByID), normalizedAgentID))
	}
	return strings.Join(items, ",")
}

func formatReplyRoute(route protocol.RoomReplyRoute, sourceAgentID string, agentNameByID map[string]string) string {
	switch route.Mode {
	case protocol.RoomReplyRoutePublic:
		return "public (this turn's final reply will enter public_feed)"
	case protocol.RoomReplyRoutePrivate:
		recipients := formatReplyRecipients(route.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		wake := route.WakePolicy
		if wake == "" {
			wake = protocol.RoomWakePolicyNone
		}
		nextRoute := ""
		if route.NextReplyRoute != nil {
			nextRoute = fmt.Sprintf(" next_reply_route=%s", formatReplyRouteCompact(*route.NextReplyRoute, agentNameByID))
		}
		return fmt.Sprintf("private recipients=%s wake=%s%s (this turn's final reply will not enter public_feed)", recipients, wake, nextRoute)
	case protocol.RoomReplyRouteNone:
		return "none (this turn's final reply only ends this run; it is not projected to any member and will not enter public_feed)"
	default:
		return ""
	}
}

func formatReplyRouteCompact(route protocol.RoomReplyRoute, agentNameByID map[string]string) string {
	switch route.Mode {
	case protocol.RoomReplyRoutePublic:
		return "public"
	case protocol.RoomReplyRoutePrivate:
		recipients := formatReplyRecipients(route.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		wake := route.WakePolicy
		if wake == "" {
			wake = protocol.RoomWakePolicyNone
		}
		return fmt.Sprintf("private recipients=%s wake=%s", recipients, wake)
	case protocol.RoomReplyRouteNone:
		return "none"
	default:
		return ""
	}
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
	default:
		return ""
	}
	if content == "" {
		return ""
	}

	switch role {
	case "user":
		return "User: " + content
	case "assistant":
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
