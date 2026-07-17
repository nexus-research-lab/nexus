package room

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ContextBoundary 表示预算实际消费到的消息边界。
type ContextBoundary struct {
	MessageID string
	Timestamp int64
}

// PublicAnchorMetadata 是 Nexus 产品侧可用于冷启动锚点的稳定对话元数据。
type PublicAnchorMetadata struct {
	RoomName          string
	RoomDescription   string
	ConversationTitle string
}

// RoomContextUsage 提供不含正文的上下文预算诊断数据。
type RoomContextUsage struct {
	ContextWindowTokens  int
	BudgetTokens         int
	UsedTokens           int
	CurrentMessageTokens int
	TriggerTokens        int
	PublicDeltaTokens    int
	PrivateDeltaTokens   int
	PublicAnchorTokens   int
	ColdStart            bool
}

// VisibleContextPlan 是 Room 可见上下文及其安全 checkpoint 边界。
type VisibleContextPlan struct {
	Text            string
	PublicBoundary  ContextBoundary
	PrivateBoundary ContextBoundary
	Usage           RoomContextUsage
}

type contextLine struct {
	text      string
	messageID string
	timestamp int64
}

type contextSelection struct {
	texts  map[int]string
	tokens int
}

func newContextSelection() contextSelection {
	return contextSelection{texts: make(map[int]string)}
}

// BuildVisibleContextPlan 按统一预算构造 Room 动态上下文。
func BuildVisibleContextPlan(input VisibleContextInput) VisibleContextPlan {
	return buildVisibleContextPlan(input, false)
}

// BuildGuidedPublicInputContextPlan 构造运行中 round 的预算化公区增量。
func BuildGuidedPublicInputContextPlan(input VisibleContextInput) VisibleContextPlan {
	if len(input.PublicMessages) == 0 && strings.TrimSpace(input.LatestTrigger.TriggerType) == "" &&
		strings.TrimSpace(input.LatestTrigger.Content) == "" {
		budget := NewRoomContextBudget(input.ContextWindowTokens)
		return VisibleContextPlan{Usage: RoomContextUsage{
			ContextWindowTokens: budget.ContextWindowTokens,
			BudgetTokens:        budget.TotalTokens,
		}}
	}
	return buildVisibleContextPlan(input, true)
}

func buildVisibleContextPlan(input VisibleContextInput, guided bool) VisibleContextPlan {
	budget := NewRoomContextBudget(input.ContextWindowTokens)
	remaining := budget.contentTokens()
	publicLines := buildPublicContextLines(input)
	privateLines := buildPrivateContextLines(input.RoomMessages, input.AgentNameByID)
	currentPrivateIndex := currentDirectedMessageIndex(input, privateLines)

	currentPrivate := newContextSelection()
	if currentPrivateIndex >= 0 {
		limit := min(remaining, budget.currentMessageLimit())
		text, tokens := fitRoomText(privateLines[currentPrivateIndex].text, limit)
		if text != "" {
			currentPrivate.texts[currentPrivateIndex] = text
			currentPrivate.tokens = tokens
			remaining -= tokens
		}
	}

	triggerText := formatRoomTrigger(input.LatestTrigger, input.AgentNameByID)
	triggerLimit := min(remaining, budget.currentMessageLimit())
	triggerText, triggerTokens := fitRoomText(triggerText, triggerLimit)
	remaining -= triggerTokens

	publicSelection := newContextSelection()
	anchorCandidates := []contextLine(nil)
	publicLimit := min(remaining, budget.publicDeltaLimit())
	if input.ColdStart {
		publicSelection = selectContextSuffix(publicLines, publicLimit, nil)
		anchorCandidates = unselectedContextLines(publicLines, publicSelection.texts)
	} else {
		publicSelection = selectContextPrefix(publicLines, publicLimit, nil)
	}
	publicSelectionWasTruncated := contextSelectionWasTruncated(publicLines, publicSelection)
	remaining -= publicSelection.tokens

	excludedPrivate := make(map[int]struct{}, 1)
	if currentPrivateIndex >= 0 {
		excludedPrivate[currentPrivateIndex] = struct{}{}
	}
	privateSelection := selectContextPrefix(
		privateLines,
		min(remaining, budget.privateDeltaLimit()),
		excludedPrivate,
	)
	remaining -= privateSelection.tokens
	if !publicSelectionWasTruncated && !hasPrivateContextDelta(privateLines, excludedPrivate) && remaining > 0 {
		// 私域为空时，把原本预留给 private delta 的空间回流给公区；
		// 不让“空分区”白白吞掉 public feed 的可见事实。
		previousPublicTokens := publicSelection.tokens
		extraPublicTokens := remaining
		if input.ColdStart {
			// 冷启动仍保留最小 anchor 空间，避免只剩一段 delta 而丢掉
			// 产品侧的历史定位信息。
			extraPublicTokens = max(0, extraPublicTokens-budget.publicAnchorLimit())
		}
		expandedPublicLimit := publicSelection.tokens + extraPublicTokens
		if input.ColdStart {
			publicSelection = selectContextSuffix(publicLines, expandedPublicLimit, nil)
			anchorCandidates = unselectedContextLines(publicLines, publicSelection.texts)
		} else {
			publicSelection = selectContextPrefix(publicLines, expandedPublicLimit, nil)
		}
		remaining -= max(0, publicSelection.tokens-previousPublicTokens)
	}

	anchorText := ""
	anchorTokens := 0
	if input.ColdStart && remaining > 0 {
		anchorText, anchorTokens = buildPublicAnchor(
			input.PublicAnchor,
			anchorCandidates,
			min(remaining, budget.publicAnchorLimit()),
		)
	}

	selectedPrivate := mergeContextSelections(currentPrivate, privateSelection)
	text := renderVisibleContext(
		input,
		guided,
		anchorText,
		selectedContextTexts(publicLines, publicSelection.texts),
		triggerText,
		selectedContextTexts(privateLines, selectedPrivate.texts),
	)
	plan := VisibleContextPlan{
		Text: strings.TrimSpace(text),
		Usage: RoomContextUsage{
			ContextWindowTokens:  budget.ContextWindowTokens,
			BudgetTokens:         budget.TotalTokens,
			UsedTokens:           estimateRoomTokens(text),
			CurrentMessageTokens: currentPrivate.tokens,
			TriggerTokens:        triggerTokens,
			PublicDeltaTokens:    publicSelection.tokens,
			PrivateDeltaTokens:   privateSelection.tokens,
			PublicAnchorTokens:   anchorTokens,
			ColdStart:            input.ColdStart,
		},
	}
	plan.PublicBoundary = consumedPublicBoundary(input, publicLines, publicSelection.texts)
	plan.PrivateBoundary = consumedContextBoundary(privateLines, selectedPrivate.texts)
	return plan
}

