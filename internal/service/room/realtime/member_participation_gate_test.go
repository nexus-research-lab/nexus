// INPUT: participation_paused 成员、Goal lead 与普通/WorkGraph 输入队列项。
// OUTPUT: 暂停目标不可派发，其他成员继续运行，恢复后原工作重新可派发。
// POS: Room queue、Goal 与 WorkGraph 共用参与闸门的纯行为测试。
package realtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestParticipationGateKeepsPausedWorkGraphItemAndDispatchesOtherAgent(t *testing.T) {
	const (
		conversationID = "conversation-participation-gate"
		pausedAgentID  = "agent-paused"
		activeAgentID  = "agent-active"
	)
	service := &Service{rounds: newRoomRoundRegistry()}
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	members := []protocol.MemberRecord{
		{
			MemberType:          protocol.MemberTypeAgent,
			MemberAgentID:       pausedAgentID,
			ParticipationPaused: true,
		},
		{
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: activeAgentID,
		},
	}
	workItem := protocol.InputQueueItem{
		ID:             "paused-workgraph-dispatch",
		AgentID:        pausedAgentID,
		TargetAgentIDs: []string{pausedAgentID},
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	activeItem := protocol.InputQueueItem{
		ID:             "active-user-input",
		AgentID:        activeAgentID,
		TargetAgentIDs: []string{activeAgentID},
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}
	entries := []roomInputQueueEntry{{Item: workItem}, {Item: activeItem}}

	dispatchable, ok := service.findDispatchableInputQueueEntry(
		sessionKey,
		conversationID,
		members,
		entries,
	)
	if !ok || dispatchable.Item.ID != activeItem.ID {
		t.Fatalf("dispatchable item = %+v, want active Agent item", dispatchable.Item)
	}
	if service.canDispatchInputQueueItem(
		sessionKey,
		conversationID,
		members,
		protocol.InputQueueItem{
			AgentID:        activeAgentID,
			TargetAgentIDs: []string{activeAgentID, pausedAgentID},
			DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		},
	) {
		t.Fatal("混合目标队列项不得越过暂停成员的精确调度闸门")
	}

	members[0].ParticipationPaused = false
	dispatchable, ok = service.findDispatchableInputQueueEntry(
		sessionKey,
		conversationID,
		members,
		entries,
	)
	if !ok || dispatchable.Item.ID != workItem.ID {
		t.Fatalf("恢复后 dispatchable item = %+v, want preserved WorkGraph item", dispatchable.Item)
	}
}

func TestParticipationGateDefersPausedGoalLeadWithoutAgentDirectory(t *testing.T) {
	const (
		conversationID = "conversation-paused-goal-lead"
		leadAgentID    = "agent-goal-lead"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	provider := &participationGoalContextProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		current: &protocol.Goal{
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataRoomGoalLeadAgentID: leadAgentID,
			},
		},
	}
	service := &Service{
		goals:  provider,
		rounds: newRoomRoundRegistry(),
	}
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: conversationID},
		Members: []protocol.MemberRecord{
			{
				MemberType:          protocol.MemberTypeAgent,
				MemberAgentID:       leadAgentID,
				ParticipationPaused: true,
			},
			{
				MemberType:    protocol.MemberTypeAgent,
				MemberAgentID: "agent-peer",
			},
		},
	}

	if !service.shouldDeferGoalContinuationForTargetStateLocked(
		context.Background(),
		sessionKey,
		contextValue,
	) {
		t.Fatal("paused Goal lead must defer continuation before Agent directory lookup")
	}
}

type participationGoalContextProvider struct {
	*fakeRoomGoalContextProvider
	current *protocol.Goal
}

func (p *participationGoalContextProvider) CurrentOptional(
	context.Context,
	string,
) (*protocol.Goal, error) {
	return cloneRoomGoal(p.current), nil
}
