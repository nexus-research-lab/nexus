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
}
