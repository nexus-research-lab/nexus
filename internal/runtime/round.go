// INPUT: session/round 标识、取消函数与权限模式更新。
// OUTPUT: round 注册、完成清理、查询与 Agent 级权限同步。
// POS: runtime Manager 的 round 生命周期入口。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// ErrRuntimeRoundAlreadyRunning 表示同一 round 已经登记为运行中。
var ErrRuntimeRoundAlreadyRunning = errors.New("runtime round is already running")

// ErrRuntimeProviderInterruptInProgress 表示 session-wide provider interrupt
// 正占用当前 client，successor 必须等待该窗口结束后再登记。
var ErrRuntimeProviderInterruptInProgress = errors.New("runtime provider interrupt is in progress")

var errRuntimeRoundInvalid = errors.New("runtime round requires session key and round id")

// goalAccountingHooks 聚合一个 round 的 Goal 计量回调与 revision fence。
type goalAccountingHooks struct {
	flush             GoalAccountingFlush
	clear             GoalAccountingClear
	finalize          GoalAccountingFinalize
	activate          GoalAccountingActivate
	guard             goalAccountingGuard
	objectiveRevision *atomic.Int64
	goalID            func() string
}

func (h goalAccountingHooks) empty() bool {
	return h.flush == nil &&
		h.clear == nil &&
		h.finalize == nil &&
		h.activate == nil &&
		h.guard.consumed == nil &&
		h.objectiveRevision == nil &&
		h.goalID == nil
}

// roundState 保存一个 round 从运行到收尾所需的全部状态。
type roundState struct {
	running      bool
	cancel       context.CancelFunc
	done         chan struct{}
	interruption string
	goal         goalAccountingHooks
}

func (s *roundState) empty() bool {
	return !s.running &&
		s.cancel == nil &&
		s.done == nil &&
		s.interruption == "" &&
		s.goal.empty()
}

// roundRegistry 以 round ID 为唯一键管理执行、退出和 Goal accounting 状态。
type roundRegistry struct {
	items                    map[string]*roundState
	providerInterruptRoundID string
}

func (r *roundRegistry) ensure(roundID string) *roundState {
	state := r.items[roundID]
	if state == nil {
		state = &roundState{}
		r.items[roundID] = state
	}
	return state
}

func (r *roundRegistry) get(roundID string) *roundState {
	if r == nil {
		return nil
	}
	return r.items[roundID]
}

func (r *roundRegistry) matchingIDs(match func(*roundState) bool) []string {
	if r == nil || len(r.items) == 0 {
		return nil
	}
	roundIDs := make([]string, 0, len(r.items))
	for roundID, state := range r.items {
		if state != nil && match(state) {
			roundIDs = append(roundIDs, roundID)
		}
	}
	slices.Sort(roundIDs)
	return roundIDs
}

func (r *roundRegistry) runningIDs() []string {
	return r.matchingIDs(func(state *roundState) bool { return state.running })
}

func (r *roundRegistry) active() bool {
	if r == nil {
		return false
	}
	for _, state := range r.items {
		if state != nil && state.running {
			return true
		}
	}
	return false
}

func (r *roundRegistry) runningCount() int {
	count := 0
	if r == nil {
		return count
	}
	for _, state := range r.items {
		if state != nil && state.running {
			count++
		}
	}
	return count
}

func (r *roundRegistry) selected(roundIDs []string) []*roundState {
	if r == nil || len(r.items) == 0 {
		return nil
	}
	if len(roundIDs) == 0 {
		roundIDs = r.matchingIDs(func(*roundState) bool { return true })
	}
	states := make([]*roundState, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		if state := r.items[roundID]; state != nil {
			states = append(states, state)
		}
	}
	return states
}

func (r *roundRegistry) cancelFuncs(roundIDs ...string) []context.CancelFunc {
	states := r.selected(roundIDs)
	cancels := make([]context.CancelFunc, 0, len(states))
	for _, state := range states {
		if state.cancel != nil {
			cancels = append(cancels, state.cancel)
		}
	}
	return cancels
}

