package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

var errRuntimeClientChanged = errors.New("runtime client changed")

// ClientStartup 串行化同一个 session key 的配置、连接与失败处置。
// 调用方必须 Close；事务内可关闭并重建 session，而不会跨代释放互斥权。
// 同一个事务由单一 goroutine 顺序使用，不支持方法之间或方法与 Close 并发。
type ClientStartup struct {
	manager            *Manager
	sessionKey         string
	gate               *sessionStartupGate
	ownerLease         *ownerStartupLease
	closeEpoch         uint64
	release            func()
	expectedState      *sessionState
	expectedClient     Client
	expectedGeneration uint64
}

// ClientLease 标识一次已经成功取得 client 的启动代次。
// 它只用于条件关闭，防止旧操作误伤同一 adapter 上的新连接。
type ClientLease struct {
	manager     *Manager
	sessionKey  string
	state       *sessionState
	client      Client
	generation  uint64
	ownerUserID string
}

// BeginClientStartup 获取 session key 级启动事务；等待过程响应 ctx 取消。
func (m *Manager) BeginClientStartup(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
) (*ClientStartup, error) {
	return m.beginClientStartup(ctx, sessionKey, ownerUserID)
}

func (m *Manager) beginClientStartup(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
) (*ClientStartup, error) {
	if m == nil {
		return nil, agentclient.ErrNotConnected
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sessionKey = strings.TrimSpace(sessionKey)
	ownerUserID = strings.TrimSpace(ownerUserID)

	m.mu.Lock()
	ownerLease, err := m.beginOwnerStartupLocked(ownerUserID, sessionKey)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	if m.startupGates == nil {
		m.startupGates = make(map[string]*sessionStartupGate)
	}
	gate := m.startupGates[sessionKey]
	if gate == nil {
		gate = &sessionStartupGate{token: make(chan struct{}, 1)}
		gate.token <- struct{}{}
		m.startupGates[sessionKey] = gate
	}
	if gate.closeBlocks > 0 {
		m.releaseOwnerStartupLocked(ownerLease)
		m.mu.Unlock()
		return nil, ErrRuntimeSessionClosing
	}
	gate.refs++
	closeEpoch := gate.closeEpoch
	m.mu.Unlock()

	select {
	case <-ctx.Done():
		m.releaseClientStartup(sessionKey, gate, ownerLease, false)
		return nil, ctx.Err()
	case <-gate.token:
	}
	startup := &ClientStartup{
		manager:    m,
		sessionKey: sessionKey,
		gate:       gate,
		ownerLease: ownerLease,
		closeEpoch: closeEpoch,
	}
	m.mu.RLock()
	valid := m.startupGates[sessionKey] == gate && gate.closeEpoch == closeEpoch &&
		m.validateOwnerStartupLocked(ownerLease) == nil
	m.mu.RUnlock()
	if !valid {
		m.releaseClientStartup(sessionKey, gate, ownerLease, true)
		return nil, agentclient.ErrAborted
	}
	startup.release = func() {
		m.releaseClientStartup(sessionKey, gate, ownerLease, true)
	}
	return startup, nil
}

func (m *Manager) releaseClientStartup(
	sessionKey string,
	gate *sessionStartupGate,
	ownerLease *ownerStartupLease,
	held bool,
) {
	if m == nil || gate == nil {
		return
	}
	m.mu.Lock()
	if held && m.startupGates[sessionKey] == gate {
		m.removeClientlessSessionIfIdleLocked(sessionKey, nil, gate)
	}
	m.releaseOwnerStartupLocked(ownerLease)
	gate.refs--
	if gate.refs == 0 && m.startupGates[sessionKey] == gate {
		delete(m.startupGates, sessionKey)
	}
	m.mu.Unlock()
	if held {
		// state 清理与 gate 引用更新必须先完成；否则等待者拿到 token 后
		// 可能观察到即将被前任 startup 删除的空 state。
		gate.token <- struct{}{}
	}
}

func (m *Manager) beginSessionCloseGateLocked(sessionKey string) *sessionStartupGate {
	if m.startupGates == nil {
		m.startupGates = make(map[string]*sessionStartupGate)
	}
	gate := m.startupGates[sessionKey]
	if gate == nil {
		gate = &sessionStartupGate{token: make(chan struct{}, 1)}
		gate.token <- struct{}{}
		m.startupGates[sessionKey] = gate
	}
	gate.refs++
	gate.closeBlocks++
	gate.closeEpoch++
	return gate
}

func (m *Manager) releaseSessionCloseGate(sessionKey string, gate *sessionStartupGate) {
	if m == nil || gate == nil {
		return
	}
	m.mu.Lock()
	gate.closeBlocks--
	gate.refs--
	if gate.refs == 0 && m.startupGates[sessionKey] == gate {
		delete(m.startupGates, sessionKey)
	}
	m.mu.Unlock()
}

// Close 释放启动事务。
func (s *ClientStartup) Close() {
	if s == nil || s.release == nil {
		return
	}
	release := s.release
	s.release = nil
	release()
}

func (s *ClientStartup) active() error {
	if s == nil || s.manager == nil || s.release == nil {
		return agentclient.ErrAborted
	}
	return nil
}

func (s *ClientStartup) validateCloseEpochLocked() error {
	if s == nil {
		return nil
	}
	if s.manager == nil || s.release == nil ||
		s.manager.startupGates[s.sessionKey] != s.gate ||
		s.gate == nil || s.gate.closeEpoch != s.closeEpoch {
		return agentclient.ErrAborted
	}
	return s.manager.validateOwnerStartupLocked(s.ownerLease)
}

// SessionKey 返回当前事务归一化后的 session key。
func (s *ClientStartup) SessionKey() string {
	if s == nil {
		return ""
	}
	return s.sessionKey
}

// GetOrCreate 获取或创建 client，并在复用时应用最新运行时配置。
func (m *Manager) GetOrCreate(ctx context.Context, sessionKey string, options agentclient.Options) (Client, error) {
	startup, err := m.BeginClientStartup(ctx, sessionKey, runtimeOwnerUserID(options))
	if err != nil {
		return nil, err
	}
	defer startup.Close()
	return startup.GetOrCreateWithFactory(ctx, options, m.factory)
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
	startup, err := m.BeginClientStartup(ctx, sessionKey, runtimeOwnerUserID(options))
	if err != nil {
		return nil, err
	}
	defer startup.Close()
	return startup.GetOrCreateWithFactory(ctx, options, factory)
}

// GetOrCreateWithFactory 在当前启动事务内获取或创建 client。
func (s *ClientStartup) GetOrCreateWithFactory(
	ctx context.Context,
	options agentclient.Options,
	factory Factory,
) (Client, error) {
	if err := s.active(); err != nil {
		return nil, err
	}
	client, state, err := s.manager.getOrCreateWithFactory(
		ctx,
		s,
		s.sessionKey,
		options,
		factory,
	)
	s.expectedState = state
	s.expectedClient = client
	s.expectedGeneration = 0
	if state != nil && client != nil {
		s.manager.mu.Lock()
		ownershipErr := s.validateCloseEpochLocked()
		if ownershipErr == nil {
			ownershipErr = runtimeClientOwnershipError(
				s.manager.sessions[s.sessionKey],
				state,
				client,
			)
		}
		if ownershipErr == nil {
			if err == nil {
				state.StartupGeneration++
			}
			s.expectedGeneration = state.StartupGeneration
		} else if err == nil {
			err = ownershipErr
		}
		s.manager.mu.Unlock()
	}
	return client, err
}

func (m *Manager) getOrCreateWithFactory(
	ctx context.Context,
	startup *ClientStartup,
	sessionKey string,
	options agentclient.Options,
	factory Factory,
) (Client, *sessionState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if factory == nil {
		factory = m.factory
	}
	sessionKey = strings.TrimSpace(sessionKey)
	runtimeKind := normalizedManagedRuntimeKind(options.Runtime.Kind)
	ownerUserID := runtimeOwnerUserID(options)
	agentID := runtimeSessionAgentID(sessionKey)
	processPolicyFingerprint := managedRuntimeProcessPolicyFingerprint(options)
	startupOwnerUserID := ""
	if startup.ownerLease != nil {
		startupOwnerUserID = startup.ownerLease.ownerUserID
	}
	if startupOwnerUserID != ownerUserID {
		return nil, nil, fmt.Errorf(
			"runtime startup owner mismatch: startup=%s requested=%s",
			startupOwnerUserID,
			ownerUserID,
		)
	}
	cacheSurface, err := cacheSurfaceProfileFromOptions(ctx, options)
	if err != nil {
		return nil, nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		m.mu.Lock()
		if err := startup.validateCloseEpochLocked(); err != nil {
			m.mu.Unlock()
			return nil, nil, err
		}
		if err := m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID); err != nil {
			m.mu.Unlock()
			return nil, nil, err
		}
		state := m.sessions[sessionKey]
		if state != nil && state.Closing {
			m.mu.Unlock()
			return nil, state, ErrRuntimeSessionClosing
		}
		if state != nil && runtimeOwnerMismatch(state.OwnerUserID, ownerUserID) {
			existingOwnerUserID := state.OwnerUserID
			m.mu.Unlock()
			return nil, state, fmt.Errorf(
				"runtime session owner mismatch: existing=%s requested=%s",
				existingOwnerUserID,
				ownerUserID,
			)
		}
		if state == nil || state.Client == nil {
			expectedState := state
			// Factory implementations may start processes or wait on external state.
			// Keep that work outside Manager.mu so revocation can publish its tombstone
			// before this startup performs the second ownership check.
			m.mu.Unlock()
			client := factory.New(options)
			if client == nil {
				return nil, expectedState, agentclient.ErrNotConnected
			}

			m.mu.Lock()
			current := m.sessions[sessionKey]
			ownershipErr := startup.validateCloseEpochLocked()
			if ownershipErr == nil {
				ownershipErr = m.runtimeAgentAdmissionErrorLocked(
					sessionKey,
					ownerUserID,
					agentID,
				)
			}
			switch {
			case ownershipErr != nil:
			case expectedState != nil && current != expectedState:
				ownershipErr = agentclient.ErrAborted
			case current != nil && current.Closing:
				ownershipErr = ErrRuntimeSessionClosing
			case current != nil && runtimeOwnerMismatch(current.OwnerUserID, ownerUserID):
				ownershipErr = fmt.Errorf(
					"runtime session owner mismatch: existing=%s requested=%s",
					current.OwnerUserID,
					ownerUserID,
				)
			case current != nil && current.Client != nil:
				ownershipErr = errRuntimeClientChanged
			}
			if ownershipErr != nil {
				m.mu.Unlock()
				retireUnusedRuntimeClient(client)
				if errors.Is(ownershipErr, errRuntimeClientChanged) {
					continue
				}
				return nil, current, ownershipErr
			}
			if current == nil {
				current = m.ensureStateLocked(sessionKey)
			}
			current.Client = client
			current.RuntimeKind = runtimeKind
			current.OwnerUserID = ownerUserID
			current.AgentID = agentID
			current.ProcessPolicyFingerprint = processPolicyFingerprint
			current.CacheSurface = cacheSurface
			m.touchStateLocked(current)
			m.mu.Unlock()
			if err := m.runtimeAgentAdmissionError(sessionKey, ownerUserID, agentID); err != nil {
				return nil, current, err
			}
			return client, current, nil
		}

		existing := state.Client
		existingKind := state.RuntimeKind
		existingProcessPolicyFingerprint := state.ProcessPolicyFingerprint
		m.touchStateLocked(state)
		m.mu.Unlock()
		if (existingKind != "" && existingKind != runtimeKind) ||
			(existingProcessPolicyFingerprint != "" &&
				existingProcessPolicyFingerprint != processPolicyFingerprint) {
			next, err := m.replaceRuntimeClient(ctx, startup, sessionKey, state, existing, options, factory)
			if errors.Is(err, errRuntimeClientChanged) {
				continue
			}
			return next, state, err
		}

		reconfigureErr := existing.Reconfigure(ctx, options)
		m.mu.Lock()
		if err := startup.validateCloseEpochLocked(); err != nil {
			m.mu.Unlock()
			return nil, state, err
		}
		if err := m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID); err != nil {
			m.mu.Unlock()
			return nil, state, err
		}
		current := m.sessions[sessionKey]
		switch {
		case current != state:
			m.mu.Unlock()
			return nil, state, agentclient.ErrAborted
		case current.Closing:
			m.mu.Unlock()
			return nil, state, ErrRuntimeSessionClosing
		case current.Client != existing:
			m.mu.Unlock()
			continue
		case reconfigureErr == nil:
			current.RuntimeKind = runtimeKind
			if ownerUserID != "" {
				current.OwnerUserID = ownerUserID
			}
			if agentID != "" {
				current.AgentID = agentID
			}
			current.ProcessPolicyFingerprint = processPolicyFingerprint
			current.CacheSurface = cacheSurface
			m.touchStateLocked(current)
			m.mu.Unlock()
			if err := m.runtimeAgentAdmissionError(sessionKey, ownerUserID, agentID); err != nil {
				return nil, state, err
			}
			return existing, state, nil
		default:
			m.mu.Unlock()
		}

		if shouldReplaceRuntimeClientAfterReconfigureError(reconfigureErr) {
			next, err := m.replaceRuntimeClient(ctx, startup, sessionKey, state, existing, options, factory)
			if errors.Is(err, errRuntimeClientChanged) {
				continue
			}
			return next, state, err
		}
		return existing, state, reconfigureErr
	}
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

