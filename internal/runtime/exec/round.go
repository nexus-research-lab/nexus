// INPUT: runtime query、SDK 消息流、人工交互暂停信号与 map/persist/emit 回调。
// OUTPUT: 排除人工等待时间的单轮终态；本地后处理失败时仍携带 provider terminal usage。
// POS: runtime exec 包的 query → receive → map → persist → emit 主状态机。
package exec

import (
	"context"
	"errors"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// ExecuteRound 统一执行 query -> receive -> map -> persist -> emit 的主链路。
func ExecuteRound(
	ctx context.Context,
	request RoundExecutionRequest,
) (RoundExecutionResult, error) {
	execution, err := newRoundExecution(ctx, request)
	if err != nil {
		return RoundExecutionResult{}, err
	}
	if err = execution.query(); err != nil {
		return RoundExecutionResult{}, err
	}
	return execution.receive()
}

// roundExecution 保存接收循环的可变状态，让主入口只表达 query 与 receive 两个业务阶段。
type roundExecution struct {
	ctx                     context.Context
	request                 RoundExecutionRequest
	startedAt               time.Time
	messageCh               <-chan sdkprotocol.ReceivedMessage
	messagesSeen            int
	lastMessage             sdkprotocol.ReceivedMessage
	streamDiagnostics       roundStreamDiagnostics
	idleTimeout             time.Duration
	idleTimer               *time.Timer
	idleTimeoutCh           <-chan time.Time
	idlePaused              bool
	idlePauseChanged        <-chan struct{}
	assistantTerminalResult *RoundExecutionResult
	assistantTerminalTimer  <-chan time.Time
}

type roundReceiveOutcome struct {
	result RoundExecutionResult
	done   bool
}

func newRoundExecution(ctx context.Context, request RoundExecutionRequest) (*roundExecution, error) {
	if request.Client == nil {
		return nil, errors.New("round client is required")
	}
	if request.Mapper == nil {
		return nil, errors.New("round mapper is required")
	}
	return &roundExecution{
		ctx:         ctx,
		request:     request,
		startedAt:   time.Now(),
		idleTimeout: normalizeRoundIdleTimeout(request.IdleTimeout),
	}, nil
}

func (e *roundExecution) query() error {
	content := roundQueryContent(e.request)
	var (
		queryContent any
		err          error
	)
	if e.request.AtomicInput {
		queryContent, err = runtimectx.PrepareAtomicRoundContent(
			e.ctx,
			e.request.Client,
			content,
		)
	} else {
		queryContent, err = runtimectx.PrepareRoundContentWithContext(
			e.ctx,
			e.request.Client,
			content,
			e.request.ContextualInputs,
		)
	}
	if err != nil {
		return err
	}
	if err = runtimectx.QueryClientContentWithOptions(e.ctx, e.request.Client, queryContent, e.request.InputOptions); err != nil {
		if e.ctx.Err() != nil || isRoundAbortError(err) {
			return ErrRoundInterrupted
		}
		return err
	}
	if e.request.AfterQuery == nil {
		return nil
	}
	return e.request.AfterQuery()
}

func (e *roundExecution) receive() (RoundExecutionResult, error) {
	e.startReceiving()
	defer e.stopIdleTimer()
	for {
		if e.interruptedDrainRequired() {
			return e.receiveInterruptedTerminal(nil)
		}
		select {
		case <-e.ctx.Done():
			if e.explicitInterruptRequested() {
				return e.receiveInterruptedTerminal(nil)
			}
			return RoundExecutionResult{}, ErrRoundInterrupted
		case <-e.assistantTerminalTimer:
			if e.explicitInterruptRequested() {
				return e.receiveInterruptedTerminal(nil)
			}
			return roundResultWithElapsed(*e.assistantTerminalResult, e.startedAt), nil
		case <-e.idleTimeoutCh:
			e.refreshIdlePauseState()
			if e.idlePaused {
				continue
			}
			if e.explicitInterruptRequested() {
				return e.receiveInterruptedTerminal(nil)
			}
			return e.handleIdleTimeout()
		case <-e.idlePauseChanged:
			e.refreshIdlePauseState()
		case incoming, ok := <-e.messageCh:
			if e.interruptedDrainRequired() {
				if !ok {
					disconnectUncleanRoundClient(e.request.Client)
					return RoundExecutionResult{}, ErrRoundInterrupted
				}
				return e.receiveInterruptedTerminal(&incoming)
			}
			if !ok {
				return e.handleStreamClosed()
			}
			outcome, err := e.handleIncoming(incoming)
			if err != nil {
				return outcome.result, err
			}
			if outcome.done {
				return outcome.result, nil
			}
		}
	}
}

// receiveInterruptedTerminal 在宿主强制取消 round 后继续占有当前消息流，直到
// runtime 给出本 turn 的 terminal result 边界。若直接释放共享流，迟到的 result
// 会被下一 round 当作自己的结果；超时则关闭 client，确保未收口的进程不再复用。
func (e *roundExecution) receiveInterruptedTerminal(first *sdkprotocol.ReceivedMessage) (RoundExecutionResult, error) {
	if result, done := e.consumeInterruptedMessage(first); done {
		return result, ErrRoundInterrupted
	}
	timer := time.NewTimer(normalizeInterruptedTerminalTimeout(e.request.InterruptedTerminalTimeout))
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			disconnectUncleanRoundClient(e.request.Client)
			return RoundExecutionResult{}, ErrRoundInterrupted
		case incoming, ok := <-e.messageCh:
			if !ok {
				disconnectUncleanRoundClient(e.request.Client)
				return RoundExecutionResult{}, ErrRoundInterrupted
			}
			if result, done := e.consumeInterruptedMessage(&incoming); done {
				return result, ErrRoundInterrupted
			}
		}
	}
}

