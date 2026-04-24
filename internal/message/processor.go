package message

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// MessageContext 表示单轮消息处理上下文。
type MessageContext struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	AgentID        string
	RoundID        string
	ParentID       string
}

// StreamPayload 表示统一 stream 数据。
type StreamPayload struct {
	MessageID string
	Data      map[string]any
}

// Output 表示处理单条 SDK 消息后的统一输出。
type Output struct {
	StreamEvents        []StreamPayload
	DurableMessages     []protocol.Message
	EphemeralMessages   []protocol.Message
	RegisteredSessionID string
	TerminalStatus      string
	ResultSubtype       string
	StreamStarted       bool
	AssistantCompleted  bool
	Err                 error
}

// Processor 负责把 SDK 消息转换成统一协议语义。
type Processor struct {
	ctx       MessageContext
	sessionID string
	segment   AssistantSegment

	streamStarted                bool
	streamTerminalObserved       bool
	lastDurableAssistantSnapshot protocol.Message
}

// NewProcessor 创建统一消息处理器。
func NewProcessor(ctx MessageContext, sessionID string) *Processor {
	return &Processor{
		ctx:       ctx,
		sessionID: strings.TrimSpace(sessionID),
	}
}

// CurrentMessageID 返回当前 assistant message_id。
func (p *Processor) CurrentMessageID() string {
	return p.segment.MessageID()
}

// SessionID 返回当前 SDK session_id。
func (p *Processor) SessionID() string {
	return strings.TrimSpace(p.sessionID)
}

// Process 处理一条 SDK 消息。
func (p *Processor) Process(message sdkprotocol.ReceivedMessage) Output {
	output := Output{}
	updated, err := p.registerSessionID(message)
	if err != nil {
		output.Err = err
		return output
	}
	if updated != "" {
		output.RegisteredSessionID = updated
	}

	switch message.Type {
	case sdkprotocol.MessageTypeStreamEvent:
		return p.processStreamEvent(message, output)
	case sdkprotocol.MessageTypeAssistant:
		if durable := p.processAssistantMessage(message); durable != nil {
			output.DurableMessages = append(output.DurableMessages, *durable)
			if (*durable)["is_complete"] == true {
				output.AssistantCompleted = true
			}
		}
	case sdkprotocol.MessageTypeSystem:
		durableMessages, ephemeralMessages := p.processSystemMessage(message)
		output.DurableMessages = append(output.DurableMessages, durableMessages...)
		output.EphemeralMessages = append(output.EphemeralMessages, ephemeralMessages...)
	case sdkprotocol.MessageTypeResult:
		subtype := normalizeResultSubtype(message.Result)
		output.DurableMessages = append(output.DurableMessages, p.buildResultMessage(message, subtype))
		output.ResultSubtype = subtype
		output.TerminalStatus = statusFromResultSubtype(subtype)
	case sdkprotocol.MessageTypeToolProgress:
		if messageValue := p.processToolProgressMessage(message); messageValue != nil {
			output.DurableMessages = append(output.DurableMessages, *messageValue)
		}
	case sdkprotocol.MessageTypeUser:
		if durable := p.processToolResultMessage(message); durable != nil {
			output.DurableMessages = append(output.DurableMessages, *durable)
			output.AssistantCompleted = true
		}
	}
	return output
}

