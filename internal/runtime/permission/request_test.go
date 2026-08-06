package permission

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type permissionTestSender struct {
	key    string
	closed bool
	events chan protocol.EventMessage
}

type permissionTestRoomBroadcaster struct {
	roomIDs chan string
	events  chan protocol.EventMessage
}

func newPermissionTestRoomBroadcaster() *permissionTestRoomBroadcaster {
	return &permissionTestRoomBroadcaster{
		roomIDs: make(chan string, 8),
		events:  make(chan protocol.EventMessage, 8),
	}
}

func (b *permissionTestRoomBroadcaster) Broadcast(
	_ context.Context,
	roomID string,
	event protocol.EventMessage,
) []error {
	b.roomIDs <- roomID
	b.events <- event
	return nil
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

	senderA := newPermissionTestSender("sender-a")
	senderB := newPermissionTestSender("sender-b")

	ctx.BindSession(sessionKey, senderA)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "go.mod",
			},
		})
		resultCh <- decision
	}()

	firstEvent := readPermissionEventByType(t, senderA.events, protocol.EventTypePermissionRequest)
	if firstEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", firstEvent)
	}
	if firstEvent.Data["tool_name"] != "Read" {
		t.Fatalf("tool_name 不正确: %+v", firstEvent.Data)
	}
	if _, ok := firstEvent.Data["expires_at"]; ok {
		t.Fatalf("不限时请求不应下发 expires_at: %+v", firstEvent.Data)
	}
	firstRequestID, _ := firstEvent.Data["request_id"].(string)
	if firstRequestID == "" {
		t.Fatalf("request_id 为空: %+v", firstEvent.Data)
	}

	ctx.UnbindSession(sessionKey, senderA)
	select {
	case decision := <-resultCh:
		t.Fatalf("断线等待期间不应自动结束: %+v", decision)
	case <-time.After(20 * time.Millisecond):
	}
	ctx.BindSession(sessionKey, senderB)

	replayed := readPermissionEventByType(t, senderB.events, protocol.EventTypePermissionRequest)
	if replayed.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望重放 permission_request，实际: %+v", replayed)
	}
	requestID, _ := replayed.Data["request_id"].(string)
	if requestID != firstRequestID {
		t.Fatalf("重连必须重放同一 pending 请求: got %q, want %q", requestID, firstRequestID)
	}
	if _, ok := replayed.Data["expires_at"]; ok {
		t.Fatalf("重放的不限时请求不应下发 expires_at: %+v", replayed.Data)
	}
	if !ctx.HandlePermissionResponse(map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("处理 permission_response 失败")
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("期望 allow，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待权限结果超时")
	}

	resolved := readPermissionEventByType(t, senderB.events, protocol.EventTypePermissionRequestResolved)
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

func TestContextReplayPendingRequestsUsesStableCreationAndRequestOrder(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-replay-order"
	createdAt := time.Now()
	pendingRequests := []*PendingRequest{
		{
			RequestID:          "permission-later",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/later"},
			CreatedAt:          createdAt.Add(time.Second),
		},
		{
			RequestID:          "permission-b",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/b"},
			CreatedAt:          createdAt,
		},
		{
			RequestID:          "permission-a",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/a"},
			CreatedAt:          createdAt,
		},
	}

	ctx.mu.Lock()
	for _, pending := range pendingRequests {
		ctx.pendingRequests[pending.RequestID] = pending
	}
	ctx.pendingRequests["permission-other-session"] = &PendingRequest{
		RequestID:          "permission-other-session",
		SessionKey:         "agent:nexus:ws:dm:other",
		DispatchSessionKey: "agent:nexus:ws:dm:other",
		ToolName:           "Read",
		ToolInput:          map[string]any{"file_path": "/tmp/other"},
		CreatedAt:          createdAt.Add(-time.Second),
	}
	ctx.mu.Unlock()

	sender := newPermissionTestSender("sender-replay-order")
	ctx.BindSession(sessionKey, sender)

	got := make([]string, 0, len(pendingRequests))
	for range pendingRequests {
		event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
		requestID, _ := event.Data["request_id"].(string)
		got = append(got, requestID)
	}
	want := []string{"permission-a", "permission-b", "permission-later"}
	if !slices.Equal(got, want) {
		t.Fatalf("pending 重放顺序不稳定: got %v, want %v", got, want)
	}
}