func (e *roundExecution) interruptedDrainRequired() bool {
	return e.ctx.Err() != nil && e.explicitInterruptRequested()
}

func (e *roundExecution) explicitInterruptRequested() bool {
	return resolveInterruptReason(e.request.InterruptReason) != ""
}

func (e *roundExecution) consumeInterruptedMessage(incoming *sdkprotocol.ReceivedMessage) (RoundExecutionResult, bool) {
	if incoming == nil {
		return RoundExecutionResult{}, false
	}
	e.observeIncoming(*incoming)
	if incoming.Type != sdkprotocol.MessageTypeResult {
		return RoundExecutionResult{}, false
	}
	return observedResultMessage(incoming.Result, e.startedAt), true
}

func (e *roundExecution) startReceiving() {
	e.messageCh = e.request.Client.ReceiveMessages(e.ctx)
	e.refreshIdlePauseState()
}

func (e *roundExecution) handleIdleTimeout() (RoundExecutionResult, error) {
	if shouldTreatAsInterrupted(e.ctx, e.request.InterruptReason) {
		return RoundExecutionResult{}, ErrRoundInterrupted
	}
	abortRoundClientAfterIdleTimeout(e.request.Client)
	return RoundExecutionResult{}, buildRoundStreamIdleTimeoutError(
		e.idleTimeout,
		e.messagesSeen,
		e.lastMessage,
		e.streamDiagnostics.Snapshot(e.messagesSeen, time.Now()),
	)
}

func (e *roundExecution) handleStreamClosed() (RoundExecutionResult, error) {
	if shouldTreatAsInterrupted(e.ctx, e.request.InterruptReason) {
		return RoundExecutionResult{}, ErrRoundInterrupted
	}
	if e.assistantTerminalResult != nil {
		return roundResultWithElapsed(*e.assistantTerminalResult, e.startedAt), nil
	}
	if clientStreamAbortError(e.request.Client) != nil {
		return RoundExecutionResult{}, ErrRoundInterrupted
	}
	return RoundExecutionResult{}, buildRoundStreamClosedError(
		e.request.Client,
		e.messagesSeen,
		e.lastMessage,
		e.streamDiagnostics.Snapshot(e.messagesSeen, time.Now()),
	)
}

