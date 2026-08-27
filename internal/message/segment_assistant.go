package message

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// AssistantSegment 维护单段 assistant 输出状态。
type AssistantSegment struct {
	messageID  string
	content    []map[string]any
	model      string
	stopReason string
	usage      map[string]any
	timestamp  int64
	streamSlot map[int]int
	// toolInputJSON 按逻辑块保存流式 input_json_delta；普通工具只在完整
	// JSON 可解析后更新 input，show_widget 额外投影已到达的字符串字段。
	toolInputJSON map[int]string
}

// Reset 重置当前段。
func (s *AssistantSegment) Reset() {
	s.messageID = ""
	s.content = nil
	s.model = ""
	s.stopReason = ""
	s.usage = nil
	s.timestamp = 0
	s.streamSlot = nil
	s.toolInputJSON = nil
}

// Start 开始新的 assistant 段。
func (s *AssistantSegment) Start(messageID string, model string, usage map[string]any, timestamp int64) {
	s.Reset()
	s.messageID = firstNonEmpty(messageID, fmt.Sprintf("assistant_%d", time.Now().UnixMilli()))
	s.model = strings.TrimSpace(model)
	s.usage = cloneMap(usage)
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	s.timestamp = timestamp
}

// EnsureStarted 确保段已经初始化。
func (s *AssistantSegment) EnsureStarted() {
	if strings.TrimSpace(s.messageID) != "" {
		return
	}
	s.Start("", "", nil, 0)
}

// IsStarted 表示当前段是否已经初始化。
func (s *AssistantSegment) IsStarted() bool {
	return strings.TrimSpace(s.messageID) != ""
}

// ApplyBlock 按索引设置内容块。
func (s *AssistantSegment) ApplyBlock(index int, block map[string]any) int {
	s.EnsureStarted()
	clonedBlock := cloneMap(block)
	logicalIndex := s.resolveLogicalIndex(index, normalizeString(clonedBlock["type"]))
	logicalIndex = s.resolveStreamBlockConflict(index, logicalIndex, clonedBlock)
	for len(s.content) <= logicalIndex {
		s.content = append(s.content, map[string]any{"type": "text", "text": ""})
	}
	s.content[logicalIndex] = clonedBlock
	if normalizeString(clonedBlock["type"]) == "tool_use" {
		if s.toolInputJSON == nil {
			s.toolInputJSON = map[int]string{}
		}
		s.toolInputJSON[logicalIndex] = ""
	}
	return logicalIndex
}

// ApplyDelta 应用流式增量。
func (s *AssistantSegment) ApplyDelta(index int, delta map[string]any) (int, bool) {
	s.EnsureStarted()
	inferredType := inferBlockTypeFromDelta(delta)
	logicalIndex := s.resolveExistingLogicalIndex(index)
	if logicalIndex < 0 {
		logicalIndex = s.resolveLogicalIndex(index, inferredType)
	}
	for len(s.content) <= logicalIndex {
		blockType := "text"
		if len(s.content) == logicalIndex && inferredType != "" {
			blockType = inferredType
		}
		s.content = append(s.content, emptyAssistantBlock(blockType))
	}
	block := s.content[logicalIndex]
	blockType := normalizeString(block["type"])
	deltaType := normalizeString(delta["type"])

	switch {
	case blockType == "text" && deltaType == "text_delta":
		block["text"] = rawString(block["text"]) + rawString(delta["text"])
	case blockType == "thinking" && deltaType == "thinking_delta":
		block["thinking"] = rawString(block["thinking"]) + rawString(delta["thinking"])
	case blockType == "thinking" && deltaType == "signature_delta":
		block["signature"] = rawString(block["signature"]) + rawString(delta["signature"])
	case blockType == "tool_use" && deltaType == "input_json_delta":
		if s.toolInputJSON == nil {
			s.toolInputJSON = map[int]string{}
		}
		partial := s.toolInputJSON[logicalIndex] + rawString(delta["partial_json"])
		s.toolInputJSON[logicalIndex] = partial
		applyToolInputProjection(block, partial)
	default:
		return logicalIndex, false
	}
	s.content[logicalIndex] = block
	return logicalIndex, true
}

func applyToolInputProjection(block map[string]any, partial string) {
	input := map[string]any{}
	if strings.TrimSpace(partial) != "" && json.Unmarshal([]byte(partial), &input) == nil {
		block["input"] = input
		return
	}
	if isVisualizeShowWidgetTool(normalizeString(block["name"])) {
		for _, field := range []string{"title", "widget_code"} {
			if value, ok := decodePartialJSONStringField(partial, field); ok {
				input[field] = value
			}
		}
	}
	if len(input) > 0 {
		block["input"] = input
	} else if block["input"] == nil {
		block["input"] = map[string]any{}
	}
}

func isVisualizeShowWidgetTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "show_widget",
		"mcp__nexus__show_widget",
		"nexus__show_widget",
		"nexus.show_widget",
		"nexus/show_widget",
		// 旧 server 包装名只用于恢复已持久化 transcript。
		"mcp__nexus_visualize__show_widget",
		"nexus_visualize__show_widget",
		"nexus_visualize.show_widget",
		"nexus_visualize/show_widget":
		return true
	default:
		return false
	}
}

func decodePartialJSONStringField(raw string, field string) (string, bool) {
	marker, _ := json.Marshal(field)
	searchStart := 0
	for searchStart < len(raw) {
		index := strings.Index(raw[searchStart:], string(marker))
		if index < 0 {
			return "", false
		}
		index += searchStart
		cursor := skipJSONSpace(raw, index+len(marker))
		if cursor < len(raw) && raw[cursor] == ':' {
			cursor = skipJSONSpace(raw, cursor+1)
			if cursor < len(raw) && raw[cursor] == '"' {
				return decodeJSONStringPrefix(raw, cursor)
			}
		}
		searchStart = index + len(marker)
	}
	return "", false
}

func jsonStringEnd(raw string, start int) (int, bool) {
	for index := start + 1; index < len(raw); index++ {
		switch raw[index] {
		case '\\':
			index++
		case '"':
			return index + 1, true
		}
	}
	return len(raw), false
}

func skipJSONSpace(raw string, start int) int {
	for start < len(raw) {
		switch raw[start] {
		case ' ', '\n', '\r', '\t':
			start++
		default:
			return start
		}
	}
	return start
}

func decodeJSONStringPrefix(raw string, start int) (string, bool) {
	if end, complete := jsonStringEnd(raw, start); complete {
		var value string
		if json.Unmarshal([]byte(raw[start:end]), &value) == nil {
			return value, true
		}
		return "", false
	}
	// JSON escape 最长为 \uXXXX；最多回退 6 bytes 就能丢弃未完成转义。
	for trim := 0; trim <= 6 && len(raw)-trim > start; trim++ {
		var value string
		candidate := raw[start:len(raw)-trim] + `"`
		if json.Unmarshal([]byte(candidate), &value) == nil {
			return value, true
		}
	}
	return "", false
}

// UpdateMeta 更新消息级元信息。
func (s *AssistantSegment) UpdateMeta(model string, usage map[string]any, stopReason string) {
	model = strings.TrimSpace(model)
	if model != "" {
		s.model = model
	}
	if len(usage) > 0 {
		if s.usage == nil {
			s.usage = map[string]any{}
		}
		for key, value := range usage {
			s.usage[key] = value
		}
	}
	stopReason = strings.TrimSpace(stopReason)
	if stopReason != "" {
		s.stopReason = stopReason
	}
}

// ReplaceFromSnapshot 用 SDK assistant 快照补齐当前段。
func (s *AssistantSegment) ReplaceFromSnapshot(content []map[string]any, model string, usage map[string]any, stopReason string) {
	s.EnsureStarted()
	if len(s.content) == 0 {
		s.content = cloneBlockSlice(content)
	} else {
		for _, block := range content {
			s.upsertBlock(block)
		}
	}
	s.UpdateMeta(model, usage, stopReason)
}

// AppendTaskProgress 追加或更新任务进度块。
func (s *AssistantSegment) AppendTaskProgress(block map[string]any) {
	s.EnsureStarted()
	s.upsertBlock(block)
}

// AppendToolResults 追加工具结果块。
func (s *AssistantSegment) AppendToolResults(content []map[string]any) {
	s.EnsureStarted()
	for _, block := range content {
		s.upsertBlock(block)
	}
}

// HasContent 表示当前段是否已有内容。
func (s *AssistantSegment) HasContent() bool {
	return len(s.content) > 0
}

// FindToolName 根据 tool_use_id 在已累积的 content 中反查工具名称。
func (s *AssistantSegment) FindToolName(toolUseID string) string {
	for _, block := range s.content {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		if normalizeString(block["id"]) == toolUseID {
			return normalizeString(block["name"])
		}
	}
	return ""
}

// FindToolUse 返回指定 tool_use 的完整内容块。
func (s *AssistantSegment) FindToolUse(toolUseID string) map[string]any {
	trimmedToolUseID := normalizeString(toolUseID)
	if trimmedToolUseID == "" {
		return nil
	}
	for _, block := range s.content {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		if normalizeString(block["id"]) == trimmedToolUseID {
			return cloneMap(block)
		}
	}
	return nil
}

// HasToolUse 表示当前段是否包含指定工具调用。
func (s *AssistantSegment) HasToolUse(toolUseID string) bool {
	trimmedToolUseID := normalizeString(toolUseID)
	if trimmedToolUseID == "" {
		return false
	}
	for _, block := range s.content {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		if normalizeString(block["id"]) == trimmedToolUseID {
			return true
		}
	}
	return false
}

