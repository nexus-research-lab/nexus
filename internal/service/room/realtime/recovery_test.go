package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type deletedConversationStore struct{}

func (deletedConversationStore) GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (deletedConversationStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) MarkConversationStarted(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

// A Room deleted after its WorkGraph stalled must be skipped by startup
// recovery, not reported as a host failure that blocks every later graph.
func TestQueueExecutionRecoveryReportsDeletedConversationAsUnavailable(t *testing.T) {
	service := &Service{rooms: deletedConversationStore{}}

	err := service.QueueExecutionRecovery(context.Background(), protocol.Execution{
		ID: "execution-deleted-room", OwnerUserID: "owner", ScopeKind: protocol.ExecutionScopeRoom,
		RoomID: "room-gone", ConversationID: "conversation-gone", CoordinatorAgentID: "lead",
	}, "startup:execution-deleted-room:1")

	if !errors.Is(err, orchestrationsvc.ErrExecutionRecoveryTargetUnavailable) {
		t.Fatalf("err = %v, want ErrExecutionRecoveryTargetUnavailable", err)
	}
}
