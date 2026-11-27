// input_queue_test.go 覆盖 DM 输入队列、游标、配置溯源、派发锁与中断测试。
package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

type dmConversationActivitySpy struct {
	mu              sync.Mutex
	conversationIDs []string
}

func (s *dmConversationActivitySpy) MarkConversationStarted(
	_ context.Context,
	conversationID string,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversationIDs = append(s.conversationIDs, conversationID)
	return nil
}

func (s *dmConversationActivitySpy) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.conversationIDs...)
}

func TestValidateDMRequestRejectsNonInternalHiddenOrSyntheticInput(t *testing.T) {
	service := &Service{}
	base := Request{
		SessionKey: "agent:nexus:ws:dm:input-kind",
		Content:    "input",
	}
	tests := []struct {
		name         string
		internal     bool
		inputOptions sdkprotocol.OutboundMessageOptions
		wantErr      bool
	}{
		{
			name:         "hidden user",
			inputOptions: sdkprotocol.OutboundMessageOptions{HiddenFromUser: true},
			wantErr:      true,
		},
		{
			name:         "synthetic user",
			inputOptions: sdkprotocol.OutboundMessageOptions{Synthetic: true},
			wantErr:      true,
		},
		{
			name:         "internal hidden synthetic",
			internal:     true,
			inputOptions: sdkprotocol.OutboundMessageOptions{HiddenFromUser: true, Synthetic: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			request.Internal = test.internal
			request.InputOptions = test.inputOptions
			_, _, err := service.validateRequest(request)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRequest() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestDMGuidanceAppliedAckDoesNotConsumeNewerBatch(t *testing.T) {
	storeRoot := t.TempDir()
	store := workspacestore.NewInputQueueStore(storeRoot)
	location := workspacestore.InputQueueLocation{
		Scope: protocol.InputQueueScopeDM, WorkspacePath: storeRoot, SessionKey: "agent:nexus:ws:dm:ack-race",
	}
	items, err := store.Enqueue(location, protocol.InputQueueItem{
		ID: "same-item", Content: "new batch", Source: protocol.InputQueueSourceUser,
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide, RootRoundID: "round-1",
	})
	if err != nil || len(items) != 1 {
		t.Fatalf("enqueue newer batch: items=%+v err=%v", items, err)
	}
	current := preparedDMGuidance{item: items[0], sourceRoundID: "new", targetRoundID: "round-1", content: "new batch"}
	service := &Service{
		Host: runtimehost.Host{InputQueue: store},
		inputQueueGuidancePending: map[string][]preparedDMGuidance{
			pendingDMGuidanceKey(location.SessionKey, "round-1"): {current},
		},
	}
	stale := current
	stale.item.Content = "old batch"
	stale.item.UpdatedAt--
	stale.content = "old batch"
	if err = service.confirmPendingInputQueueGuidance(
		context.Background(), location.SessionKey, location, "round-1", []preparedDMGuidance{stale},
	); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.Snapshot(location)
	if err != nil || len(remaining) != 1 || remaining[0].Content != "new batch" {
		t.Fatalf("stale ACK consumed newer DM batch: items=%+v err=%v", remaining, err)
	}
}

func TestServiceHandleChatGuidePolicyQueuesHookGuidance(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		client.sendResultAsync(client.sessionID, "result-guide-cleanup", "interrupted", "")
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	activity := &dmConversationActivitySpy{}
	service.SetRoomConversationActivityStore(activity)
	sender := newDMTestSender("sender-guide")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查一下项目结构",
		RoundID:    "round-guide-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	if got := activity.snapshot(); len(got) != 1 || got[0] != "test-guide" {
		t.Fatalf("首条直接用户消息应消费 Room DM draft: %+v", got)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:                  sessionKey,
		Content:                     "等工具结果回来后优先看错误日志",
		ClientMessageID:             "client-message-guide-2",
		RoundID:                     "round-guide-2",
		UserMessageID:               "msg-user-guide-2",
		AgentRoundID:                "agent-round-guide-2",
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage:        true,
		TrustedConfigurationContext: true,
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
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 引导队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != "round-guide-2" ||
		items[0].SourceMessageID != "msg-user-guide-2" ||
		items[0].AgentRoundID != "agent-round-guide-2" ||
		items[0].ClientMessageID != "client-message-guide-2" ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "round-guide-1" {
		t.Fatalf("引导消息应先持久等待当前 round 的 PostToolUse: %+v", items)
	}
	if got := activity.snapshot(); len(got) != 1 {
		t.Fatalf("尚未物化的 DM guide 不应提前消费 draft: %+v", got)
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
	items, err = service.InputQueue.Snapshot(location)
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
		guidanceEvent.Data["agent_round_id"] != "agent-round-guide-2" ||
		guidanceEvent.Data["client_message_id"] != "client-message-guide-2" ||
		guidanceEvent.DeliveryMode != "durable" {
		t.Fatalf("已消费引导应作为 durable user 归入当前回复: %+v", guidanceEvent)
	}
	if got := activity.snapshot(); len(got) != 2 || got[1] != "test-guide" {
		t.Fatalf("DM guide 物化为用户消息后应消费 draft: %+v", got)
	}
	items, err = service.InputQueue.Snapshot(location)
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
				row["source_round_id"] == "round-guide-2" &&
				row["agent_round_id"] == "agent-round-guide-2" &&
				row["client_message_id"] == "client-message-guide-2"
		}
	}
	if !foundGuidance {
		t.Fatalf("已消费引导应持久化到实际回复 round: %+v", rows)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
}

func TestServiceAckRuntimeGuidanceWaitsForAppliedAckAcrossAssistantAndTerminal(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.hookResponseAck = true
	fallbackStarted := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		if !strings.Contains(prompt, "ACK 后才能消费") {
			return
		}
		fallbackStarted <- prompt
		client.sendResultAsync(client.sessionID, "result-ack-guidance-fallback", "success", "")
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-ack-guidance")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-ack-guidance"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先执行当前任务",
		RoundID:    "round-ack-guidance-1",
	}); err != nil {
		t.Fatalf("启动 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	if err := service.HandleChat(context.Background(), Request{
		SessionKey:                  sessionKey,
		Content:                     "ACK 后才能消费",
		RoundID:                     "round-ack-guidance-2",
		UserMessageID:               "msg-user-ack-guidance-2",
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
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
		"tool-ack",
	)
	if err != nil || output.SpecificOutput == nil || output.OnApplied == nil {
		t.Fatalf("ACK runtime 应返回待确认引导: output=%+v err=%v", output, err)
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:      "assistant-before-guidance-ack",
			Model:   "sonnet",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "当前任务已完成。"}},
		}},
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage &&
			event.Data["message_id"] == "assistant-before-guidance-ack"
	})
	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("assistant 输出不能代替 applied ACK: items=%+v err=%v", items, err)
	}

	unlockDispatch, err := service.dispatchLocks.lock(context.Background(), sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	defer unlockDispatch()
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-before-guidance-ack",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
		},
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-ack-guidance-1" &&
			event.Data["status"] == "finished"
	})
	items, err = service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("result 与终态不能代替 applied ACK: items=%+v err=%v", items, err)
	}
	for _, row := range readDMSessionHistory(t, cfg, service, sessionKey) {
		if row["message_id"] == "msg-user-ack-guidance-2" {
			t.Fatalf("未 applied ACK 的引导不应写入当前 round: %+v", row)
		}
	}
	unlockDispatch()

	select {
	case prompt := <-fallbackStarted:
		if !strings.Contains(prompt, "ACK 后才能消费") {
			t.Fatalf("未确认引导的下一轮缺少原内容: %q", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("未确认引导没有作为下一轮继续")
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-ack-guidance-2" &&
			event.Data["status"] == "finished"
	})
}

