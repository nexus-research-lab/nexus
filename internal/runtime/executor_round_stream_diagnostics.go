package runtime

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// RoundStreamStopDiagnostics 表示最近一次 provider message_stop 的定位信息。
type RoundStreamStopDiagnostics struct {
	Observed      bool
	MessageIndex  int
	MessagesAfter int
	Age           time.Duration
	Summary       string
	StopReason    string
	SessionID     string
	MessageID     string
	Model         string
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
	if message.Type != sdkprotocol.MessageTypeStreamEvent || message.Stream == nil {
		return
	}
	payload := streamEventPayload(message)
	eventType := strings.TrimSpace(rawString(payload["type"]))
	switch eventType {
	case "message_start":
		startMessage := rawMap(payload["message"])
		d.currentMessageID = strings.TrimSpace(rawString(startMessage["id"]))
		d.currentModel = strings.TrimSpace(rawString(startMessage["model"]))
		d.currentStopReason = ""
	case "message_delta":
		delta := rawMap(payload["delta"])
		if stopReason := strings.TrimSpace(rawString(delta["stop_reason"])); stopReason != "" {
			d.currentStopReason = stopReason
		}
	case "message_stop":
		d.lastStreamStopAt = observedAt
		d.lastStreamStop = RoundStreamStopDiagnostics{
			Observed:     true,
			MessageIndex: messageIndex,
			Summary:      strings.TrimSpace(BuildSDKMessageLogSummary(message)),
			StopReason:   firstNonEmpty(strings.TrimSpace(rawString(payload["stop_reason"])), d.currentStopReason),
			SessionID:    strings.TrimSpace(message.SessionID),
			MessageID:    firstNonEmpty(strings.TrimSpace(receivedMessageID(message)), d.currentMessageID),
			Model:        firstNonEmpty(strings.TrimSpace(rawString(payload["model"])), d.currentModel),
		}
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

func streamEventPayload(message sdkprotocol.ReceivedMessage) map[string]any {
	if message.Stream == nil {
		return nil
	}
	if payload := rawMap(message.Stream.Event); len(payload) > 0 {
		return payload
	}
	return rawMap(message.Stream.Data)
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
