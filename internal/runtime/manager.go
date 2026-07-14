// INPUT: SDK client factory 与 session/round 生命周期状态。
// OUTPUT: 并发安全的 runtime session 状态管理器。
// POS: runtime client、round、guidance 与 Goal accounting 的共享状态根。
package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type sessionState struct {
	Client                   Client
	RunningRounds            map[string]struct{}
	RoundCancels             map[string]context.CancelFunc
	RoundDone                map[string]chan struct{}
	Interruptions            map[string]string
	GoalAccountingFlushers   map[string]GoalAccountingFlush
	GoalAccountingClearers   map[string]GoalAccountingClear
	GoalAccountingActivators map[string]GoalAccountingActivate
	GoalObjectiveRevisions   map[string]*atomic.Int64
	GuidedInputs             []GuidedInput
	IdleMessageCancel        context.CancelFunc
	IdleMessageDrainID       int64
	RuntimeKind              agentclient.RuntimeKind
	HasSubagentHistory       bool
	LastUsedAt               time.Time
}

// Manager 管理 session_key -> SDK client 与运行中 round。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*sessionState
	factory  Factory
	now      func() time.Time
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
		now:      time.Now,
	}
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			RunningRounds:            make(map[string]struct{}),
			RoundCancels:             make(map[string]context.CancelFunc),
			RoundDone:                make(map[string]chan struct{}),
			Interruptions:            make(map[string]string),
			GoalAccountingFlushers:   make(map[string]GoalAccountingFlush),
			GoalAccountingClearers:   make(map[string]GoalAccountingClear),
			GoalAccountingActivators: make(map[string]GoalAccountingActivate),
			GoalObjectiveRevisions:   make(map[string]*atomic.Int64),
		}
		m.sessions[sessionKey] = state
	}
	if state.LastUsedAt.IsZero() {
		m.touchStateLocked(state)
	}
	return state
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
