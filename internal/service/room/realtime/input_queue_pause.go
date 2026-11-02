// INPUT: Authorized conversation and exact member targets from a user action.
// OUTPUT: Durable queue pause/resume gates under the conversation dispatch lock.
// POS: Room stop decisions survive reconnect and process restart.
package realtime

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"slices"
)

func (s *Service) setRoomInputQueuePaused(ctx context.Context, value *protocol.ConversationContextAggregate, targets []string, paused bool) error {
	locations, err := s.roomInputQueueLocations(ctx, value)
	if err != nil {
		return err
	}
	for _, entry := range locations {
		if len(targets) > 0 && !slices.Contains(targets, entry.AgentID) {
			continue
		}
		if err := s.InputQueue.SetDispatchPaused(entry.Location, paused); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) pauseRoomRoundQueue(ctx context.Context, round *activeRoomRound, targets []string, agentRoundIDs ...string) error {
	lease := s.lockRoomDispatch(round.SessionKey, round.ConversationID)
	defer lease.Unlock()
	if s.findActiveRoundByRoundID(round.SessionKey, round.RootRoundID) != round {
		return nil
	}
	for _, id := range agentRoundIDs {
		current, slot := s.findActiveSlotByAgentRoundID(round.SessionKey, id)
		if current != round || slot == nil {
			return nil
		}
	}
	if err := s.setRoomInputQueuePaused(ctx, round.Context, targets, true); err != nil {
		return err
	}
	if err := s.broadcastRoomInputQueueSnapshot(ctx, round.SessionKey, round.Context); err != nil {
		s.LoggerFor(ctx).Warn("Broadcast paused queue failed", "err", err)
	}
	return nil
}

// A shared queue item may target members other than its physical queue owner.
// Any stopped target holds the whole input until explicit send resumes it.
func projectRoomQueuePause(entries []roomInputQueueEntry, pausedAgents map[string]bool) {
	for index := range entries {
		for _, target := range inputQueueTargetAgentIDs(entries[index].Item) {
			if pausedAgents[target] {
				entries[index].Item.DispatchPaused = true
				break
			}
		}
	}
}
