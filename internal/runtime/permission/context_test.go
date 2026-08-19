package permission

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type stubSender struct {
	key    string
	closed bool
	events []protocol.EventMessage
}

func (s *stubSender) Key() string {
	return s.key
}

func (s *stubSender) IsClosed() bool {
	return s.closed
}

func (s *stubSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events = append(s.events, event)
	return nil
}

func TestContextBindAndUnbindSession(t *testing.T) {
	ctx := NewContext()
	senderA := &stubSender{key: "a"}
	senderB := &stubSender{key: "b"}

	ctx.BindSession("session-1", senderA)
	ctx.BindSession("session-1", senderB)
	if !ctx.IsBound("session-1", senderA) || !ctx.IsBound("session-1", senderB) {
		t.Fatal("sender 应绑定到 session")
	}
	if senders := ctx.ResolveSessionSenders("session-1"); len(senders) != 2 {
		t.Fatalf("应返回 2 个绑定 sender，实际: %d", len(senders))
	}

	ctx.UnbindSession("session-1", senderA)
	if ctx.IsBound("session-1", senderA) {
		t.Fatal("senderA 应已解绑")
	}
	if senders := ctx.ResolveSessionSenders("session-1"); len(senders) != 1 || senders[0].Key() != "b" {
		t.Fatalf("解绑后应只剩 senderB，实际: %+v", senders)
	}
}

func TestContextBroadcastSessionStatus(t *testing.T) {
	ctx := NewContext()
	senderA := &stubSender{key: "a"}
	senderB := &stubSender{key: "b"}
	ctx.BindSession("session-1", senderA)
	ctx.BindSession("session-1", senderB)
	ctx.BindSessionRoute("session-1", RouteContext{
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	})

	errs := ctx.BroadcastSessionStatus(context.Background(), "session-1", []string{"round-1"})
	if len(errs) != 0 {
		t.Fatalf("广播不应失败: %+v", errs)
	}
	if len(senderA.events) != 1 || len(senderB.events) != 1 {
		t.Fatalf("广播未 fan-out 到全部连接: a=%d b=%d", len(senderA.events), len(senderB.events))
	}

	event := senderA.events[0]
	if event.EventType != protocol.EventTypeSessionStatus {
		t.Fatalf("事件类型错误: %+v", event)
	}
	if event.Data["is_generating"] != true {
		t.Fatalf("生成状态错误: %+v", event.Data)
	}
	if _, ok := event.Data["running_round_ids"]; !ok {
		t.Fatalf("running_round_ids 缺失: %+v", event.Data)
	}
	if event.RoomID != "room-1" || event.ConversationID != "conversation-1" {
		t.Fatalf("session_status 必须携带精确聊天 route: %+v", event)
	}
}

func TestContextProjectsRoutedRoundStatusAndListsRoomActivityRoutes(t *testing.T) {
	ctx := NewContext()
	broadcaster := newPermissionTestRoomBroadcaster()
	ctx.SetRoomBroadcaster(broadcaster)
	ctx.BindSessionRoute("session-b", RouteContext{
		DispatchSessionKey: "session-b",
		RoomID:             "room-1",
		ConversationID:     "conversation-b",
		AgentID:            "agent-b",
	})
	ctx.BindSessionRoute("session-a", RouteContext{
		DispatchSessionKey: "session-a",
		RoomID:             "room-1",
		ConversationID:     "conversation-a",
		AgentID:            "agent-a",
	})
	ctx.BindSessionRoute("session-other", RouteContext{
		RoomID:         "room-2",
		ConversationID: "conversation-other",
	})

	event := protocol.NewRoundStatusEvent("session-a", "round-a", protocol.RoundStatusRunning, "")
	if errs := ctx.BroadcastEvent(t.Context(), "session-a", event); len(errs) != 0 {
		t.Fatalf("Room 生命周期投影失败: %v", errs)
	}
	if roomID := <-broadcaster.roomIDs; roomID != "room-1" {
		t.Fatalf("Room 投影目标错误: %q", roomID)
	}
	projected := <-broadcaster.events
	if projected.RoomID != "room-1" || projected.ConversationID != "conversation-a" {
		t.Fatalf("Room 生命周期缺少精确 route: %+v", projected)
	}
	if projected.DeliveryMode != protocol.DeliveryModeDurable {
		t.Fatalf("Room 生命周期必须封住订阅竞态: %+v", projected)
	}

	routes := ctx.SessionActivityRoutesForRoom("room-1")
	if len(routes) != 2 || routes[0].SessionKey != "session-a" || routes[1].SessionKey != "session-b" {
		t.Fatalf("Room route 快照必须稳定且隔离: %+v", routes)
	}
}

func TestContextStaleSessionRouteLeaseCannotDeleteReplacement(t *testing.T) {
	ctx := NewContext()
	const runtimeSessionKey = "agent:shared-runtime"

	oldLease := ctx.BindSessionRoute(runtimeSessionKey, RouteContext{
		DispatchSessionKey: "room:old",
		RoundID:            "round-old",
	})
	replacementLease := ctx.BindSessionRoute(runtimeSessionKey, RouteContext{
		DispatchSessionKey: "room:replacement",
		RoundID:            "round-replacement",
	})

	ctx.UnbindSessionRoute(oldLease)

	if got := ctx.ResolveDispatchSessionKey(runtimeSessionKey); got != "room:replacement" {
		t.Fatalf("旧 lease 不得删除替换路由，实际 dispatch session: %q", got)
	}
	if got := ctx.resolveRouteContext(runtimeSessionKey).RoundID; got != "round-replacement" {
		t.Fatalf("旧 lease 不得回退替换路由上下文，实际 round: %q", got)
	}

	ctx.UnbindSessionRoute(replacementLease)
	if got := ctx.ResolveDispatchSessionKey(runtimeSessionKey); got != runtimeSessionKey {
		t.Fatalf("当前 owner 释放后应回退 runtime session，实际: %q", got)
	}
}