func (m *Manager) runtimeAgentAdmissionError(
	sessionKey string,
	ownerUserID string,
	agentID string,
) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID)
}

// Connect 只连接当前仍由 Manager 持有且未被删除墓碑撤销的 client。
//
// GetOrCreate 与 SDK Connect 之间存在锁外进程启动窗口，因此连接前后都要核验；
// 删除若在 Connect 中间发生，后置核验会再次断开迟到启动的进程。
func (m *Manager) Connect(ctx context.Context, sessionKey string, client Client) error {
	if m == nil || client == nil {
		return agentclient.ErrNotConnected
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionKey = strings.TrimSpace(sessionKey)
	m.mu.RLock()
	admissionErr := m.runtimeClientAdmissionErrorLocked(sessionKey, client)
	m.mu.RUnlock()
	if admissionErr != nil {
		return admissionErr
	}

	connectErr := client.Connect(ctx)
	m.mu.RLock()
	admissionErr = m.runtimeClientAdmissionErrorLocked(sessionKey, client)
	m.mu.RUnlock()
	if admissionErr == nil {
		return connectErr
	}
	cleanupErr := disconnectRuntimeClientWithTimeout(client)
	return errors.Join(admissionErr, connectErr, cleanupErr)
}

func (m *Manager) runtimeClientAdmissionErrorLocked(sessionKey string, client Client) error {
	state := m.sessions[strings.TrimSpace(sessionKey)]
	ownerUserID := ""
	agentID := runtimeSessionAgentID(sessionKey)
	if state != nil {
		ownerUserID = strings.TrimSpace(state.OwnerUserID)
		if strings.TrimSpace(state.AgentID) != "" {
			agentID = strings.TrimSpace(state.AgentID)
		}
	}
	if err := m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID); err != nil {
		return err
	}
	if state == nil || state.Client == nil || state.Client != client {
		return agentclient.ErrNotConnected
	}
	if state.Closing {
		return ErrRuntimeSessionClosing
	}
	return nil
}

