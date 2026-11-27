// main_inbox_test.go 覆盖 DM 主收件箱、跟进、宿主运行与恢复测试。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type mainInboxFixture struct {
	service    *Service
	client     *fakeDMClient
	factory    *fakeDMFactory
	sender     *dmTestSender
	inbox      *inboxstore.Repository
	sessionKey string
}

func newMainInboxFixture(t *testing.T, sessionRef string) mainInboxFixture {
	t.Helper()
	cfg := newDMTestConfig(t)
	client := newFakeDMClient()
	client.hookResponseAck = true
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	permission := permissionctx.NewContext()
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	inbox := newDMInboxStore(t, cfg)
	service.SetInboxStore(inbox)
	sessionKey := "agent:nexus:ws:dm:" + sessionRef
	sender := newDMTestSender("sender-" + sessionRef)
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)
	return mainInboxFixture{service: service, client: client, factory: factory, sender: sender, inbox: inbox, sessionKey: sessionKey}
}

func (f mainInboxFixture) send(t *testing.T, content string, clientMessageID string) {
	t.Helper()
	if err := f.service.HandleRealtimeChat(context.Background(), Request{
		SessionKey:      f.sessionKey,
		Content:         content,
		ClientMessageID: clientMessageID,
		ClientRequestID: "request-" + clientMessageID,
	}); err != nil {
		t.Fatalf("主窗口发送 %q 失败: %v", content, err)
	}
}

func (f mainInboxFixture) open(t *testing.T) []inboxstore.Event {
	t.Helper()
	events, err := f.inbox.ListOpen(context.Background(), f.sessionKey, time.Now().UTC())
	if err != nil {
		t.Fatalf("读取 inbox 失败: %v", err)
	}
	return events
}

func (f mainInboxFixture) waitOpen(t *testing.T, want int) []inboxstore.Event {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		events := f.open(t)
		if len(events) == want {
			return events
		}
		if time.Now().After(deadline) {
			t.Fatalf("inbox 未完成事件数 = %d, want %d: %+v", len(events), want, events)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f mainInboxFixture) runSafePoint(t *testing.T) []sdkhook.Output {
	t.Helper()
	var outputs []sdkhook.Output
	for _, matcher := range f.factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1")
			if err != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", err)
			}
			if output.SpecificOutput != nil {
				outputs = append(outputs, output)
			}
		}
	}
	return outputs
}

func (f mainInboxFixture) finishRound(id string) {
	f.client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: f.client.sessionID,
		UUID:      "result-" + id,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success", DurationMS: 1, NumTurns: 1},
	}
}

func (f mainInboxFixture) prompts() []string {
	f.client.mu.Lock()
	defer f.client.mu.Unlock()
	return append([]string(nil), f.client.queryPrompts...)
}

func (f mainInboxFixture) waitPrompts(t *testing.T, want int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		prompts := f.prompts()
		if len(prompts) >= want {
			return prompts
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime 查询次数 = %d, want %d", len(prompts), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMainInboxInjectsRunningInputAtSafePointAfterAppliedAck(t *testing.T) {
	f := newMainInboxFixture(t, "inbox-inject")
	f.send(t, "先查一下项目结构", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.waitOpen(t, 0)

	f.send(t, "顺便看看错误日志", "client-2")
	f.send(t, "再列一下 TODO", "client-3")
	if got := f.waitOpen(t, 2); got[0].Payload.Content != "顺便看看错误日志" || got[1].Seq <= got[0].Seq {
		t.Fatalf("运行中输入应按序保留为 pending: %+v", got)
	}

	outputs := f.runSafePoint(t)
	if len(outputs) != 1 {
		t.Fatalf("安全点应恰好注入一次: %+v", outputs)
	}
	injected := outputs[0].SpecificOutput.AdditionalContext
	if !strings.Contains(injected, "顺便看看错误日志") || !strings.Contains(injected, "再列一下 TODO") {
		t.Fatalf("安全点未注入全部 pending: %q", injected)
	}
	for _, event := range f.open(t) {
		if event.Status != inboxstore.StatusDelivering {
			t.Fatalf("applied 之前事件应为 delivering: %+v", event)
		}
	}
	if again := f.runSafePoint(t); len(again) != 0 {
		t.Fatalf("等待 applied 的事件不能被重复注入: %+v", again)
	}

	outputs[0].OnApplied(sdkhook.AppliedAck{})
	f.waitOpen(t, 0)
	userMessages := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["content"] == "再列一下 TODO"
	})
	last := userMessages[len(userMessages)-1]
	if last.Data["role"] != "user" || last.Data["client_message_id"] != "client-3" || last.DeliveryMode != "durable" {
		t.Fatalf("注入的输入应作为 durable 用户消息归入当前回复: %+v", last)
	}

	f.finishRound("inbox-inject")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
	time.Sleep(50 * time.Millisecond)
	if prompts := f.prompts(); len(prompts) != 1 {
		t.Fatalf("已在安全点消费的输入不应再起新一轮: %+v", prompts)
	}
}

