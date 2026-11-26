package dm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// fakeRecoveryContext 只回答执行恢复是否仍然有效，其余 WorkGraph 上下文为空。
type fakeRecoveryContext struct{ current bool }

func (f fakeRecoveryContext) RuntimeContext(context.Context, orchestrationsvc.ActorContext) (string, error) {
	return "", nil
}

func (f fakeRecoveryContext) RecoveryExecutionCurrent(context.Context, string, string, string) (bool, error) {
	return f.current, nil
}

// testWorkBinding 是一份完整的 DM 工作绑定（DM 不带 dispatch_id）。
func testWorkBinding(execution string) protocol.ExecutionWorkBinding {
	return protocol.ExecutionWorkBinding{ExecutionID: execution, PlanID: "p1", WorkItemID: "w1", SpecID: "s1", AssignmentID: "as1", AttemptID: "a1"}
}

func (f mainInboxFixture) target(t *testing.T) mainInboxTarget {
	t.Helper()
	target, ok, err := f.service.resolveMainInboxTarget(context.Background(), f.sessionKey, "")
	if err != nil || !ok {
		t.Fatalf("解析主窗口失败: ok=%v err=%v", ok, err)
	}
	return target
}

func (f mainInboxFixture) waitStatus(t *testing.T, key string, want string) inboxstore.Event {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		event, found, err := f.inbox.FindByDedupe(context.Background(), f.sessionKey, hashedInboxDedupeKey("host_recovery", key))
		if err != nil {
			t.Fatal(err)
		}
		if found && event.Status == want {
			return event
		}
		if time.Now().After(deadline) {
			t.Fatalf("恢复事件 %s 状态 = %+v found=%v, want %s", key, event, found, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMainInboxRecoveryEnqueueIsIdempotentAndCoalesces(t *testing.T) {
	f := newMainInboxFixture(t, "recovery-coalesce")
	ctx := runtimehost.DeferRecoveryDispatch(context.Background())
	target := f.target(t)
	source := inboxstore.RecoverySource{ExecutionID: "exec-1"}
	first, err := f.service.enqueueMainInboxRecovery(ctx, target, "key-1", "inspect", source)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.service.enqueueMainInboxRecovery(ctx, target, "key-1", "inspect", source)
	if err != nil || again.EventID != first.EventID {
		t.Fatalf("同键同意图应返回原事件: %+v %v", again, err)
	}
	if _, err = f.service.enqueueMainInboxRecovery(ctx, target, "key-1", "other", source); !errors.Is(err, workspacestore.ErrInputQueueIdempotencyConflict) {
		t.Fatalf("同键不同意图应冲突: %v", err)
	}
	merged, err := f.service.enqueueMainInboxRecovery(ctx, target, "key-2", "inspect", source)
	if err != nil || merged.EventID != first.EventID {
		t.Fatalf("同一 execution 的待处理恢复应合并: %+v %v", merged, err)
	}
	if alias := f.waitStatus(t, "key-2", inboxstore.StatusConsumed); alias.EventID == first.EventID {
		t.Fatal("合并的入队应留下自己的记录行")
	}
	binding := testWorkBinding("exec-1")
	work := &binding
	distinct, err := f.service.enqueueMainInboxRecovery(ctx, target, "key-3", "inspect", inboxstore.RecoverySource{ExecutionID: "exec-1", Work: work})
	if err != nil || distinct.EventID == first.EventID {
		t.Fatalf("绑定不同的恢复不能合并: %+v %v", distinct, err)
	}
	if open := pendingEvents(f.open(t)); len(open) != 2 {
		t.Fatalf("待处理恢复数 = %d, want 2: %+v", len(open), open)
	}
}

func TestMainInboxContinuationAdmissionFollowsGateRevision(t *testing.T) {
	f := newMainInboxFixture(t, "recovery-gate")
	ctx := runtimehost.DeferRecoveryDispatch(context.Background())
	target := f.target(t)
	if err := f.service.pauseMainInbox(ctx, target); err != nil {
		t.Fatal(err)
	}
	gate, err := f.inbox.Gate(ctx, f.sessionKey)
	if err != nil || !gate.Paused || gate.Revision != 1 {
		t.Fatalf("停止后闸门 = %+v %v", gate, err)
	}
	work := testWorkBinding("exec-1")
	stale := workGraphRecoverySource(&work, nil, 0)
	if _, err = f.service.enqueueMainInboxRecovery(ctx, target, "stale", "continue", stale); !errors.Is(err, workspacestore.ErrContinuationStopped) {
		t.Fatalf("过期代数的续跑应被拒绝: %v", err)
	}
	admitted, err := f.service.enqueueMainInboxRecovery(ctx, target, "admitted", "continue", workGraphRecoverySource(&work, nil, gate.Revision))
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := f.service.enqueueMainInboxRecovery(ctx, target, "blocked", "inspect", inboxstore.RecoverySource{ExecutionID: "exec-2"})
	if err != nil {
		t.Fatal(err)
	}
	open := f.open(t)
	if got := eventIDs(dispatchableEvents(open, gate)); strings.Join(got, ",") != admitted.EventID {
		t.Fatalf("暂停时只放行显式续跑: got %v, admitted %s blocked %s", got, admitted.EventID, blocked.EventID)
	}
	if err = f.service.resumeMainInbox(ctx, target); err != nil {
		t.Fatal(err)
	}
	if gate, err = f.inbox.Gate(ctx, f.sessionKey); err != nil || gate.Paused || gate.Revision != 2 {
		t.Fatalf("恢复后闸门 = %+v %v", gate, err)
	}
	if err = f.service.resumeMainInbox(ctx, target); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.inbox.Gate(ctx, f.sessionKey); again.Revision != 2 {
		t.Fatalf("未暂停时恢复不应推进代数: %+v", again)
	}
	if got := dispatchableEvents(open, gate); len(got) != 2 {
		t.Fatalf("恢复后全部恢复输入可派发: %+v", got)
	}
}

func TestMainInboxStaleRecoveryIsConsumedWithoutRound(t *testing.T) {
	f := newMainInboxFixture(t, "recovery-stale")
	f.service.SetExecutionContextProvider(fakeRecoveryContext{current: false})
	if _, err := f.service.enqueueMainInboxRecovery(context.Background(), f.target(t), "stale", "inspect", inboxstore.RecoverySource{ExecutionID: "exec-1"}); err != nil {
		t.Fatal(err)
	}
	f.waitStatus(t, "stale", inboxstore.StatusConsumed)
	if prompts := f.prompts(); len(prompts) != 0 {
		t.Fatalf("陈旧恢复不应起轮: %v", prompts)
	}
}

func TestMainInboxCurrentRecoveryRunsHiddenRound(t *testing.T) {
	f := newMainInboxFixture(t, "recovery-current")
	f.service.SetExecutionContextProvider(fakeRecoveryContext{current: true})
	event, err := f.service.enqueueMainInboxRecovery(context.Background(), f.target(t), "current", "inspect execution exec-1", inboxstore.RecoverySource{ExecutionID: "exec-1"})
	if err != nil {
		t.Fatal(err)
	}
	if prompts := f.waitPrompts(t, 1); !strings.Contains(prompts[0], "inspect execution exec-1") {
		t.Fatalf("恢复轮缺少宿主指令: %q", prompts[0])
	}
	f.finishRound("recovery-current")
	consumed := f.waitStatus(t, "current", inboxstore.StatusConsumed)
	if want := strings.Replace(event.EventID, "inbox_", "round_", 1); consumed.RoundID != want {
		t.Fatalf("恢复轮 ID = %q, want %q", consumed.RoundID, want)
	}
}

func TestMainWindowStopPausesGateAndNextMessageResumes(t *testing.T) {
	f := newMainInboxFixture(t, "recovery-stop")
	f.send(t, "第一条", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	if err := f.service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: f.sessionKey, UserStop: true}); err != nil {
		t.Fatal(err)
	}
	if gate, err := f.inbox.Gate(context.Background(), f.sessionKey); err != nil || !gate.Paused {
		t.Fatalf("用户停止应暂停主窗口闸门: %+v %v", gate, err)
	}
	f.send(t, "第二条", "client-2")
	if gate, err := f.inbox.Gate(context.Background(), f.sessionKey); err != nil || gate.Paused {
		t.Fatalf("用户开口应恢复主窗口闸门: %+v %v", gate, err)
	}
}

func TestInboxKindSpecsAreComplete(t *testing.T) {
	for kind, spec := range inboxKinds {
		if spec.start == nil {
			t.Errorf("%s 缺少起轮入口", kind)
		}
		if spec.host && spec.joinsRunning && spec.prompt == nil {
			t.Errorf("%s 可注入运行中轮次却没有提示词", kind)
		}
	}
	for _, kind := range []string{inboxstore.KindUserMessage, inboxstore.KindHostRecovery, inboxstore.KindIMReply} {
		if _, ok := inboxKinds[kind]; !ok {
			t.Errorf("缺少 %s 的规格", kind)
		}
	}
}
