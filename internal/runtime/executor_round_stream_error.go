package runtime

import (
	"fmt"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// RoundStreamClosedError 携带 SDK 流提前关闭时的定位信息。
type RoundStreamClosedError struct {
	MessagesSeen       int
	LastMessageType    string
	LastMessageSummary string
	LastSessionID      string
	LastMessageID      string
	ReadError          string
	WaitError          string
	LastStreamStop     RoundStreamStopDiagnostics
}

func (e *RoundStreamClosedError) Error() string {
	if e == nil {
		return ErrRoundStreamClosedBeforeTerminal.Error()
	}
	detail := fmt.Sprintf(
		"%s: messages_seen=%d last_type=%s last_summary=%q last_session_id=%s last_message_id=%s",
		ErrRoundStreamClosedBeforeTerminal,
		e.MessagesSeen,
		e.LastMessageType,
		e.LastMessageSummary,
		e.LastSessionID,
		e.LastMessageID,
	)
	if strings.TrimSpace(e.WaitError) != "" {
		detail += " wait_error=" + strings.TrimSpace(e.WaitError)
	}
	if strings.TrimSpace(e.ReadError) != "" {
		detail += " read_error=" + strings.TrimSpace(e.ReadError)
	}
	detail = appendRoundStreamStopErrorDetail(detail, e.LastStreamStop)
	return detail
}

func (e *RoundStreamClosedError) Unwrap() error {
	return ErrRoundStreamClosedBeforeTerminal
}

// RoundStreamIdleTimeoutError 携带 SDK 流空闲超时时的定位信息。
type RoundStreamIdleTimeoutError struct {
	IdleTimeout        time.Duration
	MessagesSeen       int
	LastMessageType    string
	LastMessageSummary string
	LastSessionID      string
	LastMessageID      string
	LastStreamStop     RoundStreamStopDiagnostics
}

func (e *RoundStreamIdleTimeoutError) Error() string {
	if e == nil {
		return ErrRoundStreamIdleTimeout.Error()
	}
	detail := fmt.Sprintf(
		"%s after %s: messages_seen=%d last_type=%s last_summary=%q last_session_id=%s last_message_id=%s",
		ErrRoundStreamIdleTimeout,
		e.IdleTimeout,
		e.MessagesSeen,
		e.LastMessageType,
		e.LastMessageSummary,
		e.LastSessionID,
		e.LastMessageID,
	)
	return appendRoundStreamStopErrorDetail(detail, e.LastStreamStop)
}

func (e *RoundStreamIdleTimeoutError) Unwrap() error {
	return ErrRoundStreamIdleTimeout
}

func appendRoundStreamStopErrorDetail(detail string, diagnostics RoundStreamStopDiagnostics) string {
	if !diagnostics.Observed {
		return detail
	}
	detail += fmt.Sprintf(
		" last_stream_stop_summary=%q last_stream_stop_reason=%s messages_after_last_stream_stop=%d",
		diagnostics.Summary,
		diagnostics.StopReason,
		diagnostics.MessagesAfter,
	)
	if diagnostics.MessagesAfter > 0 {
		detail += fmt.Sprintf(
			" conversation_after_last_stream_stop=%d progress_after_last_stream_stop=%d passive_after_last_stream_stop=%d unknown_after_last_stream_stop=%d",
			diagnostics.ConversationMessagesAfter,
			diagnostics.ProgressMessagesAfter,
			diagnostics.PassiveMessagesAfter,
			diagnostics.UnknownMessagesAfter,
		)
	}
	if diagnostics.Age > 0 {
		detail += " last_stream_stop_age=" + diagnostics.Age.String()
	}
	return detail
}

func buildRoundStreamIdleTimeoutError(
	idleTimeout time.Duration,
	messagesSeen int,
	lastMessage sdkprotocol.ReceivedMessage,
	lastStreamStop RoundStreamStopDiagnostics,
) error {
	return &RoundStreamIdleTimeoutError{
		IdleTimeout:        idleTimeout,
		MessagesSeen:       messagesSeen,
		LastMessageType:    strings.TrimSpace(string(lastMessage.Type)),
		LastMessageSummary: strings.TrimSpace(BuildSDKMessageLogSummary(lastMessage)),
		LastSessionID:      strings.TrimSpace(lastMessage.SessionID),
		LastMessageID:      strings.TrimSpace(receivedMessageID(lastMessage)),
		LastStreamStop:     lastStreamStop,
	}
}

type clientWaiter interface {
	Wait() error
}

type clientStreamErrorer interface {
	StreamError() error
}

func buildRoundStreamClosedError(
	client Client,
	messagesSeen int,
	lastMessage sdkprotocol.ReceivedMessage,
	lastStreamStop RoundStreamStopDiagnostics,
) error {
	result := &RoundStreamClosedError{
		MessagesSeen:       messagesSeen,
		LastMessageType:    strings.TrimSpace(string(lastMessage.Type)),
		LastMessageSummary: strings.TrimSpace(BuildSDKMessageLogSummary(lastMessage)),
		LastSessionID:      strings.TrimSpace(lastMessage.SessionID),
		LastMessageID:      strings.TrimSpace(receivedMessageID(lastMessage)),
		LastStreamStop:     lastStreamStop,
	}
	if streamErrorer, ok := client.(clientStreamErrorer); ok {
		if err := streamErrorer.StreamError(); err != nil {
			result.ReadError = err.Error()
		}
	}
	if waiter, ok := client.(clientWaiter); ok {
		if err := waiter.Wait(); err != nil {
			result.WaitError = err.Error()
		}
	}
	return result
}
