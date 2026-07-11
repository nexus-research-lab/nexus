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
	queryContent, err := runtimectx.PrepareRoundContentWithContext(
		e.ctx,
		e.request.Client,
		roundQueryContent(e.request),
		e.request.ContextualInputs,
	)
	if err != nil {
		return err
	}
	if err = runtimectx.QueryClientContentWithOptions(e.ctx, e.request.Client, queryContent, e.request.InputOptions); err != nil {
		if isRoundAbortError(e.ctx, err) {
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
	if e.idleTimer != nil {
		defer e.idleTimer.Stop()
	}
	for {
		select {
		case <-e.ctx.Done():
			return RoundExecutionResult{}, ErrRoundInterrupted
		case <-e.assistantTerminalTimer:
			return roundResultWithElapsed(*e.assistantTerminalResult, e.startedAt), nil
		case <-e.idleTimeoutCh:
			return e.handleIdleTimeout()
		case incoming, ok := <-e.messageCh:
			if !ok {
				return e.handleStreamClosed()
			}
			outcome, err := e.handleIncoming(incoming)
			if err != nil {
				return RoundExecutionResult{}, err
			}
			if outcome.done {
				return outcome.result, nil
			}
		}
	}
}

func (e *roundExecution) startReceiving() {
	e.messageCh = e.request.Client.ReceiveMessages(e.ctx)
	if e.idleTimeout <= 0 {
		return
	}
	e.idleTimer = time.NewTimer(e.idleTimeout)
	e.idleTimeoutCh = e.idleTimer.C
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
	mapResult, err := e.request.Mapper.Map(incoming, resolveInterruptReason(e.request.InterruptReason))
	if err != nil {
		return roundReceiveOutcome{}, err
	}
	sessionID := resolveSessionID(
		e.request.Mapper.SessionID(),
		incoming.SessionID,
		e.request.Client.SessionID(),
	)
	if err = e.syncSessionID(sessionID); err != nil {
		return roundReceiveOutcome{}, err
	}
	if err = e.persistDurableMessages(mapResult.DurableMessages, sessionID); err != nil {
		return roundReceiveOutcome{}, err
	}
	if err = e.emitEvents(mapResult.Events); err != nil {
		return roundReceiveOutcome{}, err
	}
	if strings.TrimSpace(mapResult.TerminalStatus) != "" {
		return roundReceiveOutcome{
			result: terminalRoundResult(mapResult, e.assistantTerminalResult, incoming.Result, e.startedAt),
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
	resetRoundIdleTimer(e.idleTimer, e.idleTimeout)
	if e.request.ObserveIncomingMessage != nil {
		e.request.ObserveIncomingMessage(incoming)
	}
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