func buildPublicContextLines(input VisibleContextInput) []contextLine {
	lines := make([]contextLine, 0, len(input.PublicMessages))
	triggerMessageID := strings.TrimSpace(input.LatestTrigger.MessageID)
	for _, message := range input.PublicMessages {
		messageID := normalizeAnyString(message["message_id"])
		text := ""
		if messageID == "" || messageID != triggerMessageID {
			if isVisiblePublicInputMessage(message, input.TargetAgentID) {
				text = formatHistoryLine(message, input.AgentNameByID)
			}
		}
		lines = append(lines, contextLine{
			text:      text,
			messageID: messageID,
			timestamp: normalizeInt64(message["timestamp"]),
		})
	}
	return lines
}

func buildPrivateContextLines(
	messages []protocol.RoomDirectedMessageRecord,
	agentNameByID map[string]string,
) []contextLine {
	lines := make([]contextLine, 0, len(messages))
	for _, message := range messages {
		lines = append(lines, contextLine{
			text:      formatRoomDirectedMessageLine(message, agentNameByID),
			messageID: strings.TrimSpace(message.MessageID),
			timestamp: message.Timestamp,
		})
	}
	return lines
}

func currentDirectedMessageIndex(input VisibleContextInput, lines []contextLine) int {
	if input.LatestTrigger.TriggerType != "room_directed_message" {
		return -1
	}
	targetMessageID := strings.TrimSpace(input.LatestTrigger.MessageID)
	if targetMessageID == "" {
		return -1
	}
	for index := len(lines) - 1; index >= 0; index-- {
		if lines[index].messageID == targetMessageID {
			return index
		}
	}
	return -1
}

func hasPrivateContextDelta(lines []contextLine, excluded map[int]struct{}) bool {
	for index, line := range lines {
		if _, skip := excluded[index]; skip {
			continue
		}
		if strings.TrimSpace(line.text) != "" {
			return true
		}
	}
	return false
}

func contextSelectionWasTruncated(lines []contextLine, selection contextSelection) bool {
	for index, text := range selection.texts {
		if strings.TrimSpace(text) == "" || index < 0 || index >= len(lines) {
			continue
		}
		if strings.TrimSpace(text) != strings.TrimSpace(lines[index].text) {
			return true
		}
	}
	return false
}

func selectContextPrefix(lines []contextLine, maxTokens int, excluded map[int]struct{}) contextSelection {
	selection := newContextSelection()
	for index, line := range lines {
		if _, skip := excluded[index]; skip {
			continue
		}
		if strings.TrimSpace(line.text) == "" {
			selection.texts[index] = ""
			continue
		}
		remaining := maxTokens - selection.tokens
		if remaining <= 0 {
			break
		}
		text, tokens := fitRoomText(line.text, remaining)
		if text == "" {
			break
		}
		selection.texts[index] = text
		selection.tokens += tokens
		if text != strings.TrimSpace(line.text) {
			break
		}
	}
	return selection
}