// MessageID 返回当前 assistant message_id。
func (s *AssistantSegment) MessageID() string {
	s.EnsureStarted()
	return s.messageID
}

// Model 返回当前段 model。
func (s *AssistantSegment) Model() string {
	return s.model
}

// StopReason 返回当前 stop_reason。
func (s *AssistantSegment) StopReason() string {
	return s.stopReason
}

// Usage 返回 usage 快照。
func (s *AssistantSegment) Usage() map[string]any {
	return cloneMap(s.usage)
}

// CurrentBlock 返回指定索引的当前块。
func (s *AssistantSegment) CurrentBlock(index int) map[string]any {
	logicalIndex := index
	if mappedIndex := s.resolveExistingLogicalIndex(index); mappedIndex >= 0 {
		logicalIndex = mappedIndex
	}
	if logicalIndex < 0 || logicalIndex >= len(s.content) {
		return nil
	}
	return redactConfigurationToolBlock(cloneMap(s.content[logicalIndex]))
}

// BuildAssistantMessage 构建 assistant 消息。
func (s *AssistantSegment) BuildAssistantMessage(ctx MessageContext, sessionID string, isComplete bool) map[string]any {
	s.EnsureStarted()
	payload := baseMessageEnvelope(ctx, sessionID, s.messageID, "assistant")
	payload["content"] = s.normalizedContent()
	payload["model"] = emptyToNil(s.model)
	payload["usage"] = nilIfEmptyMap(s.usage)
	payload["is_complete"] = isComplete
	if strings.TrimSpace(s.stopReason) != "" {
		payload["stop_reason"] = s.stopReason
	}
	if s.timestamp > 0 {
		payload["timestamp"] = s.timestamp
	}
	return payload
}

func (s *AssistantSegment) normalizedContent() []map[string]any {
	content := cloneBlockSlice(s.content)
	for index := range content {
		content[index] = redactConfigurationToolBlock(content[index])
	}
	if len(content) <= 1 {
		return content
	}

	thinkingIndex := -1
	for index, block := range content {
		if normalizeString(block["type"]) == "thinking" {
			thinkingIndex = index
			break
		}
	}
	if thinkingIndex <= 0 {
		return content
	}

	// Python 主链路会把 thinking 固定放在内容首位，
	// 这样前端无论实时替换还是历史回放，都会稳定先渲染思考过程。
	thinkingBlock := content[thinkingIndex]
	copy(content[1:thinkingIndex+1], content[0:thinkingIndex])
	content[0] = thinkingBlock
	return content
}

func redactConfigurationToolBlock(block map[string]any) map[string]any {
	if normalizeString(block["type"]) != "tool_use" {
		return block
	}
	input, ok := block["input"].(map[string]any)
	if !ok {
		return block
	}
	block["input"] = secretinput.RedactConfigurationToolInput(
		normalizeString(block["name"]),
		input,
	)
	return block
}

type assistantBlockMatcher func(map[string]any, map[string]any) bool

var assistantBlockMatchers = map[string]assistantBlockMatcher{
	"thinking": func(map[string]any, map[string]any) bool { return true },
	"tool_use": func(current map[string]any, incoming map[string]any) bool {
		currentID := normalizeString(current["id"])
		incomingID := normalizeString(incoming["id"])
		// 无 content_block_start 时，流式 input_json_delta 会先留下无 ID
		// 占位；最终 assistant 快照到达后应替换它，而不是生成孤儿工具块。
		return currentID == "" || incomingID == "" || currentID == incomingID
	},
	"tool_result":   blockFieldMatcher("tool_use_id"),
	"task_progress": blockFieldMatcher("task_id"),
	protocol.ContentBlockTypeWorkGraphArtifact: blockFieldMatcher("id"),
	protocol.ContentBlockTypeWorkspaceFileArtifact: func(current map[string]any, incoming map[string]any) bool {
		return workspaceFileArtifactKey(current) == workspaceFileArtifactKey(incoming)
	},
	"text": func(current map[string]any, incoming map[string]any) bool {
		currentText := rawString(current["text"])
		incomingText := rawString(incoming["text"])
		return currentText == incomingText ||
			strings.HasPrefix(currentText, incomingText) ||
			strings.HasPrefix(incomingText, currentText)
	},
}

func blockFieldMatcher(field string) assistantBlockMatcher {
	return func(current map[string]any, incoming map[string]any) bool {
		return normalizeString(current[field]) == normalizeString(incoming[field])
	}
}

