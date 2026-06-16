package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	// ErrRoundInterrupted 表示 round 在收到终态前被中断。
	ErrRoundInterrupted = errors.New("round interrupted")
	// ErrRoundStreamClosedBeforeTerminal 表示 SDK 在产出终态前提前结束消息流。
	ErrRoundStreamClosedBeforeTerminal = errors.New("round stream closed before terminal")
	// ErrRoundStreamIdleTimeout 表示 SDK 消息流长时间无新事件且未结束。
	ErrRoundStreamIdleTimeout = errors.New("round stream idle timeout")
)

const (
	defaultAssistantTerminalGrace = 1500 * time.Millisecond
	defaultRoundIdleTimeout       = 5 * time.Minute
	roundIdleAbortTimeout         = 5 * time.Second
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
	Content                any
	ContextualInputs       []ContextualInputBlock
	InputOptions           sdkprotocol.OutboundMessageOptions
	Client                 Client
	Mapper                 RoundMapper
	IdleTimeout            time.Duration
	InterruptReason        func() string
	AssistantTerminalGrace time.Duration
	SyncSessionID          func(string) error
	AfterQuery             func() error
	HandleDurableMessage   func(protocol.Message) error
	EmitEvent              func(protocol.EventMessage) error
	ObserveIncomingMessage func(sdkprotocol.ReceivedMessage)
}

// RoundExecutionResult 表示 round 执行的终态结果。
type RoundExecutionResult struct {
	TerminalStatus       string
	ResultSubtype        string
	ErrorMessage         string
	TerminalCategory     sdkprotocol.TerminalCategory
	Usage                sdkprotocol.TokenUsage
	ElapsedTimeSeconds   int64
	CompletedByAssistant bool
	UsageLimitReached    bool
	UsageLimitReason     string
}

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
	if diagnostics.Age > 0 {
		detail += " last_stream_stop_age=" + diagnostics.Age.String()
	}
	return detail
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
	startedAt := time.Now()

	queryContent, err := prepareRoundContentWithContext(ctx, request.Client, roundQueryContent(request), request.ContextualInputs)
	if err != nil {
		return RoundExecutionResult{}, err
	}
	if err := QueryClientContentWithOptions(ctx, request.Client, queryContent, request.InputOptions); err != nil {
		if isRoundAbortError(ctx, err) {
			return RoundExecutionResult{}, ErrRoundInterrupted
		}
		return RoundExecutionResult{}, err
	}
	if request.AfterQuery != nil {
		if err := request.AfterQuery(); err != nil {
			return RoundExecutionResult{}, err
		}
	}

	messageCh := request.Client.ReceiveMessages(ctx)
	messagesSeen := 0
	lastMessage := sdkprotocol.ReceivedMessage{}
	streamDiagnostics := roundStreamDiagnostics{}
	idleTimeout := normalizeRoundIdleTimeout(request.IdleTimeout)
	var idleTimer *time.Timer
	var idleTimeoutCh <-chan time.Time
	if idleTimeout > 0 {
		idleTimer = time.NewTimer(idleTimeout)
		defer idleTimer.Stop()
		idleTimeoutCh = idleTimer.C
	}
	var assistantTerminalResult *RoundExecutionResult
	var assistantTerminalTimer <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return RoundExecutionResult{}, ErrRoundInterrupted
		case <-assistantTerminalTimer:
			return roundResultWithElapsed(*assistantTerminalResult, startedAt), nil
		case <-idleTimeoutCh:
			if shouldTreatAsInterrupted(ctx, request.InterruptReason) {
				return RoundExecutionResult{}, ErrRoundInterrupted
			}
			abortRoundClientAfterIdleTimeout(request.Client)
			return RoundExecutionResult{}, buildRoundStreamIdleTimeoutError(
				idleTimeout,
				messagesSeen,
				lastMessage,
				streamDiagnostics.Snapshot(messagesSeen, time.Now()),
			)
		case incoming, ok := <-messageCh:
			if !ok {
				if shouldTreatAsInterrupted(ctx, request.InterruptReason) {
					return RoundExecutionResult{}, ErrRoundInterrupted
				}
				if assistantTerminalResult != nil {
					return roundResultWithElapsed(*assistantTerminalResult, startedAt), nil
				}
				return RoundExecutionResult{}, buildRoundStreamClosedError(
					request.Client,
					messagesSeen,
					lastMessage,
					streamDiagnostics.Snapshot(messagesSeen, time.Now()),
				)
			}
			messagesSeen++
			lastMessage = incoming
			streamDiagnostics.Observe(incoming, messagesSeen, time.Now())
			resetRoundIdleTimer(idleTimer, idleTimeout)
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
				return terminalRoundResult(mapResult, assistantTerminalResult, incoming.Result, startedAt), nil
			}
			if assistantResult, ok := terminalAssistantResult(mapResult); ok {
				assistantTerminalResult = &assistantResult
				if assistantTerminalTimer == nil {
					assistantTerminalTimer = time.After(normalizeAssistantTerminalGrace(request.AssistantTerminalGrace))
				}
			}
		}
	}
}

