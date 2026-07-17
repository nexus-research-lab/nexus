// INPUT: session/round 标识、取消函数与权限模式更新。
// OUTPUT: round 注册、完成清理、查询与 Agent 级权限同步。
// POS: runtime Manager 的 round 生命周期入口。
package runtime

import (
	"context"
	"maps"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// StartRound 注册运行中的 round，并记录其取消函数。
func (m *Manager) StartRound(sessionKey string, roundID string, cancel context.CancelFunc) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
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
	delete(state.GoalAccountingFlushers, roundID)
	delete(state.GoalAccountingClearers, roundID)
	delete(state.GoalAccountingActivators, roundID)
	delete(state.GoalObjectiveRevisions, roundID)
	m.touchStateLocked(state)
	if len(state.RunningRounds) == 0 {
		state.GuidedInputs = nil
	}
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
		if state == nil || state.Client == nil || !strings.HasPrefix(sessionKey, prefix) {
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

// UpdateEnvironmentForAgent 将 WebSearch 等运行期环境同步到指定 Agent 的 nxs 会话。
func (m *Manager) UpdateEnvironmentForAgent(ctx context.Context, agentID string, environment map[string]string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || len(environment) == 0 {
		return nil
	}
	prefix := "agent:" + agentID + ":"
	clients := make([]environmentUpdater, 0)
	m.mu.RLock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Client == nil || state.RuntimeKind != agentclient.RuntimeNXS || !strings.HasPrefix(sessionKey, prefix) {
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
