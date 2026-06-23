package runtime

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"
)

const interruptForceCancelDelay = 150 * time.Millisecond

// InterruptSession 中断当前 session 的全部运行中 round。
func (m *Manager) InterruptSession(ctx context.Context, sessionKey string, reason string) ([]string, error) {
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		m.mu.RUnlock()
		return nil, nil
	}

	roundIDs := slices.Sorted(maps.Keys(state.RunningRounds))
	doneSignals := make([]chan struct{}, 0, len(state.RoundDone))
	cancels := make([]context.CancelFunc, 0, len(state.RoundCancels))
	for _, roundID := range roundIDs {
		if done, ok := state.RoundDone[roundID]; ok && done != nil {
			doneSignals = append(doneSignals, done)
		}
		if cancel, ok := state.RoundCancels[roundID]; ok && cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	client := state.Client
	m.mu.RUnlock()

	if len(roundIDs) == 0 {
		return nil, nil
	}

	interruptReason := strings.TrimSpace(reason)

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	m.touchStateLocked(state)
	for _, roundID := range roundIDs {
		state.Interruptions[roundID] = interruptReason
	}
	client = state.Client
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
	if err := client.Interrupt(ctx); err != nil {
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
	return strings.TrimSpace(state.Interruptions[roundID])
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