func TestServiceGuidancePreflightFailureKeepsQueuedInput(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		client.sendResultAsync(client.sessionID, "result-guide-preflight-cleanup", "interrupted", "")
	}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client}),
		permission,
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-guide-preflight-failure")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide-preflight-failure"
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
		if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey, UserStop: true}); err != nil {
			t.Errorf("清理运行中 round 失败: %v", err)
			return
		}
		// Stopping retains the invalid input without attempting another dispatch.
		_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
		if err != nil {
			t.Error(err)
			return
		}
		paused, err := service.InputQueue.DispatchPaused(location)
		if err != nil || !paused {
			t.Errorf("queue stop gate = %v, %v", paused, err)
		}
		// 等在途派发退出临界区。
		if unlock, lockErr := service.dispatchLocks.lock(context.Background(), sessionKey); lockErr == nil {
			unlock()
		}
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:                  sessionKey,
		Content:                     "补充一个失效附件",
		RoundID:                     "round-guide-preflight-2",
		UserMessageID:               "msg-user-guide-preflight-2",
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
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
	before, err := service.InputQueue.Snapshot(location)
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

	after, err := service.InputQueue.Snapshot(location)
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
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-guide-error-fallback")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide-error-fallback"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先执行一个工具任务",
		RoundID:    "round-guide-error-1",
	}); err != nil {
		t.Fatalf("启动 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	if err := service.HandleChat(context.Background(), Request{
		SessionKey:                  sessionKey,
		Content:                     "失败后作为下一轮继续",
		RoundID:                     "round-guide-error-2",
		UserMessageID:               "msg-user-guide-error-2",
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
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
	items, err := service.InputQueue.Snapshot(location)
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

func TestServiceInputQueueGuideWaitsForPostToolUse(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-guide-input-queue")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide-input-queue"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-input-queue-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if _, err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey:      sessionKey,
		ClientMessageID: "client-message-guide-input-queue",
		Action:          "enqueue",
		Content:         "路径发给我吧",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 DM 待发送队列失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("待发送队列应保留一条消息: %+v", items)
	}
	itemID := items[0].ID

	if _, err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "guide",
		ItemID:     itemID,
	}); err != nil {
		t.Fatalf("标记 DM 引导队列失败: %v", err)
	}
	items, err = service.InputQueue.Snapshot(location)
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
	var onApplied func(sdkhook.AppliedAck)
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
			if output.OnApplied != nil {
				onApplied = output.OnApplied
			}
		}
	}
	if !strings.Contains(additionalContext, "路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_"+itemID) {
		t.Fatalf("PostToolUse hook 未注入队列引导: %q", additionalContext)
	}
	items, err = service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取待确认 DM 引导失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("首次 hook 返回后必须等待后续可观察行为确认: %+v", items)
	}
	if onApplied == nil {
		t.Fatal("DM guide output 缺少 runtime applied ACK callback")
	}
	if _, err = service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "reorder",
		OrderedIDs: []string{itemID},
	}); err == nil {
		t.Fatal("已返回给 runtime 但尚未 applied ACK 的 DM 引导不应允许重排")
	}
	onApplied(sdkhook.AppliedAck{RequestID: "dm-guide-applied-1"})
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
	items, err = service.InputQueue.Snapshot(location)
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

