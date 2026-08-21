package room_test

import (
	"context"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type recordingInitialConversationObserver struct {
	items []protocol.ConversationContextAggregate
}

func (o *recordingInitialConversationObserver) Schedule(
	_ context.Context,
	aggregate protocol.ConversationContextAggregate,
) {
	o.items = append(o.items, aggregate)
}

func TestRoomServiceOnlySchedulesDirectWelcomeForUserOpen(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	observer := &recordingInitialConversationObserver{}
	roomService.SetInitialConversationObserver(observer)

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "欢迎语助手")
	if _, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID); err != nil {
		t.Fatalf("内部物化 DM 失败: %v", err)
	}
	if len(observer.items) != 0 {
		t.Fatalf("内部物化 DM 不应生成欢迎语: %+v", observer.items)
	}

	direct, err := roomService.EnsureDirectRoomWithWelcome(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("用户打开 DM 失败: %v", err)
	}
	if len(observer.items) != 1 || observer.items[0].Conversation.ID != direct.Conversation.ID {
		t.Fatalf("用户打开 DM 应调度欢迎语: %+v", observer.items)
	}

	group, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "欢迎语 Room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	if len(observer.items) != 2 || observer.items[1].Conversation.ID != group.Conversation.ID {
		t.Fatalf("显式创建 Room 应调度欢迎语: %+v", observer.items)
	}
}
