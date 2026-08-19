// INPUT: Room child usage snapshot、retry 与 external Goal activation 的 owner/session/root scope。
// OUTPUT: 同一 durable usage scope 的进程内串行化与绑定前 pending checkpoint flush。
// POS: Room child source 持久化和 from-now binding 之间的线性化边界。
package realtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type roomGoalUsageScopeLockEntry struct {
	mu   sync.Mutex
	refs int
}

type roomGoalUsageScopeLockRegistry struct {
	mu      sync.Mutex
	entries map[string]*roomGoalUsageScopeLockEntry
}

func newRoomGoalUsageScopeLockRegistry() roomGoalUsageScopeLockRegistry {
	return roomGoalUsageScopeLockRegistry{
		entries: make(map[string]*roomGoalUsageScopeLockEntry),
	}
}

func (r *roomGoalUsageScopeLockRegistry) lock(key string) func() {
	r.mu.Lock()
	if r.entries == nil {
		r.entries = make(map[string]*roomGoalUsageScopeLockEntry)
	}
	entry := r.entries[key]
	if entry == nil {
		entry = &roomGoalUsageScopeLockEntry{}
		r.entries[key] = entry
	}
	entry.refs++
	r.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		r.mu.Lock()
		entry.refs--
		if entry.refs == 0 && r.entries[key] == entry {
			delete(r.entries, key)
		}
		r.mu.Unlock()
	}
}

func roomGoalUsageScopeLockKey(ctx context.Context, slot *activeRoomSlot) string {
	return strings.Join([]string{
		goalUsageOwnerUserIDForRoomSlot(ctx, slot),
		goalUsageSessionKeyForRoomSlot(slot, goalSessionKeyForSlot(slot)),
		goalUsageScopeRoundIDForRoomSlot(slot),
	}, "\x00")
}

func (s *Service) lockRoomGoalUsageScope(ctx context.Context, slot *activeRoomSlot) func() {
	if slot == nil {
		return func() {}
	}
	return s.goalUsageScopeLocks.lock(roomGoalUsageScopeLockKey(ctx, slot))
}

func (s *Service) roomGoalUsagePersistenceSlotsForScope(
	ctx context.Context,
	origin *activeRoomSlot,
) []*activeRoomSlot {
	if origin == nil {
		return nil
	}
	ownerUserID := goalUsageOwnerUserIDForRoomSlot(ctx, origin)
	sessionKey := goalUsageSessionKeyForRoomSlot(origin, goalSessionKeyForSlot(origin))
	slots := make([]*activeRoomSlot, 0)
	for _, candidate := range s.roomGoalUsageSlotsForScope(origin, sessionKey) {
		if goalUsageOwnerUserIDForRoomSlot(ctx, candidate) != ownerUserID {
			continue
		}
		slots = append(slots, candidate)
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].RuntimeSessionKey != slots[j].RuntimeSessionKey {
			return slots[i].RuntimeSessionKey < slots[j].RuntimeSessionKey
		}
		return slots[i].AgentRoundID < slots[j].AgentRoundID
	})
	return slots
}

// flushRoomSubagentUsageBeforeExternalBind 把当前进程已知的 child pending 先写成
// 旧/空 Goal checkpoint。BindUsageScopeFromNow 随后才能把此刻以前的累计值排除，
// 同时保留运行中 child evidence 作为新 Goal 的 terminal barrier。
func (s *Service) flushRoomSubagentUsageBeforeExternalBind(
	ctx context.Context,
	origin *activeRoomSlot,
) error {
	_, ok := s.goals.(roomGoalUsageSourceRecorder)
	if !ok {
		return nil
	}
	for _, slot := range s.roomGoalUsagePersistenceSlotsForScope(ctx, origin) {
		pending := slot.subagentUsageObservationPendingSnapshot()
		if len(pending) == 0 {
			continue
		}
		taskIDs := make([]string, 0, len(pending))
		for taskID := range pending {
			taskIDs = append(taskIDs, taskID)
		}
		sort.Strings(taskIDs)
		goalID := strings.TrimSpace(slot.childGoalIDForUsage())
		goalSessionKey := goalUsageSessionKeyForRoomSlot(slot, goalSessionKeyForSlot(slot))
		for _, taskID := range taskIDs {
			observation := pending[taskID]
			var err error
			for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
				if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
					return ctx.Err()
				}
				_, err = s.persistSubagentGoalUsageObservationForSlot(
					ctx,
					slot,
					taskID,
					observation,
					goalID,
					goalSessionKey,
				)
				if err == nil {
					break
				}
			}
			if err != nil {
				return fmt.Errorf(
					"flush Room child usage before external Goal bind: runtime=%q task=%q: %w",
					slot.RuntimeSessionKey,
					taskID,
					err,
				)
			}
			slot.clearSubagentUsageObservationPending(taskID, observation)
		}
	}
	return nil
}