// A claim that survived a crash after its round was persisted must be
// consumed, not dispatched as a second identical round.
func TestSurvivingQueueClaimWithPersistedRoundIsConsumedWithoutReplay(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:queue-claim-replay"
	t.Cleanup(func() { _ = runtimeManager.CloseSession(context.Background(), sessionKey) })
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.InputQueue.Enqueue(location, protocol.InputQueueItem{ID: "claimed", Content: "already sent"}); err != nil {
		t.Fatal(err)
	}
	if err := service.InputQueue.ClaimDispatch(location, "claimed"); err != nil {
		t.Fatal(err)
	}
	if err := service.History.ForOwner(location.OwnerUserID).AppendRoundMarkerWithOptions(
		location.WorkspacePath, sessionKey, inputQueueItemRoundID(protocol.InputQueueItem{ID: "claimed"}), "already sent", 1, workspacestore.RoundMarkerOptions{},
	); err != nil {
		t.Fatal(err)
	}

	service.dispatchNextInputQueueItemAtLocation(context.Background(), sessionKey, "", location)

	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 0 {
		t.Fatalf("surviving claim not consumed: %+v %v", items, err)
	}
	client.mu.Lock()
	queries := len(client.queryPrompts)
	client.mu.Unlock()
	if queries != 0 {
		t.Fatalf("persisted queued round was replayed %d time(s)", queries)
	}
}

type dmInputCursorHarness struct {
	service        *Service
	runtimeManager *runtimectx.Manager
	sender         *dmTestSender
	sessionKey     string
}

