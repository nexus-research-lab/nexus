package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CloseIdleSessions 回收超过空闲阈值且没有运行中 round 的 SDK client。
func (m *Manager) CloseIdleSessions(ctx context.Context, idleFor time.Duration) (int, error) {
	if idleFor <= 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := m.nowTime().UTC()
	targets := make([]*sessionCloseTarget, 0)
	owners := make(map[string]struct{})

	m.mu.Lock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Closing || len(state.RunningRounds) > 0 {
			continue
		}
		lastUsedAt := state.LastUsedAt
		if lastUsedAt.IsZero() {
			state.LastUsedAt = now
			continue
		}
		if now.Sub(lastUsedAt) < idleFor {
			continue
		}
		target, started, _ := m.beginSessionCloseLocked(sessionKey)
		if !started {
			continue
		}
		targets = append(targets, target)
		owners[target.ownerUserID] = struct{}{}
	}
	reaperErrs := make([]error, 0)
	for ownerUserID := range owners {
		if err := m.reapOwnerIfLastLocked(ctx, ownerUserID); err != nil {
			reaperErrs = append(reaperErrs, fmt.Errorf("reap owner runtime processes: %w", err))
		}
	}
	m.mu.Unlock()

	errs := make([]error, 0, len(targets)+len(reaperErrs))
	errs = append(errs, reaperErrs...)
	for _, target := range targets {
		cancelSessionCloseTarget(target)
		var disconnectErr error
		if target.client != nil {
			disconnectCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
			disconnectErr = target.client.Disconnect(disconnectCtx)
			cancel()
		}
		backgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
		roundErr := waitRoundDoneForClose(ctx, target.roundDone)
		clientCleanupPending := errors.Is(disconnectErr, context.Canceled) ||
			errors.Is(disconnectErr, context.DeadlineExceeded)
		if clientCleanupPending || backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target, clientCleanupPending)
		} else {
			m.finishSessionClose(target)
		}
		err := errors.Join(disconnectErr, backgroundErr, roundErr)
		if err != nil && !IsRuntimeTransportClosedError(err) {
			errs = append(errs, fmt.Errorf("close idle runtime session %s: %w", target.sessionKey, err))
		}
	}
	return len(targets), errors.Join(errs...)
}

func copyRoundCancels(input map[string]context.CancelFunc) []context.CancelFunc {
	if len(input) == 0 {
		return nil
	}
	output := make([]context.CancelFunc, 0, len(input))
	for _, cancel := range input {
		if cancel != nil {
			output = append(output, cancel)
		}
	}
	return output
}
