package message

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// InterruptWithoutMessage 表示用户主动停止但不需要把默认停止文案写入结果正文。
const InterruptWithoutMessage = "__nexus_interrupt_without_message__"

// MessageContext 表示单轮消息处理上下文。
type MessageContext struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	AgentID        string
	WorkspacePath  string
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
		if durable := p.processAssistantAPIError(message); durable != nil {
			output.DurableMessages = append(output.DurableMessages, *durable)
			output.ResultSubtype = "error"
			output.TerminalStatus = "error"
			return output
		}
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
	case sdkprotocol.MessageTypeTaskStarted:
		if messageValue := p.processTaskStartedMessage(message); messageValue != nil {
			output.DurableMessages = append(output.DurableMessages, *messageValue)
		}
	case sdkprotocol.MessageTypeTaskProgress:
		if messageValue := p.processTaskProgressMessage(message); messageValue != nil {
			output.DurableMessages = append(output.DurableMessages, *messageValue)
		}
	case sdkprotocol.MessageTypeToolProgress:
		if messageValue := p.processToolProgressMessage(message); messageValue != nil {
			output.DurableMessages = append(output.DurableMessages, *messageValue)
		}
	case sdkprotocol.MessageTypeTaskNotification:
		if messageValue := p.processTaskNotificationMessage(message); messageValue != nil {
			output.DurableMessages = append(output.DurableMessages, *messageValue)
		}
	case sdkprotocol.MessageTypeUser:
		if durable := p.processToolResultMessage(message); durable != nil {
			output.DurableMessages = append(output.DurableMessages, *durable)
			output.AssistantCompleted = true
		}
	case sdkprotocol.MessageTypeStreamRequestStart,
		sdkprotocol.MessageTypeToolUseSummary,
		sdkprotocol.MessageTypeRateLimitEvent,
		sdkprotocol.MessageTypePromptSuggestion,
		sdkprotocol.MessageTypeAuthStatus:
		return output
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
		streamPayload := p.buildStreamPayload("message_start")
		streamPayload.Data["message"] = map[string]any{
			"model": emptyToNil(p.segment.Model()),
		}
		streamPayload.Data["usage"] = p.segment.Usage()
		output.StreamEvents = append(output.StreamEvents, streamPayload)
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
		streamPayload := p.buildStreamPayload("message_delta")
		streamPayload.Data["message"] = map[string]any{
			"model":       emptyToNil(p.segment.Model()),
			"stop_reason": emptyToNil(p.segment.StopReason()),
		}
		streamPayload.Data["usage"] = p.segment.Usage()
		output.StreamEvents = append(output.StreamEvents, streamPayload)
		if p.segment.HasContent() && strings.TrimSpace(p.segment.StopReason()) != "" {
			p.streamTerminalObserved = true
			if durable := p.buildAssistantDurableMessage(true, true, ""); durable != nil {
				output.DurableMessages = append(output.DurableMessages, *durable)
				output.AssistantCompleted = true
			}
		}
	case "message_stop":
		streamPayload := p.buildStreamPayload("message_stop")
		streamPayload.Data["message"] = map[string]any{
			"model":       emptyToNil(p.segment.Model()),
			"stop_reason": emptyToNil(p.segment.StopReason()),
		}
		streamPayload.Data["usage"] = p.segment.Usage()
		output.StreamEvents = append(output.StreamEvents, streamPayload)
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

func (p *Processor) buildBlockStreamPayload(streamType string, index int, block map[string]any) StreamPayload {
	payload := p.buildStreamPayload(streamType)
	payload.Data["index"] = index
	payload.Data["content_block"] = cloneMap(block)
	return payload
}

func (p *Processor) buildStreamPayload(streamType string) StreamPayload {
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
			"timestamp":       time.Now().UnixMilli(),
		},
	}
}

func (p *Processor) registerSessionID(message sdkprotocol.ReceivedMessage) (string, error) {
	currentSessionID := strings.TrimSpace(p.sessionID)
	candidates := []string{strings.TrimSpace(message.SessionID)}
	if message.Type == sdkprotocol.MessageTypeSystem && message.System != nil {
		candidates = append(candidates, normalizeString(message.System.Data["session_id"]))
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

func baseMessageEnvelope(ctx MessageContext, sessionID string, messageID string, role string) map[string]any {
	sessionID = strings.TrimSpace(sessionID)
	parentID := strings.TrimSpace(ctx.ParentID)
	roomID := strings.TrimSpace(ctx.RoomID)
	conversationID := strings.TrimSpace(ctx.ConversationID)
	payload := map[string]any{
		"message_id":  strings.TrimSpace(messageID),
		"session_key": ctx.SessionKey,
		"agent_id":    ctx.AgentID,
		"round_id":    ctx.RoundID,
		"role":        role,
		"timestamp":   time.Now().UnixMilli(),
	}
	if sessionID != "" {
		payload["session_id"] = sessionID
	}
	if parentID != "" && role != "user" {
		payload["parent_id"] = parentID
	}
	if roomID != "" {
		payload["room_id"] = roomID
	}
	if conversationID != "" {
		payload["conversation_id"] = conversationID
	}
	return payload
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
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		payload["parent_id"] = parentID
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
	return normalizeString(previous["message_id"]) == normalizeString(current["message_id"]) &&
		normalizeString(previous["parent_id"]) == normalizeString(current["parent_id"]) &&
		normalizeString(previous["model"]) == normalizeString(current["model"]) &&
		normalizeString(previous["stop_reason"]) == normalizeString(current["stop_reason"]) &&
		normalizeString(previous["session_id"]) == normalizeString(current["session_id"]) &&
		normalizeString(previous["round_id"]) == normalizeString(current["round_id"]) &&
		boolValue(previous["is_complete"]) == boolValue(current["is_complete"]) &&
		reflect.DeepEqual(previous["content"], current["content"])
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
