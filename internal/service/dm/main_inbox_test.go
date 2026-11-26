package dm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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
	migrateDMSQLite(t, cfg.DatabaseURL)
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
		return inboxstore.Event{EventID: id, Payload: inboxstore.Payload{Content: content}}
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
