package dm

import (
	"context"
	"strings"
	"testing"
	"time"

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
		SessionKey:           sessionKey,
		Content:              "先做一个长任务",
		RoundID:              "round-queue-1",
		UserMessageID:        "msg-user-queue-1",
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "这是补充要求",
		RoundID:              "round-queue-2",
		UserMessageID:        "msg-user-queue-2",
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("第二条排队消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-queue-2"
	})
	messageEvents := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["message_id"] == "msg-user-queue-2"
	})
	queuedMessage := messageEvents[len(messageEvents)-1].Data
	if queuedMessage["round_id"] != "round-queue-1" || queuedMessage["source_round_id"] != "round-queue-2" {
		t.Fatalf("运行中 DM 输入应归入实际回复 round: %+v", queuedMessage)
	}

	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	var persisted protocol.Message
	for _, row := range rows {
		if row["message_id"] == "msg-user-queue-2" {
			persisted = row
			break
		}
	}
	if persisted == nil || persisted["round_id"] != "round-queue-1" || persisted["source_round_id"] != "round-queue-2" {
		t.Fatalf("运行中 DM 输入历史归组不正确: %+v", rows)
	}
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取测试 Agent 失败: %v", err)
	}
	sessionItem, err := service.ensureSession(context.Background(), agentValue, protocol.ParseSessionKey(sessionKey), sessionKey)
	if err != nil {
		t.Fatalf("读取测试 session 失败: %v", err)
	}
	index, err := service.history.ReadRoundIndex(agentValue.WorkspacePath, sessionItem, nil)
	if err != nil {
		t.Fatalf("读取 DM round 索引失败: %v", err)
	}
	if len(index.Items) != 1 || index.Items[0].RoundID != "round-queue-1" {
		t.Fatalf("运行中 DM 输入不应生成空 source round: %+v", index.Items)
	}

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	// 轮内排队注入不再带 runtime context（情绪态），避免污染前缀缓存。
	if len(sentContents) != 1 ||
		!strings.Contains(sentContents[0], "这是补充要求") {
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
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "等工具结果回来后优先看错误日志",
		RoundID:              "round-guide-2",
		UserMessageID:        "msg-user-guide-2",
		DeliveryPolicy:       protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("引导消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-guide-2"
	})
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 引导队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != "round-guide-2" ||
		items[0].SourceMessageID != "msg-user-guide-2" ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "round-guide-1" {
		t.Fatalf("引导消息应先持久等待当前 round 的 PostToolUse: %+v", items)
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
	if count := runtimeManager.PendingGuidanceCount(sessionKey); count != 0 {
		t.Fatalf("DM 引导应由持久队列而非内存 runtime 队列承载: count=%d", count)
	}
	if len(factory.options) != 1 {
		t.Fatalf("引导不应创建新 runtime client: got=%d want=1", len(factory.options))
	}
	if matchers := factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse]; len(matchers) == 0 {
		t.Fatalf("runtime options 未挂载 PostToolUse 引导 hook: %+v", factory.options[0].Hooks)
	}
	var additionalContext string
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", hookErr)
			}
			if output.SpecificOutput != nil {
				additionalContext += output.SpecificOutput.AdditionalContext
			}
		}
	}
	if !strings.Contains(additionalContext, "等工具结果回来后优先看错误日志") ||
		!strings.Contains(additionalContext, "round-guide-2") {
		t.Fatalf("PostToolUse hook 未注入引导: %q", additionalContext)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取待确认 DM 引导队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("hook 返回后、模型继续输出前必须保留 durable 引导: %+v", items)
	}
	for _, row := range readDMSessionHistory(t, cfg, service, sessionKey) {
		if row["message_id"] == "msg-user-guide-2" {
			t.Fatalf("模型继续输出前不应提前重排引导消息: %+v", row)
		}
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:      "assistant-guide-confirmed",
			Model:   "sonnet",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "我会优先看错误日志。"}},
		}},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-guide-confirmed",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
		},
	}
	guidanceEvents := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["message_id"] == "msg-user-guide-2"
	})
	guidanceEvent := guidanceEvents[len(guidanceEvents)-1]
	if guidanceEvent.Data["role"] != "user" ||
		guidanceEvent.Data["round_id"] != "round-guide-1" ||
		guidanceEvent.Data["source_round_id"] != "round-guide-2" ||
		guidanceEvent.DeliveryMode != "durable" {
		t.Fatalf("已消费引导应作为 durable user 归入当前回复: %+v", guidanceEvent)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取已消费 DM 引导队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PostToolUse 消费后应移除引导队列项: %+v", items)
	}
	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	foundGuidance := false
	for _, row := range rows {
		if row["message_id"] == "msg-user-guide-2" {
			foundGuidance = row["role"] == "user" &&
				row["round_id"] == "round-guide-1" &&
				row["source_round_id"] == "round-guide-2"
		}
	}
	if !foundGuidance {
		t.Fatalf("已消费引导应持久化到实际回复 round: %+v", rows)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
}

