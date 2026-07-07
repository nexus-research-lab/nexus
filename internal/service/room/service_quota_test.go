package room_test

import (
	"context"
	"errors"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type fakeRoomQuotaChecker struct {
	err error
}

func (f fakeRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestRealtimeServiceHandleChatBlocksRuntimeWhenQuotaExceeded(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-quota",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "额度助手")
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	factory := &fakeRoomFactory{clients: []*fakeRoomClient{newFakeRoomClient()}}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
	)
	errQuota := errors.New("quota exceeded")
	service.SetQuotaChecker(fakeRoomQuotaChecker{err: errQuota})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "你好",
		RoundID:        "room-round-quota",
	})
	if !errors.Is(err, errQuota) {
		t.Fatalf("quota exceeded 应阻止 Room runtime，实际: %v", err)
	}
	if got := factory.LastOptions(); got.Model != "" {
		t.Fatalf("quota exceeded 时不应创建 Room runtime: %+v", got)
	}
}
