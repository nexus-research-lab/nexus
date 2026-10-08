package permission

import (
	"context"
	"errors"
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

type permissionTestApprovalRecorder struct {
	approvals chan HumanToolApproval
	err       error
}

func (r *permissionTestApprovalRecorder) RecordHumanToolApproval(
	_ context.Context,
	approval HumanToolApproval,
) error {
	if len(approval.ConfigurationSecrets) > 0 {
		values := make(map[string]string, len(approval.ConfigurationSecrets))
		for key, value := range approval.ConfigurationSecrets {
			values[key] = value
		}
		approval.ConfigurationSecrets = values
	}
	r.approvals <- approval
	return r.err
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

func TestResolveSessionPermissionRequestAppliesSDKPersistenceSuggestion(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:wx:dm:runtime-permission"
	sender := newPermissionTestSender("im-permission-sender")
	ctx.BindSession(sessionKey, sender)

	decisions := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "Write",
			Input:    map[string]any{"file_path": "notes.txt"},
			PermissionSuggestions: []sdkpermission.Update{{
				Type:        "addRules",
				Behavior:    sdkpermission.BehaviorAllow,
				Destination: sdkpermission.UpdateDestinationSession,
				Rules: []sdkpermission.RuleValue{{
					ToolName:    "Write",
					RuleContent: "notes.txt",
				}},
			}},
		})
		decisions <- decision
	}()

	event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	requestID, _ := event.Data["request_id"].(string)
	resolution := ctx.ResolveSessionPermissionRequest(
		t.Context(),
		sessionKey,
		"",
		sdkpermission.BehaviorAllow,
		true,
	)
	if !resolution.Found || !resolution.Resolved || !resolution.Persisted || !resolution.PersistenceSupported ||
		resolution.RequestID != requestID || resolution.MatchingRequests != 1 {
		t.Fatalf("持续允许没有解析同一 pending request: %+v", resolution)
	}
	select {
	case decision := <-decisions:
		if decision.Behavior != sdkpermission.BehaviorAllow || len(decision.UpdatedPermissions) != 1 ||
			decision.UpdatedPermissions[0].Destination != sdkpermission.UpdateDestinationSession ||
			len(decision.UpdatedPermissions[0].Rules) != 1 ||
			decision.UpdatedPermissions[0].Rules[0].RuleContent != "notes.txt" {
			t.Fatalf("SDK persistence suggestion 未原样返回: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("持续允许后 runtime 未恢复")
	}
}

func TestResolveSessionPermissionRequestWithoutIDRejectsAmbiguousSession(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:wx:dm:ambiguous-runtime-permission"
	sender := newPermissionTestSender("im-ambiguous-permission-sender")
	ctx.BindSession(sessionKey, sender)
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, toolName := range []string{"Write", "Bash"} {
		toolName := toolName
		go func() {
			_, _ = ctx.RequestPermission(requestCtx, sessionKey, sdkpermission.Request{ToolName: toolName})
		}()
	}
	_ = readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	_ = readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	deadline := time.Now().Add(2 * time.Second)
	for ctx.CountSessionPermissionRequests(sessionKey, "") != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := ctx.CountSessionPermissionRequests(sessionKey, ""); got != 2 {
		t.Fatalf("两个请求尚未都登记为 pending: got %d", got)
	}

	resolution := ctx.ResolveSessionPermissionRequest(
		t.Context(),
		sessionKey,
		"",
		sdkpermission.BehaviorAllow,
		false,
	)
	if resolution.Found || resolution.Resolved || resolution.MatchingRequests != 2 {
		t.Fatalf("无 ID 决策不得猜测同一 session 的多个请求: %+v", resolution)
	}
	if got := ctx.CountSessionPermissionRequests(sessionKey, ""); got != 2 {
		t.Fatalf("只读 pending 计数 = %d; want 2", got)
	}
	if got := ctx.CountSessionPermissionRequests(sessionKey, "missing"); got != 0 {
		t.Fatalf("未知请求只读 pending 计数 = %d; want 0", got)
	}
}

func TestConfigurationPermissionAllowBindsExactRuntimeRoute(t *testing.T) {
	permissionContext := NewContext()
	runtimeSessionKey := "agent:worker:ws:group:conversation"
	dispatchSessionKey := "room:group:conversation"
	roundID := "round-human-approval"
	recorder := &permissionTestApprovalRecorder{
		approvals: make(chan HumanToolApproval, 1),
	}
	permissionContext.SetHumanToolApprovalRecorder(recorder)
	routeLease := permissionContext.BindSessionRoute(runtimeSessionKey, RouteContext{
		DispatchSessionKey: dispatchSessionKey,
		RoomID:             "room-human-approval",
		ConversationID:     "conversation-human-approval",
		AgentID:            "worker",
		RoundID:            roundID,
	})
	defer permissionContext.UnbindSessionRoute(routeLease)
	sender := newPermissionTestSender("sender-human-approval")
	permissionContext.BindSession(dispatchSessionKey, sender)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := permissionContext.RequestPermission(
			context.Background(),
			runtimeSessionKey,
			sdkpermission.Request{
				ToolName: "mcp__nexus_config__apply_nexus_configuration_change",
				Input: map[string]any{
					"request_id":        "approval-route-01",
					"domain":            "rooms",
					"operation":         "set_collaboration_policy",
					"expected_revision": "sha256:before",
					"plan_digest":       "hmac:plan",
				},
			},
		)
		resultCh <- decision
	}()

	event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	requestID, _ := event.Data["request_id"].(string)
	if permissionContext.HandlePermissionResponse(t.Context(), runtimeSessionKey, map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("runtime session must not answer a request routed to a different visible session")
	}
	if !permissionContext.HandlePermissionResponse(t.Context(), dispatchSessionKey, map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("exact dispatch session could not answer configuration permission")
	}

	select {
	case approval := <-recorder.approvals:
		if approval.PermissionRequestID != requestID ||
			approval.RuntimeSessionKey != runtimeSessionKey ||
			approval.DispatchSessionKey != dispatchSessionKey ||
			approval.Route.RoundID != roundID ||
			approval.Route.RoomID != "room-human-approval" {
			t.Fatalf("approval lost server-bound route: %+v", approval)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("human approval recorder was not called")
	}
	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("configuration permission decision = %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting for configuration permission decision")
	}
}