// newDMInputCursorHarness 每次调用都装配全新的服务、runtime 与连接，只共用落盘状态，等价于进程重启后的宿主。
func newDMInputCursorHarness(t *testing.T, cfg config.Config, sessionKey string) *dmInputCursorHarness {
	t.Helper()
	client := newFakeDMClient()
	replies := 0
	client.onQuery = func(context.Context, string) {
		replies++
		reply := replies
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
					ID:      fmt.Sprintf("assistant-%d", reply),
					Model:   "sonnet",
					Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "收到"}},
				}},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      fmt.Sprintf("result-%d", reply),
				Result:    &sdkprotocol.ResultMessage{Subtype: "success", NumTurns: 1, Result: "done"},
			}
		}()
	}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetUsageRecorder(&fakeTokenUsageRecorder{})
	sender := newDMTestSender("sender-input-cursor")
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)
	return &dmInputCursorHarness{service: service, runtimeManager: runtimeManager, sender: sender, sessionKey: sessionKey}
}

// accept 受理一条会广播 durable 用户行的输入，返回 ACK 与实时用户行各自携带的身份和游标。
func (h *dmInputCursorHarness) accept(t *testing.T, content string) (userMessageID string, inputSeq int64) {
	t.Helper()
	if err := h.service.HandleChat(context.Background(), Request{
		SessionKey:           h.sessionKey,
		Content:              content,
		ClientMessageID:      "local-" + content,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	events := collectEventsUntil(t, h.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	waitForDMRuntimeIdle(t, h.runtimeManager, h.sessionKey)

	var ackSeq, liveSeq int64
	var liveMessageID string
	for _, event := range events {
		switch {
		case event.EventType == protocol.EventTypeChatAck:
			userMessageID = anyToString(event.Data["user_message_id"])
			ackSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
		case event.EventType == protocol.EventTypeMessage && event.Data["role"] == "user":
			liveMessageID = anyToString(event.Data["message_id"])
			liveSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
		}
	}
	if ackSeq <= 0 || liveSeq != ackSeq || liveMessageID != userMessageID {
		t.Fatalf("ACK 与实时用户行必须给出同一身份与游标: ack=(%s,%d) live=(%s,%d)", userMessageID, ackSeq, liveMessageID, liveSeq)
	}
	return userMessageID, ackSeq
}

// historyInputSeqs 用全新的历史 store 读取落盘历史，返回每条用户行的游标。
func (h *dmInputCursorHarness) historyInputSeqs(t *testing.T, cfg config.Config) map[string]int64 {
	t.Helper()
	cursors := map[string]int64{}
	for _, row := range readDMSessionHistory(t, cfg, h.service, h.sessionKey) {
		if row["role"] == "user" {
			cursors[anyToString(row["message_id"])] = protocol.Int64FromAny(row[protocol.InputSeqField])
		}
	}
	return cursors
}

func TestDMUserRowSharesInputSeqAcrossAckLiveRowAndHistory(t *testing.T) {
	cfg := newDMTestConfig(t)
	sessionKey := "agent:nexus:ws:dm:input-cursor"
	live := map[string]int64{}
	assertHistory := func(stage string, harness *dmInputCursorHarness) {
		t.Helper()
		got := harness.historyInputSeqs(t, cfg)
		if len(got) != len(live) {
			t.Fatalf("%s: 历史用户行 = %v，实时受理 = %v", stage, got, live)
		}
		for messageID, inputSeq := range live {
			if got[messageID] != inputSeq {
				t.Fatalf("%s: %s 的历史游标 = %d，受理时是 %d", stage, messageID, got[messageID], inputSeq)
			}
		}
	}

	first := newDMInputCursorHarness(t, cfg, sessionKey)
	firstID, firstSeq := first.accept(t, "第一条")
	live[firstID] = firstSeq
	assertHistory("受理后读取", first)

	secondID, secondSeq := first.accept(t, "第二条")
	if secondSeq <= firstSeq {
		t.Fatalf("后受理的输入游标必须更大: first=%d second=%d", firstSeq, secondSeq)
	}
	live[secondID] = secondSeq
	assertHistory("追加一轮后刷新", first)

	restarted := newDMInputCursorHarness(t, cfg, sessionKey)
	assertHistory("换新服务实例后读取", restarted)
	thirdID, thirdSeq := restarted.accept(t, "第三条")
	// 这里只换服务实例，分配器仍是同一进程的；真正跨进程的先后取决于宿主时钟（见 protocol.NewInputSeq）。
	if thirdSeq <= secondSeq {
		t.Fatalf("换新服务实例后受理的输入游标必须大于已落盘的游标: second=%d third=%d", secondSeq, thirdSeq)
	}
	live[thirdID] = thirdSeq
	assertHistory("重启后再受理一轮", restarted)
}

func TestDirectUserDMQueueKeepsConfigurationContextAfterAdmissionClaim(t *testing.T) {
	service, client, sources, bindings, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	ctx := trustedDMQueueContext()

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "keep the first round running",
		RoundID:                     "round-provenance-first",
		UserMessageID:               "message-provenance-first",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuePrompt(t, prompts, "keep the first round running")

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "update my safe settings",
		RoundID:                     "round-provenance-queued",
		UserMessageID:               "message-provenance-queued",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	finishDMQueueRound(client, "result-provenance-first")
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuedHumanBinding(t, bindings)
	expectDMQueuePrompt(t, prompts, "update my safe settings")

	status := waitForDMQueueAdmissionStatus(t, db, "round-provenance-queued", queueadmissionstore.StatusConsumed)
	if status != queueadmissionstore.StatusConsumed {
		t.Fatalf("queue admission status = %q, want consumed", status)
	}
	finishDMQueueRound(client, "result-provenance-queued")
}

func TestTamperedDMQueuePayloadLosesConfigurationContextAndRevokesAdmission(t *testing.T) {
	service, client, sources, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	ctx := trustedDMQueueContext()

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "keep the first round running",
		RoundID:                     "round-tamper-first",
		UserMessageID:               "message-tamper-first",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuePrompt(t, prompts, "keep the first round running")

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "original queued request",
		RoundID:                     "round-tamper-queued",
		UserMessageID:               "message-tamper-queued",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.Config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("queued items = %+v, err=%v", items, err)
	}
	tampered := items[0]
	tampered.Content = "tampered destructive request"
	if _, err = service.InputQueue.Enqueue(location, tampered); err != nil {
		t.Fatal(err)
	}

	finishDMQueueRound(client, "result-tamper-first")
	expectDMQueueSource(t, sources, "agent_queue")
	expectDMQueuePrompt(t, prompts, "tampered destructive request")

	status := waitForDMQueueAdmissionStatus(t, db, "round-tamper-queued", queueadmissionstore.StatusRevoked)
	if status != queueadmissionstore.StatusRevoked {
		t.Fatalf("tampered queue admission status = %q, want revoked", status)
	}
	finishDMQueueRound(client, "result-tamper-queued")
}

type rejectingDMQueueAdmissionStore struct {
	err error
}

func (s rejectingDMQueueAdmissionStore) Record(context.Context, queueadmissionstore.Admission) error {
	return s.err
}

func (rejectingDMQueueAdmissionStore) Claim(context.Context, queueadmissionstore.Binding) (queueadmissionstore.Claim, bool, error) {
	return queueadmissionstore.Claim{}, false, nil
}

func (rejectingDMQueueAdmissionStore) Release(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Consume(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Revoke(context.Context, queueadmissionstore.Binding) error {
	return nil
}

func TestDMQueueAdmissionFailureKeepsAndRecoversDurableUserInput(t *testing.T) {
	service, client, _, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	admissionErr := errors.New("queue admission unavailable")
	service.SetQueueAdmissionStore(rejectingDMQueueAdmissionStore{err: admissionErr})
	request := InputQueueRequest{
		SessionKey: sessionKey, ClientMessageID: "client-dm-queue-retained",
		Action: "enqueue", Content: "keep this DM input for retry",
		TrustedConfigurationContext: true,
	}
	if _, err := service.HandleInputQueue(trustedDMQueueContext(), request); !errors.Is(err, admissionErr) {
		t.Fatalf("HandleInputQueue error = %v, want %v", err, admissionErr)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.Config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 ||
		items[0].ClientMessageID != request.ClientMessageID ||
		items[0].Content != request.Content {
		t.Fatalf("durable DM input was lost after admission failure: %+v", items)
	}

	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(service.Config, db))
	retry, err := service.HandleInputQueue(trustedDMQueueContext(), request)
	if err != nil {
		t.Fatalf("retry retained DM input: %v", err)
	}
	if !retry.Duplicate || retry.ItemID != items[0].ID {
		t.Fatalf("retry result = %+v, want original item %q", retry, items[0].ID)
	}
	expectDMQueuePrompt(t, prompts, request.Content)
	finishDMQueueRound(client, "result-retained-dm-queue")
}

func trustedDMQueueContext() context.Context {
	return authctx.WithState(context.Background(), authctx.State{AuthRequired: false})
}

func newDMQueueProvenanceFixture(
	t *testing.T,
) (*Service, *fakeDMClient, <-chan string, <-chan authctx.QueuedHumanPrincipalBinding, <-chan string, *sql.DB, string) {
	t.Helper()
	cfg := newDMTestConfig(t)
	db := dmTestDB(t, cfg)

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	prompts := make(chan string, 4)
	client.onQuery = func(_ context.Context, prompt string) {
		prompts <- prompt
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	sessionKey := protocol.BuildAgentSessionKey(
		newPlainDMAgent(t, agentService).AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"queue-provenance",
		"",
	)
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runtimeManager.CloseSession(closeCtx, sessionKey)
	})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(cfg, db))
	sources := make(chan string, 4)
	bindings := make(chan authctx.QueuedHumanPrincipalBinding, 2)
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		sourceContextType string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		sources <- sourceContextType
		if binding, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok {
			bindings <- binding
		}
		return nil
	})
	return service, client, sources, bindings, prompts, db, sessionKey
}

