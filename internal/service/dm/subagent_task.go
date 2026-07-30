// INPUT: DM parent terminal 后到达的 nxs child lifecycle/usage 消息。
// OUTPUT: child durable history、最终 Goal usage join 与一次性 post-round 派发。
// POS: DM parent 结束后等待 child source 收敛的协调边界。
package dm

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const (
	subagentParentTerminalNormal      = "normal"
	subagentParentTerminalFailed      = "failed"
	subagentParentTerminalInterrupted = "interrupted"
)

// dmSubagentUsageObservation 是尚未确认持久化的 child checkpoint 与 lifecycle
// evidence。状态单调推进且 observation time 在 retry 间保持不变，避免旧请求
// 成功返回后误清除更新的 terminal 状态，或越过 external from-now 边界。
type dmSubagentUsageObservation struct {
	cumulativeTotal            int64
	terminal                   bool
	terminalTokenUsageObserved bool
	observedAt                 time.Time
}

func (r *roundRunner) startIdleSubagentNotificationDrain() {
	if r == nil || r.service == nil || r.service.runtime == nil || !r.service.runtime.HasSubagentHistory(r.sessionKey) {
		return
	}
	r.service.runtime.StartIdleMessageDrain(r.sessionKey, r.handleIdleSubagentMessage)
}

func (r *roundRunner) handleIdleSubagentMessage(ctx context.Context, incoming sdkprotocol.ReceivedMessage) bool {
	events, durableMessages, _, _, err := r.mapper.Map(incoming)
	if err != nil {
		r.service.loggerFor(ctx).Warn("处理 DM idle subagent 通知失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"err", err,
		)
		return true
	}
	for _, message := range durableMessages {
		if message == nil {
			continue
		}
		if err := r.handleDurableMessage(message); err != nil {
			r.service.loggerFor(ctx).Warn("写入 DM idle subagent 通知失败",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
				"err", err,
			)
			return true
		}
	}
	for _, event := range events {
		r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
	}
	if r.hasRunningSubagentTask() {
		return true
	}
	r.completeSubagentJoinAfterParentTerminal()
	// nxs 支持用同 task ID 唤醒终态 task，因此 idle drain 不能在首次完成时退出。
	return true
}

func (r *roundRunner) annotateSubagentTaskRuntimeKind(message protocol.Message) {
	if r == nil || message == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	if strings.TrimSpace(dmAnyString(metadata["task_id"])) == "" {
		return
	}
	switch strings.TrimSpace(dmAnyString(metadata["subtype"])) {
	case "task_started", "task_progress", "task_updated", "task_notification":
		if runtimeKind := strings.TrimSpace(r.runtimeKind); runtimeKind != "" {
			metadata["runtime_kind"] = runtimeKind
		}
	}
}

func (r *roundRunner) rememberSubagentTaskMessage(message protocol.Message) {
	if r == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(dmAnyString(metadata["task_id"]))
	if taskID == "" {
		return
	}
	subtype := strings.TrimSpace(dmAnyString(metadata["subtype"]))
	status := strings.TrimSpace(dmAnyString(metadata["status"]))
	if !dmMetadataLooksLikeSubagentTask(metadata) && !r.knowsSubagentTask(taskID) {
		return
	}
	r.goalUsageMu.Lock()
	if r.subagentTasks == nil {
		r.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
			break
		}
		r.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
		}
	}
	r.goalUsageMu.Unlock()
	if r.service != nil && r.service.runtime != nil {
		r.service.runtime.MarkSubagentHistory(r.sessionKey)
	}
}

func (r *roundRunner) knowsSubagentTask(taskID string) bool {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	taskID = strings.TrimSpace(taskID)
	if _, ok := r.subagentTasks[taskID]; ok {
		return true
	}
	_, ok := r.subagentUsagePending[taskID]
	return ok
}

func (r *roundRunner) hasRunningSubagentTask() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return len(r.subagentTasks) > 0 || len(r.subagentUsagePending) > 0
}

// markSubagentUsagePending 建立独立的 source 持久化 join barrier，并保留每个
// task 最新的累计值。它与 runtime task 生命周期分开，防止终态消息先移除
// task、后写 checkpoint 时被并发 finalization 穿透。
func (r *roundRunner) markSubagentUsagePending(taskID string, totalTokens int64) {
	r.markSubagentUsageObservationPending(taskID, dmSubagentUsageObservation{
		cumulativeTotal: totalTokens,
		observedAt:      time.Now().UTC(),
	})
}

