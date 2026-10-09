// INPUT: 单个 Agent round（DM round 或 Room slot）运行期间的 Goal 用量、完成收据与子任务观察。
// OUTPUT: DM 与 Room 共用的每轮 Goal 状态及其加锁访问。
// POS: 每轮 Goal 状态的唯一实现；DM roundRunner 与 Room slot 都嵌入它，Mu 保护全部字段。
package runtimehost

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalruntimeusage "github.com/nexus-research-lab/nexus/internal/service/goal/runtimeusage"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

// GoalRoundState 是一个 Agent round 的 Goal 运行状态；Mu 保护全部字段。
type GoalRoundState struct {
	Mu                      sync.RWMutex
	Context                 string
	IDForUsage              string
	ChildIDForUsage         string
	Usage                   *goalsvc.RuntimeUsageAccumulator
	UsageStartedAt          time.Time
	LastAssistant           protocol.Message
	CompletionCandidateID   string
	CompletionAssistant     protocol.Message
	CompletionReceipt       protocol.GoalCompletionReceipt
	CompletionReceiptStored bool
	ToolProgress            bool
	CommandReceiptSequence  uint64
	SubagentTasks           map[string]struct{}
	SubagentUsagePending    map[string]goalsvc.SubagentUsageObservation
	UsageRetrying           bool
	UsageClaimPending       bool
	UsageScopeConsumed      bool
	ResultUsageWritten      bool
}

// KnowsSubagentTask 判断 taskID 是否是本轮已登记或仍待结算的子任务。
func (g *GoalRoundState) KnowsSubagentTask(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	if _, ok := g.SubagentTasks[taskID]; ok {
		return true
	}
	_, ok := g.SubagentUsagePending[taskID]
	return ok
}

// HasRunningSubagentTask 判断本轮是否仍有运行中或待结算用量的子任务。
func (g *GoalRoundState) HasRunningSubagentTask() bool {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return len(g.SubagentTasks) > 0 || len(g.SubagentUsagePending) > 0
}

// RememberGoalAssistantMessage 记录本轮最后一条 assistant 消息，供用量与完成收据使用。
func (g *GoalRoundState) RememberGoalAssistantMessage(message protocol.Message) {
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	g.Mu.Lock()
	g.LastAssistant = protocol.Clone(message)
	g.Mu.Unlock()
}

// LastGoalAssistantMessage 返回本轮最后一条 assistant 消息的副本。
func (g *GoalRoundState) LastGoalAssistantMessage() protocol.Message {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return protocol.Clone(g.LastAssistant)
}

// HasGoalCompletionCandidate 判断本轮是否已有待确认完成的 Goal。
func (g *GoalRoundState) HasGoalCompletionCandidate() bool {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return strings.TrimSpace(g.CompletionCandidateID) != ""
}

// GoalCompletionReceiptSnapshot 返回完成候选、对应 assistant、收据与是否已持久化。
func (g *GoalRoundState) GoalCompletionReceiptSnapshot() (string, protocol.Message, protocol.GoalCompletionReceipt, bool) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return strings.TrimSpace(g.CompletionCandidateID),
		protocol.Clone(g.CompletionAssistant),
		g.CompletionReceipt,
		g.CompletionReceiptStored
}

// MarkGoalCompletionReceiptStored 在收据持久化后记录，仅当候选仍是同一 Goal 时生效。
func (g *GoalRoundState) MarkGoalCompletionReceiptStored(goalID string, receipt protocol.GoalCompletionReceipt) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if strings.TrimSpace(g.CompletionCandidateID) != strings.TrimSpace(goalID) {
		return
	}
	g.CompletionReceipt = receipt
	g.CompletionReceiptStored = true
}

// RememberGoalCompletionAssistant 为当前完成候选记录最终 assistant 消息。
func (g *GoalRoundState) RememberGoalCompletionAssistant(message protocol.Message) {
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if strings.TrimSpace(g.CompletionCandidateID) == "" {
		return
	}
	g.CompletionAssistant = protocol.Clone(message)
}

// GoalUsageScopeConsumed 判断本轮 Goal 用量 scope 是否已被认领。
func (g *GoalRoundState) GoalUsageScopeConsumed() bool {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return g.UsageScopeConsumed
}

// SubagentUsageSettlement 是一条待结算的子任务用量观察。
type SubagentUsageSettlement struct {
	TaskID      string
	Observation goalsvc.SubagentUsageObservation
}

// SubagentUsageObservations 从消息中提取属于本轮子任务的用量观察。
func (g *GoalRoundState) SubagentUsageObservations(message protocol.Message) []SubagentUsageSettlement {
	observations := goalruntimeusage.SubagentObservations(message, g.KnowsSubagentTask)
	result := make([]SubagentUsageSettlement, 0, len(observations))
	for _, item := range observations {
		result = append(result, SubagentUsageSettlement{TaskID: item.TaskID, Observation: item.Usage})
	}
	return result
}

// RememberSubagentTaskMessage 按子任务生命周期消息更新运行中任务集合；
// 返回 true 表示消息属于本轮子任务，调用方据此记录宿主侧的子任务历史。
func (g *GoalRoundState) RememberSubagentTaskMessage(message protocol.Message) bool {
	metadata, _ := message["metadata"].(map[string]any)
	taskID := textutil.AnyString(metadata["task_id"])
	if taskID == "" {
		return false
	}
	if !messageutil.IsSubagentTaskMetadata(metadata) && !g.KnowsSubagentTask(taskID) {
		return false
	}
	subtype := textutil.AnyString(metadata["subtype"])
	terminal := messageutil.IsTerminalSubagentTaskStatus(textutil.AnyString(metadata["status"]))
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if g.SubagentTasks == nil {
		g.SubagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if terminal {
			delete(g.SubagentTasks, taskID)
		} else {
			g.SubagentTasks[taskID] = struct{}{}
		}
	case "task_notification":
		if terminal {
			delete(g.SubagentTasks, taskID)
		}
	}
	return true
}

