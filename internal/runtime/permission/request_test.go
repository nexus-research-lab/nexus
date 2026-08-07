package permission

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
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

	ctx.UnbindSession(sessionKey, senderA)
	ctx.BindSession(sessionKey, senderB)

	replayed := readPermissionEventByType(t, senderB.events, protocol.EventTypePermissionRequest)
	if replayed.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望重放 permission_request，实际: %+v", replayed)
	}

	requestID, _ := replayed.Data["request_id"].(string)
	if requestID == "" {
		t.Fatalf("request_id 为空: %+v", replayed.Data)
	}
	if !ctx.HandlePermissionResponse(t.Context(), sessionKey, map[string]any{
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

func TestConfigurationPermissionCarriesHumanOnlySecretsOutsideToolInput(t *testing.T) {
	permissionContext := NewContext()
	sessionKey := "agent:nexus:ws:dm:configuration-secret"
	recorder := &permissionTestApprovalRecorder{
		approvals: make(chan HumanToolApproval, 1),
	}
	permissionContext.SetHumanToolApprovalRecorder(recorder)
	sender := newPermissionTestSender("sender-configuration-secret")
	permissionContext.BindSession(sessionKey, sender)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := permissionContext.RequestPermission(
			context.Background(),
			sessionKey,
			sdkpermission.Request{
				ToolName: "mcp__nexus_config__apply_nexus_configuration_change",
				Input: map[string]any{
					"request_id":        "configuration-secret-01",
					"domain":            "providers",
					"operation":         "create",
					"expected_revision": "sha256:before",
					"plan_digest":       "hmac:plan",
					"input": map[string]any{
						"provider": "custom",
						"auth_token": map[string]any{
							"$secret": "provider.auth_token",
						},
					},
				},
			},
		)
		resultCh <- decision
	}()

	event := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	slots, ok := event.Data["configuration_secret_slots"].([]secretinput.Slot)
	if !ok || len(slots) != 1 ||
		slots[0] != (secretinput.Slot{ID: "provider.auth_token", Path: "auth_token"}) {
		t.Fatalf("permission event slots = %#v", event.Data["configuration_secret_slots"])
	}
	requestID, _ := event.Data["request_id"].(string)
	if !permissionContext.HandlePermissionResponse(t.Context(), sessionKey, map[string]any{
		"request_id": requestID,
		"decision":   "allow",
		"configuration_secrets": map[string]any{
			"provider.auth_token": "human-only-token",
		},
	}) {
		t.Fatal("permission response was not consumed")
	}

	select {
	case approval := <-recorder.approvals:
		if approval.ConfigurationSecrets["provider.auth_token"] != "human-only-token" {
			t.Fatalf("recorder did not receive transient human value: %#v", approval.ConfigurationSecrets)
		}
		toolPayload, err := json.Marshal(approval.ToolInput)
		if err != nil {
			t.Fatalf("marshal approval tool input: %v", err)
		}
		if strings.Contains(string(toolPayload), "human-only-token") {
			t.Fatalf("human secret entered model-visible tool input: %s", toolPayload)
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

func TestRecordedHumanApprovalToolsIncludeConnectorAuthorization(t *testing.T) {
	tests := map[string]bool{
		"apply_nexus_configuration_change":                         true,
		"mcp__nexus_config__apply_nexus_configuration_change":      true,
		"start_connector_authorization":                            true,
		"mcp__nexus_connector_auth__start_connector_authorization": true,
		"get_connector_authorization":                              false,
		"Read":                                                     false,
	}
	for toolName, expected := range tests {
		t.Run(toolName, func(t *testing.T) {
			if actual := isRecordedHumanApprovalTool(toolName); actual != expected {
				t.Fatalf(
					"isRecordedHumanApprovalTool(%q) = %t, want %t",
					toolName,
					actual,
					expected,
				)
			}
		})
	}
}

func TestContextReplayPendingRequestsUsesStableExpirationAndRequestOrder(t *testing.T) {
	ctx := NewContext()
	sessionKey := "agent:nexus:ws:dm:test-replay-order"
	expiresAt := time.Now().Add(time.Minute)
	pendingRequests := []*PendingRequest{
		{
			RequestID:          "permission-later",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/later"},
			ExpiresAt:          expiresAt.Add(time.Second),
		},
		{
			RequestID:          "permission-b",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/b"},
			ExpiresAt:          expiresAt,
		},
		{
			RequestID:          "permission-a",
			SessionKey:         sessionKey,
			DispatchSessionKey: sessionKey,
			ToolName:           "Read",
			ToolInput:          map[string]any{"file_path": "/tmp/a"},
			ExpiresAt:          expiresAt,
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
		ExpiresAt:          expiresAt.Add(-time.Second),
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

func TestContextRequestPermissionTimeoutBroadcastsResolved(t *testing.T) {
	ctx := NewContext()
	ctx.requestTimeout = 20 * time.Millisecond
	sessionKey := "agent:nexus:ws:dm:test-timeout"
	sender := newPermissionTestSender("sender-timeout")
	ctx.BindSession(sessionKey, sender)

	resultCh := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := ctx.RequestPermission(context.Background(), sessionKey, sdkpermission.Request{
			ToolName: "Read",
			Input: map[string]any{
				"file_path": "README.md",
			},
		})
		resultCh <- decision
	}()

	requestEvent := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequest)
	if requestEvent.EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("期望 permission_request，实际: %+v", requestEvent)
	}
	resolved := readPermissionEventByType(t, sender.events, protocol.EventTypePermissionRequestResolved)
	if resolved.EventType != protocol.EventTypePermissionRequestResolved {
		t.Fatalf("期望 permission_request_resolved，实际: %+v", resolved)
	}
	if resolved.Data["status"] != "expired" {
		t.Fatalf("timeout resolved status 不正确: %+v", resolved.Data)
	}

	select {
	case decision := <-resultCh:
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("期望 deny，实际: %+v", decision)
		}
		if decision.ErrorCode != sdkpermission.ErrorCodeRequestTimeout {
			t.Fatalf("期望结构化 timeout error code，实际: %+v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待超时结果失败")
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
