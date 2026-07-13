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
	AgentRoundID   string
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
	handler := messageHandlers[message.Type]
	if handler == nil {
		return output
	}
	return handler(p, message, output)
}

type messageHandler func(*Processor, sdkprotocol.ReceivedMessage, Output) Output

var messageHandlers = map[sdkprotocol.MessageType]messageHandler{
	sdkprotocol.MessageTypeStreamEvent:      handleStreamEvent,
	sdkprotocol.MessageTypeAssistant:        handleAssistant,
	sdkprotocol.MessageTypeSystem:           handleSystem,
	sdkprotocol.MessageTypeResult:           handleResult,
	sdkprotocol.MessageTypeTaskStarted:      handleTaskStarted,
	sdkprotocol.MessageTypeTaskProgress:     handleTaskProgress,
	sdkprotocol.MessageTypeToolProgress:     handleToolProgress,
	sdkprotocol.MessageTypeTaskNotification: handleTaskNotification,
	sdkprotocol.MessageTypeUser:             handleUser,
}

type streamEventHandler func(*Processor, map[string]any, Output) Output

var streamEventHandlers = map[string]streamEventHandler{
	"message_start":       handleMessageStartStream,
	"content_block_start": handleContentBlockStartStream,
	"content_block_delta": handleContentBlockDeltaStream,
	"message_delta":       handleMessageDeltaStream,
	"message_stop":        handleMessageStopStream,
}

func handleStreamEvent(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	return p.processStreamEvent(message, output)
}

func handleAssistant(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	if durable := p.processAssistantAPIError(message); durable != nil {
		output.DurableMessages = append(output.DurableMessages, *durable)
		output.ResultSubtype = "error"
		output.TerminalStatus = "error"
		return output
	}
	durable := p.processAssistantMessage(message)
	if durable == nil {
		return output
	}
	output.DurableMessages = append(output.DurableMessages, *durable)
	output.AssistantCompleted = (*durable)["is_complete"] == true
	return output
}

func handleSystem(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	durable, ephemeral := p.processSystemMessage(message)
	output.DurableMessages = append(output.DurableMessages, durable...)
	output.EphemeralMessages = append(output.EphemeralMessages, ephemeral...)
	return output
}

func handleResult(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	subtype := normalizeResultSubtype(message.Result)
	output.DurableMessages = append(output.DurableMessages, p.buildResultMessage(message, subtype))
	output.ResultSubtype = subtype
	output.TerminalStatus = statusFromResultSubtype(subtype)
	return output
}

func handleTaskStarted(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	return appendDurableMessage(output, p.processTaskStartedMessage(message))
}

func handleTaskProgress(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	return appendDurableMessage(output, p.processTaskProgressMessage(message))
}

func handleToolProgress(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	return appendDurableMessage(output, p.processToolProgressMessage(message))
}

func handleTaskNotification(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	return appendDurableMessage(output, p.processTaskNotificationMessage(message))
}

func handleUser(p *Processor, message sdkprotocol.ReceivedMessage, output Output) Output {
	durable := p.processToolResultMessage(message)
	if durable == nil {
		return output
	}
	output.DurableMessages = append(output.DurableMessages, *durable)
	output.AssistantCompleted = true
	return output
}

func appendDurableMessage(output Output, message *protocol.Message) Output {
	if message != nil {
		output.DurableMessages = append(output.DurableMessages, *message)
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
	handler := streamEventHandlers[eventType]
	if handler == nil {
		return output
	}
	return handler(p, payload, output)
}

func handleMessageStartStream(p *Processor, payload map[string]any, output Output) Output {
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
	streamPayload.Data["message"] = map[string]any{"model": emptyToNil(p.segment.Model())}
	streamPayload.Data["usage"] = p.segment.Usage()
	output.StreamEvents = append(output.StreamEvents, streamPayload)
	return output
}

func handleContentBlockStartStream(p *Processor, payload map[string]any, output Output) Output {
	block := normalizeContentBlock(payload["content_block"])
	if len(block) == 0 {
		return output
	}
	logicalIndex := p.segment.ApplyBlock(normalizeInt(payload["index"]), block)
	if normalizeString(block["type"]) != "tool_use" {
		output.StreamEvents = append(output.StreamEvents, p.buildBlockStreamPayload("content_block_start", logicalIndex, block))
	}
	return output
}

func handleContentBlockDeltaStream(p *Processor, payload map[string]any, output Output) Output {
	delta, _ := payload["delta"].(map[string]any)
	logicalIndex, applied := p.segment.ApplyDelta(normalizeInt(payload["index"]), delta)
	if !applied {
		return output
	}
	block := p.segment.CurrentBlock(logicalIndex)
	if normalizeString(block["type"]) != "tool_use" {
		output.StreamEvents = append(output.StreamEvents, p.buildBlockStreamPayload("content_block_delta", logicalIndex, block))
	}
	return output
}

func handleMessageDeltaStream(p *Processor, payload map[string]any, output Output) Output {
	delta, _ := payload["delta"].(map[string]any)
	usage, _ := payload["usage"].(map[string]any)
	p.segment.UpdateMeta("", usage, normalizeString(delta["stop_reason"]))
	output.StreamEvents = append(output.StreamEvents, p.buildMessageMetaStreamPayload("message_delta"))
	if !p.segment.HasContent() || strings.TrimSpace(p.segment.StopReason()) == "" {
		return output
	}
	p.streamTerminalObserved = true
	if durable := p.buildAssistantDurableMessage(true, true, ""); durable != nil {
		output.DurableMessages = append(output.DurableMessages, *durable)
		output.AssistantCompleted = true
	}
	return output
}

func handleMessageStopStream(p *Processor, _ map[string]any, output Output) Output {
	output.StreamEvents = append(output.StreamEvents, p.buildMessageMetaStreamPayload("message_stop"))
	return output
}

func (p *Processor) buildMessageMetaStreamPayload(eventType string) StreamPayload {
	payload := p.buildStreamPayload(eventType)
	payload.Data["message"] = map[string]any{
		"model":       emptyToNil(p.segment.Model()),
		"stop_reason": emptyToNil(p.segment.StopReason()),
	}
	payload.Data["usage"] = p.segment.Usage()
	return payload
}

func (p *Processor) processAssistantMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	// 同一轮内 assistant 会分多段（不同 message id）。直播时靠 stream 的
	// message_start 轮转段；历史投影没有 stream 事件，必须在快照 id 变化时
	// 主动轮转，否则整轮坍缩进第一个 id、内容相互覆盖。
	incomingID := strings.TrimSpace(message.Assistant.Message.ID)
	if !p.segment.IsStarted() ||
		(incomingID != "" && incomingID != p.segment.MessageID()) {
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
			"agent_round_id":  emptyToNil(p.ctx.AgentRoundID),
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
	if agentRoundID := strings.TrimSpace(ctx.AgentRoundID); agentRoundID != "" {
		payload["agent_round_id"] = agentRoundID
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
