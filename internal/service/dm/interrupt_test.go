package dm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestServiceHandleInterruptEmitsInterruptedRound(t *testing.T) {
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
				UUID:      "result-interrupted",
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
	sender := newDMTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-interrupt"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")
	assertNotContainsResultSubtype(t, events, "interrupted")
	assertNoInternalInterruptSentinel(t, events)

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("期望 fake client 收到 interrupt")
	}

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	if messages[1]["role"] != "assistant" {
		t.Fatalf("中断后应返回合成 assistant: %+v", messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok || summary["subtype"] != "interrupted" {
		t.Fatalf("中断后未挂载 interrupted result_summary: %+v", messages)
	}
	assertNoInternalInterruptSentinel(t, messages)
	overlayPath := workspacestore.New(cfg.WorkspacePath).SessionOverlayPath(workspacePath, sessionKey)
	overlayPayload, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("读取中断 overlay 失败: %v", err)
	}
	assertNoInternalInterruptSentinel(t, string(overlayPayload))
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
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.interruptErrors = []error{errors.New("os: process already finished")}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
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
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		if strings.Contains(prompt, "第二轮") {
			go func() {
				client.messages <- sdkprotocol.ReceivedMessage{
					Type:      sdkprotocol.MessageTypeResult,
					SessionID: client.sessionID,
					UUID:      "result-interrupt-policy",
					Result: &sdkprotocol.ResultMessage{
						Subtype:    "success",
						DurationMS: 1,
						NumTurns:   1,
						Result:     "ok",
					},
				}
			}()
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
	sender := newDMTestSender("sender-interrupt-policy")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-policy"
	permission.BindSession(sessionKey, sender)

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

func TestServiceHandleInterruptCoercesTerminalErrorIntoInterrupted(t *testing.T) {
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
				UUID:      "result-error-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "error",
					DurationMS:    8,
					DurationAPIMS: 123,
					NumTurns:      2,
					Result:        "",
					IsError:       true,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-error")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-error"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "停止测试",
		RoundID:    "round-interrupt-error",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")
	assertNotContainsResultSubtype(t, events, "interrupted")

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-error-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "停止测试",
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断错误收口后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("中断错误未挂载 result_summary: %+v", messages)
	}
	if summary["subtype"] != "interrupted" {
		t.Fatalf("中断错误应收口为 interrupted: %+v", summary)
	}
	if _, exists := summary["result"]; exists {
		t.Fatalf("中断错误不应再补默认文案: %+v", summary)
	}
}

func TestServiceHandleChatAfterInterruptKeepsSameClientAndConsumesExplicitStop(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

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
	sender := newDMTestSender("sender-reconnect")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-reconnect"
	permission.BindSession(sessionKey, sender)

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