func TestConfigurationPermissionRecorderFailureDeniesTool(t *testing.T) {
	permissionContext := NewContext()
	sessionKey := "agent:nexus:ws:dm:approval-failure"
	recorder := &permissionTestApprovalRecorder{
		approvals: make(chan HumanToolApproval, 1),
		err:       errors.New("approval verification failed"),
	}
	permissionContext.SetHumanToolApprovalRecorder(recorder)
	sender := newPermissionTestSender("sender-approval-failure")
	permissionContext.BindSession(sessionKey, sender)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := permissionContext.RequestPermission(
			context.Background(),
			sessionKey,
			sdkpermission.Request{
				ToolName: "apply_nexus_configuration_change",
				Input: map[string]any{
					"request_id":        "approval-failure-01",
					"expected_revision": "sha256:before",
					"plan_digest":       "hmac:plan",
				},
			},
		)
		resultCh <- decision
	}()
	event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	requestID, _ := event.Data["request_id"].(string)
	if !permissionContext.HandlePermissionResponse(t.Context(), sessionKey, map[string]any{
		"request_id": requestID,
		"decision":   "allow",
	}) {
		t.Fatal("permission response was not consumed")
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("recorder failure must deny configuration tool: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting for denied configuration permission")
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

	if !ctx.HandlePermissionResponse(t.Context(), sessionKey, map[string]any{
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

	if !ctx.HandlePermissionResponse(t.Context(), sessionKey, map[string]any{
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

// 同一个 Room 公区承载多个成员，手机审批只能解析绑定成员的请求。
func TestMemberSessionPermissionDoesNotResolveOtherRoomMember(t *testing.T) {
	c := NewContext()
	shared := protocol.BuildRoomSharedSessionKey("topic")
	first := protocol.BuildRoomAgentSessionKey("topic", "first", protocol.RoomTypeGroup)
	second := protocol.BuildRoomAgentSessionKey("topic", "second", protocol.RoomTypeGroup)
	sender := newPermissionTestSender("room-members")
	c.BindSession(shared, sender)
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ids := make(map[string]string)
	for _, session := range []string{first, second} {
		lease := c.BindSessionRoute(session, RouteContext{DispatchSessionKey: shared})
		defer c.UnbindSessionRoute(lease)
		go func() { _, _ = c.RequestPermission(requestCtx, session, sdkpermission.Request{ToolName: "Write"}) }()
		event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
		ids[session], _ = event.Data["request_id"].(string)
	}
	if c.CountSessionPermissionRequests(shared, "") != 2 || c.CountSessionPermissionRequests(first, "") != 1 {
		t.Fatal("成员权限计数必须与公区聚合隔离")
	}
	if c.CountSessionPermissionRequests(first, ids[second]) != 0 {
		t.Fatal("不能查询另一个成员的请求")
	}
	if got := c.ResolveSessionPermissionRequest(t.Context(), first, ids[second], sdkpermission.BehaviorAllow, false); got.Found || got.Resolved {
		t.Fatalf("跨成员批准: %+v", got)
	}
	if got := c.ResolveSessionPermissionRequest(t.Context(), first, "", sdkpermission.BehaviorAllow, false); !got.Resolved || got.RequestID != ids[first] {
		t.Fatalf("未批准唯一绑定成员请求: %+v", got)
	}
	if c.CountSessionPermissionRequests(second, ids[second]) != 1 {
		t.Fatal("另一个成员的请求必须保持待确认")
	}
}
