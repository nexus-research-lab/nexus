package dm

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	_ "modernc.org/sqlite"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestServiceHandleChatQueuesRunningRoundByDefault(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-queue-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-queue")
	sessionKey := "agent:nexus:ws:dm:test-queue"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先做一个长任务",
		RoundID:    "round-queue-1",
		ReqID:      "round-queue-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "这是补充要求",
		RoundID:    "round-queue-2",
		ReqID:      "round-queue-2",
	}); err != nil {
		t.Fatalf("第二条排队消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-queue-2"
	})

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 1 ||
		!strings.Contains(sentContents[0], "这是补充要求") ||
		!strings.Contains(sentContents[0], "<nexus_runtime_context>") {
		t.Fatalf("运行中 DM round 未收到排队输入: %+v", sentContents)
	}
	if len(factory.options) != 1 {
		t.Fatalf("排队输入不应创建新 runtime client: got=%d want=1", len(factory.options))
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceHandleChatGuidePolicyQueuesHookGuidance(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-guide-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide")
	sessionKey := "agent:nexus:ws:dm:test-guide"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查一下项目结构",
		RoundID:    "round-guide-1",
		ReqID:      "round-guide-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "等工具结果回来后优先看错误日志",
		RoundID:              "round-guide-2",
		ReqID:                "round-guide-2",
		DeliveryPolicy:       protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("引导消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-guide-2"
	})
	guidanceEvents := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "system"
	})
	guidanceEvent := guidanceEvents[len(guidanceEvents)-1]
	if guidanceEvent.Data["round_id"] != "round-guide-1" || guidanceEvent.Data["message_id"] != "round-guide-2" {
		t.Fatalf("引导消息应归入运行中的 round: %+v", guidanceEvent.Data)
	}
	if guidanceEvent.DeliveryMode != "ephemeral" {
		t.Fatalf("引导消息只应作为实时展示事件广播: %+v", guidanceEvent)
	}
	metadata, _ := guidanceEvent.Data["metadata"].(map[string]any)
	if metadata["subtype"] != "guided_input" {
		t.Fatalf("引导消息缺少 typed metadata: %+v", guidanceEvent.Data)
	}

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("引导不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 0 {
		t.Fatalf("引导不应走普通 streaming input: %+v", sentContents)
	}
	if count := runtimeManager.PendingGuidanceCount(sessionKey); count != 1 {
		t.Fatalf("引导输入未登记到运行时队列: count=%d", count)
	}
	if len(factory.options) != 1 {
		t.Fatalf("引导不应创建新 runtime client: got=%d want=1", len(factory.options))
	}
	if matchers := factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse]; len(matchers) == 0 {
		t.Fatalf("runtime options 未挂载 PostToolUse 引导 hook: %+v", factory.options[0].Hooks)
	}
	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	for _, row := range rows {
		if row["message_id"] == "round-guide-2" {
			t.Fatalf("引导消息不应直接写入 overlay 历史，历史回放应来自 runtime transcript: %+v", rows)
		}
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceInputQueueGuideWaitsForPostToolUse(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide-input-queue")
	sessionKey := "agent:nexus:ws:dm:test-guide-input-queue"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-input-queue-1",
		ReqID:      "round-guide-input-queue-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey:     sessionKey,
		Action:         "enqueue",
		Content:        "路径发给我吧",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 DM 待发送队列失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("待发送队列应保留一条消息: %+v", items)
	}
	itemID := items[0].ID

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "guide",
		ItemID:     itemID,
	}); err != nil {
		t.Fatalf("标记 DM 引导队列失败: %v", err)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取标记后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != itemID ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "" {
		t.Fatalf("点击引导后应保留为可跨 round 注入的队列项: %+v", items)
	}

	var additionalContext string
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPostToolUse,
			}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", hookErr)
			}
			if output.SpecificOutput != nil && output.SpecificOutput.AdditionalContext != "" {
				text := output.SpecificOutput.AdditionalContext
				additionalContext = text
			}
		}
	}
	if !strings.Contains(additionalContext, "路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_"+itemID) {
		t.Fatalf("PostToolUse hook 未注入队列引导: %q", additionalContext)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取消费后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PostToolUse 真正注入后才应消费队列项: %+v", items)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}