func disconnectRuntimeClientWithTimeout(client Client) error {
	if client == nil {
		return nil
	}
	disconnectCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
	err := client.Disconnect(disconnectCtx)
	cancel()
	return err
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
		errors.Is(err, agentclient.ErrRestartRequired)
}

func (m *Manager) replaceRuntimeClient(
	ctx context.Context,
	startup *ClientStartup,
	sessionKey string,
	expectedState *sessionState,
	stale Client,
	options agentclient.Options,
	factory Factory,
) (Client, error) {
	cacheSurface, err := cacheSurfaceProfileFromOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	ownerUserID := runtimeOwnerUserID(options)
	agentID := runtimeSessionAgentID(sessionKey)
	m.mu.RLock()
	admissionErr := m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID)
	m.mu.RUnlock()
	if admissionErr != nil {
		return nil, admissionErr
	}
	next := factory.New(options)
	if next == nil {
		return nil, agentclient.ErrNotConnected
	}
	if next == stale {
		return nil, errors.New("runtime factory reused retired client")
	}

	m.mu.Lock()
	state := m.sessions[sessionKey]
	ownershipErr := startup.validateCloseEpochLocked()
	switch {
	case ownershipErr != nil:
	case m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID) != nil:
		ownershipErr = m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID)
	case state != expectedState:
		ownershipErr = agentclient.ErrAborted
	case state.Closing:
		ownershipErr = ErrRuntimeSessionClosing
	case state.Client != stale:
		ownershipErr = errRuntimeClientChanged
	}
	if ownershipErr != nil {
		m.mu.Unlock()
		retireUnusedRuntimeClient(next)
		return nil, ownershipErr
	}
	stale.Retire()
	drain := state.IdleMessageDrain
	if drain != nil {
		drain.cancel()
	}
	m.mu.Unlock()

	// Runtime adapter/SDK owns the subprocess termination window. Reusing the
	// same fixed timeout here races its forced-close boundary: the process may
	// have exited successfully while this outer deadline reports a failed
	// replacement. The durable startup context still cancels deletion/shutdown,
	// while a normal replacement waits for the old process to be fully reaped.
	disconnectErr := stale.Disconnect(ctx)
	idleDrainErr := waitIdleMessageDrain(ctx, drain)
	if errors.Is(disconnectErr, context.Canceled) || errors.Is(disconnectErr, context.DeadlineExceeded) ||
		idleDrainErr != nil {
		retireUnusedRuntimeClient(next)
		m.finishRetiredSessionCloseWhenDone(sessionKey, expectedState, stale)
		return nil, errors.Join(disconnectErr, idleDrainErr)
	}
	if err := ctx.Err(); err != nil {
		retireUnusedRuntimeClient(next)
		m.finishRetiredSessionCloseWhenDone(sessionKey, expectedState, stale)
		return nil, err
	}

	// 旧进程真正退出后才发布 next；启动事务 gate 会阻止同 key 的观察者
	// 在清理窗口连接或重配置候选 client。
	m.mu.Lock()
	state = m.sessions[sessionKey]
	ownershipErr = startup.validateCloseEpochLocked()
	switch {
	case ownershipErr != nil:
	case m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID) != nil:
		ownershipErr = m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID)
	case state != expectedState:
		ownershipErr = agentclient.ErrAborted
	case state.Closing:
		ownershipErr = ErrRuntimeSessionClosing
	case state.Client != stale:
		ownershipErr = errRuntimeClientChanged
	default:
		ownershipErr = nil
	}
	if ownershipErr != nil {
		m.mu.Unlock()
		retireUnusedRuntimeClient(next)
		return nil, ownershipErr
	}
	state.Client = next
	state.RuntimeKind = normalizedManagedRuntimeKind(options.Runtime.Kind)
	state.OwnerUserID = ownerUserID
	state.AgentID = agentID
	state.ProcessPolicyFingerprint = managedRuntimeProcessPolicyFingerprint(options)
	state.CacheSurface = cacheSurface
	state.ContextUsageByAgent = nil
	// 新进程不持有旧 task/thread；只有再次观测到 task 事件后才允许保活。
	state.HasSubagentHistory = false
	m.touchStateLocked(state)
	m.mu.Unlock()
	if err := m.runtimeAgentAdmissionError(sessionKey, ownerUserID, agentID); err != nil {
		return nil, err
	}
	return next, nil
}