func expectDMQueuedHumanBinding(
	t *testing.T,
	bindings <-chan authctx.QueuedHumanPrincipalBinding,
) {
	t.Helper()
	select {
	case binding := <-bindings:
		if binding.UserID != authctx.SystemUserID ||
			binding.AuthMethod != authctx.AuthMethodLocal ||
			binding.SessionID != "" {
			t.Fatalf("queued DM human binding = %+v", binding)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for queued DM human binding")
	}
}

func expectDMQueueSource(t *testing.T, sources <-chan string, want string) {
	t.Helper()
	select {
	case got := <-sources:
		if got != want {
			t.Fatalf("configuration source = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for configuration source %q", want)
	}
}

func expectDMQueuePrompt(t *testing.T, prompts <-chan string, content string) {
	t.Helper()
	select {
	case prompt := <-prompts:
		if !strings.Contains(prompt, content) {
			t.Fatalf("runtime prompt does not contain %q: %s", content, prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for prompt containing %q", content)
	}
}

func finishDMQueueRound(client *fakeDMClient, resultID string) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      resultID,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success", DurationMS: 1, NumTurns: 1},
	}
}

func waitForDMQueueAdmissionStatus(
	t *testing.T,
	db *sql.DB,
	itemID string,
	want string,
) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		err := db.QueryRow(
			`SELECT status FROM configuration_queue_admissions
WHERE queue_item_id = ? AND scope = 'dm'`,
			itemID,
		).Scan(&status)
		if err == nil && status == want {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	return status
}

func TestDispatchLocksSerializeOneSessionOnly(t *testing.T) {
	var locks dispatchLocks
	ctx := context.Background()
	unlockA, err := locks.lock(ctx, "session-a")
	if err != nil {
		t.Fatal(err)
	}
	otherDone := make(chan struct{})
	go func() {
		unlock, lockErr := locks.lock(ctx, "session-b")
		if lockErr == nil {
			unlock()
		}
		close(otherDone)
	}()
	select {
	case <-otherDone:
	case <-time.After(time.Second):
		t.Fatal("其他会话被 session-a 的锁挡住")
	}
	sameDone := make(chan struct{})
	go func() {
		unlock, lockErr := locks.lock(ctx, " session-a ")
		if lockErr == nil {
			unlock()
		}
		close(sameDone)
	}()
	select {
	case <-sameDone:
		t.Fatal("同一会话（含首尾空白的同一 key）没有串行")
	case <-time.After(50 * time.Millisecond):
	}
	unlockA()
	unlockA()
	select {
	case <-sameDone:
	case <-time.After(time.Second):
		t.Fatal("释放后等待者没有拿到锁")
	}
	if n := locks.size(); n != 0 {
		t.Fatalf("释放后锁表残留 %d 个会话", n)
	}
}

func TestDispatchLocksWaitHonoursContext(t *testing.T) {
	var locks dispatchLocks
	unlock, err := locks.lock(context.Background(), "session-a")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = locks.lock(ctx, "session-a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("等待应随 ctx 超时放弃: %v", err)
	}
	unlock()
	if n := locks.size(); n != 0 {
		t.Fatalf("放弃等待与释放后锁表残留 %d 个会话", n)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err = locks.lock(cancelled, "session-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的 ctx 不应取锁: %v", err)
	}
}

func TestMainWindowStartsWhileAnotherSessionHoldsDispatchLock(t *testing.T) {
	f := newMainInboxFixture(t, "dispatch-lock-independent")
	unlock, err := f.service.dispatchLocks.lock(context.Background(), "agent:nexus:ws:dm:some-other-session")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	f.send(t, "不受其他会话影响", "client-1")
	f.waitPrompts(t, 1)
	f.finishRound("dispatch-lock-independent")
}

// size 返回仍在使用中的会话数，供测试确认释放后不残留。
func (l *dispatchLocks) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func assertNoInternalInterruptSentinel(t *testing.T, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("编码中断投影失败: %v", err)
	}
	if strings.Contains(string(payload), messagepkg.InterruptWithoutMessage) {
		t.Fatalf("内部中断哨兵泄漏到用户可见投影: %s", payload)
	}
}

func TestServiceHandleInterruptCleansStaleRuntimeWhenClientInterruptFails(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.interruptErrors = []error{errors.New("os: process already finished")}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-interrupt-stale")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-stale"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "停止一个已经退出的进程",
		RoundID:    "round-interrupt-stale",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("失效进程中断应被业务层清理而不是返回错误: %v", err)
	}
	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeSessionStatus && event.Data["is_generating"] == false
	})
	if len(runtimeManager.GetRunningRoundIDs(sessionKey)) != 0 {
		t.Fatal("失效进程清理后不应残留 running round")
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if sessionValue.Status != "closed" || sessionValue.IsActive {
		t.Fatalf("失效进程清理后 session meta 应关闭: %+v events=%+v", sessionValue, events)
	}
}