func (p *Processor) processStreamEvent(message sdkprotocol.ReceivedMessage, output Output) Output {
	if message.Stream == nil {
		return output
	}
	payload, ok := message.Stream.Event.(map[string]any)
	if !ok {
		payload = message.Stream.Data
	}
	eventType := normalizeString(payload["type"])
	if eventType == "" {
		return output
	}

	switch eventType {
	case "message_start":
		messagePayload, _ := payload["message"].(map[string]any)
		usage, _ := messagePayload["usage"].(map[string]any)
		p.segment.Start(
			normalizeString(messagePayload["id"]),
			normalizeString(messagePayload["model"]),
			usage,
			time.Now().UnixMilli(),
		)
		p.streamStarted = true
		p.streamTerminalObserved = false
		p.lastDurableAssistantSnapshot = nil
		output.StreamStarted = true
		output.StreamEvents = append(output.StreamEvents, StreamPayload{
			MessageID: p.segment.MessageID(),
			Data: map[string]any{
				"message_id":      p.segment.MessageID(),
				"session_key":     p.ctx.SessionKey,
				"room_id":         emptyToNil(p.ctx.RoomID),
				"conversation_id": emptyToNil(p.ctx.ConversationID),
				"agent_id":        p.ctx.AgentID,
				"round_id":        p.ctx.RoundID,
				"session_id":      emptyToNil(p.sessionID),
				"type":            "message_start",
				"message": map[string]any{
					"model": emptyToNil(p.segment.Model()),
				},
				"usage":     p.segment.Usage(),
				"timestamp": time.Now().UnixMilli(),
			},
		})
	case "content_block_start":
		index := normalizeInt(payload["index"])
		block := normalizeContentBlock(payload["content_block"])
		if len(block) == 0 {
			return output
		}
		logicalIndex := p.segment.ApplyBlock(index, block)
		if normalizeString(block["type"]) == "tool_use" {
			return output
		}
		output.StreamEvents = append(output.StreamEvents, p.buildBlockStreamPayload("content_block_start", logicalIndex, block))
	case "content_block_delta":
		index := normalizeInt(payload["index"])
		delta, _ := payload["delta"].(map[string]any)
		logicalIndex, applied := p.segment.ApplyDelta(index, delta)
		if !applied {
			return output
		}
		block := p.segment.CurrentBlock(logicalIndex)
		if normalizeString(block["type"]) == "tool_use" {
			return output
		}
		output.StreamEvents = append(output.StreamEvents, p.buildBlockStreamPayload("content_block_delta", logicalIndex, block))
	case "message_delta":
		delta, _ := payload["delta"].(map[string]any)
		usage, _ := payload["usage"].(map[string]any)
		p.segment.UpdateMeta("", usage, normalizeString(delta["stop_reason"]))
		output.StreamEvents = append(output.StreamEvents, StreamPayload{
			MessageID: p.segment.MessageID(),
			Data: map[string]any{
				"message_id":      p.segment.MessageID(),
				"session_key":     p.ctx.SessionKey,
				"room_id":         emptyToNil(p.ctx.RoomID),
				"conversation_id": emptyToNil(p.ctx.ConversationID),
				"agent_id":        p.ctx.AgentID,
				"round_id":        p.ctx.RoundID,
				"session_id":      emptyToNil(p.sessionID),
				"type":            "message_delta",
				"message": map[string]any{
					"model":       emptyToNil(p.segment.Model()),
					"stop_reason": emptyToNil(p.segment.StopReason()),
				},
				"usage":     p.segment.Usage(),
				"timestamp": time.Now().UnixMilli(),
			},
		})
		if p.segment.HasContent() && strings.TrimSpace(p.segment.StopReason()) != "" {
			p.streamTerminalObserved = true
			if durable := p.buildAssistantDurableMessage(true, true, ""); durable != nil {
				output.DurableMessages = append(output.DurableMessages, *durable)
				output.AssistantCompleted = true
			}
		}
	case "message_stop":
		output.StreamEvents = append(output.StreamEvents, StreamPayload{
			MessageID: p.segment.MessageID(),
			Data: map[string]any{
				"message_id":      p.segment.MessageID(),
				"session_key":     p.ctx.SessionKey,
				"room_id":         emptyToNil(p.ctx.RoomID),
				"conversation_id": emptyToNil(p.ctx.ConversationID),
				"agent_id":        p.ctx.AgentID,
				"round_id":        p.ctx.RoundID,
				"session_id":      emptyToNil(p.sessionID),
				"type":            "message_stop",
				"message": map[string]any{
					"model":       emptyToNil(p.segment.Model()),
					"stop_reason": emptyToNil(p.segment.StopReason()),
				},
				"usage":     p.segment.Usage(),
				"timestamp": time.Now().UnixMilli(),
			},
		})
	}
	return output
}

func (p *Processor) processAssistantMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if !p.segment.IsStarted() {
		p.segment.Start(
			message.Assistant.Message.ID,
			message.Assistant.Message.Model,
			message.Assistant.Message.Usage,
			time.Now().UnixMilli(),
		)
		p.streamStarted = false
		p.streamTerminalObserved = false
		p.lastDurableAssistantSnapshot = nil
	}
	content := normalizeContentBlocks(message.Assistant.Message.Content)
	p.segment.ReplaceFromSnapshot(
		content,
		message.Assistant.Message.Model,
		firstNonNilMap(message.Assistant.Message.Usage, p.segment.Usage()),
		normalizeAnyString(message.Assistant.Message.StopReason),
	)
	includeStopReason := !p.streamStarted || p.streamTerminalObserved
	isComplete := includeStopReason && strings.TrimSpace(p.segment.StopReason()) != ""
	parentID := normalizePointerString(message.Assistant.ParentToolUseID)
	durable := p.buildAssistantDurableMessage(isComplete, includeStopReason, parentID)
	if durable == nil {
		return nil
	}
	return durable
}

