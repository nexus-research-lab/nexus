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

func TestContextBindAndPromoteController(t *testing.T) {
	ctx := NewContext()
	senderA := &stubSender{key: "a"}
	senderB := &stubSender{key: "b"}
	senderC := &stubSender{key: "c"}

	snapshot := ctx.BindSession("session-1", senderA, "client-a", true)
	if snapshot.ControllerClientID != "client-a" || snapshot.BoundClientCount != 1 || snapshot.ObserverCount != 0 {
		t.Fatalf("首次绑定快照错误: %+v", snapshot)
	}

	snapshot = ctx.BindSession("session-1", senderB, "client-b", false)
	if snapshot.ControllerClientID != "client-a" || snapshot.BoundClientCount != 2 || snapshot.ObserverCount != 1 {
		t.Fatalf("观察者绑定快照错误: %+v", snapshot)
	}

	snapshot = ctx.BindSession("session-1", senderC, "client-c", true)
	if snapshot.ControllerClientID != "client-c" || snapshot.BoundClientCount != 3 || snapshot.ObserverCount != 2 {
		t.Fatalf("抢占控制权失败: %+v", snapshot)
	}

	snapshot = ctx.UnbindSession("session-1", senderC)
	if snapshot.ControllerClientID != "client-b" {
		t.Fatalf("控制端晋升错误: %+v", snapshot)
	}
	if !ctx.IsSessionController("session-1", senderB) {
		t.Fatal("senderB 应晋升为控制端")
	}
}

func TestContextBroadcastSessionStatus(t *testing.T) {
	ctx := NewContext()
	senderA := &stubSender{key: "a"}
	senderB := &stubSender{key: "b"}
	ctx.BindSession("session-1", senderA, "client-a", true)
	ctx.BindSession("session-1", senderB, "client-b", false)

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
	if event.Data["controller_client_id"] != "client-a" {
		t.Fatalf("控制端字段错误: %+v", event.Data)
	}
	if event.Data["bound_client_count"] != 2 {
		t.Fatalf("绑定数错误: %+v", event.Data)
	}
}
