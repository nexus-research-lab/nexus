package runtime

import (
	"context"
	"errors"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// GetOrCreate 获取或创建 client，并在复用时应用最新运行时配置。
func (m *Manager) GetOrCreate(ctx context.Context, sessionKey string, options agentclient.Options) (Client, error) {
	m.mu.Lock()
	state := m.sessions[sessionKey]
	var existing Client
	if state != nil && state.Client != nil {
		existing = state.Client
		m.touchStateLocked(state)
	}
	m.mu.Unlock()
	if existing != nil {
		if err := existing.Reconfigure(ctx, options); err != nil {
			if shouldReplaceRuntimeClientAfterReconfigureError(err) {
				return m.replaceRuntimeClient(ctx, sessionKey, existing, options)
			}
			return nil, err
		}
		return existing, nil
	}

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	if state.Client == nil {
		state.Client = m.factory.New(options)
		m.touchStateLocked(state)
		m.mu.Unlock()
		return state.Client, nil
	}
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()
	if err := client.Reconfigure(ctx, options); err != nil {
		if shouldReplaceRuntimeClientAfterReconfigureError(err) {
			return m.replaceRuntimeClient(ctx, sessionKey, client, options)
		}
		return nil, err
	}
	return client, nil
}

func shouldReplaceRuntimeClientAfterReconfigureError(err error) bool {
	return IsRuntimeTransportClosedError(err) ||
		errors.Is(err, agentclient.ErrBypassPermissionsNotAllowed) ||
		errors.Is(err, errManagedGoalMCPServerSetChanged) ||
		errors.Is(err, agentclient.ErrRestartRequired)
}

func (m *Manager) replaceRuntimeClient(
	ctx context.Context,
	sessionKey string,
	stale Client,
	options agentclient.Options,
) (Client, error) {
	next := m.factory.New(options)
	m.mu.Lock()
	state := m.ensureStateLocked(sessionKey)
	if state.Client != stale {
		next = state.Client
		m.mu.Unlock()
		if next == nil {
			return nil, agentclient.ErrNotConnected
		}
		return next, nil
	}
	state.Client = next
	m.touchStateLocked(state)
	m.mu.Unlock()

	disconnectCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
	defer cancel()
	if err := stale.Disconnect(disconnectCtx); err != nil && !IsRuntimeTransportClosedError(err) {
		return nil, err
	}
	if next == nil {
		return nil, agentclient.ErrNotConnected
	}
	return next, nil
}

// CloseSession 关闭指定 session。
func (m *Manager) CloseSession(ctx context.Context, sessionKey string) error {
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if ok {
		delete(m.sessions, sessionKey)
	}
	m.mu.Unlock()
	if !ok || state.Client == nil {
		return nil
	}
	if state.IdleMessageCancel != nil {
		state.IdleMessageCancel()
	}
	for _, cancel := range state.RoundCancels {
		if cancel != nil {
			cancel()
		}
	}
	return state.Client.Disconnect(ctx)
}
