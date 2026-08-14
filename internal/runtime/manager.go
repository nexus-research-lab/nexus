// INPUT: SDK client factory 与 session/round 生命周期状态。
// OUTPUT: 并发安全的 runtime session 状态管理器。
// POS: runtime client、round、guidance 与 Goal accounting 的共享状态根。
package runtime

import (
	"context"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sessionState struct {
	Client                   Client
	StartupGeneration        uint64
	ContextUsageByAgent      map[string]protocol.ContextUsageData
	AgentID                  string
	Rounds                   roundRegistry
	BackgroundTasks          map[uint64]context.CancelFunc
	BackgroundDone           chan struct{}
	NextBackgroundTaskID     uint64
	Closing                  bool
	CloseDone                chan struct{}
	GuidedInputs             []GuidedInput
	SubagentHooks            map[string]SubagentHookCallbacks
	SubagentHookBindings     map[string]subagentHookBinding
	NextSubagentBindingSeq   uint64
	IdleMessageDrain         *idleMessageDrain
	RuntimeKind              agentclient.RuntimeKind
	OwnerUserID              string
	ProcessPolicyFingerprint string
	CacheSurface             CacheSurfaceProfile
	HasSubagentHistory       bool
	LastUsedAt               time.Time
}

type sessionStartupGate struct {
	token       chan struct{}
	refs        int
	closeBlocks int
	closeEpoch  uint64
}

// Manager 管理 session_key -> SDK client 与运行中 round。
type Manager struct {
	mu                    sync.RWMutex
	sessions              map[string]*sessionState
	startupGates          map[string]*sessionStartupGate
	revokedAgents         map[agentRuntimeIdentity]struct{}
	revokedSessionKeys    map[string]struct{}
	sessionDeletionBlocks map[string]uint64
	nextSessionDeletionID uint64
	factory               Factory
	now                   func() time.Time
	ownerProcessReaper    OwnerProcessReaper
	owners                map[string]*ownerLifecycle
	// subagentUsageTotals 只服务非 SQL goal provider 的兼容路径；
	// 放在 Manager 根上，避免 idle session 回收后立刻丢失高水位。
	subagentUsageTotals map[string]int64
}

// OwnerProcessReaper 在 owner 权限撤销时回收脱离父进程的 runtime 子树。
type OwnerProcessReaper interface {
	ReapOwnerProcesses(context.Context, string) error
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
		sessions:              make(map[string]*sessionState),
		startupGates:          make(map[string]*sessionStartupGate),
		revokedAgents:         make(map[agentRuntimeIdentity]struct{}),
		revokedSessionKeys:    make(map[string]struct{}),
		sessionDeletionBlocks: make(map[string]uint64),
		factory:               factory,
		now:                   time.Now,
		owners:                make(map[string]*ownerLifecycle),
		subagentUsageTotals:   make(map[string]int64),
	}
}

// SetOwnerProcessReaper 注入 owner 级 cgroup 回收器。
func (m *Manager) SetOwnerProcessReaper(reaper OwnerProcessReaper) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.ownerProcessReaper = reaper
	m.mu.Unlock()
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			ContextUsageByAgent:  make(map[string]protocol.ContextUsageData),
			BackgroundTasks:      make(map[uint64]context.CancelFunc),
			BackgroundDone:       closedSignal(),
			SubagentHooks:        make(map[string]SubagentHookCallbacks),
			SubagentHookBindings: make(map[string]subagentHookBinding),
		}
		m.sessions[sessionKey] = state
	}
	if state.LastUsedAt.IsZero() {
		m.touchStateLocked(state)
	}
	return state
}

// removeClientlessSessionIfIdleLocked 回收已经没有 client 与异步生命周期的空状态。
// expectedState 非空时同时校验 state 身份，避免旧任务退出时误删同 key 的新状态；
// 活动 startup 默认阻止删除，只有持有 token 的释放路径可传入自己的 gate。
// 调用者必须持有 Manager.mu。
func (m *Manager) removeClientlessSessionIfIdleLocked(
	sessionKey string,
	expectedState *sessionState,
	allowedStartupGate *sessionStartupGate,
) bool {
	state := m.sessions[sessionKey]
	if state == nil ||
		(expectedState != nil && state != expectedState) ||
		(m.startupGates[sessionKey] != nil && m.startupGates[sessionKey] != allowedStartupGate) ||
		state.Closing ||
		state.Client != nil ||
		len(state.ContextUsageByAgent) > 0 ||
		len(state.BackgroundTasks) > 0 ||
		!state.Rounds.empty() ||
		len(state.GuidedInputs) > 0 ||
		len(state.SubagentHooks) > 0 ||
		len(state.SubagentHookBindings) > 0 ||
		state.HasSubagentHistory ||
		state.IdleMessageDrain != nil {
		return false
	}
	delete(m.sessions, sessionKey)
	return true
}

func closedSignal() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (m *Manager) touchStateLocked(state *sessionState) {
	if state == nil {
		return
	}
	state.LastUsedAt = m.nowTime().UTC()
}

func (m *Manager) nowTime() time.Time {
	if m.now == nil {
		return time.Now()
	}
	return m.now()
}
