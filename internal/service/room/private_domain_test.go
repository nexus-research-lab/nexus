package room_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

func TestRoomServiceProjectsAgentPrivateDomain(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-private-domain",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	sam := createTestAgent(t, agentService, ctx, "Sam")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID, sam.AgentID},
		Name:     "狼人杀调试房",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	realtime := roomsvc.NewRealtimeService(cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	createAction := func(request protocol.CreateRoomActionRequest) {
		t.Helper()
		if request.WakePolicy == "" &&
			(request.ActionType == protocol.RoomActionTypePrivateMessage ||
				request.ActionType == protocol.RoomActionTypeRequestReply) {
			request.WakePolicy = protocol.RoomWakePolicyNone
		}
		if _, err = realtime.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, request); err != nil {
			t.Fatalf("创建 Room action 失败: %v", err)
		}
	}
	createAction(protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "今晚请先给出查验目标",
	})
	createAction(protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: devin.AgentID,
		TargetAgentID: amy.AgentID,
		Content:       "我想查验 Sam",
	})
	createAction(protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{devin.AgentID, sam.AgentID},
		Content:          "狼人小范围讨论",
	})
	createAction(protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateNote,
		SourceAgentID: sam.AgentID,
		Content:       "Sam 自己的私有备注",
	})

	devinPage, err := roomService.ListAgentPrivateThreads(ctx, devin.AgentID, roomsvc.AgentPrivateDomainQuery{})
	if err != nil {
		t.Fatalf("读取 Devin 私域线程失败: %v", err)
	}
	directThread := mustFindPrivateThread(t, devinPage.Items, []string{amy.AgentID})
	audienceThread := mustFindPrivateThread(t, devinPage.Items, []string{amy.AgentID, sam.AgentID})
	if directThread.Scope != "direct" || audienceThread.Scope != "audience" {
		t.Fatalf("线程 scope 不正确: direct=%+v audience=%+v", directThread, audienceThread)
	}

	eventPage, err := roomService.ListAgentPrivateEvents(ctx, devin.AgentID, directThread.ThreadID, roomsvc.AgentPrivateDomainQuery{})
	if err != nil {
		t.Fatalf("读取 Devin 私域事件失败: %v", err)
	}
	if len(eventPage.Items) != 2 {
		t.Fatalf("direct 私域事件数量不正确: %+v", eventPage.Items)
	}
	if eventPage.Items[0].Direction != "incoming" || eventPage.Items[1].Direction != "outgoing" {
		t.Fatalf("direct 私域事件方向不正确: %+v", eventPage.Items)
	}

	samPage, err := roomService.ListAgentPrivateThreads(ctx, sam.AgentID, roomsvc.AgentPrivateDomainQuery{})
	if err != nil {
		t.Fatalf("读取 Sam 私域线程失败: %v", err)
	}
	_ = mustFindPrivateThread(t, samPage.Items, []string{amy.AgentID, devin.AgentID})
	_ = mustFindPrivateThread(t, samPage.Items, nil)
	if _, ok := findPrivateThreadByPeers(samPage.Items, []string{amy.AgentID}); ok {
		t.Fatalf("Sam 不应看到 Devin 与 Amy 的 direct 私域线程: %+v", samPage.Items)
	}
}

func mustFindPrivateThread(
	t *testing.T,
	threads []protocol.AgentPrivateThread,
	peerAgentIDs []string,
) protocol.AgentPrivateThread {
	t.Helper()
	thread, ok := findPrivateThreadByPeers(threads, peerAgentIDs)
	if !ok {
		t.Fatalf("未找到私域线程 peers=%v threads=%+v", peerAgentIDs, threads)
	}
	return thread
}

func findPrivateThreadByPeers(
	threads []protocol.AgentPrivateThread,
	peerAgentIDs []string,
) (protocol.AgentPrivateThread, bool) {
	want := append([]string(nil), peerAgentIDs...)
	sort.Strings(want)
	for _, thread := range threads {
		got := append([]string(nil), thread.PeerAgentIDs...)
		sort.Strings(got)
		if reflect.DeepEqual(got, want) {
			return thread, true
		}
	}
	return protocol.AgentPrivateThread{}, false
}
