package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// cancelRootPublicHandoffs 把 root 取消传播到 ledger 与尚未派发的 queue item。
// 已经进入 target runtime 的 slot 由 interruptActiveRound 继续负责中断。
func (s *RealtimeService) cancelRootPublicHandoffs(
	ctx context.Context,
	roundValue *activeRoomRound,
	status string,
) {
	if s == nil || s.publicHandoffs == nil || roundValue == nil {
		return
	}
	rootRoundID := roomRootRoundID(roundValue)
	edges, err := s.publicHandoffs.ListRoot(roundValue.ConversationID, rootRoundID)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room root handoff 失败", "root", rootRoundID, "err", err)
		return
	}
	if err = s.publicHandoffs.CancelForRoot(roundValue.ConversationID, rootRoundID, status); err != nil {
		s.loggerFor(ctx).Warn("取消 Room root handoff 失败", "root", rootRoundID, "err", err)
		return
	}
	if s.inputQueue == nil || roundValue.Context == nil || len(edges) == 0 {
		return
	}
	cancelledIDs := make(map[string]struct{}, len(edges))
	for _, edge := range edges {
		if handoffID := strings.TrimSpace(edge.HandoffID); handoffID != "" {
			cancelledIDs[handoffID] = struct{}{}
		}
	}
	entries, err := s.roomInputQueueEntries(ctx, roundValue.Context)
	if err != nil {
		s.loggerFor(ctx).Warn("读取待取消的 Room handoff queue 失败", "root", rootRoundID, "err", err)
		return
	}
	changed := false
	for _, entry := range entries {
		if entry.Item.Source != protocol.InputQueueSourceAgentPublicMention {
			continue
		}
		if _, ok := cancelledIDs[strings.TrimSpace(entry.Item.HandoffID)]; !ok {
			continue
		}
		if _, err = s.inputQueue.Delete(entry.Location, entry.Item.ID); err != nil {
			s.loggerFor(ctx).Warn("删除已取消的 Room handoff queue 失败", "item_id", entry.Item.ID, "err", err)
			continue
		}
		changed = true
	}
	if changed {
		if err = s.broadcastRoomInputQueueSnapshot(ctx, roundValue.SessionKey, roundValue.Context); err != nil {
			s.loggerFor(ctx).Warn("广播取消后的 Room queue 快照失败", "root", rootRoundID, "err", err)
		}
	}
}
