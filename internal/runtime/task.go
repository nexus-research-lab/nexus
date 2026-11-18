package runtime

import (
	"context"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// StopTask 停止指定 session 中的后台任务。
func (m *Manager) StopTask(ctx context.Context, sessionKey string, taskID string) error {
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	if sessionKey == "" || taskID == "" {
		return agentclient.ErrNotConnected
	}

	m.mu.Lock()
	state := m.sessions[sessionKey]
	var client Client
	if state != nil {
		client = state.Client
		if client != nil {
			m.touchStateLocked(state)
		}
	}
	m.mu.Unlock()
	if client == nil {
		return agentclient.ErrNotConnected
	}
	return client.StopTask(ctx, taskID)
}

// SendTaskMessage 向指定 session 的后台任务投递一条后续消息。
func (m *Manager) SendTaskMessage(ctx context.Context, sessionKey string, taskID string, message string, summary string) error {
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	message = strings.TrimSpace(message)
	if sessionKey == "" || taskID == "" || message == "" {
		return agentclient.ErrNotConnected
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	memoryErr := m.memoryAdmissionError()
	m.mu.Lock()
	state := m.sessions[sessionKey]
	if state == nil || state.Client == nil || state.Closing {
		m.mu.Unlock()
		return agentclient.ErrNotConnected
	}
	run := state.SubagentRuns[taskID]
	if !run.busy() {
		if memoryErr != nil {
			m.mu.Unlock()
			return memoryErr
		}
		if m.subagentLimitReachedLocked() {
			m.mu.Unlock()
			return ErrRuntimeResourceBusy
		}
	}
	client := state.Client
	run = state.subagentRun(taskID)
	seen := run.observations
	run.sending++
	m.touchStateLocked(state)
	m.mu.Unlock()

	var receipt sdkprotocol.TaskRunState
	var err error
	if sender, ok := client.(interface {
		SendTaskMessageWithState(context.Context, string, string, string) (sdkprotocol.TaskRunState, error)
	}); ok {
		receipt, err = sender.SendTaskMessageWithState(ctx, taskID, message, strings.TrimSpace(summary))
	} else {
		err = client.SendTaskMessage(ctx, taskID, message, strings.TrimSpace(summary))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.sessions[sessionKey]; current != state || current.Client != client || current.SubagentRuns[taskID] != run {
		return err
	}
	run.sending--
	if err == nil {
		if receipt.Generation > 0 {
			run.observe(receipt.Generation, receipt.Status)
		} else if run.observations == seen && !run.running {
			// 旧 runtime 的排队回执不含代次，等后续 running 事件接管此名额。
			run.awaiting = true
		}
	}
	if !run.busy() {
		// 已结束的任务不再保留条目；同一有序消息流里不会再出现更早代次的事件。
		delete(state.SubagentRuns, taskID)
		m.notifyResourceChangeLocked()
	}
	return err
}

// subagentRun 在消息投递、任务事件及回收判定间共享同一物理任务的状态。
type subagentRun struct {
	generation   int
	observations uint64
	sending      int
	running      bool
	awaiting     bool
}

func (r *subagentRun) busy() bool { return r != nil && (r.running || r.awaiting || r.sending > 0) }

func (s *sessionState) subagentRun(id string) *subagentRun {
	if s.SubagentRuns == nil {
		s.SubagentRuns = make(map[string]*subagentRun)
	}
	if s.SubagentRuns[id] == nil {
		s.SubagentRuns[id] = &subagentRun{}
	}
	return s.SubagentRuns[id]
}

// observe 同一代次的终态不可回退；上一轮的迟到通知不能释放续跑的名额。
func (r *subagentRun) observe(generation int, status string) {
	var running bool
	switch status {
	case "running":
		running = true
	case "completed", "failed", "error", "stopped", "killed", "canceled", "cancelled", "deleted":
	default:
		return
	}
	if generation < r.generation || (generation > 0 && generation == r.generation && !r.running && running) {
		return
	}
	if generation == 0 && r.awaiting && !running {
		return
	}
	r.generation, r.running, r.awaiting = generation, running, false
	r.observations++
}

// ObserveSubagentRun 消费同一 client 的有序任务消息；父轮与 idle drain 共用此入口。
func (m *Manager) ObserveSubagentRun(sessionKey string, client Client, msg sdkprotocol.ReceivedMessage) {
	started, updated, finished := msg.TaskStarted, msg.TaskUpdated, msg.TaskNotification
	if msg.System != nil {
		started, updated, finished = msg.System.TaskStarted, msg.System.TaskUpdated, msg.System.TaskNotification
	}
	var id, kind, toolID, status string
	generation := 0
	switch {
	case updated != nil:
		id, status, generation = updated.TaskID, updated.Patch.Status, updated.RunGeneration
		if status == "" {
			status = updated.Status
		}
		kind, _ = updated.Additional["task_type"].(string)
		if queued, _ := updated.Patch.Additional["will_resume"].(bool); queued && generation > 0 {
			generation++
			status = "running"
		}
	case started != nil:
		id, kind, toolID, status = started.TaskID, started.TaskType, started.ToolUseID, "running"
	case finished != nil:
		id, status, generation = finished.TaskID, finished.Status, finished.RunGeneration
	default:
		return
	}
	if id == "" || (kind != "" && kind != "local_agent") {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil || state.Client != client || state.Closing || (kind != "local_agent" && state.SubagentRuns[id] == nil) {
		return
	}
	if binding, ok := state.SubagentHookBindings[toolID]; toolID != "" && ok && binding.SDKTaskID == "" {
		binding.SDKTaskID = id
		state.SubagentHookBindings[toolID] = binding
	}
	run := state.subagentRun(id)
	wasBusy := run.busy()
	run.observe(generation, status)
	state.HasSubagentHistory = true
	if !run.busy() {
		// 长会话中已结束的子任务不累积；之后同一任务的续跑会按新代次重建条目。
		delete(state.SubagentRuns, id)
		if wasBusy {
			m.notifyResourceChangeLocked()
		}
	}
}

// ObserveSubagentUsage 把单调累计的 child task total 转换为跨 round 去重增量。
// nxs task follow-up 会复用 task ID，因此高水位跟随 runtime session 生命周期。
func (m *Manager) ObserveSubagentUsage(sessionKey string, taskID string, cumulativeTokens int64) int64 {
	if cumulativeTokens <= 0 {
		return 0
	}
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	if sessionKey == "" || taskID == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey + "\x00" + taskID
	previous := m.subagentUsageTotals[key]
	if cumulativeTokens <= previous {
		return 0
	}
	m.subagentUsageTotals[key] = cumulativeTokens
	state := m.ensureStateLocked(sessionKey)
	m.touchStateLocked(state)
	return cumulativeTokens - previous
}
