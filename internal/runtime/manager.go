// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：manager.go
// @Date   ：2026/04/11 02:10:00
// @Author ：leemysw
// 2026/04/11 02:10:00   Create
// =====================================================

package runtime

import (
	"context"
	"sort"
	"strings"
	"sync"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

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
	if options.PermissionMode != "" {
		if err := c.SetPermissionMode(ctx, options.PermissionMode); err != nil {
			return err
		}
	}
	if strings.TrimSpace(options.Model) != "" {
		if err := c.SetModel(ctx, options.Model); err != nil {
			return err
		}
	}
	if options.MaxThinkingTokens > 0 {
		if err := c.SetMaxThinkingTokens(ctx, options.MaxThinkingTokens); err != nil {
			return err
		}
	}
	return nil
}

func (f defaultFactory) New(options agentclient.Options) Client {
	return WrapSDKClient(agentclient.New(options))
}

type sessionState struct {
	Client        Client
	RunningRounds map[string]struct{}
	RoundCancels  map[string]context.CancelFunc
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
	if cancel != nil {
		state.RoundCancels[roundID] = cancel
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
func (m *Manager) InterruptSession(ctx context.Context, sessionKey string) ([]string, error) {
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		m.mu.Unlock()
		return nil, nil
	}

	roundIDs := make([]string, 0, len(state.RunningRounds))
	cancels := make([]context.CancelFunc, 0, len(state.RoundCancels))
	for roundID := range state.RunningRounds {
		roundIDs = append(roundIDs, roundID)
	}
	for roundID, cancel := range state.RoundCancels {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
		delete(state.RoundCancels, roundID)
	}
	for roundID := range state.RunningRounds {
		delete(state.RunningRounds, roundID)
	}
	client := state.Client
	m.mu.Unlock()

	sort.Strings(roundIDs)
	for _, cancel := range cancels {
		cancel()
	}
	if client == nil {
		return roundIDs, nil
	}
	return roundIDs, client.Interrupt(ctx)
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
	return state.Client.Disconnect(ctx)
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			RunningRounds: make(map[string]struct{}),
			RoundCancels:  make(map[string]context.CancelFunc),
		}
		m.sessions[sessionKey] = state
	}
	return state
}

func sessionBelongsToAgent(sessionKey string, agentID string) bool {
	return strings.HasPrefix(sessionKey, "agent:"+agentID+":")
}
