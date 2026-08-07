// INPUT: Room Agent member 的暂停/恢复请求、持久 Room contexts 与活跃 slots。
// OUTPUT: 在全部 conversation 派发锁内持久化并中断，或恢复 queue/Goal/WorkGraph 调度。
// POS: 持久成员参与状态与 Room realtime 调度之间的唯一控制面。
package realtime

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomMemberParticipationInterruptTimeout = 10 * time.Second

type roomMemberParticipationStore interface {
	GetRoomContexts(context.Context, string) ([]protocol.ConversationContextAggregate, error)
	SetRoomMemberParticipation(context.Context, string, string, bool) (*protocol.ConversationContextAggregate, error)
}

type activeRoomGoalContinuationProvider interface {
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	DispatchActiveGoalContinuation(context.Context, protocol.Goal)
}

func partitionRoomParticipationTargets(
	members []protocol.MemberRecord,
	targetAgentIDs []string,
) (participating []string, paused []string) {
	seen := make(map[string]struct{}, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		if roomdomain.IsMemberParticipationPaused(members, agentID) {
			paused = append(paused, agentID)
			continue
		}
		participating = append(participating, agentID)
	}
	return participating, paused
}

// SetRoomMemberParticipation 原子化“状态持久化”和 runtime 调度边界。
// pause 持锁等待当前成员 slots 收口；resume 只有在释放全部锁后才唤醒待办。
func (s *Service) SetRoomMemberParticipation(
	ctx context.Context,
	roomID string,
	agentID string,
	paused bool,
) (*protocol.ConversationContextAggregate, error) {
	if s == nil || s.rooms == nil {
		return nil, errors.New("Room participation store is unavailable")
	}
	store, ok := s.rooms.(roomMemberParticipationStore)
	if !ok {
		return nil, errors.New("Room participation store is not configured")
	}
	normalizedRoomID := strings.TrimSpace(roomID)
	normalizedAgentID := strings.TrimSpace(agentID)
	contexts, err := store.GetRoomContexts(ctx, normalizedRoomID)
	if err != nil {
		return nil, err
	}
	sort.Slice(contexts, func(i int, j int) bool {
		return contexts[i].Conversation.ID < contexts[j].Conversation.ID
	})
	leases := make([]*roomDispatchLease, 0, len(contexts))
	for _, contextValue := range contexts {
		conversationID := strings.TrimSpace(contextValue.Conversation.ID)
		if conversationID == "" {
			continue
		}
		leases = append(leases, s.lockRoomDispatch(
			protocol.BuildRoomSharedSessionKey(conversationID),
			conversationID,
		))
	}
	unlock := func() {
		for index := len(leases) - 1; index >= 0; index-- {
			leases[index].Unlock()
		}
	}
	defer unlock()

	updated, err := store.SetRoomMemberParticipation(
		ctx,
		normalizedRoomID,
		normalizedAgentID,
		paused,
	)
	if err != nil {
		return nil, err
	}
	if paused {
		interruptCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			roomMemberParticipationInterruptTimeout,
		)
		interruptErr := s.InterruptAgentTasks(
			interruptCtx,
			normalizedRoomID,
			normalizedAgentID,
			"成员已暂停参与",
		)
		cancel()
		if interruptErr != nil {
			s.loggerFor(ctx).Warn(
				"暂停 Room 成员后收口活跃任务失败，持久调度闸门保持关闭",
				"room_id", normalizedRoomID,
				"agent_id", normalizedAgentID,
				"err", interruptErr,
			)
		}
		return updated, nil
	}

	unlock()
	s.resumeRoomMemberWork(contexts)
	return updated, nil
}

func (s *Service) resumeRoomMemberWork(
	contexts []protocol.ConversationContextAggregate,
) {
	for _, contextValue := range contexts {
		contextValue := contextValue
		conversationID := strings.TrimSpace(contextValue.Conversation.ID)
		if conversationID == "" {
			continue
		}
		sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
		s.startSessionBackgroundTask(
			sessionKey,
			contextValue.Room.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(
					taskCtx,
					sessionKey,
					contextValue.Room.ID,
					conversationID,
				)
				s.dispatchResumedRoomGoal(taskCtx, sessionKey)
			},
		)
	}
}

func (s *Service) dispatchResumedRoomGoal(ctx context.Context, sessionKey string) {
	provider, ok := s.goals.(activeRoomGoalContinuationProvider)
	if !ok {
		return
	}
	goal, err := provider.CurrentOptional(ctx, sessionKey)
	if err != nil {
		s.loggerFor(ctx).Warn(
			"恢复 Room 成员后读取 active Goal 失败",
			"session_key", sessionKey,
			"err", err,
		)
		return
	}
	if goal != nil {
		provider.DispatchActiveGoalContinuation(ctx, *goal)
	}
}
