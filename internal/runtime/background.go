package runtime

import (
	"context"
	"strings"
)

// StartBackgroundTask 在指定 runtime session 生命周期内运行一个后台任务。
//
// 任务通常负责队列接力、Goal 续跑等会继续写入 workspace 的工作。绑定到
// session 后，关闭 session 会先取消并等待这些任务，避免 owner 数据目录在
// 清理或权限撤销后仍被异步写入。
func (m *Manager) StartBackgroundTask(
	sessionKey string,
	task func(context.Context),
) bool {
	return m.startBackgroundTask(sessionKey, "", task)
}

// StartBackgroundTaskForOwner 在指定 owner 的 runtime session 生命周期内运行
// 后台任务。即使 session 尚未建立 client，也会登记 owner，确保权限撤销或
// Room/DM 清理时能够取消这类临时文件任务。
func (m *Manager) StartBackgroundTaskForOwner(
	sessionKey string,
	ownerUserID string,
	task func(context.Context),
) bool {
	return m.startBackgroundTask(sessionKey, strings.TrimSpace(ownerUserID), task)
}

func (m *Manager) startBackgroundTask(
	sessionKey string,
	ownerUserID string,
	task func(context.Context),
) bool {
	if task == nil {
		return false
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return false
	}

	m.mu.Lock()
	if ownerUserID != "" && m.ownerReapActiveLocked(ownerUserID) {
		m.mu.Unlock()
		return false
	}
	if err := m.runtimeAgentAdmissionErrorLocked(
		sessionKey,
		ownerUserID,
		runtimeSessionAgentID(sessionKey),
	); err != nil {
		m.mu.Unlock()
		return false
	}
	// 队列接力可能发生在首个 runtime client 建立之前。先登记一个
	// 无 client 的 session 状态，才能把这段会写盘的工作纳入同一生命周期。
	state := m.ensureStateLocked(sessionKey)
	if state.Closing {
		m.mu.Unlock()
		return false
	}
	if ownerUserID != "" {
		if state.OwnerUserID != "" && state.OwnerUserID != ownerUserID {
			m.mu.Unlock()
			return false
		}
		state.OwnerUserID = ownerUserID
	}
	if len(state.BackgroundTasks) == 0 {
		state.BackgroundDone = make(chan struct{})
	}
	state.NextBackgroundTaskID++
	taskID := state.NextBackgroundTaskID
	taskContext, cancel := context.WithCancel(context.Background())
	state.BackgroundTasks[taskID] = cancel
	m.touchStateLocked(state)
	m.mu.Unlock()

	go func() {
		defer m.finishBackgroundTask(sessionKey, state, taskID)
		task(taskContext)
	}()
	return true
}

func (m *Manager) finishBackgroundTask(sessionKey string, state *sessionState, taskID uint64) {
	if state == nil {
		return
	}
	m.mu.Lock()
	delete(state.BackgroundTasks, taskID)
	if len(state.BackgroundTasks) == 0 && state.BackgroundDone != nil {
		close(state.BackgroundDone)
		state.BackgroundDone = nil
	}
	// 仅由后台队列任务临时创建、且从未建立 client/round 的状态不应
	// 长驻 manager；expected state 防止旧任务退出时误删同 key 的新状态。
	m.removeClientlessSessionIfIdleLocked(sessionKey, state, nil)
	m.mu.Unlock()
}

func waitBackgroundTasks(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitBackgroundTasks 等待 session 已登记的后台文件任务结束。
func (m *Manager) WaitBackgroundTasks(ctx context.Context, sessionKey string) error {
	sessionKey = strings.TrimSpace(sessionKey)
	m.mu.RLock()
	state := m.sessions[sessionKey]
	var done <-chan struct{}
	if state != nil {
		done = state.BackgroundDone
	}
	m.mu.RUnlock()
	return waitBackgroundTasks(ctx, done)
}