func (r *roundRegistry) doneSignals(roundIDs ...string) []chan struct{} {
	states := r.selected(roundIDs)
	done := make([]chan struct{}, 0, len(states))
	for _, state := range states {
		if state.done != nil {
			done = append(done, state.done)
		}
	}
	return done
}

func (r *roundRegistry) prune(roundID string) {
	if state := r.get(roundID); state != nil && state.empty() {
		delete(r.items, roundID)
	}
}

func (r *roundRegistry) cleanup(roundID string) {
	state := r.get(roundID)
	if state == nil {
		return
	}
	delete(r.items, roundID)
	if state.done != nil {
		close(state.done)
	}
}

func (r *roundRegistry) beginProviderInterrupt(roundID string) {
	r.providerInterruptRoundID = roundID
}

func (r *roundRegistry) providerInterrupting() bool {
	return r != nil && r.providerInterruptRoundID != ""
}

func (r *roundRegistry) finishProviderInterrupt(roundID string) bool {
	if r == nil || r.providerInterruptRoundID != roundID {
		return false
	}
	r.providerInterruptRoundID = ""
	return true
}

func (r *roundRegistry) empty() bool {
	return r == nil || len(r.items) == 0 && !r.providerInterrupting()
}

// StartRound 注册运行中的 round，并记录其取消函数。
//
// 返回错误表示 round 未登记成功；传入的 cancel 仍会被调用，避免调用方
// 持有的执行上下文泄漏。
func (m *Manager) StartRound(
	ctx context.Context,
	sessionKey string,
	roundID string,
	cancel context.CancelFunc,
) error {
	fail := func(err error) error {
		if cancel != nil {
			cancel()
		}
		return err
	}
	if sessionKey == "" || roundID == "" {
		return fail(errRuntimeRoundInvalid)
	}
	sessionKey = strings.TrimSpace(sessionKey)
	for {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}

		m.mu.Lock()
		ownerUserID := ""
		agentID := runtimeSessionAgentID(sessionKey)
		if current := m.sessions[sessionKey]; current != nil {
			ownerUserID = strings.TrimSpace(current.OwnerUserID)
			if strings.TrimSpace(current.AgentID) != "" {
				agentID = strings.TrimSpace(current.AgentID)
			}
		}
		if err := m.runtimeAgentAdmissionErrorLocked(sessionKey, ownerUserID, agentID); err != nil {
			m.mu.Unlock()
			return fail(err)
		}
		state := m.ensureStateLocked(sessionKey)
		if state.Closing {
			m.mu.Unlock()
			return fail(ErrRuntimeSessionClosing)
		}
		if state.Rounds.providerInterrupting() {
			m.mu.Unlock()
			return fail(ErrRuntimeProviderInterruptInProgress)
		}
		if drain := state.IdleMessageDrain; drain != nil {
			drain.cancel()
			m.mu.Unlock()
			if err := waitIdleMessageDrain(ctx, drain); err != nil {
				return fail(err)
			}
			continue
		}
		round := state.Rounds.get(roundID)
		if round != nil && round.running {
			m.mu.Unlock()
			return fail(ErrRuntimeRoundAlreadyRunning)
		}
		round = state.Rounds.ensure(roundID)
		round.running = true
		round.cancel = cancel
		round.interruption = ""
		if round.done == nil {
			round.done = make(chan struct{})
		}
		m.touchStateLocked(state)
		m.mu.Unlock()
		return nil
	}
}

// MarkRoundTerminal 把 round 从运行态中移除，但保留退出信号。
//
// runtime 已给出终态后，调用方通常还要持久化结果、广播事件并登记后续
// workspace 任务。关闭 session 必须等待这些收尾完成，不能把“用户看到终态”
// 等同于“round 协程已经退出”。
func (m *Manager) MarkRoundTerminal(sessionKey string, roundID string) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		return
	}
	m.markRoundTerminalLocked(state, roundID)
}