func TestServiceHandleChatInterruptPolicyStopsRunningRound(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		if strings.Contains(prompt, "第二轮") {
			client.sendResultAsync(client.sessionID, "result-interrupt-policy", "success", "ok")
		}
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-interrupt-policy-old",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
					Result:     messagepkg.InterruptWithoutMessage,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-interrupt-policy")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-policy"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-policy-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:     sessionKey,
		Content:        "第二轮",
		RoundID:        "round-interrupt-policy-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyInterrupt,
	}); err != nil {
		t.Fatalf("打断策略 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-interrupt-policy-2" &&
			event.Data["status"] == "finished"
	})
	assertContainsRoundStatus(t, events, "finished")
	assertNoInternalInterruptSentinel(t, events)

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	interruptReasons := append([]string(nil), client.interruptReasons...)
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("打断策略应中断运行中 DM round")
	}
	if len(interruptReasons) == 0 || interruptReasons[0] != "interrupt" {
		t.Fatalf("打断策略应传递 submit-interrupt reason: %+v", interruptReasons)
	}
	if len(sentContents) != 0 {
		t.Fatalf("打断策略不应走 streaming input: %+v", sentContents)
	}
	_, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	overlayPath := workspacestore.New(cfg.WorkspacePath).SessionOverlayPath(workspacePath, sessionKey)
	overlayPayload, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("读取打断策略 overlay 失败: %v", err)
	}
	assertNoInternalInterruptSentinel(t, string(overlayPayload))
	if !strings.Contains(string(overlayPayload), "收到新的用户消息，上一轮已停止") {
		t.Fatalf("原生 interrupted result 未保留自定义打断文案: %s", overlayPayload)
	}
}

