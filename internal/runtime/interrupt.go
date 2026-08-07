// INPUT: exact runtime session/round identity、中断原因与 Manager 持有的 round/client 状态。
// OUTPUT: provider interrupt、本地 round cancel、已结束或不支持的真实结果。
// POS: 不误伤同 session successor 的物理 round 中断边界。
package runtime

import (
	"context"
	"strings"
	"time"
)

const interruptForceCancelDelay = 150 * time.Millisecond
const clientInterruptReasonSubmit = "interrupt"
const clientInterruptWithoutMessage = "__nexus_interrupt_without_message__"

const exactRoundCancellationUnavailableDetail = "exact runtime round cancellation is unavailable"

// ExactRoundInterruptOutcome 区分 provider 已接受的 interrupt、本地 round
// context cancel 与 exact target 已结束。bridge 的 Query/Receive context
// cancellation 不会自动发送 provider interrupt，因此两者不能合并宣称。
type ExactRoundInterruptOutcome string

const (
	ExactRoundProviderInterrupted  ExactRoundInterruptOutcome = "provider_interrupted"
	ExactRoundLocalCancelled       ExactRoundInterruptOutcome = "local_round_cancelled"
	ExactRoundAlreadyEnded         ExactRoundInterruptOutcome = "already_ended"
	ExactRoundInterruptUnsupported ExactRoundInterruptOutcome = "unsupported"
)

// ExactRoundInterruptResult 是 durable cancellation consumer 可审计的物理结果。
type ExactRoundInterruptResult struct {
	Outcome        ExactRoundInterruptOutcome
	LimitationCode string
	Detail         string
}

type reasonInterruptClient interface {
	InterruptWithReason(context.Context, string) error
}

// InterruptRound 只在 target 仍是 session 唯一 running round 时调用共享 client
// 的 provider interrupt。多个 round 共用 session 时只取消 exact local context，
// 并明确返回 local_round_cancelled；绝不为追求 provider interrupt 误伤 successor。
//
// Provider interrupt 期间 StartRound 会 fail closed，封住“唯一性检查后 successor
// 抢先进入”的竞态。bridge 的 round context 只控制 Nexus query/receive 协程，
// 不会自动向 provider 发送 interrupt，所以 local 与 provider outcome 必须分开。
func (m *Manager) InterruptRound(
	ctx context.Context,
	sessionKey string,
	roundID string,
	reason string,
) (ExactRoundInterruptResult, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return ExactRoundInterruptResult{Outcome: ExactRoundAlreadyEnded}, nil
	}
	m.mu.Lock()
	state := m.sessions[sessionKey]
	if state == nil || state.Closing {
		m.mu.Unlock()
		return ExactRoundInterruptResult{Outcome: ExactRoundAlreadyEnded}, nil
	}
	round := state.Rounds.get(roundID)
	if round == nil || !round.running {
		m.mu.Unlock()
		return ExactRoundInterruptResult{Outcome: ExactRoundAlreadyEnded}, nil
	}
	cancel := round.cancel
	done := round.done
	client := state.Client
	providerSafe := state.Rounds.runningCount() == 1 && client != nil
	if providerSafe {
		state.Rounds.beginProviderInterrupt(roundID)
	}
	if cancel == nil && !providerSafe {
		m.mu.Unlock()
		return ExactRoundInterruptResult{
			Outcome:        ExactRoundInterruptUnsupported,
			LimitationCode: "exact_local_cancel_unavailable",
			Detail:         exactRoundCancellationUnavailableDetail,
		}, nil
	}
	round.interruption = strings.TrimSpace(reason)
	m.touchStateLocked(state)
	m.mu.Unlock()

	if providerSafe {
		defer m.clearProviderInterruptFence(sessionKey, roundID)
		providerErr := interruptClient(ctx, client, reason)
		if providerErr == nil {
			if done != nil {
				var forceCancel func()
				if cancel != nil {
					forceCancel = cancel
				}
				if waitErr := waitRoundDoneSignals(
					ctx,
					[]chan struct{}{done},
					forceCancel,
				); waitErr != nil {
					return ExactRoundInterruptResult{}, waitErr
				}
			}
			return ExactRoundInterruptResult{
				Outcome: ExactRoundProviderInterrupted,
				Detail:  "provider interrupt accepted for the sole running round",
			}, nil
		}
		if cancel == nil {
			return ExactRoundInterruptResult{
				Outcome:        ExactRoundInterruptUnsupported,
				LimitationCode: "provider_interrupt_failed_without_local_cancel",
				Detail:         providerErr.Error(),
			}, nil
		}
		cancel()
		if done != nil {
			if waitErr := waitRoundDoneSignals(
				ctx,
				[]chan struct{}{done},
				nil,
			); waitErr != nil {
				return ExactRoundInterruptResult{}, waitErr
			}
		}
		return ExactRoundInterruptResult{
			Outcome:        ExactRoundLocalCancelled,
			LimitationCode: "provider_interrupt_failed",
			Detail:         providerErr.Error(),
		}, nil
	}

	cancel()
	if done != nil {
		if err := waitRoundDoneSignals(ctx, []chan struct{}{done}, nil); err != nil {
			return ExactRoundInterruptResult{}, err
		}
	}
	return ExactRoundInterruptResult{
		Outcome:        ExactRoundLocalCancelled,
		LimitationCode: "provider_interrupt_unsafe_shared_session",
		Detail:         "exact local round cancelled; provider session also has another running round",
	}, nil
}

func (m *Manager) clearProviderInterruptFence(sessionKey string, roundID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state != nil && state.Rounds.finishProviderInterrupt(roundID) {
		m.touchStateLocked(state)
		m.removeClientlessSessionIfIdleLocked(sessionKey, state, nil)
	}
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