func (s *AssistantSegment) upsertBlock(incoming map[string]any) {
	block := cloneMap(incoming)
	incomingType := normalizeString(block["type"])
	matcher := assistantBlockMatchers[incomingType]
	if matcher == nil {
		s.content = append(s.content, block)
		return
	}
	for index, current := range s.content {
		currentType := normalizeString(current["type"])
		if currentType != incomingType || !matcher(current, block) {
			continue
		}
		s.content[index] = block
		return
	}
	s.content = append(s.content, block)
}

func workspaceFileArtifactKey(block map[string]any) string {
	if id := normalizeString(block["id"]); id != "" {
		return id
	}
	sourceToolUseID := normalizeString(block["source_tool_use_id"])
	path := normalizeString(block["path"])
	if sourceToolUseID == "" || path == "" {
		return ""
	}
	return sourceToolUseID + ":" + path
}

func (s *AssistantSegment) resolveLogicalIndex(rawIndex int, blockType string) int {
	if s.streamSlot == nil {
		s.streamSlot = make(map[int]int)
	}
	if logicalIndex, exists := s.streamSlot[rawIndex]; exists {
		if logicalIndex >= 0 && logicalIndex < len(s.content) {
			currentType := normalizeString(s.content[logicalIndex]["type"])
			if currentType == "" || currentType == blockType {
				return logicalIndex
			}
		}
	}

	// SDK 的原始 stream index 可能在 thinking 结束后被 text 复用。
	// 为了和 Python 后端保持一致，这里暴露给前端的是“累计逻辑索引”，
	// 同一轮中新块出现时始终追加到 content 尾部，避免 text 把 think 顶掉。
	logicalIndex := len(s.content)
	s.streamSlot[rawIndex] = logicalIndex
	return logicalIndex
}

func (s *AssistantSegment) resolveStreamBlockConflict(rawIndex int, logicalIndex int, block map[string]any) int {
	if logicalIndex < 0 || logicalIndex >= len(s.content) {
		return logicalIndex
	}
	if !isConflictingStreamToolUse(s.content[logicalIndex], block) {
		return logicalIndex
	}

	// 部分 SDK 会在同一段 assistant 中复用 raw index=0 输出多个 tool_use。
	// 按 tool_use id 拆成独立逻辑块，避免后一个工具调用覆盖前一个。
	nextIndex := len(s.content)
	s.streamSlot[rawIndex] = nextIndex
	return nextIndex
}

func isConflictingStreamToolUse(current map[string]any, incoming map[string]any) bool {
	if normalizeString(current["type"]) != "tool_use" || normalizeString(incoming["type"]) != "tool_use" {
		return false
	}
	currentID := normalizeString(current["id"])
	incomingID := normalizeString(incoming["id"])
	return currentID != "" && incomingID != "" && currentID != incomingID
}

func (s *AssistantSegment) resolveExistingLogicalIndex(rawIndex int) int {
	if s.streamSlot == nil {
		return -1
	}
	logicalIndex, exists := s.streamSlot[rawIndex]
	if !exists {
		return -1
	}
	return logicalIndex
}

func inferBlockTypeFromDelta(delta map[string]any) string {
	switch normalizeString(delta["type"]) {
	case "thinking_delta", "signature_delta":
		return "thinking"
	case "text_delta":
		return "text"
	case "input_json_delta":
		return "tool_use"
	default:
		return ""
	}
}

func emptyAssistantBlock(blockType string) map[string]any {
	switch blockType {
	case "thinking":
		return map[string]any{"type": "thinking", "thinking": ""}
	case "tool_use":
		return map[string]any{"type": "tool_use", "input": map[string]any{}}
	default:
		return map[string]any{"type": "text", "text": ""}
	}
}

const maxToolUseSummaryRunes = 240

func (p *Processor) projectToolUseSummary(summary sdkprotocol.ToolUseSummaryMessage) *protocol.Message {
	text := sanitizeToolUseSummary(summary.Summary)
	if text == "" {
		return nil
	}
	payload := protocol.Message(baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		toolUseSummaryMessageID(p.ctx),
		"assistant",
	))
	payload["content"] = []map[string]any{{
		"type":                   "progress_update",
		"text":                   text,
		"preceding_tool_use_ids": append([]string(nil), summary.PrecedingToolUseIDs...),
	}}
	payload["is_complete"] = false
	payload["metadata"] = map[string]any{
		"subtype": "tool_use_summary",
	}
	return &payload
}

func toolUseSummaryMessageID(ctx MessageContext) string {
	identity := strings.Join([]string{
		strings.TrimSpace(ctx.RoundID),
		strings.TrimSpace(ctx.AgentRoundID),
		strings.TrimSpace(ctx.AgentID),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "msg_assistant_progress_" + hex.EncodeToString(digest[:12])
}

func sanitizeToolUseSummary(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > maxToolUseSummaryRunes {
		value = string([]rune(value)[:maxToolUseSummaryRunes-1]) + "…"
	}
	return strings.TrimSpace(value)
}