func TestServiceHandleChatAfterInterruptKeepsSameClientAndConsumesExplicitStop(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()

	client := newFakeDMClient()
	client.sessionID = "sdk-interrupt-old"
	queryCount := 0
	oldTerminalSent := make(chan struct{})
	client.onQuery = func(_ context.Context, _ string) {
		queryCount++
		if queryCount != 2 {
			return
		}
		go func() {
			<-oldTerminalSent
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			// 晚于 Manager 的强制取消宽限，复现真实 nxs 中断尾包落入下一轮。
			time.Sleep(250 * time.Millisecond)
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "error_during_execution",
					DurationMS: 25707,
					NumTurns:   1,
					IsError:    true,
					Errors:     []string{"stale interrupted request"},
				},
			}
			close(oldTerminalSent)
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-reconnect")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-reconnect"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第二轮",
		RoundID:    "round-interrupt-2",
	}); err != nil {
		t.Fatalf("第二轮 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == "finished" &&
			event.Data["round_id"] == "round-interrupt-2"
	})

	if len(factory.options) != 1 {
		t.Fatalf("只应创建一次 runtime client，第二轮应复用现有 client: got=%d want=1", len(factory.options))
	}
	if len(client.reconfigureOps) == 0 {
		t.Fatalf("第二轮应复用 client 并执行 reconfigure")
	}
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["round_id"] != "round-interrupt-2" {
			continue
		}
		summary, ok := event.Data["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "interrupted" {
			t.Fatalf("第二轮不应消费上一轮残留结果: %+v", events)
		}
	}
}