func TestMainInboxRunningInputConfirmsOnlyConsumedEvents(t *testing.T) {
	f := newMainInboxFixture(t, "inbox-running-input")
	f.client.runningInput = true
	f.send(t, "先查一下项目结构", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.waitOpen(t, 0)

	f.send(t, "顺便看看错误日志", "client-2")
	f.send(t, "再列一下 TODO", "client-3")
	sent := f.waitRunningInputs(t, 2)
	if sent[0].content != "顺便看看错误日志" || sent[1].content != "再列一下 TODO" {
		t.Fatalf("运行中输入应按 inbox 顺序逐条发送: %+v", sent)
	}
	if outputs := f.runSafePoint(t); len(outputs) != 0 {
		t.Fatalf("协商运行中输入后 PostToolUse 不应再注入: %+v", outputs)
	}

	// 只回执第一条：第二条在 result 前没有回执，视为未消费。
	f.client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeSystem,
		SessionID: f.client.sessionID,
		UUID:      "consumed-1",
		System: &sdkprotocol.SystemMessage{
			Subtype:       sdkprotocol.SystemSubtypeInputConsumed,
			InputConsumed: &sdkprotocol.InputConsumedMessage{InputIDs: []string{sent[0].id}},
		},
	}
	if open := f.waitOpen(t, 1); open[0].EventID != sent[1].id || open[0].Status != inboxstore.StatusDelivering {
		t.Fatalf("未回执的输入应仍为 delivering: %+v", open)
	}
	userMessages := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["content"] == "顺便看看错误日志"
	})
	if last := userMessages[len(userMessages)-1]; last.Data["role"] != "user" || last.Data["client_message_id"] != "client-2" {
		t.Fatalf("回执的输入应落盘为当前回复内的用户消息: %+v", last)
	}

	f.finishRound("inbox-running-input")
	prompts := f.waitPrompts(t, 2)
	if prompts[1] != "再列一下 TODO" {
		t.Fatalf("未消费的输入应在轮末退回并单独起下一轮: %+v", prompts)
	}
}