func selectContextSuffix(lines []contextLine, maxTokens int, excluded map[int]struct{}) contextSelection {
	selection := newContextSelection()
	for index := len(lines) - 1; index >= 0; index-- {
		if _, skip := excluded[index]; skip {
			continue
		}
		if strings.TrimSpace(lines[index].text) == "" {
			selection.texts[index] = ""
			continue
		}
		remaining := maxTokens - selection.tokens
		if remaining <= 0 {
			break
		}
		text, tokens := fitRoomText(lines[index].text, remaining)
		if text == "" {
			break
		}
		selection.texts[index] = text
		selection.tokens += tokens
		if text != strings.TrimSpace(lines[index].text) {
			break
		}
	}
	return selection
}

func mergeContextSelections(items ...contextSelection) contextSelection {
	result := newContextSelection()
	for _, item := range items {
		for index, text := range item.texts {
			result.texts[index] = text
		}
		result.tokens += item.tokens
	}
	return result
}

func selectedContextTexts(lines []contextLine, selected map[int]string) []string {
	result := make([]string, 0, len(selected))
	for index := range lines {
		text, ok := selected[index]
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		result = append(result, text)
	}
	return result
}

func unselectedContextLines(lines []contextLine, selected map[int]string) []contextLine {
	result := make([]contextLine, 0, len(lines))
	for index, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			continue
		}
		if _, ok := selected[index]; ok {
			continue
		}
		result = append(result, line)
	}
	return result
}

func buildPublicAnchor(metadata PublicAnchorMetadata, older []contextLine, maxTokens int) (string, int) {
	if maxTokens <= 0 || (len(older) == 0 && metadata == (PublicAnchorMetadata{})) {
		return "", 0
	}
	metadataLines := make([]string, 0, 3)
	if value := strings.TrimSpace(metadata.RoomName); value != "" {
		metadataLines = append(metadataLines, "Room: "+value)
	}
	if value := strings.TrimSpace(metadata.ConversationTitle); value != "" && value != strings.TrimSpace(metadata.RoomName) {
		metadataLines = append(metadataLines, "Conversation: "+value)
	}
	if value := strings.TrimSpace(metadata.RoomDescription); value != "" {
		metadataLines = append(metadataLines, "Description: "+value)
	}

	prefix := "Nexus public anchor. Earlier public history was compacted here; the public feed remains the source of truth."
	base := strings.Join(append([]string{prefix}, metadataLines...), "\n")
	base, baseTokens := fitRoomText(base, maxTokens)
	remaining := maxTokens - baseTokens
	if remaining <= 0 || len(older) == 0 {
		return base, baseTokens
	}
	selection := selectContextSuffix(older, remaining, nil)
	selected := selectedContextTexts(older, selection.texts)
	if len(selected) == 0 {
		return base, baseTokens
	}
	text := base + "\nEarlier public context:\n" + strings.Join(selected, "\n")
	return text, estimateRoomTokens(text)
}

func consumedPublicBoundary(
	input VisibleContextInput,
	lines []contextLine,
	selected map[int]string,
) ContextBoundary {
	if input.ColdStart {
		return lastContextBoundary(lines)
	}
	return consumedContextBoundary(lines, selected)
}

func consumedContextBoundary(lines []contextLine, selected map[int]string) ContextBoundary {
	boundary := ContextBoundary{}
	for index, line := range lines {
		if strings.TrimSpace(line.text) != "" {
			if _, ok := selected[index]; !ok {
				break
			}
		}
		boundary = ContextBoundary{MessageID: line.messageID, Timestamp: line.timestamp}
	}
	return boundary
}

func lastContextBoundary(lines []contextLine) ContextBoundary {
	if len(lines) == 0 {
		return ContextBoundary{}
	}
	line := lines[len(lines)-1]
	return ContextBoundary{MessageID: line.messageID, Timestamp: line.timestamp}
}

func renderVisibleContext(
	input VisibleContextInput,
	guided bool,
	anchor string,
	publicLines []string,
	trigger string,
	privateLines []string,
) string {
	sections := make([]string, 0, 5)
	if guided {
		sections = append(sections, "New public Room messages arrived while you were running. Treat them as public facts already in the Room. If they affect your current work, incorporate them and continue.")
	}
	if strings.TrimSpace(anchor) != "" {
		sections = append(sections, fmt.Sprintf("<public_anchor>\n%s\n</public_anchor>", anchor))
	}
	if len(publicLines) == 0 {
		publicLines = []string{"(No new public messages this turn.)"}
	}
	sections = append(sections, fmt.Sprintf("<public_feed>\n%s\n</public_feed>", strings.Join(publicLines, "\n")))
	if strings.TrimSpace(trigger) == "" {
		trigger = "(No trigger message.)"
	}
	sections = append(sections, fmt.Sprintf("<latest_trigger>\n%s\n</latest_trigger>", trigger))
	if len(privateLines) > 0 {
		sections = append(sections, wrapRoomDirectedMessageContext(privateLines, input.AgentNameByID, input.TargetAgentID))
	}
	return strings.Join(slices.DeleteFunc(sections, func(value string) bool {
		return strings.TrimSpace(value) == ""
	}), "\n\n")
}