func (r *roundRunner) markSubagentUsageObservationPending(
	taskID string,
	observation dmSubagentUsageObservation,
) {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	r.goalUsageMu.Lock()
	r.markSubagentUsageObservationPendingLocked(taskID, observation)
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) markSubagentUsageObservationPendingLocked(
	taskID string,
	observation dmSubagentUsageObservation,
) {
	if observation.observedAt.IsZero() {
		observation.observedAt = time.Now().UTC()
	}
	if r.subagentUsagePending == nil {
		r.subagentUsagePending = make(map[string]dmSubagentUsageObservation)
	}
	taskID = strings.TrimSpace(taskID)
	current := r.subagentUsagePending[taskID]
	if observation.cumulativeTotal > current.cumulativeTotal {
		current.cumulativeTotal = observation.cumulativeTotal
		current.observedAt = observation.observedAt
	}
	if observation.terminal && !current.terminal {
		current.observedAt = observation.observedAt
	}
	if current.observedAt.IsZero() {
		current.observedAt = observation.observedAt
	}
	current.terminal = current.terminal || observation.terminal
	current.terminalTokenUsageObserved =
		current.terminalTokenUsageObserved || observation.terminalTokenUsageObserved
	r.subagentUsagePending[taskID] = current
}

// clearSubagentUsagePending 只清除不新于已落库累计值的 pending。并发中的旧
// checkpoint 成功不能覆盖随后到达、但仍未持久化的新累计值。
func (r *roundRunner) clearSubagentUsagePending(taskID string, settledTotalTokens int64) {
	r.clearSubagentUsageObservationPending(taskID, dmSubagentUsageObservation{
		cumulativeTotal:            settledTotalTokens,
		terminal:                   true,
		terminalTokenUsageObserved: true,
	})
}

func (r *roundRunner) clearSubagentUsageObservationPending(
	taskID string,
	settled dmSubagentUsageObservation,
) {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	r.goalUsageMu.Lock()
	r.clearSubagentUsageObservationPendingLocked(taskID, settled)
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) clearSubagentUsageObservationPendingLocked(
	taskID string,
	settled dmSubagentUsageObservation,
) {
	taskID = strings.TrimSpace(taskID)
	if pending, ok := r.subagentUsagePending[taskID]; ok &&
		pending.cumulativeTotal <= settled.cumulativeTotal &&
		(!pending.terminal || settled.terminal) &&
		(!pending.terminalTokenUsageObserved || settled.terminalTokenUsageObserved) {
		delete(r.subagentUsagePending, taskID)
	}
}

func (r *roundRunner) markSubagentParentTerminal(status string) {
	if r == nil {
		return
	}
	r.goalUsageMu.Lock()
	r.subagentParentTerminal = strings.TrimSpace(status)
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) subagentParentTerminalStatus() string {
	if r == nil {
		return ""
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.subagentParentTerminal
}

func (r *roundRunner) completeSubagentJoinAfterParentTerminal() bool {
	switch r.subagentParentTerminalStatus() {
	case subagentParentTerminalNormal:
		return r.dispatchPostRoundWorkAfterSubagents()
	case subagentParentTerminalFailed, subagentParentTerminalInterrupted:
		if !r.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
			r.startGoalUsageRetryWorker()
			r.service.loggerFor(context.Background()).Warn(
				"DM 异常终态 Goal usage 等待后续重试",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
			)
			return false
		}
	}
	return true
}

func (r *roundRunner) dispatchPostRoundWorkAfterSubagents() bool {
	if r.subagentPostRoundWasDispatched() {
		return true
	}
	if !r.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		r.startGoalUsageRetryWorker()
		r.service.loggerFor(context.Background()).Warn(
			"DM Goal usage 等待后续重试，暂不派发 post-round work",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
		)
		return false
	}
	if !r.claimSubagentPostRoundDispatch() {
		return true
	}
	r.dispatchPostRoundWork()
	return true
}

func (r *roundRunner) subagentPostRoundWasDispatched() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.subagentPostRoundDispatched
}

func (r *roundRunner) claimSubagentPostRoundDispatch() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if len(r.subagentTasks) > 0 ||
		len(r.subagentUsagePending) > 0 ||
		r.subagentPostRoundDispatched {
		return false
	}
	r.subagentPostRoundDispatched = true
	return true
}

func dmMetadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	taskType := strings.ToLower(strings.TrimSpace(dmAnyString(metadata["task_type"])))
	if taskType == "local_shell" {
		return false
	}
	if taskType != "" {
		return taskType == "local_agent"
	}
	return strings.TrimSpace(dmAnyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(dmAnyString(metadata["agent_type"])) != ""
}

func dmIsTerminalSubagentTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "error", "stopped", "killed", "cancelled":
		return true
	default:
		return false
	}
}

func dmAnyString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