func (f mainInboxFixture) waitRunningInputs(t *testing.T, want int) []fakeRunningInput {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.client.mu.Lock()
		sent := append([]fakeRunningInput(nil), f.client.runningInputs...)
		f.client.mu.Unlock()
		if len(sent) >= want {
			return sent
		}
		if time.Now().After(deadline) {
			t.Fatalf("运行中输入发送次数 = %d, want %d", len(sent), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMainInboxMergesBacklogIntoNextRound(t *testing.T) {
	f := newMainInboxFixture(t, "inbox-backlog")
	f.send(t, "第一件事", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.send(t, "第二件事", "client-2")
	f.send(t, "第三件事", "client-3")
	f.waitOpen(t, 2)

	// 最后一个安全点之后到达的输入：轮末退回并立即合并成下一轮。
	f.finishRound("inbox-backlog-1")
	prompts := f.waitPrompts(t, 2)
	if !strings.Contains(prompts[1], "第二件事\n\n第三件事") {
		t.Fatalf("积压输入应合并为一轮: %q", prompts[1])
	}
	f.waitOpen(t, 0)
	f.finishRound("inbox-backlog-2")
}

func TestMainInboxWithdrawsOnlyPendingInput(t *testing.T) {
	f := newMainInboxFixture(t, "inbox-withdraw")
	f.send(t, "第一件事", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.send(t, "算了不用了", "client-2")
	pending := f.waitOpen(t, 1)[0]

	withdraw := func(itemID string) error {
		_, err := f.service.HandleInputQueue(context.Background(), InputQueueRequest{
			SessionKey: f.sessionKey,
			Action:     "delete",
			ItemID:     itemID,
		})
		return err
	}
	if err := withdraw(pending.EventID); err != nil {
		t.Fatalf("pending 输入应可撤回: %v", err)
	}
	f.waitOpen(t, 0)
	if err := withdraw(pending.EventID); !errors.Is(err, ErrInboxEventNotPending) {
		t.Fatalf("重复撤回 error = %v, want ErrInboxEventNotPending", err)
	}

	f.finishRound("inbox-withdraw")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
	time.Sleep(50 * time.Millisecond)
	if prompts := f.prompts(); len(prompts) != 1 {
		t.Fatalf("已撤回的输入不应被执行: %+v", prompts)
	}
}

func TestMainInboxDuplicateClientMessageIsAcceptedOnce(t *testing.T) {
	f := newMainInboxFixture(t, "inbox-duplicate")
	f.send(t, "第一件事", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.send(t, "只发一次", "client-2")
	f.send(t, "只发一次", "client-2")
	f.waitOpen(t, 1)
	f.finishRound("inbox-duplicate-1")
	f.waitPrompts(t, 2)
	f.finishRound("inbox-duplicate-2")
}

func TestRoundBatchKeepsSlashCommandsAlone(t *testing.T) {
	event := func(id string, content string) inboxstore.Event {
		return inboxstore.Event{EventID: id, Kind: inboxstore.KindUserMessage, Payload: inboxstore.Payload{Content: content}}
	}
	cases := []struct {
		name    string
		pending []inboxstore.Event
		round   []string
		guide   []string
	}{
		{"empty", nil, nil, nil},
		{"merge plain", []inboxstore.Event{event("a", "x"), event("b", "y")}, []string{"a", "b"}, []string{"a", "b"}},
		{"command first", []inboxstore.Event{event("a", "/compact"), event("b", "y")}, []string{"a"}, nil},
		{"stop before command", []inboxstore.Event{event("a", "x"), event("b", "/compact"), event("c", "y")}, []string{"a"}, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := eventIDs(roundBatch(tc.pending)); strings.Join(got, ",") != strings.Join(tc.round, ",") {
				t.Fatalf("roundBatch = %v, want %v", got, tc.round)
			}
			if got := eventIDs(guidanceBatch(tc.pending)); strings.Join(got, ",") != strings.Join(tc.guide, ",") {
				t.Fatalf("guidanceBatch = %v, want %v", got, tc.guide)
			}
		})
	}
}

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

type fakeMainWindowRooms struct{ conversationID string }

func (r fakeMainWindowRooms) EnsureDirectRoom(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return &protocol.ConversationContextAggregate{Conversation: protocol.ConversationRecord{ID: r.conversationID}}, nil
}

func newMainWindowHostFixture(t *testing.T, ref string) mainInboxFixture {
	t.Helper()
	f := newMainInboxFixture(t, ref)
	f.service.SetMainWindowRooms(fakeMainWindowRooms{conversationID: ref})
	return f
}

func (f mainInboxFixture) appendTask(t *testing.T, runID string) {
	t.Helper()
	sessionKey, err := f.service.AppendMainWindowTask(context.Background(), "nexus", MainWindowTask{
		JobID:       "job-1",
		RunID:       runID,
		Name:        "日报",
		Instruction: "汇总今天的提交",
		DedupeKey:   "scheduled_task:job-1:" + runID,
	})
	if err != nil {
		t.Fatalf("写入主窗口任务失败: %v", err)
	}
	if sessionKey != f.sessionKey {
		t.Fatalf("session key = %q, want %q", sessionKey, f.sessionKey)
	}
}

func isScheduledTaskRow(data map[string]any) bool {
	metadata, _ := data["metadata"].(map[string]any)
	return data["role"] == "user" && data["is_synthetic"] == true &&
		metadata["input_kind"] == scheduledTaskInputKind && metadata["task_name"] == "日报" && metadata["run_id"] != ""
}

func TestMainWindowTaskStartsOwnRoundWhenIdle(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-idle")
	f.appendTask(t, "run-1")
	prompts := f.waitPrompts(t, 1)
	if !strings.Contains(prompts[0], "<scheduled-task name=\"日报\">") || !strings.Contains(prompts[0], "汇总今天的提交") {
		t.Fatalf("模型输入应标明来自定时任务: %q", prompts[0])
	}
	rows := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isScheduledTaskRow(event.Data)
	})
	if len(rows) == 0 {
		t.Fatal("时间线应收到定时任务触发行")
	}
	f.waitOpen(t, 0)
	f.finishRound("host-idle")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
}

func TestMainWindowTaskInjectsAtSafePointWhileRunning(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-running")
	f.send(t, "先查一下项目结构", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.waitOpen(t, 0)

	f.appendTask(t, "run-1")
	f.appendTask(t, "run-1")
	if pending := f.waitOpen(t, 1); pending[0].Kind != inboxstore.KindScheduledTask {
		t.Fatalf("同一次到点只应留一条宿主事件: %+v", pending)
	}
	outputs := f.runSafePoint(t)
	if len(outputs) != 1 || !strings.Contains(outputs[0].SpecificOutput.AdditionalContext, "<scheduled-task name=\"日报\">") {
		t.Fatalf("运行中到点的任务应在安全点注入: %+v", outputs)
	}
	outputs[0].OnApplied(sdkhook.AppliedAck{})
	f.waitOpen(t, 0)
	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isScheduledTaskRow(event.Data)
	})

	f.finishRound("host-running")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
	time.Sleep(50 * time.Millisecond)
	if prompts := f.prompts(); len(prompts) != 1 {
		t.Fatalf("已注入的任务不应再起新一轮: %+v", prompts)
	}
}

func TestMainWindowTaskRunsAloneAndStaysOutOfQueue(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-queue")
	f.send(t, "第一件事", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.appendTask(t, "run-1")
	f.send(t, "第二件事", "client-2")
	f.waitOpen(t, 2)
	// 待发送列表只列用户输入：宿主事件不是用户写的，也不可撤回。
	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		items, _ := event.Data["items"].([]protocol.InputQueueItem)
		return event.EventType == protocol.EventTypeInputQueue && len(items) == 1 && items[0].Content == "第二件事"
	})
	f.finishRound("host-queue-1")
	// 任务独占一轮，用户输入随后单独起轮。
	prompts := f.waitPrompts(t, 2)
	if !strings.Contains(prompts[1], "<scheduled-task") || strings.Contains(prompts[1], "第二件事") {
		t.Fatalf("宿主事件应独占一轮: %q", prompts[1])
	}
	f.finishRound("host-queue-2")
	prompts = f.waitPrompts(t, 3)
	if prompts[2] != "第二件事" {
		t.Fatalf("用户输入应在任务轮之后执行: %q", prompts[2])
	}
	f.finishRound("host-queue-3")
}

func TestRoundBatchKeepsHostEventsAlone(t *testing.T) {
	user := func(id string) inboxstore.Event {
		return inboxstore.Event{EventID: id, Kind: inboxstore.KindUserMessage, Payload: inboxstore.Payload{Content: id}}
	}
	host := func(id string) inboxstore.Event {
		return inboxstore.Event{EventID: id, Kind: inboxstore.KindScheduledTask, Payload: inboxstore.Payload{Content: "/compact"}}
	}
	if got := strings.Join(eventIDs(roundBatch([]inboxstore.Event{host("t"), user("a")})), ","); got != "t" {
		t.Fatalf("宿主事件应独占一轮: %s", got)
	}
	if got := strings.Join(eventIDs(roundBatch([]inboxstore.Event{user("a"), host("t")})), ","); got != "a" {
		t.Fatalf("用户输入不应与宿主事件合并: %s", got)
	}
	// 宿主事件的指令文本不是斜杠命令，安全点可照常注入。
	if got := strings.Join(eventIDs(guidanceBatch([]inboxstore.Event{user("a"), host("t")})), ","); got != "a,t" {
		t.Fatalf("安全点应注入宿主事件: %s", got)
	}
}

func (f mainInboxFixture) reply(id string, text string) {
	f.client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: f.client.sessionID,
		UUID:      "assistant-" + id,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-" + id,
				Model:      "sonnet",
				StopReason: "end_turn",
				Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: text}},
			},
		},
	}
}