// RecordResultUsage 用 write 写入 result 消息的 token 用量，成功后记住本轮已写过 result 用量。
func (g *GoalRoundState) RecordResultUsage(message protocol.Message, write func(protocol.Message) bool) {
	if protocol.MessageRole(message) != "result" || !usagesvc.MessageHasUsage(message) {
		return
	}
	if write(message) {
		g.Mu.Lock()
		g.ResultUsageWritten = true
		g.Mu.Unlock()
	}
}

// RecordTerminalAssistantUsage 仅在本轮没有 result 用量时，用终态 assistant 消息的用量兜底，避免重复计量。
func (g *GoalRoundState) RecordTerminalAssistantUsage(message protocol.Message, write func(protocol.Message) bool) {
	if protocol.MessageRole(message) != "assistant" || !usagesvc.MessageHasUsage(message) {
		return
	}
	g.Mu.RLock()
	written := g.ResultUsageWritten
	g.Mu.RUnlock()
	if !written {
		write(message)
	}
}

// MarkSubagentUsagePendingLocked 合并一条尚未落库的子任务用量观察；调用方持有 Mu。
func (g *GoalRoundState) MarkSubagentUsagePendingLocked(taskID string, observation goalsvc.SubagentUsageObservation) {
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	}
	if g.SubagentUsagePending == nil {
		g.SubagentUsagePending = make(map[string]goalsvc.SubagentUsageObservation)
	}
	taskID = strings.TrimSpace(taskID)
	g.SubagentUsagePending[taskID] = g.SubagentUsagePending[taskID].Merge(observation)
}

// MarkSubagentUsagePending 是 MarkSubagentUsagePendingLocked 的加锁版本。
func (g *GoalRoundState) MarkSubagentUsagePending(taskID string, observation goalsvc.SubagentUsageObservation) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.MarkSubagentUsagePendingLocked(taskID, observation)
}

// ClearSubagentUsagePendingLocked 在 settled 覆盖待落库观察时清除它，避免旧回执吞掉后到的累计值；调用方持有 Mu。
func (g *GoalRoundState) ClearSubagentUsagePendingLocked(taskID string, settled goalsvc.SubagentUsageObservation) {
	taskID = strings.TrimSpace(taskID)
	if pending, ok := g.SubagentUsagePending[taskID]; ok && pending.CoveredBy(settled) {
		delete(g.SubagentUsagePending, taskID)
	}
}

// ClearSubagentUsagePending 是 ClearSubagentUsagePendingLocked 的加锁版本。
func (g *GoalRoundState) ClearSubagentUsagePending(taskID string, settled goalsvc.SubagentUsageObservation) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.ClearSubagentUsagePendingLocked(taskID, settled)
}

// RememberGoalToolProgress 记录本轮工具调用推进过 Goal。
func (g *GoalRoundState) RememberGoalToolProgress(progressed bool) {
	if !progressed {
		return
	}
	g.Mu.Lock()
	g.ToolProgress = true
	g.Mu.Unlock()
}

// ConsumeCommandReceipts 返回 state 中本轮尚未消费的命令回执并推进游标。
func (g *GoalRoundState) ConsumeCommandReceipts(state *nexusmcp.CommandReceiptState) []nexusmcp.CommandReceipt {
	if state == nil {
		return nil
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	receipts, sequence := state.Since(g.CommandReceiptSequence)
	g.CommandReceiptSequence = sequence
	return receipts
}

// PrepareGoalCompletionReceipt 为本轮完成候选构造附着收据的最终 assistant 消息。
// 已持久化且未要求 refresh、读不到新用量或收据未变化时返回 false。
func (g *GoalRoundState) PrepareGoalCompletionReceipt(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	roundID string,
	refresh bool,
) (string, protocol.Message, protocol.GoalCompletionReceipt, bool) {
	goalID, assistant, previous, stored := g.GoalCompletionReceiptSnapshot()
	if goalID == "" || len(assistant) == 0 || (stored && !refresh) {
		return "", nil, protocol.GoalCompletionReceipt{}, false
	}
	report, reportOK := GoalCompletionReport(ctx, provider, logger, goalID)
	if !reportOK && stored {
		return "", nil, protocol.GoalCompletionReceipt{}, false
	}
	receipt := messageutil.BuildGoalCompletionReceipt(goalID, roundID, report)
	if stored && previous.Equal(receipt) {
		return "", nil, protocol.GoalCompletionReceipt{}, false
	}
	message, ok := messageutil.AttachGoalCompletionReceipt(assistant, receipt)
	return goalID, message, receipt, ok
}

// UsageGoalID 返回本轮用量当前归属的 Goal。
func (g *GoalRoundState) UsageGoalID() string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return strings.TrimSpace(g.IDForUsage)
}

// ElapsedSecondsSince 返回从 startedAt 到现在的整秒数；未开始或时钟回拨时为 0。
func ElapsedSecondsSince(startedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}
	return max(int64(time.Since(startedAt).Seconds()), 0)
}
