package exec

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
)

// RoundStreamStopDiagnostics 表示最近一次 provider message_stop 的定位信息。
type RoundStreamStopDiagnostics struct {
	Observed                  bool
	MessageIndex              int
	MessagesAfter             int
	ConversationMessagesAfter int
	ProgressMessagesAfter     int
	PassiveMessagesAfter      int
	UnknownMessagesAfter      int
	Age                       time.Duration
	Summary                   string
	StopReason                string
	SessionID                 string
	MessageID                 string
	Model                     string
}

// RoundStreamStopDiagnosticLogFields 返回 message_stop 诊断日志字段。
func RoundStreamStopDiagnosticLogFields(diagnostics RoundStreamStopDiagnostics) []any {
	if !diagnostics.Observed {
		return nil
	}
	fields := []any{
		"stream_last_stop_summary", diagnostics.Summary,
		"stream_last_stop_reason", diagnostics.StopReason,
		"stream_last_stop_session_id", diagnostics.SessionID,
		"stream_last_stop_message_id", diagnostics.MessageID,
		"stream_last_stop_model", diagnostics.Model,
		"stream_last_stop_message_index", diagnostics.MessageIndex,
		"stream_messages_after_last_stop", diagnostics.MessagesAfter,
	}
	if diagnostics.MessagesAfter > 0 {
		fields = append(
			fields,
			"stream_conversation_messages_after_last_stop", diagnostics.ConversationMessagesAfter,
			"stream_progress_messages_after_last_stop", diagnostics.ProgressMessagesAfter,
			"stream_passive_messages_after_last_stop", diagnostics.PassiveMessagesAfter,
			"stream_unknown_messages_after_last_stop", diagnostics.UnknownMessagesAfter,
		)
	}
	if diagnostics.Age > 0 {
		fields = append(fields, "stream_last_stop_age", diagnostics.Age.String())
	}
	return fields
}

type roundStreamDiagnostics struct {
	currentMessageID  string
	currentModel      string
	currentStopReason string
	lastStreamStop    RoundStreamStopDiagnostics
	lastStreamStopAt  time.Time
}

func (d *roundStreamDiagnostics) Observe(message sdkprotocol.ReceivedMessage, messageIndex int, observedAt time.Time) {
	d.observeAfterLastStop(message, messageIndex)
	if message.Type != sdkprotocol.MessageTypeStreamEvent || message.Stream == nil {
		return
	}
	payload := streamEventPayload(message)
	eventType := strings.TrimSpace(trace.RawString(payload["type"]))
	switch eventType {
	case "message_start":
		startMessage := trace.RawMap(payload["message"])
		d.currentMessageID = strings.TrimSpace(trace.RawString(startMessage["id"]))
		d.currentModel = strings.TrimSpace(trace.RawString(startMessage["model"]))
		d.currentStopReason = ""
	case "message_delta":
		delta := trace.RawMap(payload["delta"])
		if stopReason := strings.TrimSpace(trace.RawString(delta["stop_reason"])); stopReason != "" {
			d.currentStopReason = stopReason
		}
	case "message_stop":
		d.lastStreamStopAt = observedAt
		d.lastStreamStop = RoundStreamStopDiagnostics{
			Observed:     true,
			MessageIndex: messageIndex,
			Summary:      strings.TrimSpace(trace.BuildSDKMessageLogSummary(message)),
			StopReason:   trace.FirstNonEmpty(strings.TrimSpace(trace.RawString(payload["stop_reason"])), d.currentStopReason),
			SessionID:    strings.TrimSpace(message.SessionID),
			MessageID:    trace.FirstNonEmpty(strings.TrimSpace(receivedMessageID(message)), d.currentMessageID),
			Model:        trace.FirstNonEmpty(strings.TrimSpace(trace.RawString(payload["model"])), d.currentModel),
		}
	}
}

func (d *roundStreamDiagnostics) observeAfterLastStop(message sdkprotocol.ReceivedMessage, messageIndex int) {
	if !d.lastStreamStop.Observed || messageIndex <= d.lastStreamStop.MessageIndex {
		return
	}
	d.lastStreamStop.MessagesAfter++
	switch roundMessageDiagnosticClass(message) {
	case "conversation":
		d.lastStreamStop.ConversationMessagesAfter++
	case "progress":
		d.lastStreamStop.ProgressMessagesAfter++
	case "passive":
		d.lastStreamStop.PassiveMessagesAfter++
	default:
		d.lastStreamStop.UnknownMessagesAfter++
	}
}

func (d roundStreamDiagnostics) Snapshot(messagesSeen int, now time.Time) RoundStreamStopDiagnostics {
	result := d.lastStreamStop
	if !result.Observed {
		return result
	}
	if messagesSeen > result.MessageIndex {
		result.MessagesAfter = messagesSeen - result.MessageIndex
	}
	if !d.lastStreamStopAt.IsZero() && !now.Before(d.lastStreamStopAt) {
		result.Age = now.Sub(d.lastStreamStopAt)
	}
	return result
}

func roundMessageDiagnosticClass(message sdkprotocol.ReceivedMessage) string {
	switch message.Type {
	case sdkprotocol.MessageTypeAssistant,
		sdkprotocol.MessageTypeUser,
		sdkprotocol.MessageTypeResult,
		sdkprotocol.MessageTypeStreamEvent:
		return "conversation"
	case sdkprotocol.MessageTypeToolProgress,
		sdkprotocol.MessageTypeTaskStarted,
		sdkprotocol.MessageTypeTaskProgress,
		sdkprotocol.MessageTypeTaskNotification:
		return "progress"
	case sdkprotocol.MessageTypeStreamRequestStart,
		sdkprotocol.MessageTypeToolUseSummary,
		sdkprotocol.MessageTypeRateLimitEvent,
		sdkprotocol.MessageTypePromptSuggestion,
		sdkprotocol.MessageTypeAuthStatus:
		return "passive"
	default:
		return "unknown"
	}
}

func streamEventPayload(message sdkprotocol.ReceivedMessage) map[string]any {
	if message.Stream == nil {
		return nil
	}
	if payload := trace.RawMap(message.Stream.Event); len(payload) > 0 {
		return payload
	}
	return trace.RawMap(message.Stream.Data)
}

func receivedMessageID(message sdkprotocol.ReceivedMessage) string {
	if uuid := strings.TrimSpace(message.UUID); uuid != "" {
		return uuid
	}
	if message.Assistant != nil {
		if messageID := strings.TrimSpace(message.Assistant.Message.ID); messageID != "" {
			return messageID
		}
	}
	if message.Stream != nil {
		if payload, ok := message.Stream.Event.(map[string]any); ok {
			if messagePayload, ok := payload["message"].(map[string]any); ok {
				return strings.TrimSpace(messageString(messagePayload["id"]))
			}
		}
		if messagePayload, ok := message.Stream.Data["message"].(map[string]any); ok {
			return strings.TrimSpace(messageString(messagePayload["id"]))
		}
	}
	return ""
}
