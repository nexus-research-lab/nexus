package runtime

import (
	"context"
	"errors"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// GetOrCreate 获取或创建 client，并在复用时应用最新运行时配置。
func (m *Manager) GetOrCreate(ctx context.Context, sessionKey string, options agentclient.Options) (Client, error) {
	return m.GetOrCreateWithFactory(ctx, sessionKey, options, m.factory)
}

// GetOrCreateWithFactory 获取或创建 client，并允许上层为该 session 指定 factory。
//
// Room 的每个 Agent slot 必须和 DM 一样进入统一 Manager，后续 task 控制才能按
// runtime session key 找回原进程；factory 仍由 Room 注入，避免破坏测试与定制启动器。
func (m *Manager) GetOrCreateWithFactory(
	ctx context.Context,
	sessionKey string,
	options agentclient.Options,
	factory Factory,
) (Client, error) {
	if factory == nil {
		factory = m.factory
	}
	runtimeKind := normalizedManagedRuntimeKind(options.Runtime.Kind)
	m.mu.Lock()
	state := m.sessions[sessionKey]
	var existing Client
	var existingKind agentclient.RuntimeKind
	if state != nil && state.Client != nil {
		existing = state.Client
		existingKind = state.RuntimeKind
		m.touchStateLocked(state)
	}
	m.mu.Unlock()
	if existing != nil {
		if existingKind != "" && existingKind != runtimeKind {
			return m.replaceRuntimeClient(ctx, sessionKey, existing, options, factory)
		}
		if err := existing.Reconfigure(ctx, options); err != nil {
			if shouldReplaceRuntimeClientAfterReconfigureError(err) {
				return m.replaceRuntimeClient(ctx, sessionKey, existing, options, factory)
			}
			return nil, err
		}
		m.setRuntimeKindIfCurrent(sessionKey, existing, runtimeKind)
		return existing, nil
	}

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	if state.Client == nil {
		state.Client = factory.New(options)
		state.RuntimeKind = runtimeKind
		m.touchStateLocked(state)
		m.mu.Unlock()
		return state.Client, nil
	}
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()
	if err := client.Reconfigure(ctx, options); err != nil {
		if shouldReplaceRuntimeClientAfterReconfigureError(err) {
			return m.replaceRuntimeClient(ctx, sessionKey, client, options, factory)
		}
		return nil, err
	}
	m.setRuntimeKindIfCurrent(sessionKey, client, runtimeKind)
	return client, nil
}

func normalizedManagedRuntimeKind(kind agentclient.RuntimeKind) agentclient.RuntimeKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "claude", "cc":
		return agentclient.RuntimeClaude
	case "", "nxs":
		return agentclient.RuntimeNXS
	default:
		// 未知 runtime 不能继承 nxs 的管理能力，否则前端会开放无法兑现的续聊入口。
		return ""
	}
}

func (m *Manager) setRuntimeKindIfCurrent(sessionKey string, client Client, kind agentclient.RuntimeKind) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.sessions[sessionKey]; state != nil && state.Client == client {
		state.RuntimeKind = kind
		m.touchStateLocked(state)
	}
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
	factory Factory,
) (Client, error) {
	next := factory.New(options)
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
	state.RuntimeKind = normalizedManagedRuntimeKind(options.Runtime.Kind)
	// 新进程不持有旧 task/thread；只有再次观测到 task 事件后才允许保活。
	state.HasSubagentHistory = false
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

// RuntimeKind 返回当前 session 实际持有的 runtime 类型。
func (m *Manager) RuntimeKind(sessionKey string) agentclient.RuntimeKind {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state := m.sessions[strings.TrimSpace(sessionKey)]; state != nil {
		return state.RuntimeKind
	}
	return ""
}

// MarkSubagentHistory 标记该 runtime 已承载过 subagent task。
// 标记随 sessionState 生命周期保留，使父 round 结束后仍可复用同一 task/thread。
func (m *Manager) MarkSubagentHistory(sessionKey string) {
	if m == nil || strings.TrimSpace(sessionKey) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(strings.TrimSpace(sessionKey))
	state.HasSubagentHistory = true
	m.touchStateLocked(state)
}

// HasSubagentHistory 判断该 runtime 是否需要为 task follow-up 保留进程。
func (m *Manager) HasSubagentHistory(sessionKey string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	return state != nil && state.HasSubagentHistory
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
