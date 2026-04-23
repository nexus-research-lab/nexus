package runtime

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

const interruptForceCancelDelay = 150 * time.Millisecond

// Client 抽象出运行时需要的最小 SDK 能力，便于测试替身接入。
type Client interface {
	Connect(context.Context) error
	Query(context.Context, string) error
	ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage
	Interrupt(context.Context) error
	Disconnect(context.Context) error
	Reconfigure(context.Context, agentclient.Options) error
	SetPermissionMode(context.Context, sdkprotocol.PermissionMode) error
	SessionID() string
}

// Factory 负责创建 SDK client。
type Factory interface {
	New(agentclient.Options) Client
}

type defaultFactory struct{}

type sdkClientAdapter struct {
	*agentclient.Client
}

func WrapSDKClient(client *agentclient.Client) Client {
	if client == nil {
		return nil
	}
	return &sdkClientAdapter{Client: client}
}

func (c *sdkClientAdapter) Reconfigure(ctx context.Context, options agentclient.Options) error {
	if c == nil || c.Client == nil {
		return nil
	}
	return c.Client.Reconfigure(ctx, options)
}

func (f defaultFactory) New(options agentclient.Options) Client {
	return WrapSDKClient(agentclient.New(options))
}

type sessionState struct {
	Client        Client
	RunningRounds map[string]struct{}
	RoundCancels  map[string]context.CancelFunc
	RoundDone     map[string]chan struct{}
	Interruptions map[string]string
}

// Manager 管理 session_key -> SDK client 与运行中 round。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*sessionState
	factory  Factory
}

// NewManager 创建运行时管理器。
func NewManager() *Manager {
	return NewManagerWithFactory(defaultFactory{})
}

// NewManagerWithFactory 使用自定义 factory 创建运行时管理器。
func NewManagerWithFactory(factory Factory) *Manager {
	if factory == nil {
		factory = defaultFactory{}
	}
	return &Manager{
		sessions: make(map[string]*sessionState),
		factory:  factory,
	}
}

// GetOrCreate 获取或创建 client，并在复用时应用最新运行时配置。
func (m *Manager) GetOrCreate(ctx context.Context, sessionKey string, options agentclient.Options) (Client, error) {
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	m.mu.RUnlock()
	if ok && state.Client != nil {
		if err := state.Client.Reconfigure(ctx, options); err != nil {
			return nil, err
		}
		return state.Client, nil
	}

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	if state.Client == nil {
		state.Client = m.factory.New(options)
		m.mu.Unlock()
		return state.Client, nil
	}
	client := state.Client
	m.mu.Unlock()
	if err := client.Reconfigure(ctx, options); err != nil {
		return nil, err
	}
	return client, nil
}

// StartRound 注册运行中的 round，并记录其取消函数。
func (m *Manager) StartRound(sessionKey string, roundID string, cancel context.CancelFunc) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	state.RunningRounds[roundID] = struct{}{}
	delete(state.Interruptions, roundID)
	if cancel != nil {
		state.RoundCancels[roundID] = cancel
	}
	if _, exists := state.RoundDone[roundID]; !exists {
		state.RoundDone[roundID] = make(chan struct{})
	}
}

// MarkRoundFinished 把 round 从运行态中移除。
func (m *Manager) MarkRoundFinished(sessionKey string, roundID string) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		return
	}
	delete(state.RunningRounds, roundID)
	delete(state.RoundCancels, roundID)
	delete(state.Interruptions, roundID)
	if done, ok := state.RoundDone[roundID]; ok {
		close(done)
		delete(state.RoundDone, roundID)
	}
}

// GetRunningRoundIDs 返回当前 session 的运行中轮次。
func (m *Manager) GetRunningRoundIDs(sessionKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || len(state.RunningRounds) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(state.RunningRounds))
	for roundID := range state.RunningRounds {
		result = append(result, roundID)
	}
	sort.Strings(result)
	return result
}

// CountRunningRounds 统计指定 Agent 当前活跃 round 数量。
func (m *Manager) CountRunningRounds(agentID string) int {
	if agentID == "" {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for sessionKey, state := range m.sessions {
		if len(state.RunningRounds) == 0 {
			continue
		}
		if !sessionBelongsToAgent(sessionKey, agentID) {
			continue
		}
		total += len(state.RunningRounds)
	}
	return total
}

// InterruptSession 中断当前 session 的全部运行中 round。
func (m *Manager) InterruptSession(ctx context.Context, sessionKey string, reason string) ([]string, error) {
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		m.mu.RUnlock()
		return nil, nil
	}

	roundIDs := make([]string, 0, len(state.RunningRounds))
	doneSignals := make([]chan struct{}, 0, len(state.RoundDone))
	cancels := make([]context.CancelFunc, 0, len(state.RoundCancels))
	for roundID := range state.RunningRounds {
		roundIDs = append(roundIDs, roundID)
	}
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

	sort.Strings(roundIDs)
	if len(roundIDs) == 0 {
		return nil, nil
	}

	interruptReason := strings.TrimSpace(reason)

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
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
	for _, cancel := range state.RoundCancels {
		if cancel != nil {
			cancel()
		}
	}
	return state.Client.Disconnect(ctx)
}

// RecycleClient 仅回收指定 session 的运行时 client，保留 round 状态与中断状态。
func (m *Manager) RecycleClient(ctx context.Context, sessionKey string) error {
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || state.Client == nil {
		m.mu.Unlock()
		return nil
	}
	client := state.Client
	state.Client = nil
	m.mu.Unlock()
	err := client.Disconnect(ctx)
	if isIgnorableRecycleDisconnectError(err) {
		return nil
	}
	return err
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			RunningRounds: make(map[string]struct{}),
			RoundCancels:  make(map[string]context.CancelFunc),
			RoundDone:     make(map[string]chan struct{}),
			Interruptions: make(map[string]string),
		}
		m.sessions[sessionKey] = state
	}
	return state
}

func sessionBelongsToAgent(sessionKey string, agentID string) bool {
	return strings.HasPrefix(sessionKey, "agent:"+agentID+":")
}

func isIgnorableRecycleDisconnectError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, agentclient.ErrNotConnected) {
		return true
	}

	// 中文注释：回收坏掉的 runtime client 时，旧进程可能已经被系统杀死，
	// Disconnect 再次触碰 transport 会返回“文件已关闭 / 进程已退出”。
	// 这类错误不影响后续创建新 client，不能阻断重建链路。
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "file already closed") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "signal: killed") ||
		strings.Contains(message, "process: command exited with error")
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
