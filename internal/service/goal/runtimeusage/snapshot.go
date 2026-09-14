// INPUT: runtime 最终结果、assistant/子任务消息与宿主观察时间。
// OUTPUT: Goal 可消费的累计/逐 turn 快照及子任务结算证据。
// POS: DM/Room 共用的 Goal 用量适配，不持有会话身份、锁或持久化能力。
package runtimeusage

import (
	"strings"
	"time"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

// FinalSnapshot 优先采用 provider result，缺失时才回退到 assistant；显式零用量仍是证据。
func FinalSnapshot(result exec.RoundExecutionResult, assistant protocol.Message, elapsedSeconds int64) (goalsvc.RuntimeUsageSnapshot, bool) {
	usage, observed := runtimectx.GoalUsageFromTokenUsageWithPresence(result.Usage)
	cumulative := observed
	if !observed && protocol.MessageRole(assistant) == "assistant" {
		usage, observed = runtimectx.GoalUsageFromRaw(assistant["usage"])
	}
	if result.ElapsedTimeSeconds > 0 {
		elapsedSeconds = result.ElapsedTimeSeconds
	}
	id, _ := assistant["message_id"].(string)
	return goalsvc.RuntimeUsageSnapshot{Usage: usage, ElapsedSeconds: elapsedSeconds, TokenUsageObserved: observed, TurnID: strings.TrimSpace(id), Cumulative: cumulative, Terminal: true}, observed || elapsedSeconds > 0
}

// AssistantSnapshot 将逐 turn 观察交给 Goal accumulator 去重，不冒充累计终态。
func AssistantSnapshot(message protocol.Message, elapsedSeconds int64) goalsvc.RuntimeUsageSnapshot {
	usage, observed := runtimectx.GoalUsageFromRaw(message["usage"])
	id, _ := message["message_id"].(string)
	return goalsvc.RuntimeUsageSnapshot{Usage: usage, ElapsedSeconds: elapsedSeconds, TokenUsageObserved: observed, TurnID: strings.TrimSpace(id)}
}

// TaskObservation 将子任务身份与其单调结算证据绑定。
type TaskObservation struct {
	TaskID string
	Usage  goalsvc.SubagentUsageObservation
}

// SubagentObservations 合并一条消息中的用量与终态；已知任务允许缺少重复身份字段。
func SubagentObservations(message protocol.Message, knowsTask func(string) bool) []TaskObservation {
	usage := messageutil.SubagentTaskUsageSnapshots(message)
	observedAt := time.Now().UTC()
	observations := make([]TaskObservation, 0, len(usage)+1)
	indexByTask := make(map[string]int, len(usage)+1)
	for _, child := range usage {
		taskID := strings.TrimSpace(child.TaskID)
		if taskID == "" || child.TotalTokens <= 0 {
			continue
		}
		indexByTask[taskID] = len(observations)
		observations = append(observations, TaskObservation{TaskID: taskID, Usage: goalsvc.SubagentUsageObservation{CumulativeTotal: child.TotalTokens, ObservedAt: observedAt}})
	}
	metadata, _ := message["metadata"].(map[string]any)
	taskID, _ := metadata["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || (!messageutil.IsSubagentTaskMetadata(metadata) && (knowsTask == nil || !knowsTask(taskID))) {
		return observations
	}
	status, _ := metadata["status"].(string)
	terminal := messageutil.IsTerminalSubagentTaskStatus(status)
	if index, exists := indexByTask[taskID]; exists {
		observations[index].Usage.Terminal = terminal
		observations[index].Usage.TerminalTokenUsageObserved = terminal && observations[index].Usage.CumulativeTotal > 0
		return observations
	}
	return append(observations, TaskObservation{TaskID: taskID, Usage: goalsvc.SubagentUsageObservation{Terminal: terminal, ObservedAt: observedAt}})
}