func (f mainInboxFixture) history(t *testing.T) []protocol.Message {
	t.Helper()
	ctx := context.Background()
	target, _, err := f.service.resolveMainInboxTarget(ctx, f.sessionKey, "")
	if err != nil {
		t.Fatalf("解析主窗口失败: %v", err)
	}
	session, err := f.service.ensureSession(ctx, target.agent, protocol.ParseSessionKey(f.sessionKey), f.sessionKey)
	if err != nil {
		t.Fatalf("读取会话失败: %v", err)
	}
	rows, err := f.service.History.ForOwner(target.owner()).ReadMessages(target.agent.WorkspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	return rows
}

func isHostRunOutcome(data map[string]any, outcome string) bool {
	metadata, _ := data["metadata"].(map[string]any)
	return data["role"] == "system" && metadata["subtype"] == messageutil.SystemMessageSubtypeHostRunOutcome && metadata["outcome"] == outcome
}

func rolesOf(rows []protocol.Message) []string {
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, protocol.MessageRole(row))
	}
	return roles
}

func TestHostRunQuietLeavesOnlyOutcomeRecord(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-run-quiet")
	f.appendTask(t, "run-1")
	f.waitPrompts(t, 1)
	f.reply("quiet", HostNoReplyMarker)
	f.finishRound("quiet")

	events := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isHostRunOutcome(event.Data, "quiet")
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage && event.Data["role"] == "assistant" {
			t.Fatalf("宿主 run 的过程不应推送: %+v", event.Data)
		}
	}
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")

	rows := f.history(t)
	if roles := rolesOf(rows); len(roles) != 2 || roles[0] != "user" || !isHostRunOutcome(rows[1], "quiet") {
		t.Fatalf("静默的宿主 run 只应留下分隔条与结局: %v", roles)
	}
}

