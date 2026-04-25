package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	// ErrRoundInterrupted 表示 round 在收到终态前被中断。
	ErrRoundInterrupted = errors.New("round interrupted")
	// ErrRoundStreamClosedBeforeTerminal 表示 SDK 在产出终态前提前结束消息流。
	ErrRoundStreamClosedBeforeTerminal = errors.New("round stream closed before terminal")
)

// RoundStreamClosedError 携带 SDK 流提前关闭时的定位信息。
type RoundStreamClosedError struct {
	MessagesSeen    int
	LastMessageType string
	LastSessionID   string
	LastMessageID   string
	WaitError       string
}

func (e *RoundStreamClosedError) Error() string {
	if e == nil {
		return ErrRoundStreamClosedBeforeTerminal.Error()
	}
	detail := fmt.Sprintf(
		"%s: messages_seen=%d last_type=%s last_session_id=%s last_message_id=%s",
		ErrRoundStreamClosedBeforeTerminal,
		e.MessagesSeen,
		e.LastMessageType,
		e.LastSessionID,
		e.LastMessageID,
	)
	if strings.TrimSpace(e.WaitError) != "" {
		detail += " wait_error=" + strings.TrimSpace(e.WaitError)
	}
	return detail
}

func (e *RoundStreamClosedError) Unwrap() error {
	return ErrRoundStreamClosedBeforeTerminal
}

// RoundMapResult 表示单条 SDK 消息映射后的统一结果。
type RoundMapResult struct {
	Events          []protocol.EventMessage
	DurableMessages []protocol.Message
	TerminalStatus  string
	ResultSubtype   string
}

// RoundMapper 负责把 SDK 消息映射成统一事件与 durable 消息。
type RoundMapper interface {
	Map(sdkprotocol.ReceivedMessage, ...string) (RoundMapResult, error)
	SessionID() string
}

// RoundExecutionRequest 表示执行单轮查询所需的回调与依赖。
type RoundExecutionRequest struct {
	Query                  string
	Client                 Client
	Mapper                 RoundMapper
	InterruptReason        func() string
	SyncSessionID          func(string) error
	HandleDurableMessage   func(protocol.Message) error
	EmitEvent              func(protocol.EventMessage) error
	ObserveIncomingMessage func(sdkprotocol.ReceivedMessage)
}

// RoundExecutionResult 表示 round 执行的终态结果。
type RoundExecutionResult struct {
	TerminalStatus string
	ResultSubtype  string
}

// ExecuteRound 统一执行 query -> receive -> map -> persist -> emit 的主链路。
func ExecuteRound(
	ctx context.Context,
	request RoundExecutionRequest,
) (RoundExecutionResult, error) {
	if request.Client == nil {
		return RoundExecutionResult{}, errors.New("round client is required")
	}
	if request.Mapper == nil {
		return RoundExecutionResult{}, errors.New("round mapper is required")
	}

	if err := request.Client.Query(ctx, request.Query); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return RoundExecutionResult{}, ErrRoundInterrupted
		}
		return RoundExecutionResult{}, err
	}

	messageCh := request.Client.ReceiveMessages(ctx)
	messagesSeen := 0
	lastMessage := sdkprotocol.ReceivedMessage{}
	for {
		select {
		case <-ctx.Done():
			return RoundExecutionResult{}, ErrRoundInterrupted
		case incoming, ok := <-messageCh:
			if !ok {
				if shouldTreatAsInterrupted(ctx, request.InterruptReason) {
					return RoundExecutionResult{}, ErrRoundInterrupted
				}
				return RoundExecutionResult{}, buildRoundStreamClosedError(request.Client, messagesSeen, lastMessage)
			}
			messagesSeen++
			lastMessage = incoming
			if request.ObserveIncomingMessage != nil {
				request.ObserveIncomingMessage(incoming)
			}

			mapResult, err := request.Mapper.Map(incoming, resolveInterruptReason(request.InterruptReason))
			if err != nil {
				return RoundExecutionResult{}, err
			}

			sessionID := resolveSessionID(
				request.Mapper.SessionID(),
				incoming.SessionID,
				request.Client.SessionID(),
			)
			if request.SyncSessionID != nil && sessionID != "" {
				if err := request.SyncSessionID(sessionID); err != nil {
					return RoundExecutionResult{}, err
				}
			}

			for _, messageValue := range mapResult.DurableMessages {
				if messageValue == nil {
					continue
				}
				if sessionID != "" && strings.TrimSpace(messageString(messageValue["session_id"])) == "" {
					messageValue["session_id"] = sessionID
				}
				if request.HandleDurableMessage != nil {
					if err := request.HandleDurableMessage(messageValue); err != nil {
						return RoundExecutionResult{}, err
					}
				}
			}

			for _, event := range mapResult.Events {
				if request.EmitEvent != nil {
					if err := request.EmitEvent(event); err != nil {
						return RoundExecutionResult{}, err
					}
				}
			}

			if strings.TrimSpace(mapResult.TerminalStatus) != "" {
				return RoundExecutionResult{
					TerminalStatus: strings.TrimSpace(mapResult.TerminalStatus),
					ResultSubtype:  strings.TrimSpace(mapResult.ResultSubtype),
				}, nil
			}
		}
	}
}

type clientWaiter interface {
	Wait() error
}

func buildRoundStreamClosedError(client Client, messagesSeen int, lastMessage sdkprotocol.ReceivedMessage) error {
	result := &RoundStreamClosedError{
		MessagesSeen:    messagesSeen,
		LastMessageType: strings.TrimSpace(string(lastMessage.Type)),
		LastSessionID:   strings.TrimSpace(lastMessage.SessionID),
		LastMessageID:   strings.TrimSpace(receivedMessageID(lastMessage)),
	}
	if waiter, ok := client.(clientWaiter); ok {
		if err := waiter.Wait(); err != nil {
			result.WaitError = err.Error()
		}
	}
	return result
}

func receivedMessageID(message sdkprotocol.ReceivedMessage) string {
	if strings.TrimSpace(message.UUID) != "" {
		return strings.TrimSpace(message.UUID)
	}
	if message.Assistant != nil && strings.TrimSpace(message.Assistant.Message.ID) != "" {
		return strings.TrimSpace(message.Assistant.Message.ID)
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

func shouldTreatAsInterrupted(ctx context.Context, interruptReason func() string) bool {
	return ctx.Err() != nil || strings.TrimSpace(resolveInterruptReason(interruptReason)) != ""
}

func resolveInterruptReason(interruptReason func() string) string {
	if interruptReason == nil {
		return ""
	}
	return strings.TrimSpace(interruptReason())
}

func resolveSessionID(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func messageString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}
