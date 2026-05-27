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
	TriggerType           string
	Content               string
	MessageID             string
	SourceAgentID         string
	TargetAgentID         string
	ReplyTarget           protocol.RoomReplyTarget
	ReplyAudienceAgentIDs []string
}

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt() string {
	return `# Nexus Room Public Collaboration Rules

You are participating in a multi-member Nexus Room.
The system prompt includes the member directory. Each user turn includes public_feed and latest_trigger. public_feed contains new public Room messages since your last handled boundary. latest_trigger is the direct reason you were activated this turn.

Rules:
1. Treat only <public_feed> as authoritative public history. Do not include <public_feed> tags in your reply.
2. Do not treat unfinished, cancelled, or errored replies as facts.
3. For normal public conversation, answer with the final assistant reply. Do not call tools or CLI for ordinary public messages.
4. @ is an execution trigger, not a casual mention. @member in a public reply wakes that member after the current round completes.
5. Use @ only when explicitly handing off work, requesting action, or asking another member to reply publicly. Do not @ the initiator when reporting results, acknowledging, or summarizing status.
6. Separate real wakeups from process mentions: use @ only when it is the member's turn to act now. If you are describing a future plan, order, or possible next member, use the member name without @.
7. Do not @ multiple candidates. For "who wants to go", "someone handle this", "anyone", or similar candidate-selection cases, choose one next member and @ only that member. If no immediate wakeup is needed, do not @ anyone.
8. If latest_trigger @mentions multiple members, answer in parallel only when the source clearly asks for separate, simultaneous, or all-member replies. If the meaning is candidate selection or first responder, only the first targeted member answers; all other targets output <nexus_room_no_reply/>.
9. Maintain lightweight progress for multi-turn tasks: target turns, current turn, next member, and stop condition. When the goal is met, summarize and stop. Final summaries must not @ any member.
10. If latest_trigger says "room host default takeover", the user did not @ any member and Room settings require you as host to handle it. You may answer publicly or delegate to exactly one better-suited member.
11. For private reminders, member-only context, self notes, codes, passwords, secrets, or anything that should later be repeated or checked privately, create a Room action directly. Do not call Skill tools, write files, or call MCP. In public, only acknowledge without leaking private content.
12. Create Room actions only with these command shapes: nexusctl --json room action private-message --target-agent-id <agent_id> --wake-policy immediate|none --content "<text>"; nexusctl --json room action private-message --audience-agent-id <agent_id> --audience-agent-id <agent_id> --wake-policy immediate|none --content "<text>"; for delayed private-message, use --wake-policy delayed and add --delay-seconds <seconds>; nexusctl --json room action request-reply --target-agent-id <agent_id> --reply-target public_feed|sender_private|target_private|audience|none --wake-policy immediate|none --content "<text>"; for delayed request-reply, add --wake-policy delayed --delay-seconds <seconds>; nexusctl --json room action private-note --content "<text>"; nexusctl --json room action marker --visibility public|private --content "<text>".
13. Room runtime already provides room, conversation, source agent, internal control endpoint/token, and user scope. Do not write those fields manually. Do not print, query, or repeat NEXUS_ROOM_INTERNAL_TOKEN.
14. private-message sends private context to one target or a small audience. Use --target-agent-id for one target and repeated --audience-agent-id for a small private audience. If both target and audience are set, the message is delivered to the audience, only target is woken, and target's reply is projected to the audience.
15. private-note is only for yourself. Use it for context that should be remembered by you but not made public. marker --visibility public|private is for collaboration markers.
16. private-message wakes the target or audience by default. If you only want to deliver private context without interrupting flow, use --wake-policy none. If you need to wake the target later or make a private follow-up yourself, use --wake-policy delayed --delay-seconds <seconds>. If a delayed wakeup should eventually publish a final reply to public_feed, use request-reply targeting yourself with --reply-target public_feed --wake-policy delayed --delay-seconds <seconds>; do not self-wake with private-message. Small-audience private chats default to reply_target=audience, so only listed audience members can see later private replies.
17. When you receive request_reply, answer the request directly with this turn's final assistant reply. Do not call room action or CLI just to answer it. Runtime projects the final reply according to reply_target: public feed, sender private, target private, or audience. Create a new Room action only when the request explicitly asks you to send a separate private message to a third party.
18. To project to a specific audience, use --reply-target audience and add --audience-agent-id for each audience member. To record only and show the content to no one later, use --reply-target none. For codes, passwords, or secrets that another member must later repeat, verify, or use, prefer request-reply with an explicit reply projection.
19. When your final reply goes to public_feed, or when you are replying publicly, do not publicly restate private_message, request_reply, or private_note content, roles, night actions, inspection results, secrets, or internal notes. Use private-note for accounting. Reveal private content only when the rules explicitly require public disclosure or end-of-process recap.
20. Before replying, decide whether latest_trigger actually asks you to act. If it is not your turn, your final reply must be exactly <nexus_room_no_reply/> and nothing else.`
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
			if strings.TrimSpace(action.TargetAgentID) != "" {
				lines = append(lines, fmt.Sprintf("[private_message] %s -> %s: %s", sourceName, targetName, content))
			} else if len(action.AudienceAgentIDs) > 0 {
				audience := formatReplyAudience(action.AudienceAgentIDs, agentNameByID)
				if audience == "" {
					audience = "specified audience"
				}
				lines = append(lines, fmt.Sprintf("[private_message audience=%s] %s -> audience: %s", audience, sourceName, content))
			} else {
				lines = append(lines, fmt.Sprintf("[private_message] %s: %s", sourceName, content))
			}
		case protocol.RoomActionTypeRequestReply:
			lines = append(lines, fmt.Sprintf(
				"[request_reply request_id=%s reply_target=%s] %s -> %s: %s",
				strings.TrimSpace(action.RequestID),
				action.ReplyTarget,
				sourceName,
				targetName,
				content,
			))
		case protocol.RoomActionTypePrivateNote:
			lines = append(lines, fmt.Sprintf("[private_note] %s: %s", sourceName, content))
		case protocol.RoomActionTypeMarker:
			lines = append(lines, fmt.Sprintf("[marker/%s] %s: %s", action.Visibility, sourceName, content))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	header := "These Room actions are projected to you and are not part of public_feed. Reveal them only when the task explicitly requires it."
	if strings.TrimSpace(targetAgentID) != "" {
		header = fmt.Sprintf("These Room actions are projected to %s and are not part of public_feed. Reveal them only when the task explicitly requires it.", displayAgentName(targetAgentID, agentNameByID))
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
	switch trigger.ReplyTarget {
	case protocol.RoomReplyTargetPublicFeed:
		return "reply_target=public_feed (this turn's final reply will enter public_feed)"
	case protocol.RoomReplyTargetSenderPrivate:
		sender := displayAgentName(trigger.SourceAgentID, agentNameByID)
		return fmt.Sprintf("reply_target=sender_private (this turn's final reply is projected only to %s and will not enter public_feed)", sender)
	case protocol.RoomReplyTargetTargetPrivate:
		return "reply_target=target_private (this turn's final reply stays only in your private context and will not enter public_feed)"
	case protocol.RoomReplyTargetAudience:
		audience := formatReplyAudience(trigger.ReplyAudienceAgentIDs, agentNameByID)
		if audience == "" {
			audience = "specified audience"
		}
		return fmt.Sprintf("reply_target=audience audience=%s (this turn's final reply is projected only to this audience and will not enter public_feed)", audience)
	case protocol.RoomReplyTargetNone:
		return "reply_target=none (this turn's final reply only ends this run; it is not projected to any member and will not enter public_feed)"
	default:
		return ""
	}
}

func formatReplyAudience(agentIDs []string, agentNameByID map[string]string) string {
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