func TestHostRunReplyCommitsOneAssistant(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-run-reply")
	f.appendTask(t, "run-1")
	f.waitPrompts(t, 1)
	f.reply("reply", "今天有 3 个提交")
	f.finishRound("reply")

	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "assistant" &&
			messageutil.ExtractAssistantDisplayText(event.Data) == "今天有 3 个提交"
	})
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")

	assistants := 0
	for _, row := range f.history(t) {
		if isHostRunOutcome(row, "quiet") {
			t.Fatalf("有回复的宿主 run 不应记沉默: %+v", row)
		}
		if protocol.MessageRole(row) == "assistant" {
			assistants++
			if text := messageutil.ExtractAssistantDisplayText(row); text != "今天有 3 个提交" {
				t.Fatalf("历史回复 = %q", text)
			}
		}
	}
	if assistants != 1 {
		t.Fatalf("宿主 run 的回复应恰好一条，实际 %d", assistants)
	}
}

func TestHostRunDefersUserInputUntilDone(t *testing.T) {
	for _, runningInput := range []bool{false, true} {
		t.Run(fmt.Sprintf("running_input=%v", runningInput), func(t *testing.T) {
			f := newMainWindowHostFixture(t, "host-run-defer")
			f.client.runningInput = runningInput
			f.appendTask(t, "run-1")
			f.waitPrompts(t, 1)
			f.send(t, "第二件事", "client-2")
			if open := f.waitOpen(t, 1); open[0].Status != inboxstore.StatusPending {
				t.Fatalf("宿主 run 期间的用户输入应留在 inbox: %+v", open)
			}
			if outputs := f.runSafePoint(t); len(outputs) != 0 {
				t.Fatalf("宿主 run 不应在安全点注入用户输入: %+v", outputs)
			}
			f.client.mu.Lock()
			sent := len(f.client.runningInputs)
			f.client.mu.Unlock()
			if sent != 0 {
				t.Fatalf("宿主 run 不应接收运行中输入: %d", sent)
			}
			f.finishRound("defer-1")
			if prompts := f.waitPrompts(t, 2); prompts[1] != "第二件事" {
				t.Fatalf("用户输入应在宿主 run 收口后起新一轮: %q", prompts[1])
			}
			f.finishRound("defer-2")
		})
	}
}

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
