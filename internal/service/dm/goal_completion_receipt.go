// INPUT: 当前 round 的 complete update_goal 观察、最终 assistant 与 Goal 聚合报告。
// OUTPUT: 与 goal_id + round_id 精确绑定、可静默补充已知用量的 durable 完成收据。
// POS: DM Goal 终态结算到最终回复/历史投影的宿主收口层。
package dm

import (
	"context"
	"errors"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (r *roundRunner) rememberGoalCompletionAssistant(message protocol.Message) {
	if r == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	r.goalUsageMu.Lock()
	if r.goalCompletionCandidateID != "" {
		r.goalCompletionAssistant = protocol.Clone(message)
	}
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) goalCompletionReceiptSnapshot() (
	string,
	protocol.Message,
	protocol.GoalCompletionReceipt,
	bool,
) {
	if r == nil {
		return "", nil, protocol.GoalCompletionReceipt{}, false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return strings.TrimSpace(r.goalCompletionCandidateID),
		protocol.Clone(r.goalCompletionAssistant),
		r.goalCompletionReceipt,
		r.goalCompletionReceiptStored
}

func (r *roundRunner) persistGoalCompletionReceipt(ctx context.Context, refresh bool) {
	if r == nil || r.service == nil {
		return
	}
	goalID, assistant, previous, stored := r.goalCompletionReceiptSnapshot()
	if goalID == "" || len(assistant) == 0 ||
		strings.TrimSpace(r.workspacePath) == "" ||
		strings.TrimSpace(r.sessionKey) == "" ||
		(stored && !refresh) {
		return
	}
	report, reportOK := r.goalCompletionReport(ctx, goalID)
	if !reportOK && stored {
		return
	}
	receipt := messageutil.BuildGoalCompletionReceipt(goalID, r.roundID, report)
	if stored && previous.Equal(receipt) {
		return
	}
	message, ok := messageutil.AttachGoalCompletionReceipt(assistant, receipt)
	if !ok || r.service.history == nil {
		return
	}
	if err := r.service.history.ForOwner(r.ownerUserID).AppendOverlayMessage(
		r.workspacePath,
		r.sessionKey,
		message,
	); err != nil {
		r.service.loggerFor(ctx).Warn(
			"DM Goal 完成收据持久化失败",
			"session_key", r.sessionKey,
			"goal_id", goalID,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	r.markGoalCompletionReceiptStored(goalID, receipt)
	if r.service.permission != nil {
		event := dmdomain.WrapSessionMessageEvent(r.session, message, protocol.DeliveryModeDurable, r.roundID)
		r.service.broadcastEventWithTimeout(ctx, r.sessionKey, event)
	}
}

func (r *roundRunner) goalCompletionReport(
	ctx context.Context,
	goalID string,
) (*protocol.GoalUsageReport, bool) {
	provider, ok := r.service.goals.(dmGoalUsageFinalizationProvider)
	if !ok {
		return nil, false
	}
	report, err := provider.UsageByGoalID(ctx, goalID)
	if err != nil {
		if !errors.Is(err, goalsvc.ErrGoalNotFound) {
			r.service.loggerFor(ctx).Debug("读取 DM Goal 完成收据数据失败", "goal_id", goalID, "err", err)
		}
		return nil, false
	}
	if report == nil ||
		protocol.NormalizeGoalStatus(report.Status) != protocol.GoalStatusComplete ||
		(strings.TrimSpace(report.GoalID) != "" && strings.TrimSpace(report.GoalID) != goalID) {
		return nil, false
	}
	return report, true
}

func (r *roundRunner) markGoalCompletionReceiptStored(
	goalID string,
	receipt protocol.GoalCompletionReceipt,
) {
	r.goalUsageMu.Lock()
	if strings.TrimSpace(r.goalCompletionCandidateID) == strings.TrimSpace(goalID) {
		r.goalCompletionReceipt = receipt
		r.goalCompletionReceiptStored = true
	}
	r.goalUsageMu.Unlock()
}
