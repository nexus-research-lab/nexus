package exec

import (
	"context"
	"errors"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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

	queryContent, err := runtimectx.PrepareRoundContentWithContext(ctx, request.Client, roundQueryContent(request), request.ContextualInputs)
	if err != nil {
		return RoundExecutionResult{}, err
	}
	if err := runtimectx.QueryClientContentWithOptions(ctx, request.Client, queryContent, request.InputOptions); err != nil {
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
				if clientStreamAbortError(request.Client) != nil {
					return RoundExecutionResult{}, ErrRoundInterrupted
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