func (e *roundExecution) handleIncoming(incoming sdkprotocol.ReceivedMessage) (roundReceiveOutcome, error) {
	e.observeIncoming(incoming)
	failureResult := observedResultMessage(incoming.Result, e.startedAt)
	mapResult, err := e.request.Mapper.Map(incoming, resolveInterruptReason(e.request.InterruptReason))
	if err != nil {
		return roundReceiveOutcome{result: failureResult}, err
	}
	var terminalResult RoundExecutionResult
	isTerminal := strings.TrimSpace(mapResult.TerminalStatus) != ""
	if isTerminal {
		terminalResult = terminalRoundResult(mapResult, e.assistantTerminalResult, incoming.Result, e.startedAt)
	}
	sessionID := resolveSessionID(
		e.request.Mapper.SessionID(),
		incoming.SessionID,
		e.request.Client.SessionID(),
	)
	if err = e.syncSessionID(sessionID); err != nil {
		return roundReceiveOutcome{result: failureResult}, err
	}
	if err = e.persistDurableMessages(mapResult.DurableMessages, sessionID); err != nil {
		return roundReceiveOutcome{result: failureResult}, err
	}
	if err = e.emitEvents(mapResult.Events); err != nil {
		return roundReceiveOutcome{result: failureResult}, err
	}
	if isTerminal {
		return roundReceiveOutcome{
			result: terminalResult,
			done:   true,
		}, nil
	}
	e.rememberAssistantTerminal(mapResult)
	return roundReceiveOutcome{}, nil
}

func (e *roundExecution) observeIncoming(incoming sdkprotocol.ReceivedMessage) {
	e.messagesSeen++
	e.lastMessage = incoming
	e.streamDiagnostics.Observe(incoming, e.messagesSeen, time.Now())
	e.refreshIdlePauseState()
	if !e.idlePaused {
		e.resetIdleTimer()
	}
	if e.request.ObserveIncomingMessage != nil {
		e.request.ObserveIncomingMessage(incoming)
	}
}

func (e *roundExecution) refreshIdlePauseState() {
	if e.idleTimeout <= 0 {
		return
	}
	wasPaused := e.idlePaused
	paused := false
	var changed <-chan struct{}
	if e.request.IdlePauseState != nil {
		paused, changed = e.request.IdlePauseState()
	}
	e.idlePaused = paused
	e.idlePauseChanged = changed
	if paused {
		e.stopIdleTimer()
		return
	}
	if wasPaused || e.idleTimer == nil {
		e.resetIdleTimer()
	}
}

func (e *roundExecution) resetIdleTimer() {
	if e.idleTimeout <= 0 || e.idlePaused {
		return
	}
	if e.idleTimer == nil {
		e.idleTimer = time.NewTimer(e.idleTimeout)
	} else {
		resetRoundIdleTimer(e.idleTimer, e.idleTimeout)
	}
	e.idleTimeoutCh = e.idleTimer.C
}

func (e *roundExecution) stopIdleTimer() {
	if e.idleTimer == nil {
		e.idleTimeoutCh = nil
		return
	}
	if !e.idleTimer.Stop() {
		select {
		case <-e.idleTimer.C:
		default:
		}
	}
	e.idleTimeoutCh = nil
}

func (e *roundExecution) syncSessionID(sessionID string) error {
	if e.request.SyncSessionID == nil || sessionID == "" {
		return nil
	}
	return e.request.SyncSessionID(sessionID)
}

func (e *roundExecution) persistDurableMessages(messages []protocol.Message, sessionID string) error {
	for _, messageValue := range messages {
		if messageValue == nil {
			continue
		}
		if sessionID != "" && strings.TrimSpace(messageString(messageValue["session_id"])) == "" {
			messageValue["session_id"] = sessionID
		}
		if e.request.HandleDurableMessage == nil {
			continue
		}
		if err := e.request.HandleDurableMessage(messageValue); err != nil {
			return err
		}
	}
	return nil
}

func (e *roundExecution) emitEvents(events []protocol.EventMessage) error {
	if e.request.EmitEvent == nil {
		return nil
	}
	for _, event := range events {
		if err := e.request.EmitEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func (e *roundExecution) rememberAssistantTerminal(mapResult RoundMapResult) {
	assistantResult, ok := terminalAssistantResult(mapResult)
	if !ok {
		return
	}
	e.assistantTerminalResult = &assistantResult
	if e.assistantTerminalTimer == nil {
		e.assistantTerminalTimer = time.After(normalizeAssistantTerminalGrace(e.request.AssistantTerminalGrace))
	}
}
