package permission

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type permissionTestSender struct {
	key    string
	closed bool
	events chan protocol.EventMessage
}

func newPermissionTestSender(key string) *permissionTestSender {
	return &permissionTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 16),
	}
}

func (s *permissionTestSender) Key() string {
	return s.key
}

func (s *permissionTestSender) IsClosed() bool {
	return s.closed
}

func (s *permissionTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestContextRequestPermissionAndReplay(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-permission"

	controllerA := newPermissionTestSender("sender-a")
	controllerB := newPermissionTestSender("sender-b")

	ctx.BindSession(sessionKey, controllerA, "client-a", true)

	resultCh := make(chan sdkprotocol.PermissionDecision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkprotocol.PermissionRequest{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "go.mod",
			},
		})
		resultCh <- decision
	}()

	firstEvent := readPermissionEventByType(t, controllerA.events, protocol.EventTypePermissionRequest)
	if firstEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", firstEvent)
	}
	if firstEvent.Data["tool_name"] != "Read" {
		t.Fatalf("tool_name 不正确: %+v", firstEvent.Data)
	}

	ctx.UnbindSession(sessionKey, controllerA)
	ctx.BindSession(sessionKey, controllerB, "client-b", true)

	replayed := readPermissionEventByType(t, controllerB.events, protocol.EventTypePermissionRequest)
	if replayed.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望重放 permission_request，实际: %+v", replayed)
	}

	requestID, _ := replayed.Data["request_id"].(string)
	if requestID == "" {
		t.Fatalf("request_id 为空: %+v", replayed.Data)
	}
	if !ctx.HandlePermissionResponse(map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("处理 permission_response 失败")
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkprotocol.PermissionBehaviorAllow {
			t.Fatalf("期望 allow，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待权限结果超时")
	}

	resolved := readPermissionEventByType(t, controllerB.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["request_id"] != requestID {
		t.Fatalf("resolved request_id 不正确: %+v", resolved.Data)
	}
	if resolved.Data["status"] != "answered" {
		t.Fatalf("resolved status 不正确: %+v", resolved.Data)
	}
}

func TestContextRequestPermissionTimeoutBroadcastsResolved(t *testing.T) {
	ctx := NewContext()
	ctx.requestTimeout = 20 * time.Millisecond
	sessionKey := "agent:nexus:ws:dm:test-timeout"
	controller := newPermissionTestSender("sender-timeout")
	ctx.BindSession(sessionKey, controller, "client-timeout", true)

	resultCh := make(chan sdkprotocol.PermissionDecision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkprotocol.PermissionRequest{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "README.md",
			},
		})
		resultCh <- decision
	}()

	requestEvent := readPermissionEventByType(t, controller.events, protocol.EventTypePermissionRequest)
	if requestEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", requestEvent)
	}
	resolved := readPermissionEventByType(t, controller.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["status"] != "expired" {
		t.Fatalf("timeout resolved status 不正确: %+v", resolved.Data)
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkprotocol.PermissionBehaviorDeny {
			t.Fatalf("期望 deny，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待超时结果失败")
	}
}

func TestContextCancelRequestsForSessionBroadcastsResolved(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-cancel"
	controller := newPermissionTestSender("sender-cancel")
	ctx.BindSession(sessionKey, controller, "client-cancel", true)

	resultCh := make(chan sdkprotocol.PermissionDecision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkprotocol.PermissionRequest{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "go.mod",
			},
		})
		resultCh <- decision
	}()

	requestEvent := readPermissionEventByType(t, controller.events, protocol.EventTypePermissionRequest)
	if requestEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", requestEvent)
	}

	if cancelled := ctx.CancelRequestsForSession(sessionKey, "session cancelled"); cancelled != 1 {
		t.Fatalf("期望取消 1 个请求，实际: %d", cancelled)
	}

	resolved := readPermissionEventByType(t, controller.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["status"] != "cancelled" {
		t.Fatalf("cancel resolved status 不正确: %+v", resolved.Data)
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkprotocol.PermissionBehaviorDeny {
			t.Fatalf("期望 deny，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待取消结果失败")
	}
}

func readPermissionEvent(t *testing.T, events <-chan protocol.EventMessage) protocol.EventMessage {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("等待权限事件超时")
		return protocol.EventMessage{}
	}
}

func readPermissionEventByType(
	t *testing.T,
	events <-chan protocol.EventMessage,
	eventType protocol.EventType,
) protocol.EventMessage {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if event.EventType == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("等待权限事件 %s 超时", eventType)
			return protocol.EventMessage{}
		}
	}
}
