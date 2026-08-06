// INPUT: session、interrupt reason 与当前 round registry 快照。
// OUTPUT: 同代 round 中断、强制取消与退出等待。
// POS: Manager 的原子中断入口。
package runtime

import (
	"context"
	"strings"
	"time"
)

const interruptForceCancelDelay = 150 * time.Millisecond
const clientInterruptReasonSubmit = "interrupt"
const clientInterruptWithoutMessage = "__nexus_interrupt_without_message__"

type reasonInterruptClient interface {
	InterruptWithReason(context.Context, string) error
}

// InterruptSession 中断当前 session 的全部运行中 round。
func (m *Manager) InterruptSession(ctx context.Context, sessionKey string, reason string) ([]string, error) {
	interruptReason := strings.TrimSpace(reason)

	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || state.Closing {
		m.mu.Unlock()
		return nil, nil
	}

	roundIDs := state.Rounds.runningIDs()
	if len(roundIDs) == 0 {
		m.mu.Unlock()
		return nil, nil
	}

	doneSignals := state.Rounds.doneSignals(roundIDs...)
	cancels := state.Rounds.cancelFuncs(roundIDs...)
	for _, roundID := range roundIDs {
		if round := state.Rounds.get(roundID); round != nil {
			round.interruption = interruptReason
		}
	}
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()

	if client == nil {
		for _, cancel := range cancels {
			cancel()
		}
		if err := waitRoundDoneSignals(ctx, doneSignals, nil); err != nil {
			return roundIDs, err
		}
		return roundIDs, nil
	}
	if err := interruptClient(ctx, client, interruptReason); err != nil {
		return roundIDs, err
	}
	if err := waitRoundDoneSignals(ctx, doneSignals, func() {
		for _, cancel := range cancels {
			cancel()
		}
	}); err != nil {
		return roundIDs, err
	}
	return roundIDs, nil
}

func interruptClient(ctx context.Context, client Client, reason string) error {
	wireReason := clientInterruptWireReason(reason)
	if wireReason != "" {
		if reasonClient, ok := client.(reasonInterruptClient); ok {
			return reasonClient.InterruptWithReason(ctx, wireReason)
		}
	}
	return client.Interrupt(ctx)
}

func clientInterruptWireReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" || trimmed == clientInterruptWithoutMessage {
		return ""
	}
	return clientInterruptReasonSubmit
}

// GetInterruptReason 返回 round 是否已收到显式中断请求。
func (m *Manager) GetInterruptReason(sessionKey string, roundID string) string {
	if strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(roundID) == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		return ""
	}
	round := state.Rounds.get(roundID)
	if round == nil {
		return ""
	}
	return strings.TrimSpace(round.interruption)
}

func waitRoundDoneSignals(
	ctx context.Context,
	doneSignals []chan struct{},
	forceCancel func(),
) error {
	if len(doneSignals) == 0 {
		return nil
	}

	timer := time.NewTimer(interruptForceCancelDelay)
	defer timer.Stop()
	forceCancelled := forceCancel == nil
	for _, done := range doneSignals {
		for {
			if forceCancelled {
				select {
				case <-done:
					goto nextDone
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			select {
			case <-done:
				goto nextDone
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				forceCancel()
				forceCancelled = true
			}
		}
	nextDone:
	}
	return nil
}