func terminalRoundResult(
	mapResult RoundMapResult,
	assistantTerminalResult *RoundExecutionResult,
	resultMessage *sdkprotocol.ResultMessage,
	startedAt time.Time,
) RoundExecutionResult {
	result := RoundExecutionResult{
		TerminalStatus:   strings.TrimSpace(mapResult.TerminalStatus),
		ResultSubtype:    strings.TrimSpace(mapResult.ResultSubtype),
		ErrorMessage:     terminalErrorMessage(mapResult),
		TerminalCategory: sdkprotocol.TerminalCategoryUnknown,
	}
	if resultMessage != nil {
		result.Usage, _ = resultMessage.TokenUsage()
		result.TerminalCategory = resultMessage.TerminalCategory()
		result.UsageLimitReached, result.UsageLimitReason = ResultUsageLimitReached(resultMessage)
	}
	if !isSuccessfulRoundResult(result) {
		return roundResultWithElapsed(result, startedAt)
	}
	if assistantResult, ok := terminalAssistantResult(mapResult); ok && assistantResult.CompletedByAssistant {
		result.CompletedByAssistant = true
		return roundResultWithElapsed(result, startedAt)
	}
	if hasSuccessfulResultMessage(mapResult) {
		result.CompletedByAssistant = true
		return roundResultWithElapsed(result, startedAt)
	}
	if assistantTerminalResult != nil && assistantTerminalResult.CompletedByAssistant {
		result.CompletedByAssistant = true
	}
	return roundResultWithElapsed(result, startedAt)
}

func isSuccessfulRoundResult(result RoundExecutionResult) bool {
	return result.TerminalStatus == "finished" &&
		(result.ResultSubtype == "" || result.ResultSubtype == "success")
}

func hasSuccessfulResultMessage(mapResult RoundMapResult) bool {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "result" {
			continue
		}
		if messageString(messageValue["subtype"]) == "error" || messageValue["is_error"] == true {
			continue
		}
		return true
	}
	return false
}

func terminalErrorMessage(mapResult RoundMapResult) string {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "result" {
			continue
		}
		if messageString(messageValue["subtype"]) != "error" && messageValue["is_error"] != true {
			continue
		}
		if resultText := strings.TrimSpace(messageString(messageValue["result"])); resultText != "" {
			return resultText
		}
		if terminalReason := strings.TrimSpace(messageString(messageValue["terminal_reason"])); terminalReason != "" {
			return terminalReason
		}
	}
	if mapResult.ResultSubtype == "error" || mapResult.TerminalStatus == "error" {
		return "Runtime request failed"
	}
	return ""
}

func roundQueryContent(request RoundExecutionRequest) any {
	if request.Content != nil {
		return request.Content
	}
	return request.Query
}

func roundResultWithElapsed(result RoundExecutionResult, startedAt time.Time) RoundExecutionResult {
	if result.ElapsedTimeSeconds > 0 || startedAt.IsZero() {
		return result
	}
	result.ElapsedTimeSeconds = int64(time.Since(startedAt).Seconds())
	return result
}

func normalizeAssistantTerminalGrace(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return defaultAssistantTerminalGrace
}

func normalizeRoundIdleTimeout(timeout time.Duration) time.Duration {
	if timeout < 0 {
		return 0
	}
	if timeout == 0 {
		return defaultRoundIdleTimeout
	}
	return timeout
}

func isRoundAbortError(ctx context.Context, err error) bool {
	return ctx.Err() != nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, agentclient.ErrAborted)
}

func resetRoundIdleTimer(timer *time.Timer, timeout time.Duration) {
	if timer == nil || timeout <= 0 {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func abortRoundClientAfterIdleTimeout(client Client) {
	if client == nil {
		return
	}
	interruptCtx, interruptCancel := context.WithTimeout(context.Background(), roundIdleAbortTimeout)
	_ = client.Interrupt(interruptCtx)
	interruptCancel()

	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), roundIdleAbortTimeout)
	_ = client.Disconnect(disconnectCtx)
	disconnectCancel()
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

func terminalAssistantResult(mapResult RoundMapResult) (RoundExecutionResult, bool) {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "assistant" {
			continue
		}
		if messageValue["is_complete"] != true {
			continue
		}
		if !isTerminalAssistantStopReason(messageString(messageValue["stop_reason"])) {
			continue
		}
		return RoundExecutionResult{
			TerminalStatus:       "finished",
			ResultSubtype:        "success",
			CompletedByAssistant: true,
		}, true
	}
	return RoundExecutionResult{}, false
}

func isTerminalAssistantStopReason(stopReason string) bool {
	switch strings.TrimSpace(stopReason) {
	case "end_turn", "stop_sequence", "max_tokens":
		return true
	default:
		return false
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