func (p *Processor) processSystemMessage(message sdkprotocol.ReceivedMessage) ([]protocol.Message, []protocol.Message) {
	if message.System == nil {
		return nil, nil
	}
	subtype := strings.TrimSpace(message.System.Subtype)
	if subtype == "task_progress" {
		progressMessage := p.buildTaskProgressMessage(
			firstNonEmpty(
				stringValue(message.System.Data["task_id"]),
				firstTaskProgressTaskID(message.System),
			),
			firstNonEmpty(
				stringValue(message.System.Data["description"]),
				firstTaskProgressDescription(message.System),
			),
			firstNonEmpty(
				stringValue(message.System.Data["tool_use_id"]),
				firstTaskProgressToolUseID(message.System),
			),
			firstNonEmpty(
				stringValue(message.System.Data["last_tool_name"]),
				firstTaskProgressToolName(message.System),
			),
			firstNonNilMap(
				mapValue(message.System.Data["usage"]),
				firstTaskProgressUsage(message.System),
			),
		)
		if progressMessage == nil {
			return nil, nil
		}
		return []protocol.Message{*progressMessage}, nil
	}

	if visible, ephemeral := p.buildVisibleSystemMessage(message.System); visible != nil {
		if ephemeral {
			return nil, []protocol.Message{*visible}
		}
		return []protocol.Message{*visible}, nil
	}
	return nil, nil
}

func (p *Processor) processToolProgressMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.ToolProgress == nil {
		return nil
	}
	return p.buildTaskProgressMessage(
		firstNonEmpty(strings.TrimSpace(message.ToolProgress.TaskID), strings.TrimSpace(message.ToolProgress.ToolUseID)),
		firstNonEmpty(strings.TrimSpace(message.ToolProgress.ToolName)+" 正在执行", "后台任务正在执行"),
		strings.TrimSpace(message.ToolProgress.ToolUseID),
		strings.TrimSpace(message.ToolProgress.ToolName),
		nil,
	)
}

func (p *Processor) processToolResultMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.User == nil {
		return nil
	}
	content := normalizeContentBlocks(message.User.Message.Content)
	if len(content) == 0 {
		return nil
	}
	for _, block := range content {
		if normalizeString(block["type"]) != "tool_result" {
			return nil
		}
	}
	for _, block := range content {
		enrichedBlock := p.enrichToolResultBlock(block)
		p.segment.AppendToolResults([]map[string]any{enrichedBlock})
	}
	return p.buildAssistantDurableMessage(true, true, "")
}

func (p *Processor) enrichToolResultBlock(block map[string]any) map[string]any {
	enriched := cloneMap(block)
	if len(enriched) == 0 {
		enriched = map[string]any{"type": "tool_result"}
	}
	if boolValue(enriched["is_error"]) {
		toolUseID := normalizeString(enriched["tool_use_id"])
		if toolUseID != "" {
			toolName := p.segment.FindToolName(toolUseID)
			errorCode := inferPermissionErrorCode(toolName, normalizeString(enriched["content"]))
			if errorCode != "" {
				enriched["error_code"] = errorCode
			}
		}
	}
	return enriched
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}
	return typed
}

func (p *Processor) buildTaskProgressMessage(taskID string, description string, toolUseID string, lastToolName string, usage map[string]any) *protocol.Message {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	p.segment.AppendTaskProgress(map[string]any{
		"type":           "task_progress",
		"task_id":        taskID,
		"description":    description,
		"tool_use_id":    emptyToNil(toolUseID),
		"last_tool_name": emptyToNil(lastToolName),
		"usage":          firstNonNilMap(usage, map[string]any{}),
	})
	return p.buildAssistantDurableMessage(false, false, "")
}

