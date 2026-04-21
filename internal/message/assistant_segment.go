package message

import (
	"fmt"
	"strings"
	"time"
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
	logicalIndex := s.resolveLogicalIndex(index, normalizeString(block["type"]))
	for len(s.content) <= logicalIndex {
		s.content = append(s.content, map[string]any{"type": "text", "text": ""})
	}
	s.content[logicalIndex] = cloneMap(block)
	return logicalIndex
}

// ApplyDelta 应用流式增量。
func (s *AssistantSegment) ApplyDelta(index int, delta map[string]any) (int, bool) {
	s.EnsureStarted()
	logicalIndex := s.resolveExistingLogicalIndex(index)
	if logicalIndex < 0 {
		logicalIndex = s.resolveLogicalIndex(index, inferBlockTypeFromDelta(delta))
	}
	for len(s.content) <= logicalIndex {
		s.content = append(s.content, map[string]any{"type": "text", "text": ""})
	}
	block := cloneMap(s.content[logicalIndex])
	blockType := normalizeString(block["type"])
	deltaType := normalizeString(delta["type"])

	switch {
	case blockType == "text" && deltaType == "text_delta":
		block["text"] = rawString(block["text"]) + rawString(delta["text"])
	case blockType == "thinking" && deltaType == "thinking_delta":
		block["thinking"] = rawString(block["thinking"]) + rawString(delta["thinking"])
	case blockType == "thinking" && deltaType == "signature_delta":
		block["signature"] = rawString(block["signature"]) + rawString(delta["signature"])
	default:
		return logicalIndex, false
	}
	s.content[logicalIndex] = block
	return logicalIndex, true
}

// UpdateMeta 更新消息级元信息。
func (s *AssistantSegment) UpdateMeta(model string, usage map[string]any, stopReason string) {
	if strings.TrimSpace(model) != "" {
		s.model = strings.TrimSpace(model)
	}
	if len(usage) > 0 {
		s.usage = cloneMap(usage)
	}
	if strings.TrimSpace(stopReason) != "" {
		s.stopReason = strings.TrimSpace(stopReason)
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
	return cloneMap(s.content[logicalIndex])
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

func (s *AssistantSegment) upsertBlock(incoming map[string]any) {
	block := cloneMap(incoming)
	incomingType := normalizeString(block["type"])
	for index, current := range s.content {
		currentType := normalizeString(current["type"])
		if currentType != incomingType {
			continue
		}
		switch incomingType {
		case "thinking":
			s.content[index] = block
			return
		case "tool_use":
			if normalizeString(current["id"]) == normalizeString(block["id"]) {
				s.content[index] = block
				return
			}
		case "tool_result":
			if normalizeString(current["tool_use_id"]) == normalizeString(block["tool_use_id"]) {
				s.content[index] = block
				return
			}
		case "task_progress":
			if normalizeString(current["task_id"]) == normalizeString(block["task_id"]) {
				s.content[index] = block
				return
			}
		case "text":
			if rawString(current["text"]) == rawString(block["text"]) {
				s.content[index] = block
				return
			}
		}
	}
	s.content = append(s.content, block)
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
	default:
		return ""
	}
}
