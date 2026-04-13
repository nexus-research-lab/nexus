// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：request_test.go
// @Date   ：2026/04/11 02:52:00
// @Author ：leemysw
// 2026/04/11 02:52:00   Create
// =====================================================

package permission

import (
	"context"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"testing"
	"time"

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

	firstEvent := readPermissionEvent(t, controllerA.events)
	if firstEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", firstEvent)
	}
	if firstEvent.Data["tool_name"] != "Read" {
		t.Fatalf("tool_name 不正确: %+v", firstEvent.Data)
	}

	ctx.UnbindSession(sessionKey, controllerA)
	ctx.BindSession(sessionKey, controllerB, "client-b", true)

	replayed := readPermissionEvent(t, controllerB.events)
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