func TestServiceGuidancePreflightFailureKeepsQueuedInput(t *testing.T) {
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
				UUID:      "result-guide-preflight-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client}),
		permission,
	)
	sender := newDMTestSender("sender-guide-preflight-failure")
	sessionKey := "agent:nexus:ws:dm:test-guide-preflight-failure"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-preflight-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	t.Cleanup(func() {
		if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
			t.Errorf("清理运行中 round 失败: %v", err)
		}
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:     sessionKey,
		Content:        "补充一个失效附件",
		RoundID:        "round-guide-preflight-2",
		UserMessageID:  "msg-user-guide-preflight-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		Attachments: []protocol.ChatAttachment{{
			FileName:         "missing.txt",
			WorkspacePath:    "missing.txt",
			WorkspaceAgentID: "missing-agent",
			Scope:            protocol.ChatAttachmentScopeAgentWorkspace,
			Kind:             protocol.ChatAttachmentKindFile,
		}},
	}); err != nil {
		t.Fatalf("写入待预检 DM 引导失败: %v", err)
	}

	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	before, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取预检前 DM 引导队列失败: %v", err)
	}
	if len(before) != 1 || before[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide {
		t.Fatalf("预检前应持久保留一条引导: %+v", before)
	}

	output, hookErr := service.inputQueueGuidanceHook(sessionKey, location)(
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-1",
	)
	if hookErr == nil {
		t.Fatal("失效附件应使 DM 引导预渲染失败")
	}
	if output.SpecificOutput != nil {
		t.Fatalf("预渲染失败不应向模型注入部分上下文: %+v", output)
	}

	after, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取预检失败后的 DM 引导队列失败: %v", err)
	}
	if len(after) != 1 ||
		after[0].ID != before[0].ID ||
		after[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		after[0].RootRoundID != before[0].RootRoundID ||
		len(after[0].Attachments) != 1 {
		t.Fatalf("预渲染失败后不应丢失或改写引导: before=%+v after=%+v", before, after)
	}
}

func TestServiceGuidanceErrorResultFallsBackToNextTurn(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	fallbackStarted := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		if !strings.Contains(prompt, "失败后作为下一轮继续") {
			return
		}
		fallbackStarted <- prompt
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
					ID:      "assistant-guide-fallback",
					Model:   "sonnet",
					Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "已继续处理。"}},
				}},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-guide-fallback-success",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide-error-fallback")
	sessionKey := "agent:nexus:ws:dm:test-guide-error-fallback"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先执行一个工具任务",
		RoundID:    "round-guide-error-1",
	}); err != nil {
		t.Fatalf("启动 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	if err := service.HandleChat(context.Background(), Request{
		SessionKey:     sessionKey,
		Content:        "失败后作为下一轮继续",
		RoundID:        "round-guide-error-2",
		UserMessageID:  "msg-user-guide-error-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatalf("写入 DM 引导失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	output, err := service.inputQueueGuidanceHook(sessionKey, location)(
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-error",
	)
	if err != nil || output.SpecificOutput == nil {
		t.Fatalf("注册待确认 DM 引导失败: output=%+v err=%v", output, err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("控制响应确认前引导必须持久保留: items=%+v err=%v", items, err)
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-guide-error",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "error",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "tool failed",
			IsError:    true,
		},
	}
	select {
	case prompt := <-fallbackStarted:
		if !strings.Contains(prompt, "失败后作为下一轮继续") {
			t.Fatalf("失败后的下一轮未收到原引导: %q", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("错误终态没有把未确认引导接成下一轮")
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == "finished" &&
			event.Data["round_id"] != "round-guide-error-1"
	})
}

func TestServiceReleaseUndeliveredGuidanceRestoresQueue(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()}),
		permissionctx.NewContext(),
	)
	sessionKey := "agent:nexus:ws:dm:test-guide-fallback"
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	if _, err = service.inputQueue.Enqueue(location, protocol.InputQueueItem{
		ID:              "round-guide-fallback-2",
		Scope:           protocol.InputQueueScopeDM,
		SessionKey:      sessionKey,
		SourceMessageID: "msg-user-guide-fallback-2",
		Content:         "如果本轮没有下一次工具调用也不能丢",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		RootRoundID:     "round-guide-fallback-1",
	}); err != nil {
		t.Fatalf("写入 DM 引导队列失败: %v", err)
	}

	service.releaseUndeliveredInputQueueGuidance(
		context.Background(),
		sessionKey,
		location,
		"round-guide-fallback-1",
	)
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取恢复后的 DM 队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != "round-guide-fallback-2" ||
		items[0].SourceMessageID != "msg-user-guide-fallback-2" ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		items[0].RootRoundID != "" {
		t.Fatalf("未消费引导应恢复成下一轮普通输入: %+v", items)
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
		items[0].RootRoundID != "round-guide-input-queue-1" {
		t.Fatalf("点击引导后应锚定当前运行 round，避免被其他回复消费: %+v", items)
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
		t.Fatalf("读取待确认 DM 引导失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("首次 hook 返回后必须等待后续可观察行为确认: %+v", items)
	}
	var confirmationContext string
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPostToolUse,
			}, "tool-2")
			if hookErr != nil {
				t.Fatalf("第二次 PostToolUse 确认前次引导失败: %v", hookErr)
			}
			if output.SpecificOutput != nil {
				confirmationContext += output.SpecificOutput.AdditionalContext
			}
		}
	}
	if strings.Contains(confirmationContext, "路径发给我吧") {
		t.Fatalf("确认前次引导时不应在同一 round 重复注入: %q", confirmationContext)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取确认后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("下一次 PostToolUse 应先确认并消费前次引导: %+v", items)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}
