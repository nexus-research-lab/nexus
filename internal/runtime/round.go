// INPUT: session/round 标识、取消函数与权限模式更新。
// OUTPUT: round 注册、完成清理、查询与 Agent 级权限同步。
// POS: runtime Manager 的 round 生命周期入口。
package runtime

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// StartRound 注册运行中的 round，并记录其取消函数。
//
// 返回 false 表示 session 已进入关闭流程，或唯一旧 round 正在执行
// session-wide provider interrupt；调用方不得在该窗口启动 successor。
// 传入的 cancel 仍会被调用，避免调用方持有的执行上下文泄漏。
func (m *Manager) StartRound(sessionKey string, roundID string, cancel context.CancelFunc) bool {
	if sessionKey == "" || roundID == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.Closing || state.ProviderInterruptRoundID != "" {
		if cancel != nil {
			cancel()
		}
		return false
	}
	if state.IdleMessageCancel != nil {
		state.IdleMessageCancel()
		state.IdleMessageCancel = nil
	}
	state.RunningRounds[roundID] = struct{}{}
	m.touchStateLocked(state)
	delete(state.Interruptions, roundID)
	if cancel != nil {
		state.RoundCancels[roundID] = cancel
	}
	if _, exists := state.RoundDone[roundID]; !exists {
		state.RoundDone[roundID] = make(chan struct{})
	}
	return true
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
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		return
	}
	m.markRoundTerminalLocked(state, roundID)
	delete(state.RoundCancels, roundID)
	delete(state.Interruptions, roundID)
	delete(state.GoalAccountingFlushers, roundID)
	delete(state.GoalAccountingClearers, roundID)
	delete(state.GoalAccountingFinalizers, roundID)
	delete(state.GoalAccountingActivators, roundID)
	delete(state.GoalAccountingGuards, roundID)
	delete(state.GoalObjectiveRevisions, roundID)
	if done, ok := state.RoundDone[roundID]; ok {
		close(done)
		delete(state.RoundDone, roundID)
	}
	m.removeClientlessSessionIfIdleLocked(sessionKey, state, nil)
}

func (m *Manager) markRoundTerminalLocked(state *sessionState, roundID string) {
	if state == nil {
		return
	}
	delete(state.RunningRounds, roundID)
	m.touchStateLocked(state)
	if len(state.RunningRounds) == 0 {
		state.GuidedInputs = nil
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
	return slices.Sorted(maps.Keys(state.RunningRounds))
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
		if !strings.HasPrefix(sessionKey, "agent:"+agentID+":") {
			continue
		}
		total += len(state.RunningRounds)
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
	clients := make([]Client, 0)
	m.mu.RLock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Closing || state.Client == nil || !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		clients = append(clients, state.Client)
	}
	m.mu.RUnlock()
	for _, client := range clients {
		if err := client.SetPermissionMode(ctx, mode); err != nil {
			return err
		}
	}
	return nil
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
	for _, client := range clients {
		if err := client.UpdateEnvironment(ctx, environment); err != nil {
			return err
		}
	}
	return nil
}
