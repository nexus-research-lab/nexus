package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type idleSessionTarget struct {
	sessionKey        string
	client            Client
	roundCancels      []context.CancelFunc
	idleMessageCancel context.CancelFunc
}

// CloseIdleSessions 回收超过空闲阈值且没有运行中 round 的 SDK client。
func (m *Manager) CloseIdleSessions(ctx context.Context, idleFor time.Duration) (int, error) {
	if idleFor <= 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := m.nowTime().UTC()
	targets := make([]idleSessionTarget, 0)

	m.mu.Lock()
	for sessionKey, state := range m.sessions {
		if state == nil || len(state.RunningRounds) > 0 {
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
		if state.Client == nil {
			delete(m.sessions, sessionKey)
			continue
		}
		targets = append(targets, idleSessionTarget{
			sessionKey:        sessionKey,
			client:            state.Client,
			roundCancels:      copyRoundCancels(state.RoundCancels),
			idleMessageCancel: state.IdleMessageCancel,
		})
		delete(m.sessions, sessionKey)
	}
	m.mu.Unlock()

	errs := make([]error, 0, len(targets))
	for _, target := range targets {
		if target.idleMessageCancel != nil {
			target.idleMessageCancel()
		}
		for _, cancel := range target.roundCancels {
			if cancel != nil {
				cancel()
			}
		}
		disconnectCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
		err := target.client.Disconnect(disconnectCtx)
		cancel()
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