func (p *Processor) buildVisibleSystemMessage(message *sdkprotocol.SystemMessage) (*protocol.Message, bool) {
	if message == nil {
		return nil, false
	}
	subtype := strings.TrimSpace(message.Subtype)
	var (
		content           string
		metadata          map[string]any
		explicitMessageID string
		ephemeral         bool
	)
	switch subtype {
	case "task_started":
		content = firstNonEmpty(
			stringValue(message.Data["description"]),
			stringValue(message.Data["prompt"]),
			firstTaskStartedDescription(message),
			"任务已开始",
		)
		metadata = map[string]any{
			"subtype":     "task_started",
			"task_id":     firstNonEmpty(stringValue(message.Data["task_id"]), firstTaskStartedTaskID(message)),
			"task_type":   firstNonEmpty(stringValue(message.Data["task_type"]), firstTaskStartedTaskType(message)),
			"tool_use_id": firstNonEmpty(stringValue(message.Data["tool_use_id"]), firstTaskStartedToolUseID(message)),
		}
	case "api_retry":
		content = firstNonEmpty(stringValue(message.Data["message"]), "API 正在重试")
		metadata = cloneMap(message.Data)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["subtype"] = "api_retry"
		explicitMessageID = "system_api_retry_" + p.ctx.RoundID
		ephemeral = true
	default:
		return nil, false
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(explicitMessageID, fmt.Sprintf("system_%s_%d", p.ctx.RoundID, time.Now().UnixMilli())),
		"system",
	)
	payload["content"] = content
	payload["metadata"] = metadata
	messageValue := protocol.Message(payload)
	return &messageValue, ephemeral
}

func (p *Processor) buildResultMessage(message sdkprotocol.ReceivedMessage, subtype string) protocol.Message {
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(strings.TrimSpace(message.UUID), "result_"+p.ctx.RoundID),
		"result",
	)
	payload["subtype"] = subtype
	payload["duration_ms"] = message.Result.DurationMS
	payload["duration_api_ms"] = message.Result.DurationAPIMS
	payload["num_turns"] = message.Result.NumTurns
	payload["total_cost_usd"] = message.Result.TotalCostUSD
	payload["usage"] = firstNonNilMap(message.Result.Usage, map[string]any{})
	payload["result"] = message.Result.Result
	payload["is_error"] = subtype == "error"
	return protocol.Message(payload)
}

func (p *Processor) buildBlockStreamPayload(streamType string, index int, block map[string]any) StreamPayload {
	return StreamPayload{
		MessageID: p.segment.MessageID(),
		Data: map[string]any{
			"message_id":      p.segment.MessageID(),
			"session_key":     p.ctx.SessionKey,
			"room_id":         emptyToNil(p.ctx.RoomID),
			"conversation_id": emptyToNil(p.ctx.ConversationID),
			"agent_id":        p.ctx.AgentID,
			"round_id":        p.ctx.RoundID,
			"session_id":      emptyToNil(p.sessionID),
			"type":            streamType,
			"index":           index,
			"content_block":   cloneMap(block),
			"timestamp":       time.Now().UnixMilli(),
		},
	}
}

func (p *Processor) registerSessionID(message sdkprotocol.ReceivedMessage) (string, error) {
	currentSessionID := strings.TrimSpace(p.sessionID)
	candidates := []string{strings.TrimSpace(message.SessionID)}
	if message.Type == sdkprotocol.MessageTypeSystem && message.System != nil {
		candidates = append(candidates, strings.TrimSpace(stringValue(message.System.Data["session_id"])))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if currentSessionID == "" {
			p.sessionID = candidate
			return candidate, nil
		}
		if currentSessionID != candidate {
			return "", fmt.Errorf(
				"processor session_id changed: current=%s incoming=%s round_id=%s",
				currentSessionID,
				candidate,
				p.ctx.RoundID,
			)
		}
	}
	return "", nil
}

// NormalizeInterruptedOutput 统一把“用户主动停止后 SDK 仍返回 error”的结果收口成 interrupted。
func NormalizeInterruptedOutput(output *Output, interruptReason string) {
	if output == nil {
		return
	}
	if output.ResultSubtype != "error" && output.TerminalStatus != "error" {
		return
	}

	resultText := strings.TrimSpace(interruptReason)
	output.ResultSubtype = "interrupted"
	output.TerminalStatus = "interrupted"
	for index := range output.DurableMessages {
		messageValue := output.DurableMessages[index]
		if protocol.MessageRole(messageValue) != "result" {
			continue
		}
		messageValue["subtype"] = "interrupted"
		messageValue["is_error"] = false
		if resultText == "" {
			delete(messageValue, "result")
		} else {
			messageValue["result"] = resultText
		}
		output.DurableMessages[index] = messageValue
	}
}