func (m *Manager) finishRetiredSessionCloseWhenDone(
	sessionKey string,
	expectedState *sessionState,
	stale Client,
) {
	m.mu.Lock()
	state := m.sessions[sessionKey]
	if state != expectedState || state == nil || state.Client != stale {
		m.mu.Unlock()
		return
	}
	target, started, _ := m.beginSessionCloseLocked(sessionKey)
	m.mu.Unlock()
	if !started {
		return
	}
	cancelSessionCloseTarget(target)
	m.finishSessionCloseWhenDone(target, true)
}

func retireUnusedRuntimeClient(client Client) {
	if client == nil {
		return
	}
	client.Retire()
	disconnectCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
	defer cancel()
	_ = client.Disconnect(disconnectCtx)
}

// RuntimeKind 返回当前 session 实际持有的 runtime 类型。
func (m *Manager) RuntimeKind(sessionKey string) agentclient.RuntimeKind {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state := m.sessions[strings.TrimSpace(sessionKey)]; state != nil && !state.Closing {
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
	return sessionStateHasConnectedClient(state)
}

func sessionStateHasConnectedClient(state *sessionState) bool {
	if state == nil || state.Closing || state.Client == nil {
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
	if state := m.sessions[strings.TrimSpace(sessionKey)]; state != nil && !state.Closing {
		return state.Client
	}
	return nil
}

// Connect 连接事务内最近一次 GetOrCreate 返回的 client，并提交新的连接代次。
func (s *ClientStartup) Connect(ctx context.Context) error {
	if err := s.active(); err != nil {
		return err
	}
	if s.expectedState == nil || s.expectedClient == nil {
		return agentclient.ErrNotConnected
	}
	err := s.manager.connectClient(
		ctx,
		s,
		s.sessionKey,
		s.expectedState,
		s.expectedClient,
		s.expectedGeneration,
	)
	return err
}

func (m *Manager) connectClient(
	ctx context.Context,
	startup *ClientStartup,
	sessionKey string,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
) error {
	if m == nil || expected == nil || expectedState == nil {
		return agentclient.ErrNotConnected
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	state := m.sessions[sessionKey]
	ownershipErr := startup.validateCloseEpochLocked()
	if ownershipErr == nil {
		ownershipErr = runtimeClientLeaseOwnershipError(
			state,
			expectedState,
			expected,
			expectedGeneration,
		)
	}
	if ownershipErr == nil {
		ownershipErr = m.runtimeClientAdmissionErrorLocked(sessionKey, expected)
	}
	if ownershipErr == nil {
		m.touchStateLocked(state)
	}
	m.mu.Unlock()
	if ownershipErr != nil {
		return ownershipErr
	}

	connectErr := expected.Connect(ctx)
	m.mu.Lock()
	state = m.sessions[sessionKey]
	ownershipErr = startup.validateCloseEpochLocked()
	if ownershipErr == nil {
		ownershipErr = runtimeClientLeaseOwnershipError(
			state,
			expectedState,
			expected,
			expectedGeneration,
		)
	}
	if ownershipErr == nil {
		ownershipErr = m.runtimeClientAdmissionErrorLocked(sessionKey, expected)
	}
	if ownershipErr != nil {
		m.mu.Unlock()
		cleanupErr := disconnectRuntimeClientWithTimeout(expected)
		return errors.Join(ownershipErr, connectErr, cleanupErr)
	}
	if connectErr != nil {
		m.mu.Unlock()
		return connectErr
	}
	m.touchStateLocked(state)
	m.mu.Unlock()
	return nil
}

func runtimeClientOwnershipError(state *sessionState, expectedState *sessionState, expected Client) error {
	switch {
	case state != expectedState:
		return agentclient.ErrAborted
	case state == nil || state.Client != expected:
		return errRuntimeClientChanged
	case state.Closing:
		return ErrRuntimeSessionClosing
	default:
		return nil
	}
}

func runtimeClientLeaseOwnershipError(
	state *sessionState,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
) error {
	if err := runtimeClientOwnershipError(state, expectedState, expected); err != nil {
		return err
	}
	if state.StartupGeneration != expectedGeneration {
		return agentclient.ErrAborted
	}
	return nil
}

// CaptureClientLease 捕获当前 client 代次，用于跨启动事务的失败清理。
func (m *Manager) CaptureClientLease(sessionKey string, expected Client) (ClientLease, bool) {
	if m == nil || expected == nil {
		return ClientLease{}, false
	}
	sessionKey = strings.TrimSpace(sessionKey)
	m.mu.RLock()
	state := m.sessions[sessionKey]
	if state == nil || state.Closing || state.Client != expected || state.StartupGeneration == 0 {
		m.mu.RUnlock()
		return ClientLease{}, false
	}
	lease := ClientLease{
		manager:     m,
		sessionKey:  sessionKey,
		state:       state,
		client:      expected,
		generation:  state.StartupGeneration,
		ownerUserID: state.OwnerUserID,
	}
	m.mu.RUnlock()
	return lease, true
}

// CloseSessionIfLease 只关闭 lease 仍指向的 session state、client 与连接代次。
func (m *Manager) CloseSessionIfLease(ctx context.Context, lease ClientLease) (bool, error) {
	if m == nil || lease.manager != m || lease.client == nil || lease.state == nil {
		return false, nil
	}
	startup, err := m.beginClientStartup(ctx, lease.sessionKey, lease.ownerUserID)
	if err != nil {
		return false, err
	}
	defer startup.Close()
	return m.closeSession(
		ctx,
		lease.sessionKey,
		lease.state,
		lease.client,
		lease.generation,
		true,
		false,
		startup.ownerLease,
	)
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
	if state.Closing {
		return
	}
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
	return state != nil && !state.Closing && state.HasSubagentHistory
}

// CloseSession 关闭指定 session。
func (m *Manager) CloseSession(ctx context.Context, sessionKey string) error {
	if m == nil {
		return nil
	}
	sessionKey = strings.TrimSpace(sessionKey)
	_, err := m.closeSession(ctx, sessionKey, nil, nil, 0, false, true, nil)
	return err
}

// CloseCurrent 只关闭当前启动事务最近一次获取的 client 代次。
func (s *ClientStartup) CloseCurrent(ctx context.Context) (bool, error) {
	if err := s.active(); err != nil {
		return false, err
	}
	if s.expectedState == nil || s.expectedClient == nil {
		return false, nil
	}
	closed, err := s.manager.closeSession(
		ctx,
		s.sessionKey,
		s.expectedState,
		s.expectedClient,
		s.expectedGeneration,
		true,
		false,
		s.ownerLease,
	)
	if closed {
		s.expectedState = nil
		s.expectedClient = nil
		s.expectedGeneration = 0
	}
	return closed, err
}

// RetireCurrent 只永久撤销并移除当前启动事务的 client，不关闭同 session 的
// round 与后台任务。启动失败后的无 resume 重试应走此入口，避免后台任务等待自己退出。
func (s *ClientStartup) RetireCurrent(ctx context.Context) (bool, error) {
	if err := s.active(); err != nil {
		return false, err
	}
	if s.expectedState == nil || s.expectedClient == nil {
		return false, nil
	}
	retired, err := s.manager.retireCurrentClient(
		ctx,
		s,
		s.sessionKey,
		s.expectedState,
		s.expectedClient,
		s.expectedGeneration,
	)
	if retired {
		s.expectedState = nil
		s.expectedClient = nil
		s.expectedGeneration = 0
	}
	return retired, err
}

// RetireExisting 永久撤销启动事务开始前已经发布的当前 client。
// 它用于配置兼容性在 GetOrCreate 前已经判定必须换代的路径；若事务已经获取过
// client，则退化为 RetireCurrent，始终受同一 startup gate 与 owner epoch 保护。
func (s *ClientStartup) RetireExisting(ctx context.Context) (bool, error) {
	if err := s.active(); err != nil {
		return false, err
	}
	if s.expectedState == nil || s.expectedClient == nil {
		s.manager.mu.Lock()
		ownershipErr := s.validateCloseEpochLocked()
		state := s.manager.sessions[s.sessionKey]
		requestedOwnerUserID := ""
		if s.ownerLease != nil {
			requestedOwnerUserID = s.ownerLease.ownerUserID
		}
		if ownershipErr == nil && state != nil {
			switch {
			case runtimeOwnerMismatch(state.OwnerUserID, requestedOwnerUserID):
				ownershipErr = fmt.Errorf(
					"runtime session owner mismatch: existing=%s requested=%s",
					state.OwnerUserID,
					requestedOwnerUserID,
				)
			case state.Closing:
				ownershipErr = ErrRuntimeSessionClosing
			case state.Client != nil:
				s.expectedState = state
				s.expectedClient = state.Client
				s.expectedGeneration = state.StartupGeneration
			}
		}
		s.manager.mu.Unlock()
		if ownershipErr != nil {
			return false, ownershipErr
		}
	}
	return s.RetireCurrent(ctx)
}

func (m *Manager) retireCurrentClient(
	ctx context.Context,
	startup *ClientStartup,
	sessionKey string,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	ownershipErr := startup.validateCloseEpochLocked()
	if ownershipErr == nil {
		ownershipErr = runtimeClientLeaseOwnershipError(
			m.sessions[sessionKey],
			expectedState,
			expected,
			expectedGeneration,
		)
	}
	if ownershipErr != nil {
		m.mu.Unlock()
		return false, ownershipErr
	}
	expected.Retire()
	drain := expectedState.IdleMessageDrain
	if drain != nil {
		// idle drain 只读取旧 client；先取消，但由 drain defer 清字段，确保
		// 空 state 只会在 goroutine 真正退出后回收。
		drain.cancel()
	}
	m.mu.Unlock()

	disconnectErr := expected.Disconnect(ctx)
	idleDrainErr := waitIdleMessageDrain(ctx, drain)
	if errors.Is(disconnectErr, context.Canceled) || errors.Is(disconnectErr, context.DeadlineExceeded) ||
		idleDrainErr != nil {
		m.finishRetiredClientResetWhenDone(sessionKey, expectedState, expected, expectedGeneration, drain)
		return true, errors.Join(disconnectErr, idleDrainErr)
	}
	m.clearRetiredClient(sessionKey, expectedState, expected, expectedGeneration)
	return true, errors.Join(disconnectErr, idleDrainErr)
}

func (m *Manager) finishRetiredClientResetWhenDone(
	sessionKey string,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
	drain *idleMessageDrain,
) {
	go func() {
		_ = expected.Disconnect(context.Background())
		_ = waitIdleMessageDrain(context.Background(), drain)
		m.clearRetiredClient(sessionKey, expectedState, expected, expectedGeneration)
	}()
}

func (m *Manager) clearRetiredClient(
	sessionKey string,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state != expectedState || state == nil || state.Closing || state.Client != expected ||
		state.StartupGeneration != expectedGeneration {
		return
	}
	state.Client = nil
	state.RuntimeKind = ""
	state.AgentID = ""
	state.ProcessPolicyFingerprint = ""
	state.ContextUsageByAgent = nil
	state.HasSubagentHistory = false
	// 活动 startup 可能已经在锁外重配置刚退休的 client。此时保留同一
	// state，让它回锁后看到 Client 已清空并沿统一路径创建新 client。
	if m.removeClientlessSessionIfIdleLocked(sessionKey, expectedState, nil) {
		return
	}
	m.touchStateLocked(state)
}

func (m *Manager) closeSession(
	ctx context.Context,
	sessionKey string,
	expectedState *sessionState,
	expected Client,
	expectedGeneration uint64,
	conditional bool,
	blockStartups bool,
	excludedOwnerStartup *ownerStartupLease,
) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	var closeGate *sessionStartupGate
	if blockStartups {
		// gate fence 与 session 的 closing 转换共享同一临界区，避免关闭与
		// 新 BeginClientStartup 之间出现无 gate/state 的 ABA 窗口。
		closeGate = m.beginSessionCloseGateLocked(sessionKey)
		defer m.releaseSessionCloseGate(sessionKey, closeGate)
	}
	if conditional {
		state := m.sessions[sessionKey]
		if state != expectedState || state == nil || state.Client != expected ||
			state.StartupGeneration != expectedGeneration {
			m.mu.Unlock()
			return false, nil
		}
	}
	target, started, closeDone := m.beginSessionCloseLocked(sessionKey)
	if !started {
		m.mu.Unlock()
		return closeDone != nil, waitSessionClose(ctx, closeDone)
	}
	reapPlan, reapFlight := m.beginOwnerReapLocked(
		target.ownerUserID,
		excludedOwnerStartup,
		false,
	)
	m.mu.Unlock()

	cancelSessionCloseTarget(target)
	m.startOwnerReap(reapPlan)

	var disconnectErr error
	if target.client != nil {
		disconnectErr = target.client.Disconnect(ctx)
	}
	idleDrainErr := waitIdleMessageDrain(ctx, target.idleMessageDrain)
	waitBackgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
	waitRoundErr := waitRoundDoneForClose(ctx, target.roundDone)
	clientCleanupPending := errors.Is(disconnectErr, context.Canceled) ||
		errors.Is(disconnectErr, context.DeadlineExceeded)
	if clientCleanupPending || idleDrainErr != nil || waitRoundErr != nil || waitBackgroundErr != nil {
		// context 可能先于后台写盘任务结束；即使 round 已退出，也不能
		// 删除 session 状态；client cleanup 也必须保留同一生命周期栅栏。
		m.finishSessionCloseWhenDone(target, clientCleanupPending)
	} else {
		m.finishSessionClose(target)
	}
	reaperErr := waitOwnerReap(ctx, reapFlight)
	return true, errors.Join(reaperErr, disconnectErr, idleDrainErr, waitBackgroundErr, waitRoundErr)
}

func cancelSessionCloseTarget(target *sessionCloseTarget) {
	if target == nil {
		return
	}
	if target.idleMessageDrain != nil {
		target.idleMessageDrain.cancel()
	}
	for _, cancel := range target.roundCancels {
		if cancel != nil {
			cancel()
		}
	}
	for _, cancel := range target.backgroundCancels {
		if cancel != nil {
			cancel()
		}
	}
}