// MarkRoundFinished 标记 round 的全部收尾已经退出，并唤醒关闭等待者。
func (m *Manager) MarkRoundFinished(sessionKey string, roundID string) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state.Rounds.get(roundID) == nil {
		m.mu.Unlock()
		return
	}
	m.markRoundTerminalLocked(state, roundID)
	state.Rounds.cleanup(roundID)
	m.removeClientlessSessionIfIdleLocked(sessionKey, state, nil)
	observer := m.roundFinishedObserver
	m.mu.Unlock()
	if observer != nil {
		observer(sessionKey, roundID)
	}
}

func (m *Manager) markRoundTerminalLocked(state *sessionState, roundID string) {
	if state == nil {
		return
	}
	if round := state.Rounds.get(roundID); round != nil {
		round.running = false
	}
	m.touchStateLocked(state)
	if !state.Rounds.active() {
		state.GuidedInputs = nil
	}
}

// GetRunningRoundIDs 返回当前 session 的运行中轮次。
func (m *Manager) GetRunningRoundIDs(sessionKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		return []string{}
	}
	return state.Rounds.runningIDs()
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
		if state == nil || !state.Rounds.active() {
			continue
		}
		if !strings.HasPrefix(sessionKey, "agent:"+agentID+":") {
			continue
		}
		total += state.Rounds.runningCount()
	}
	return total
}

// SetPermissionModeForAgent 将权限模式热同步到指定 agent 已存在的 DM runtime。
func (m *Manager) SetPermissionModeForAgent(ctx context.Context, agentID string, mode sdkpermission.Mode) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil
	}
	prefix := "agent:" + agentID + ":"
	type permissionModeTarget struct {
		sessionKey string
		client     Client
	}
	targets := make([]permissionModeTarget, 0)
	m.mu.RLock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Closing || state.Client == nil || !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		targets = append(targets, permissionModeTarget{sessionKey: sessionKey, client: state.Client})
	}
	m.mu.RUnlock()
	errs := make([]error, 0)
	for _, target := range targets {
		if err := target.client.SetPermissionMode(ctx, mode); err != nil {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			closeErr := m.CloseSession(closeCtx, target.sessionKey)
			cancel()
			errs = append(errs, fmt.Errorf(
				"session %s 权限热同步失败，已关闭旧 runtime: %w",
				target.sessionKey,
				errors.Join(err, closeErr),
			))
		}
	}
	return errors.Join(errs...)
}

type environmentUpdater interface {
	UpdateEnvironment(context.Context, map[string]string) error
}

var mutableRuntimeEnvironmentKeys = map[string]struct{}{
	"NEXUS_WEBSEARCH_API_KEY": {},
	"NEXUS_WEBSEARCH_CONFIG":  {},
}

func validateRuntimeEnvironmentUpdate(environment map[string]string) error {
	for key := range environment {
		normalizedKey := strings.ToUpper(strings.TrimSpace(key))
		if _, ok := mutableRuntimeEnvironmentKeys[normalizedKey]; !ok {
			return fmt.Errorf("runtime environment key cannot be updated: %s", key)
		}
	}
	return nil
}

// UpdateEnvironmentForAgent 将 WebSearch 等运行期环境同步到指定 Agent 的 nxs 会话。
func (m *Manager) UpdateEnvironmentForAgent(ctx context.Context, agentID string, environment map[string]string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || len(environment) == 0 {
		return nil
	}
	if err := validateRuntimeEnvironmentUpdate(environment); err != nil {
		return err
	}
	prefix := "agent:" + agentID + ":"
	clients := make([]environmentUpdater, 0)
	m.mu.RLock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Closing || state.Client == nil || state.RuntimeKind != agentclient.RuntimeNXS || !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		updater, ok := state.Client.(environmentUpdater)
		if ok {
			clients = append(clients, updater)
		}
	}
	m.mu.RUnlock()
	errs := make([]error, 0)
	for _, client := range clients {
		if err := client.UpdateEnvironment(ctx, environment); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
