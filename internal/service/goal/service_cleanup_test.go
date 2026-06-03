package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceDeleteGoalsForAgentRemovesAllAgentSessions(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	agentDM := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:dm:chat-1")
	agentGroupTopic := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1:topic:thread-1")
	otherAgent := createCleanupGoal(t, service, ctx, "agent:agent-b:ws:dm:chat-1")
	roomGoal := createCleanupGoal(t, service, ctx, "room:group:conversation-1")

	deleted, err := service.DeleteGoalsForAgent(ctx, "agent-a")
	if err != nil {
		t.Fatalf("DeleteGoalsForAgent() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	assertGoalDeleted(t, repo, agentDM.ID)
	assertGoalDeleted(t, repo, agentGroupTopic.ID)
	assertGoalExists(t, repo, otherAgent.ID)
	assertGoalExists(t, repo, roomGoal.ID)
}

func TestServiceDeleteGoalsForRoomConversationsRemovesSharedAndMemberGoals(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	shared := createCleanupGoal(t, service, ctx, "room:group:conversation-1")
	groupMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1")
	dmMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:dm:conversation-1")
	otherConversation := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-2")

	deleted, err := service.DeleteGoalsForRoomConversations(ctx, []string{"conversation-1"})
	if err != nil {
		t.Fatalf("DeleteGoalsForRoomConversations() error = %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
	assertGoalDeleted(t, repo, shared.ID)
	assertGoalDeleted(t, repo, groupMember.ID)
	assertGoalDeleted(t, repo, dmMember.ID)
	assertGoalExists(t, repo, otherConversation.ID)
}

func TestServiceDeleteGoalsForRoomMemberOnlyRemovesThatMember(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	removedMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1")
	remainingMember := createCleanupGoal(t, service, ctx, "agent:agent-b:ws:group:conversation-1")
	shared := createCleanupGoal(t, service, ctx, "room:group:conversation-1")

	deleted, err := service.DeleteGoalsForRoomMember(ctx, "agent-a", []string{"conversation-1"})
	if err != nil {
		t.Fatalf("DeleteGoalsForRoomMember() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	assertGoalDeleted(t, repo, removedMember.ID)
	assertGoalExists(t, repo, remainingMember.ID)
	assertGoalExists(t, repo, shared.ID)
}

func createCleanupGoal(t *testing.T, service *Service, ctx context.Context, sessionKey string) *protocol.Goal {
	t.Helper()
	item, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "cleanup " + sessionKey,
	})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", sessionKey, err)
	}
	return item
}

func assertGoalDeleted(t *testing.T, repo *memoryRepository, goalID string) {
	t.Helper()
	if _, ok := repo.goals[goalID]; ok {
		t.Fatalf("goal %s still exists", goalID)
	}
}

func assertGoalExists(t *testing.T, repo *memoryRepository, goalID string) {
	t.Helper()
	if _, ok := repo.goals[goalID]; !ok {
		t.Fatalf("goal %s missing", goalID)
	}
}