func TestServiceHandleInterruptExactOldRoundKeepsSuccessorRunning(t *testing.T) {
	cfg := newDMTestConfig(t)

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:test-exact-old-round"
	oldCancelled := false
	successorCancelled := false

	if err := runtimeManager.StartRound(context.Background(), sessionKey, "round-old", func() {
		oldCancelled = true
		runtimeManager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("注册旧 round 失败: %v", err)
	}
	if err := runtimeManager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		successorCancelled = true
		runtimeManager.MarkRoundFinished(sessionKey, "round-successor")
	}); err != nil {
		t.Fatalf("注册 successor round 失败: %v", err)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{
		SessionKey: sessionKey,
		RoundID:    "round-old",
	}); err != nil {
		t.Fatalf("精确中断旧 round 失败: %v", err)
	}
	if !oldCancelled {
		t.Fatal("旧 round 应被精确取消")
	}
	if successorCancelled {
		t.Fatal("旧 round 的迟到停止不得取消 successor")
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); len(running) != 1 || running[0] != "round-successor" {
		t.Fatalf("successor 运行态被破坏: %+v", running)
	}
	runtimeManager.MarkRoundFinished(sessionKey, "round-successor")
}

func TestServiceHandleInterruptStaleRoundNeverFallsBackToSession(t *testing.T) {
	cfg := newDMTestConfig(t)

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:test-stale-round-stop"
	successorCancelled := false
	if err := runtimeManager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		successorCancelled = true
		runtimeManager.MarkRoundFinished(sessionKey, "round-successor")
	}); err != nil {
		t.Fatalf("注册 successor round 失败: %v", err)
	}

	err := service.HandleInterrupt(context.Background(), InterruptRequest{
		SessionKey: sessionKey,
		RoundID:    "round-ended",
	})
	if !errors.Is(err, ErrTargetDMRoundNotRunning) {
		t.Fatalf("迟到 round 停止错误 = %v, want ErrTargetDMRoundNotRunning", err)
	}
	if successorCancelled {
		t.Fatal("找不到 exact target 时不得退化为 session 中断")
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); len(running) != 1 || running[0] != "round-successor" {
		t.Fatalf("迟到停止改变了 successor 运行态: %+v", running)
	}
	runtimeManager.MarkRoundFinished(sessionKey, "round-successor")
}

func TestServiceHandleInterruptExactUnsupportedFailsClosed(t *testing.T) {
	cfg := newDMTestConfig(t)

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:test-exact-unsupported"
	if err := runtimeManager.StartRound(context.Background(), sessionKey, "round-without-cancel", nil); err != nil {
		t.Fatalf("注册无 exact cancel 的 round 失败: %v", err)
	}
	if err := runtimeManager.StartRound(context.Background(), sessionKey, "round-successor", func() {}); err != nil {
		t.Fatalf("注册 successor round 失败: %v", err)
	}

	err := service.HandleInterrupt(context.Background(), InterruptRequest{
		SessionKey: sessionKey,
		RoundID:    "round-without-cancel",
	})
	if !errors.Is(err, ErrExactDMRoundInterruptUnsupported) {
		t.Fatalf("不支持 exact stop 时错误 = %v", err)
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); len(running) != 2 {
		t.Fatalf("不支持 exact stop 时必须 fail closed: %+v", running)
	}
	runtimeManager.MarkRoundFinished(sessionKey, "round-without-cancel")
	runtimeManager.MarkRoundFinished(sessionKey, "round-successor")
}
