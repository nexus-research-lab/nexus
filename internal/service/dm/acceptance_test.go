package dm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type blockingDMAcceptanceActivity struct {
	started chan context.Context
	release chan struct{}
}

func (a *blockingDMAcceptanceActivity) MarkConversationStarted(
	ctx context.Context,
	_ string,
	_ time.Time,
) error {
	a.started <- ctx
	<-a.release
	return ctx.Err()
}

func TestDetachedAcceptancePreservesIdentityAndSurvivesCallerCancellation(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	connectStarted := make(chan context.Context, 1)
	releaseConnect := make(chan struct{})
	client := newFakeDMClient()
	client.onConnect = func(ctx context.Context) {
		connectStarted <- ctx
		<-releaseConnect
	}
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-detached-acceptance",
				Result: &sdkprotocol.ResultMessage{
					Subtype: "success",
					Result:  "done",
				},
			}
		}()
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	sessionKey := "agent:nexus:ws:dm:detached-acceptance"
	sender := newDMTestSender("sender-detached-acceptance")
	permission.BindSession(sessionKey, sender)

	requestBase := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: authctx.SystemUserID, Role: authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
	requestBase = authctx.WithInteractiveHumanEvidence(requestBase, "desktop_session_token")
	requestCtx, cancelRequest := context.WithCancel(requestBase)
	if err := service.HandleRealtimeChat(requestCtx, Request{
		SessionKey:      sessionKey,
		Content:         "连接断开后继续执行",
		ClientRequestID: "request-detached-1",
		ClientMessageID: "message-detached-1",
		RoundID:         "round-detached-1",
	}); err != nil {
		t.Fatalf("HandleRealtimeChat() error = %v", err)
	}

	ack := <-sender.events
	if ack.EventType != protocol.EventTypeChatAck ||
		ack.Data["client_request_id"] != "request-detached-1" ||
		ack.Data["round_id"] != "round-detached-1" {
		t.Fatalf("runtime 启动前未返回 canonical ACK: %+v", ack)
	}

	var runtimeCtx context.Context
	select {
	case runtimeCtx = <-connectStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("runtime 未开始连接")
	}
	principal := authctx.PrincipalFromContext(runtimeCtx)
	if principal == nil || principal.AuthMethod != authctx.AuthMethodLocal {
		t.Fatalf("detached runtime principal = %+v", principal)
	}
	evidence, ok := authctx.InteractiveHumanEvidenceFromContext(runtimeCtx)
	if !ok || evidence.Source != "desktop_session_token" {
		t.Fatalf("detached runtime human evidence = %+v, ok = %t", evidence, ok)
	}
	cancelRequest()
	select {
	case <-runtimeCtx.Done():
		t.Fatalf("请求连接取消不应传播到已受理 round: %v", runtimeCtx.Err())
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseConnect)

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == protocol.RoundStatusFinished
	})
	assertContainsRoundStatus(t, events, protocol.RoundStatusRunning)
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)

	history := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(history) == 0 || history[0]["client_message_id"] != "message-detached-1" {
		t.Fatalf("durable 用户消息未保留客户端受理身份: %+v", history)
	}
}

func TestDetachedAcceptanceUsesRoundContextAfterDurableMarker(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-detached-post-marker",
				Result:    &sdkprotocol.ResultMessage{Subtype: "success"},
			}
		}()
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	activity := &blockingDMAcceptanceActivity{
		started: make(chan context.Context, 1),
		release: make(chan struct{}),
	}
	service.SetRoomConversationActivityStore(activity)
	sessionKey := "agent:nexus:ws:dm:detached-post-marker"
	sender := newDMTestSender("sender-detached-post-marker")
	permission.BindSession(sessionKey, sender)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	handleDone := make(chan error, 1)
	go func() {
		handleDone <- service.HandleRealtimeChat(requestCtx, Request{
			SessionKey:      sessionKey,
			Content:         "持久化后连接断开仍应继续",
			ClientRequestID: "request-detached-post-marker",
			ClientMessageID: "message-detached-post-marker",
		})
	}()
	var activityCtx context.Context
	select {
	case activityCtx = <-activity.started:
	case <-time.After(3 * time.Second):
		t.Fatal("durable marker 后未进入派生状态写入")
	}
	cancelRequest()
	select {
	case <-activityCtx.Done():
		t.Fatalf("durable marker 后的派生写入不应继承 WebSocket 取消: %v", activityCtx.Err())
	case <-time.After(50 * time.Millisecond):
	}
	close(activity.release)
	select {
	case err := <-handleDone:
		if err != nil {
			t.Fatalf("durable marker 后连接取消不应让受理失败: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("durable marker 后 HandleRealtimeChat 未返回")
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == protocol.RoundStatusFinished
	})
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestDetachedAcceptanceProjectsRuntimeStartupFailure(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	startupErr := errors.New("runtime startup failed after acceptance")
	client := newFakeDMClient()
	client.connectErrors = []error{startupErr}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	sessionKey := "agent:nexus:ws:dm:detached-startup-failure"
	sender := newDMTestSender("sender-detached-startup-failure")
	permission.BindSession(sessionKey, sender)

	if err := service.HandleRealtimeChat(context.Background(), Request{
		SessionKey:      sessionKey,
		Content:         "先受理再启动",
		ClientRequestID: "request-startup-failure",
		ClientMessageID: "message-startup-failure",
		RoundID:         "round-startup-failure",
	}); err != nil {
		t.Fatalf("已持久受理的请求不应同步返回 runtime 错误: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == protocol.RoundStatusError
	})
	if len(events) < 2 || events[0].EventType != protocol.EventTypeChatAck {
		t.Fatalf("ACK 必须先于 runtime 失败投影: %+v", events)
	}
	assertContainsResultSubtype(t, events, "error")
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)

	history := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(history) < 2 || history[0]["client_message_id"] != "message-startup-failure" {
		t.Fatalf("启动失败后 durable 历史不完整: %+v", history)
	}
}

func TestDetachedAcceptanceRejectsOversizedClientMessageID(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	service := NewService(
		cfg,
		newDMAgentService(t, cfg),
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()}),
		permissionctx.NewContext(),
	)
	err := service.HandleRealtimeChat(context.Background(), Request{
		SessionKey:      "agent:nexus:ws:dm:oversized-client-message-id",
		Content:         "限制客户端消息 ID 长度",
		ClientMessageID: strings.Repeat("x", protocol.MaxClientMessageIDBytes+1),
	})
	if err == nil || !strings.Contains(err.Error(), "client_message_id 过长") {
		t.Fatalf("超长 client_message_id 应在持久化前拒绝: %v", err)
	}
}
