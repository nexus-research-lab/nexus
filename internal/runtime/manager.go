package runtime

import (
	"context"
	"sync"
	"time"
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
	GuidedInputs             []GuidedInput
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
