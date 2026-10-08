// INPUT: 当前 round 的 complete update_goal 观察、最终 assistant 与 Goal 聚合报告。
// OUTPUT: 与 goal_id + round_id 精确绑定、可静默补充已知用量的 durable 完成收据。
// POS: DM Goal 终态结算到最终回复/历史投影的宿主收口层。
package dm

import (
	"context"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *roundRunner) persistGoalCompletionReceipt(ctx context.Context, refresh bool) {
	if strings.TrimSpace(r.workspacePath) == "" || r.sessionKey == "" {
		return
	}
	goalID, message, receipt, ok := r.PrepareGoalCompletionReceipt(ctx, r.service.goals, r.service.LoggerFor(ctx), r.roundID, refresh)
	if !ok {
		return
	}
	if err := r.service.History.ForOwner(r.ownerUserID).AppendOverlayMessage(
		r.workspacePath,
		r.sessionKey,
		message,
	); err != nil {
		r.service.LoggerFor(ctx).Warn(
			"DM Goal 完成收据持久化失败",
			"session_key", r.sessionKey,
			"goal_id", goalID,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	r.MarkGoalCompletionReceiptStored(goalID, receipt)
	event := dmdomain.WrapSessionMessageEvent(r.session, message, protocol.DeliveryModeDurable, r.roundID)
	r.service.broadcastEventWithTimeout(ctx, r.sessionKey, event)
}
