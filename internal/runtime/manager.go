// INPUT: SDK client factory 与 session/round 生命周期状态。
// OUTPUT: 并发安全的 runtime session 状态管理器。
// POS: runtime client、round、guidance 与 Goal accounting 的共享状态根。
package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sessionState struct {
	Client                   Client
	StartupGeneration        uint64
	ProcessGeneration        uint64 // Immutable while Client is reused; zero means unsupervised.
	ContextUsageByAgent      map[string]protocol.ContextUsageData
	AgentID                  string
	Rounds                   roundRegistry
	BackgroundTasks          map[uint64]context.CancelFunc
	BackgroundDone           chan struct{}
	NextBackgroundTaskID     uint64
	Closing                  bool
	CloseDone                chan struct{}
	CloseError               error // 关闭尝试完成但失败时保留状态，禁止复用同一 Session。
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
	sandboxReceiptStore   SandboxPolicyReceiptStore
	sandboxSupervisor     *SandboxProcessSupervisor
	roundFinishedObserver func(string, string)
	owners                map[string]*ownerLifecycle
	shutdownDone          chan struct{}
	shutdownErr           error
	activeStartups        int
	startupsDrained       chan struct{}
	// subagentUsageTotals 只服务非 SQL goal provider 的兼容路径；
	// 放在 Manager 根上，避免 idle session 回收后立刻丢失高水位。
	subagentUsageTotals map[string]int64
}

// OwnerProcessReaper 在 owner 权限撤销时回收脱离父进程的 runtime 子树。
type OwnerProcessReaper interface {
	ReapOwnerProcesses(context.Context, string) error
}

// SandboxPolicyReceiptStore persists the effective desktop sandbox policy
// after a runtime generation has connected. The record is audit evidence only
// and never an authorization or OS-isolation proof.
type SandboxPolicyReceiptStore interface {
	Save(context.Context, protocol.SandboxPolicyReceiptSnapshot) error
	UpdatePhase(context.Context, string, string, uint64, protocol.SandboxPolicyReceiptPhase, string) error
}

// SandboxPolicyReceiptReader is the recovery read side of a configured store.
// Stores used for owner-bound startup must provide it; a write-only sink can
// still record lifecycle transitions but cannot admit a fresh runtime.
type SandboxPolicyReceiptReader interface {
	Latest(context.Context, string, string) (protocol.SandboxPolicyReceiptSnapshot, bool, error)
}

// SandboxProcessReceiptReader 读取执行前的宿主持久启动事实。
// 当前数据库仓储同时实现本接口；不以策略回执或用户 scratch 文件代替它。
type SandboxProcessReceiptReader interface {
	LatestProcess(context.Context, string, string) (protocol.SandboxProcessSnapshot, bool, error)
}

// NewManager 创建运行时管理器。
func NewManager() *Manager {
	return NewManagerWithFactory(defaultFactory{})
}

// NewManagerWithFactory 使用非空自定义 factory 创建运行时管理器；默认实现使用 NewManager。
func NewManagerWithFactory(factory Factory) *Manager {
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
	m.mu.Lock()
	m.ownerProcessReaper = reaper
	m.mu.Unlock()
}

// SetSandboxPolicyReceiptStore installs the durable audit/recovery sink. A nil
// sink keeps the in-memory diagnostic behavior used by lightweight callers/tests.
func (m *Manager) SetSandboxPolicyReceiptStore(store SandboxPolicyReceiptStore) {
	m.mu.Lock()
	m.sandboxReceiptStore = store
	m.mu.Unlock()
}

func (m *Manager) updateSandboxReceiptPhase(
	ownerUserID string,
	sessionKey string,
	generation uint64,
	client Client,
	receipt *SandboxEffectivePolicyReceipt,
	phase protocol.SandboxPolicyReceiptPhase,
	reason string,
) error {
	if client == nil || generation == 0 {
		return nil
	}
	if receipt == nil {
		return nil
	}
	m.mu.RLock()
	store := m.sandboxReceiptStore
	m.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.UpdatePhase(
		context.Background(),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sessionKey),
		generation,
		phase,
		boundedSandboxReceiptReason(reason),
	)
}

// PersistentSandboxPolicyReceipt reads the latest durable receipt after a
// process restart. It deliberately returns no in-memory session and does not
// make a persisted receipt look like a currently connected runtime.
func (m *Manager) PersistentSandboxPolicyReceipt(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*SandboxEffectivePolicyReceipt, bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return nil, false, errors.New("owner_user_id and session_key are required")
	}
	m.mu.RLock()
	store := m.sandboxReceiptStore
	m.mu.RUnlock()
	reader, ok := store.(SandboxPolicyReceiptReader)
	if !ok {
		return nil, false, nil
	}
	snapshot, found, err := reader.Latest(ctx, ownerUserID, sessionKey)
	if err != nil || !found {
		return nil, found, err
	}
	receipt, err := sandboxPolicyReceiptFromSnapshot(snapshot)
	if err != nil {
		return nil, false, err
	}
	return receipt, true, nil
}

// SetRoundFinishedObserver 注入物理 round 完成后的业务清理入口。
func (m *Manager) SetRoundFinishedObserver(observer func(string, string)) {
	m.mu.Lock()
	m.roundFinishedObserver = observer
	m.mu.Unlock()
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			ContextUsageByAgent:  make(map[string]protocol.ContextUsageData),
			Rounds:               roundRegistry{items: make(map[string]*roundState)},
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
	return m.now()
}
