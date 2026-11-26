package dm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
)

func TestFollowUpScheduleListCancel(t *testing.T) {
	f := newMainInboxFixture(t, "follow-up-crud")
	ctx := context.Background()
	agentID := f.agentID(t)

	if _, err := f.service.ScheduleFollowUp(ctx, f.sessionKey, agentID, FollowUpRequest{After: time.Minute, Note: "x"}); err == nil {
		t.Fatal("不足 5 分钟的跟进必须拒绝")
	}
	if _, err := f.service.ScheduleFollowUp(ctx, f.sessionKey, agentID, FollowUpRequest{
		After: 10 * time.Minute, At: time.Now().Add(time.Hour), Note: "x",
	}); err == nil {
		t.Fatal("after 与 at 同时给出必须拒绝")
	}
	first, err := f.service.ScheduleFollowUp(ctx, f.sessionKey, agentID, FollowUpRequest{
		After: 30 * time.Minute, Note: "看一下部署是否结束", RequestKey: "req-follow-up-1",
	})
	if err != nil {
		t.Fatalf("设定跟进失败: %v", err)
	}
	again, err := f.service.ScheduleFollowUp(ctx, f.sessionKey, agentID, FollowUpRequest{
		After: 40 * time.Minute, Note: "重试", RequestKey: "req-follow-up-1",
	})
	if err != nil || again.ID != first.ID {
		t.Fatalf("同一 request_id 重试应返回原跟进: %+v err=%v", again, err)
	}
	items, err := f.service.ListFollowUps(ctx, f.sessionKey, agentID)
	if err != nil || len(items) != 1 || items[0].Note != "看一下部署是否结束" {
		t.Fatalf("list = %+v err=%v", items, err)
	}
	if len(f.open(t)) != 0 {
		t.Fatal("未到点的跟进不能出现在可取走事件里")
	}
	if err := f.service.CancelFollowUp(ctx, f.sessionKey, agentID, first.ID); err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	if err := f.service.CancelFollowUp(ctx, f.sessionKey, agentID, first.ID); !errors.Is(err, ErrFollowUpNotFound) {
		t.Fatalf("重复取消应返回 not found: %v", err)
	}
}

func TestFollowUpPendingCap(t *testing.T) {
	f := newMainInboxFixture(t, "follow-up-cap")
	agentID := f.agentID(t)
	for i := 0; i < FollowUpMaxPending; i++ {
		if _, err := f.service.ScheduleFollowUp(context.Background(), f.sessionKey, agentID, FollowUpRequest{
			After: time.Duration(10+i) * time.Minute, Note: "check",
		}); err != nil {
			t.Fatalf("第 %d 条失败: %v", i+1, err)
		}
	}
	if _, err := f.service.ScheduleFollowUp(context.Background(), f.sessionKey, agentID, FollowUpRequest{
		After: time.Hour, Note: "one more",
	}); err == nil {
		t.Fatal("超过上限必须拒绝")
	}
}

func TestDueFollowUpStartsRoundWithNote(t *testing.T) {
	f := newMainInboxFixture(t, "follow-up-due")
	f.appendHostEvent(t, "follow-1", inboxstore.KindFollowUp, "回头确认 CI 结果", time.Now().UTC().Add(-time.Second))
	if _, err := f.service.reconcileMainInboxes(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("reconcile 失败: %v", err)
	}
	prompts := f.waitPrompts(t, 1)
	if !strings.Contains(prompts[0], "<follow-up") || !strings.Contains(prompts[0], "回头确认 CI 结果") {
		t.Fatalf("到点跟进应以 follow-up 输入起轮: %q", prompts[0])
	}
}

func TestUserInputCancelsPendingEcho(t *testing.T) {
	f := newMainInboxFixture(t, "echo-cancel")
	f.appendHostEvent(t, "echo-1", inboxstore.KindEcho, "", time.Now().UTC().Add(time.Hour))
	f.send(t, "我回来了", "client-echo-cancel")
	pending, err := f.inbox.ListPendingKind(context.Background(), f.sessionKey, inboxstore.KindEcho)
	if err != nil || len(pending) != 0 {
		t.Fatalf("用户开口后 echo 应被取消: %+v err=%v", pending, err)
	}
}

func TestEchoIsNeverInjectedAtSafePoint(t *testing.T) {
	events := []inboxstore.Event{
		{EventID: "echo", Kind: inboxstore.KindEcho},
		{EventID: "user", Kind: inboxstore.KindUserMessage, Payload: inboxstore.Payload{Content: "继续"}},
	}
	batch := guidanceBatch(events)
	if len(batch) != 1 || batch[0].EventID != "user" {
		t.Fatalf("echo 只能在空闲时起轮，不能进安全点注入: %+v", batch)
	}
}

func (f mainInboxFixture) agentID(t *testing.T) string {
	t.Helper()
	target, ok, err := f.service.resolveMainInboxTarget(context.Background(), f.sessionKey, "")
	if err != nil || !ok {
		t.Fatalf("fixture 应是主窗口: ok=%v err=%v", ok, err)
	}
	return target.agent.AgentID
}

func (f mainInboxFixture) appendHostEvent(t *testing.T, id string, kind string, content string, availableAt time.Time) {
	t.Helper()
	target, _, err := f.service.resolveMainInboxTarget(context.Background(), f.sessionKey, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.inbox.Append(context.Background(), inboxstore.Event{
		EventID: id, OwnerUserID: target.owner(), InboxKey: f.sessionKey, Kind: kind,
		DedupeKey: kind + ":" + id, AvailableAt: availableAt, Payload: inboxstore.Payload{Content: content},
	}); err != nil {
		t.Fatal(err)
	}
}
