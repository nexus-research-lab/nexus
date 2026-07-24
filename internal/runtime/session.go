package runtime

import (
	"context"
	"errors"
	"fmt"
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
	ownerUserID := runtimeOwnerUserID(options)
	m.mu.Lock()
	state := m.sessions[sessionKey]
	var existing Client
	var existingKind agentclient.RuntimeKind
	var existingOwnerUserID string
	if state != nil && state.Client != nil {
		existing = state.Client
		existingKind = state.RuntimeKind
		existingOwnerUserID = state.OwnerUserID
		m.touchStateLocked(state)
	}
	m.mu.Unlock()
	if existing != nil {
		if runtimeOwnerMismatch(existingOwnerUserID, ownerUserID) {
			return nil, fmt.Errorf(
				"runtime session owner mismatch: existing=%s requested=%s",
				existingOwnerUserID,
				ownerUserID,
			)
		}
		if existingKind != "" && existingKind != runtimeKind {
			return m.replaceRuntimeClient(ctx, sessionKey, existing, options, factory)
		}
		if err := existing.Reconfigure(ctx, options); err != nil {
			if shouldReplaceRuntimeClientAfterReconfigureError(err) {
				return m.replaceRuntimeClient(ctx, sessionKey, existing, options, factory)
			}
			return nil, err
		}
		m.setRuntimeMetadataIfCurrent(sessionKey, existing, runtimeKind, ownerUserID)
		return existing, nil
	}

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	if state.Client == nil {
		state.Client = factory.New(options)
		state.RuntimeKind = runtimeKind
		state.OwnerUserID = ownerUserID
		m.touchStateLocked(state)
		m.mu.Unlock()
		return state.Client, nil
	}
	client := state.Client
	existingOwnerUserID = state.OwnerUserID
	m.touchStateLocked(state)
	m.mu.Unlock()
	if runtimeOwnerMismatch(existingOwnerUserID, ownerUserID) {
		return nil, fmt.Errorf(
			"runtime session owner mismatch: existing=%s requested=%s",
			existingOwnerUserID,
			ownerUserID,
		)
	}
	if err := client.Reconfigure(ctx, options); err != nil {
		if shouldReplaceRuntimeClientAfterReconfigureError(err) {
			return m.replaceRuntimeClient(ctx, sessionKey, client, options, factory)
		}
		return nil, err
	}
	m.setRuntimeMetadataIfCurrent(sessionKey, client, runtimeKind, ownerUserID)
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

func (m *Manager) setRuntimeMetadataIfCurrent(
	sessionKey string,
	client Client,
	kind agentclient.RuntimeKind,
	ownerUserID string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.sessions[sessionKey]; state != nil && state.Client == client {
		state.RuntimeKind = kind
		if ownerUserID != "" {
			state.OwnerUserID = ownerUserID
		}
		m.touchStateLocked(state)
	}
}

func runtimeOwnerUserID(options agentclient.Options) string {
	return strings.TrimSpace(options.Env["NEXUS_RUNTIME_USER_ID"])
}

func runtimeOwnerMismatch(existing string, requested string) bool {
	return existing != "" && existing != requested
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
	state.OwnerUserID = runtimeOwnerUserID(options)
	// 新进程不持有旧 task/thread；只有再次观测到 task 事件后才允许保活。
	state.HasSubagentHistory = false
	m.touchStateLocked(state)
	m.mu.Unlock()

	disconnectCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
	defer cancel()
	// 新 client 已经成为 session 的唯一事实源；旧进程清理失败不能反向污染本次切换。
	// Disconnect 会先解除 adapter 对旧 session 的引用，返回值只描述清理结果。
	_ = stale.Disconnect(disconnectCtx)
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

// HasSession 返回 session 是否已有可复用的 runtime client。
// 仅检查内存中的 client，不把已持久化但尚未连接的 resume 当作热会话。
func (m *Manager) HasSession(sessionKey string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	if state == nil || state.Client == nil {
		return false
	}
	if connected, ok := state.Client.(interface{ IsConnected() bool }); ok {
		return connected.IsConnected()
	}
	return true
}

// SessionClient 返回当前 session 保存的 client，用于判断 GetOrCreate 是否替换了 runtime。
func (m *Manager) SessionClient(sessionKey string) Client {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state := m.sessions[strings.TrimSpace(sessionKey)]; state != nil {
		return state.Client
	}
	return nil
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
	var reaperErr error
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if ok {
		delete(m.sessions, sessionKey)
		if state != nil {
			reaperErr = m.reapOwnerIfLastLocked(ctx, state.OwnerUserID)
		}
	}
	m.mu.Unlock()
	if !ok || state == nil || state.Client == nil {
		return reaperErr
	}
	if state.IdleMessageCancel != nil {
		state.IdleMessageCancel()
	}
	for _, cancel := range state.RoundCancels {
		if cancel != nil {
			cancel()
		}
	}
	return errors.Join(reaperErr, state.Client.Disconnect(ctx))
}