func TestContextRequestPermissionWaitsUntilContextCancelled(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-context-cancel"
	sender := newPermissionTestSender("sender-context-cancel")
	ctx.BindSession(sessionKey, sender)

	requestCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(requestCtx, sessionKey, sdkpermission.Request{
			ToolName: "AskUserQuestion",
			Input: map[string]any{
				"questions": []any{},
			},
		})
		resultCh <- decision
	}()

	requestEvent := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	if requestEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", requestEvent)
	}
	select {
	case decision := <-resultCh:
		t.Fatalf("人工交互不应按墙钟自动结束: %+v", decision)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("期望 deny，实际: %+v", decision)
		}
		if !decision.Interrupt {
			t.Fatalf("AskUserQuestion 随 context 取消时应中断当前交互: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 context 取消结果失败")
	}

	resolved := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["status"] != "cancelled" {
		t.Fatalf("context 取消 resolved status 不正确: %+v", resolved.Data)
	}
}

func TestContextPendingStateTracksRequestLifecycle(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-pending-state"
	sender := newPermissionTestSender("sender-pending-state")
	ctx.BindSession(sessionKey, sender)
	pending, changed := ctx.PendingRequestState(sessionKey)
	if pending {
		t.Fatal("初始 session 不应处于待确认状态")
	}

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "Write",
			Input:    map[string]any{"file_path": "README.md"},
		})
		resultCh <- decision
	}()
	requestEvent := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("pending 登记后未发布状态变化")
	}
	pending, changed = ctx.PendingRequestState(sessionKey)
	if !pending {
		t.Fatal("权限请求等待期间应暂停 round idle timer")
	}

	if !ctx.HandlePermissionResponse(map[string]any{
		"request_id": requestEvent.Data["request_id"],
		"decision":   "deny",
	}) {
		t.Fatal("处理 permission_response 失败")
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("pending 结束后未发布状态变化")
	}
	if pending, _ = ctx.PendingRequestState(sessionKey); pending {
		t.Fatal("拒绝后应恢复 round idle timer")
	}
	select {
	case <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("权限请求未在拒绝后结束")
	}
}

func TestContextProjectsPendingLifecycleAndRoomSnapshot(t *testing.T) {
	ctx := NewContext()
	broadcaster := newPermissionTestRoomBroadcaster()
	ctx.SetRoomBroadcaster(broadcaster)
	sessionKey := "agent:nexus:ws:dm:test-room-projection"
	ctx.BindSessionRoute(sessionKey, RouteContext{
		DispatchSessionKey: sessionKey,
		RoomID:             "room-1",
		ConversationID:     "conversation-1",
		RoundID:            "round-1",
	})

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "AskUserQuestion",
			Input:    map[string]any{"questions": []any{}},
		})
		resultCh <- decision
	}()
	requestEvent := readPermissionEventByType(t, broadcaster.events, protocol.EventTypePermissionRequest)
	if roomID := <-broadcaster.roomIDs; roomID != "room-1" {
		t.Fatalf("Room 投影目标不正确: %q", roomID)
	}
	if requestEvent.DeliveryMode != protocol.DeliveryModeDurable {
		t.Fatalf("Room 人工交互事件必须可按 room_seq 重放: %+v", requestEvent)
	}
	requestID, _ := requestEvent.Data["request_id"].(string)
	if got := ctx.PendingRequestIDsForRoom("room-1", "conversation-1"); !slices.Equal(got, []string{requestID}) {
		t.Fatalf("Room pending 快照不正确: %v", got)
	}
	if got := ctx.PendingRequestIDsForRoom("room-1", "conversation-other"); len(got) != 0 {
		t.Fatalf("会话过滤不正确: %v", got)
	}

	if !ctx.HandlePermissionResponse(map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("处理 permission_response 失败")
	}
	resolved := readPermissionEventByType(t, broadcaster.events, protocol.EventTypePermissionRequestResolved)
	if roomID := <-broadcaster.roomIDs; roomID != "room-1" {
		t.Fatalf("Room resolved 投影目标不正确: %q", roomID)
	}
	if resolved.DeliveryMode != protocol.DeliveryModeDurable {
		t.Fatalf("Room resolved 事件必须可按 room_seq 重放: %+v", resolved)
	}
	if got := ctx.PendingRequestIDsForRoom("room-1", ""); len(got) != 0 {
		t.Fatalf("请求结束后 Room pending 快照未清空: %v", got)
	}
	select {
	case <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("权限请求未在批准后结束")
	}
}

func TestContextCancelRequestsForSessionBroadcastsResolved(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-cancel"
	sender := newPermissionTestSender("sender-cancel")
	ctx.BindSession(sessionKey, sender)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "go.mod",
			},
		})
		resultCh <- decision
	}()

	requestEvent := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	if requestEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", requestEvent)
	}

	if cancelled := ctx.CancelRequestsForSession(sessionKey, "session cancelled"); cancelled != 1 {
		t.Fatalf("期望取消 1 个请求，实际: %d", cancelled)
	}

	resolved := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["status"] != "cancelled" {
		t.Fatalf("cancel resolved status 不正确: %+v", resolved.Data)
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("期望 deny，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待取消结果失败")
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