func baseMessageEnvelope(ctx MessageContext, sessionID string, messageID string, role string) map[string]any {
	payload := map[string]any{
		"message_id":  strings.TrimSpace(messageID),
		"session_key": ctx.SessionKey,
		"agent_id":    ctx.AgentID,
		"round_id":    ctx.RoundID,
		"role":        role,
		"timestamp":   time.Now().UnixMilli(),
	}
	if strings.TrimSpace(sessionID) != "" {
		payload["session_id"] = strings.TrimSpace(sessionID)
	}
	if strings.TrimSpace(ctx.ParentID) != "" && role != "user" {
		payload["parent_id"] = strings.TrimSpace(ctx.ParentID)
	}
	if strings.TrimSpace(ctx.RoomID) != "" {
		payload["room_id"] = strings.TrimSpace(ctx.RoomID)
	}
	if strings.TrimSpace(ctx.ConversationID) != "" {
		payload["conversation_id"] = strings.TrimSpace(ctx.ConversationID)
	}
	return payload
}

func normalizeResultSubtype(result *sdkprotocol.ResultMessage) string {
	if result == nil {
		return "error"
	}
	subtype := strings.TrimSpace(result.Subtype)
	switch subtype {
	case "success", "error", "interrupted":
		return subtype
	default:
		if result.IsError {
			return "error"
		}
		return "success"
	}
}

func statusFromResultSubtype(subtype string) string {
	switch subtype {
	case "interrupted":
		return "interrupted"
	case "error":
		return "error"
	default:
		return "finished"
	}
}

func (p *Processor) buildAssistantDurableMessage(
	isComplete bool,
	includeStopReason bool,
	parentID string,
) *protocol.Message {
	payload := protocol.Message(p.segment.BuildAssistantMessage(p.ctx, p.sessionID, isComplete))
	if !includeStopReason {
		delete(payload, "stop_reason")
		payload["is_complete"] = false
	}
	if strings.TrimSpace(parentID) != "" {
		payload["parent_id"] = strings.TrimSpace(parentID)
	}
	if assistantMessagesEqual(p.lastDurableAssistantSnapshot, payload) {
		return nil
	}
	p.lastDurableAssistantSnapshot = protocol.Clone(payload)
	return &payload
}

func assistantMessagesEqual(previous protocol.Message, current protocol.Message) bool {
	if len(previous) == 0 || len(current) == 0 {
		return false
	}
	return stringValue(previous["message_id"]) == stringValue(current["message_id"]) &&
		stringValue(previous["parent_id"]) == stringValue(current["parent_id"]) &&
		stringValue(previous["model"]) == stringValue(current["model"]) &&
		stringValue(previous["stop_reason"]) == stringValue(current["stop_reason"]) &&
		stringValue(previous["session_id"]) == stringValue(current["session_id"]) &&
		stringValue(previous["round_id"]) == stringValue(current["round_id"]) &&
		boolValue(previous["is_complete"]) == boolValue(current["is_complete"]) &&
		reflect.DeepEqual(previous["content"], current["content"])
}

func stringValue(value any) string {
	return normalizeString(value)
}

func normalizeAnyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case nil:
		return ""
	default:
		raw := strings.TrimSpace(fmt.Sprint(typed))
		if raw == "<nil>" {
			return ""
		}
		return raw
	}
}

func mapValue(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return cloneMap(typed)
}

func normalizePointerString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstTaskProgressTaskID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.TaskID)
}

func firstTaskProgressDescription(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return firstNonEmpty(strings.TrimSpace(message.TaskProgress.Summary), strings.TrimSpace(message.TaskProgress.Description))
}

func firstTaskProgressToolUseID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.ToolUseID)
}

func firstTaskProgressToolName(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.LastToolName)
}

func firstTaskProgressUsage(message *sdkprotocol.SystemMessage) map[string]any {
	if message == nil || message.TaskProgress == nil {
		return nil
	}
	usage := map[string]any{}
	if message.TaskProgress.Usage.TotalTokens > 0 {
		usage["total_tokens"] = message.TaskProgress.Usage.TotalTokens
	}
	if message.TaskProgress.Usage.ToolUses > 0 {
		usage["tool_uses"] = message.TaskProgress.Usage.ToolUses
	}
	if message.TaskProgress.Usage.DurationMS > 0 {
		usage["duration_ms"] = message.TaskProgress.Usage.DurationMS
	}
	return usage
}

func firstTaskStartedDescription(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return firstNonEmpty(
		strings.TrimSpace(message.TaskStarted.Description),
		strings.TrimSpace(message.TaskStarted.Prompt),
	)
}

func firstTaskStartedTaskID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.TaskID)
}

func firstTaskStartedTaskType(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.TaskType)
}

func firstTaskStartedToolUseID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.ToolUseID)
}
