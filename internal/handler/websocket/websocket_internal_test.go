// websocket_internal_test.go 覆盖 WebSocket 渠道授权、控制分发、错误、执行失效与 Room 订阅注册表测试。
package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type channelAuthorizationTestSender struct {
	key         string
	closed      bool
	closeReason string
	err         error
	mu          sync.Mutex
	events      []protocol.EventMessage
}

func (s *channelAuthorizationTestSender) Key() string { return s.key }

func (s *channelAuthorizationTestSender) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *channelAuthorizationTestSender) ClosePolicy(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.closeReason = reason
	return nil
}

func (s *channelAuthorizationTestSender) SendEvent(
	_ context.Context,
	event protocol.EventMessage,
) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *channelAuthorizationTestSender) snapshot() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.EventMessage(nil), s.events...)
}

type channelAuthorizationTestController struct {
	calls        int
	submission   authorizationsvc.HumanVerificationCodeSubmission
	cancelCalls  int
	cancelActor  authorizationsvc.Actor
	cancelFlowID string
	err          error
}

func (c *channelAuthorizationTestController) SubmitHumanVerificationCode(
	_ context.Context,
	submission authorizationsvc.HumanVerificationCodeSubmission,
) (*authorizationsvc.View, error) {
	c.calls++
	c.submission = submission
	if c.err != nil {
		return nil, c.err
	}
	return &authorizationsvc.View{
		FlowID:  submission.FlowID,
		Status:  "running",
		Message: "等待平台确认。",
	}, nil
}

func (c *channelAuthorizationTestController) Cancel(
	_ context.Context,
	actor authorizationsvc.Actor,
	flowID string,
) (*authorizationsvc.View, error) {
	c.cancelCalls++
	c.cancelActor = actor
	c.cancelFlowID = flowID
	if c.err != nil {
		return nil, c.err
	}
	return &authorizationsvc.View{
		FlowID:  flowID,
		Status:  "cancelled",
		Message: "授权已取消。",
	}, nil
}

func TestChannelAuthorizationPresentationTargetsExactAuthenticatedSession(t *testing.T) {
	permission := permissionctx.NewContext()
	transport := newChannelAuthorizationTransport()
	handler := &Handler{permission: permission, channelAuthorization: transport}

	ownerSender := &channelAuthorizationTestSender{key: "owner"}
	otherSender := &channelAuthorizationTestSender{key: "other"}
	registerChannelAuthorizationTestSender(t, transport, ownerSender, "owner-1")
	registerChannelAuthorizationTestSender(t, transport, otherSender, "owner-2")
	permission.BindSession("nexus/ws/agent/main", ownerSender)
	permission.BindSession("nexus/ws/agent/main", otherSender)

	presentation := channelAuthorizationTestPresentation()
	if err := handler.PresentChannelAuthorization(context.Background(), presentation); err != nil {
		t.Fatalf("PresentChannelAuthorization() error = %v", err)
	}
	ownerEvents := ownerSender.snapshot()
	if len(ownerEvents) != 1 ||
		ownerEvents[0].EventType != protocol.EventTypeChannelAuthorization {
		t.Fatalf("owner events = %+v", ownerEvents)
	}
	if got := otherSender.snapshot(); len(got) != 0 {
		t.Fatalf("other principal received presentation: %+v", got)
	}
	if ownerEvents[0].SessionKey != presentation.BusinessSessionKey {
		t.Fatalf("session key = %q", ownerEvents[0].SessionKey)
	}
	encoded, err := json.Marshal(ownerEvents[0])
	if err != nil {
		t.Fatal(err)
	}
	wire := string(encoded)
	for _, forbidden := range []string{
		presentation.RootRoundID,
		presentation.RuntimeLeaseSessionKey,
		presentation.RuntimeLeaseRoundID,
		"agent_id",
		"principal_user_id",
		"root_round_id",
		"runtime_lease_session_key",
		"runtime_lease_round_id",
	} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("presentation leaks trusted binding %q: %s", forbidden, wire)
		}
	}
	if !strings.Contains(wire, presentation.QRPayload) {
		t.Fatalf("native presentation omitted QR payload: %s", wire)
	}
}

func TestChannelAuthorizationInvalidationClosesOnlyMatchingPrincipal(t *testing.T) {
	transport := newChannelAuthorizationTransport()
	ownerSender := &channelAuthorizationTestSender{key: "owner"}
	otherSender := &channelAuthorizationTestSender{key: "other"}
	registerChannelAuthorizationTestSender(t, transport, ownerSender, "owner-1")
	registerChannelAuthorizationTestSender(t, transport, otherSender, "owner-2")

	if closed := transport.closePrincipal("owner-1"); closed != 1 {
		t.Fatalf("closed connections = %d, want 1", closed)
	}
	if !ownerSender.IsClosed() || otherSender.IsClosed() {
		t.Fatalf("owner closed = %v, other closed = %v", ownerSender.IsClosed(), otherSender.IsClosed())
	}
}

func TestChannelAuthorizationSessionInvalidationClosesOnlyExactSession(t *testing.T) {
	transport := newChannelAuthorizationTransport()
	revoked := &channelAuthorizationTestSender{key: "revoked"}
	sibling := &channelAuthorizationTestSender{key: "sibling"}
	registerChannelAuthorizationTestSenderWithSession(t, transport, revoked, "owner-1", "session-a")
	registerChannelAuthorizationTestSenderWithSession(t, transport, sibling, "owner-1", "session-b")

	if closed := transport.closeAuthSession("session-a"); closed != 1 {
		t.Fatalf("closed connections = %d, want 1", closed)
	}
	if !revoked.IsClosed() || sibling.IsClosed() {
		t.Fatalf("revoked closed = %v, sibling closed = %v", revoked.IsClosed(), sibling.IsClosed())
	}
}

func TestChannelAuthorizationPresentationFailsClosedWithoutExactRoute(t *testing.T) {
	permission := permissionctx.NewContext()
	transport := newChannelAuthorizationTransport()
	handler := &Handler{permission: permission, channelAuthorization: transport}
	sender := &channelAuthorizationTestSender{key: "owner"}
	registerChannelAuthorizationTestSender(t, transport, sender, "owner-1")
	permission.BindSession("nexus/ws/agent/different", sender)

	if err := handler.PresentChannelAuthorization(
		context.Background(),
		channelAuthorizationTestPresentation(),
	); err == nil {
		t.Fatal("presentation without exact business-session binding succeeded")
	}
	if len(sender.snapshot()) != 0 {
		t.Fatal("presentation was broadcast to the wrong session")
	}
}

func TestChannelAuthorizationCodeSubmissionUsesServerRouteAndNeverEchoesCode(t *testing.T) {
	permission := permissionctx.NewContext()
	transport := newChannelAuthorizationTransport()
	handler := &Handler{permission: permission, channelAuthorization: transport}
	controller := &channelAuthorizationTestController{}
	handler.SetChannelAuthorizationController(controller)
	sender := &channelAuthorizationTestSender{key: "owner"}
	registerChannelAuthorizationTestSender(t, transport, sender, "owner-1")
	permission.BindSession("nexus/ws/agent/main", sender)
	presentation := channelAuthorizationTestPresentation()
	if err := handler.PresentChannelAuthorization(context.Background(), presentation); err != nil {
		t.Fatal(err)
	}

	inbound := map[string]any{
		"type":               "submit_channel_authorization_code",
		"flow_id":            presentation.FlowID,
		"presentation_token": presentation.PresentationToken,
		"code":               "741852",
		// These untrusted fields are ignored rather than copied into the service submission.
		"owner_user_id":          "attacker",
		"agent_id":               "attacker-agent",
		"session_key":            "attacker-session",
		"round_id":               "attacker-round",
		"runtime_lease_round_id": "attacker-lease",
	}
	handler.handleChannelAuthorizationCode(context.Background(), sender, inbound)
	if _, exists := inbound["code"]; exists {
		t.Fatal("verification code remained in generic inbound map")
	}
	if controller.calls != 1 {
		t.Fatalf("controller calls = %d", controller.calls)
	}
	got := controller.submission
	if got.OwnerUserID != "owner-1" ||
		got.PrincipalUserID != "owner-1" ||
		got.AgentID != presentation.AgentID ||
		got.BusinessSessionKey != presentation.BusinessSessionKey ||
		got.RootRoundID != presentation.RootRoundID ||
		got.RuntimeLeaseSessionKey != presentation.RuntimeLeaseSessionKey ||
		got.RuntimeLeaseRoundID != presentation.RuntimeLeaseRoundID ||
		got.Code != "741852" {
		t.Fatalf("submission did not use server route: %+v", got)
	}
	events := sender.snapshot()
	if len(events) != 2 ||
		events[1].EventType != protocol.EventTypeChannelAuthorizationResult {
		t.Fatalf("events = %+v", events)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "741852") {
		t.Fatalf("verification code leaked into outbound events: %s", encoded)
	}
}

func TestChannelAuthorizationCodeRejectsWrongTokenOrUnboundSession(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*permissionctx.Context, *channelAuthorizationTestSender, map[string]any)
	}{
		{
			name: "wrong token",
			mutate: func(
				_ *permissionctx.Context,
				_ *channelAuthorizationTestSender,
				inbound map[string]any,
			) {
				inbound["presentation_token"] = "presentation_wrong"
			},
		},
		{
			name: "business session unbound",
			mutate: func(
				permission *permissionctx.Context,
				sender *channelAuthorizationTestSender,
				_ map[string]any,
			) {
				permission.UnbindSession("nexus/ws/agent/main", sender)
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			permission := permissionctx.NewContext()
			transport := newChannelAuthorizationTransport()
			handler := &Handler{permission: permission, channelAuthorization: transport}
			controller := &channelAuthorizationTestController{err: errors.New("must not run")}
			handler.SetChannelAuthorizationController(controller)
			sender := &channelAuthorizationTestSender{key: "owner"}
			registerChannelAuthorizationTestSender(t, transport, sender, "owner-1")
			permission.BindSession("nexus/ws/agent/main", sender)
			presentation := channelAuthorizationTestPresentation()
			if err := handler.PresentChannelAuthorization(context.Background(), presentation); err != nil {
				t.Fatal(err)
			}
			inbound := map[string]any{
				"flow_id":            presentation.FlowID,
				"presentation_token": presentation.PresentationToken,
				"code":               "never-echo-this",
			}
			testCase.mutate(permission, sender, inbound)
			handler.handleChannelAuthorizationCode(context.Background(), sender, inbound)
			if controller.calls != 0 {
				t.Fatalf("controller calls = %d", controller.calls)
			}
			events := sender.snapshot()
			encoded, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "never-echo-this") {
				t.Fatalf("rejected code leaked into events: %s", encoded)
			}
			if len(events) != 2 || events[1].Data["accepted"] != false {
				t.Fatalf("rejection event = %+v", events)
			}
		})
	}
}

func TestChannelAuthorizationCancelUsesTrustedActorAndPresentationRoute(t *testing.T) {
	permission := permissionctx.NewContext()
	transport := newChannelAuthorizationTransport()
	handler := &Handler{permission: permission, channelAuthorization: transport}
	controller := &channelAuthorizationTestController{}
	handler.SetChannelAuthorizationController(controller)
	sender := &channelAuthorizationTestSender{key: "owner"}
	registerChannelAuthorizationTestSender(t, transport, sender, "owner-1")
	permission.BindSession("nexus/ws/agent/main", sender)
	presentation := channelAuthorizationTestPresentation()
	if err := handler.PresentChannelAuthorization(context.Background(), presentation); err != nil {
		t.Fatal(err)
	}

	handler.handleChannelAuthorizationCancel(context.Background(), sender, map[string]any{
		"flow_id":            presentation.FlowID,
		"presentation_token": presentation.PresentationToken,
		"owner_user_id":      "attacker",
		"session_key":        "attacker-session",
	})
	if controller.cancelCalls != 1 || controller.cancelFlowID != presentation.FlowID {
		t.Fatalf("cancel calls = %d, flow = %q", controller.cancelCalls, controller.cancelFlowID)
	}
	actor := controller.cancelActor
	if actor.OwnerUserID != presentation.PrincipalUserID ||
		actor.AgentID != presentation.AgentID ||
		actor.SessionKey != presentation.BusinessSessionKey ||
		actor.RoundID != presentation.RootRoundID ||
		actor.LeaseSessionKey != presentation.RuntimeLeaseSessionKey ||
		actor.LeaseRoundID != presentation.RuntimeLeaseRoundID ||
		actor.ContextKind != "agent" ||
		actor.ContextID != presentation.AgentID ||
		actor.RoundLeaseRequired {
		t.Fatalf("cancel actor did not use trusted presentation route: %+v", actor)
	}
	events := sender.snapshot()
	if len(events) != 2 || events[1].Data["accepted"] != true {
		t.Fatalf("cancel result events = %+v", events)
	}
}

func registerChannelAuthorizationTestSender(
	t *testing.T,
	transport *channelAuthorizationTransport,
	sender *channelAuthorizationTestSender,
	principalUserID string,
) {
	t.Helper()
	registerChannelAuthorizationTestSenderWithSession(
		t,
		transport,
		sender,
		principalUserID,
		principalUserID+"-session",
	)
}

func registerChannelAuthorizationTestSenderWithSession(
	t *testing.T,
	transport *channelAuthorizationTransport,
	sender *channelAuthorizationTestSender,
	principalUserID string,
	sessionID string,
) {
	t.Helper()
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     principalUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
		SessionID:  &sessionID,
	})
	transport.registerAuthenticatedSender(ctx, sender)
}

func channelAuthorizationTestPresentation() authorizationsvc.HumanPresentation {
	return authorizationsvc.HumanPresentation{
		FlowID:                 "channel_flow_1",
		PresentationToken:      "presentation_1",
		Kind:                   authorizationsvc.PresentationKindQRCode,
		ChannelType:            "weixin-personal",
		AccountBinding:         "new",
		QRPayload:              "https://example.invalid/qr/private",
		QRPayloadType:          "url",
		Prompt:                 "请扫码",
		PrincipalUserID:        "owner-1",
		PrincipalAuthMethod:    authctx.AuthMethodPassword,
		PrincipalAuthSessionID: "owner-1-session",
		AgentID:                "main",
		BusinessSessionKey:     "nexus/ws/agent/main",
		RootRoundID:            "root-round-1",
		RuntimeLeaseSessionKey: "runtime-session-1",
		RuntimeLeaseRoundID:    "runtime-round-1",
		ExpiresAt:              time.Now().UTC().Add(time.Minute),
	}
}

func TestControlMessageDispatcherCloseRejectsBlockedAndLateEnqueue(t *testing.T) {
	dispatcher := newControlMessageDispatcher(context.Background())

	activeStarted := make(chan struct{})
	releaseActive := make(chan struct{})
	dispatcher.enqueueJob(
		&controlMessage{msgType: "chat"},
		func() {
			close(activeStarted)
			<-releaseActive
		},
	)
	<-activeStarted

	// Keep the consumer occupied and fill every queue slot. The next producer
	// must block inside the acceptance fence until close cancels the dispatcher.
	for range cap(dispatcher.queue) {
		dispatcher.enqueueJob(
			&controlMessage{msgType: "chat"},
			func() {},
		)
	}
	blockedDiscarded := make(chan struct{})
	blockedReturned := make(chan struct{})
	go func() {
		dispatcher.enqueueJobValue(controlMessageJob{
			run:     func() { t.Error("blocked job ran after close") },
			discard: func() { close(blockedDiscarded) },
		})
		close(blockedReturned)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		if !dispatcher.acceptanceMu.TryLock() {
			break
		}
		dispatcher.acceptanceMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("producer did not block on the full dispatcher queue")
		}
		time.Sleep(time.Millisecond)
	}

	dispatcher.close()
	// close is intentionally idempotent because handler and tests may both own
	// cleanup paths.
	dispatcher.close()
	select {
	case <-blockedDiscarded:
	case <-time.After(time.Second):
		t.Fatal("blocked enqueue was not rejected by close")
	}
	select {
	case <-blockedReturned:
	case <-time.After(time.Second):
		t.Fatal("blocked producer did not leave enqueue after close")
	}

	lateRan := make(chan struct{}, 1)
	lateDiscarded := make(chan struct{})
	dispatcher.enqueueJobValue(controlMessageJob{
		run:     func() { lateRan <- struct{}{} },
		discard: func() { close(lateDiscarded) },
	})
	select {
	case <-lateDiscarded:
	case <-time.After(time.Second):
		t.Fatal("late enqueue was not rejected")
	}
	select {
	case <-lateRan:
		t.Fatal("late enqueue ran after dispatcher close")
	default:
	}
	close(releaseActive)
}

func TestGatewayErrorLogsCauseAndDeliveryFailureWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil)).With("request_id", "connection-1")
	ctx, cancel := context.WithCancel(logx.WithLogger(context.Background(), logger))
	cancel()
	h := &Handler{}
	sender := handlershared.NewWebSocketSender(nil)
	sender.MarkClosed()
	cause := errors.New("room input_queue content must mention target agent")
	h.sendGatewayError(ctx, sender, "room:group:conversation-1", "input_queue_error", cause, map[string]any{
		"type": "input_queue", "action": "enqueue",
		"client_request_id": "request-1", "client_message_id": "message-1",
		"content": "private message body", "auth_token": "private credential",
	})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected cause and delivery logs, got %s", output.String())
	}
	for index, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]any{
			"level": "WARN", "request_id": "connection-1", "session_key": "room:group:conversation-1",
			"error_type": "input_queue_error", "failure_code": string(protocol.ConversationFailureRequestRejected),
			"type": "input_queue", "action": "enqueue", "client_request_id": "request-1",
			"client_message_id": "message-1", "err": cause.Error(), "context_err": context.Canceled.Error(),
		} {
			if entry[key] != want {
				t.Fatalf("log %d %s = %v, want %v", index, key, entry[key], want)
			}
		}
		if strings.Contains(line, "private") || entry["content"] != nil || entry["auth_token"] != nil {
			t.Fatalf("business input leaked into diagnostics: %s", line)
		}
		if index == 1 && entry["send_err"] != context.Canceled.Error() {
			t.Fatalf("delivery error lost: %s", line)
		}
	}
	if got := h.errorEventDetail("input_queue_error", cause); got != "服务内部错误" {
		t.Fatalf("public error exposed internal cause: %q", got)
	}
}

func TestExecutionInvalidationRegistryFailsClosedAndReleasesSenderScopes(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	sender := &executionInvalidationTestSender{key: "sender-1"}
	registry.Bind("", "session-1", sender)
	registry.Bind("owner-a", "", sender)
	registry.Bind("owner-a", "session-1", sender)
	registry.Bind("owner-a", "session-2", sender)

	event := protocol.NewExecutionInvalidatedEvent(
		"session-1",
		protocol.ExecutionInvalidationData{ExecutionID: "execution-1"},
	)
	registry.Broadcast(context.Background(), "", "session-1", event)
	registry.Broadcast(context.Background(), "owner-a", "", event)
	if len(sender.events) != 0 {
		t.Fatalf("incomplete scope broadcast events = %#v", sender.events)
	}

	registry.Unbind("owner-a", "session-1", sender)
	registry.Broadcast(context.Background(), "owner-a", "session-1", event)
	if len(sender.events) != 0 {
		t.Fatalf("unbound scope events = %#v", sender.events)
	}

	registry.UnregisterSender(sender)
	registry.Broadcast(context.Background(), "owner-a", "session-2", event)
	if len(sender.events) != 0 || len(registry.byScope) != 0 || len(registry.senderScopes) != 0 {
		t.Fatalf(
			"sender registry leaked: events=%#v by_scope=%#v sender_scopes=%#v",
			sender.events,
			registry.byScope,
			registry.senderScopes,
		)
	}
}

func TestHandlerProjectsExecutionInvalidationPayload(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	sender := &executionInvalidationTestSender{key: "owner"}
	registry.Bind("owner-a", "session-1", sender)
	handler := &Handler{executionInvalidations: registry}

	handler.InvalidateExecution(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a",
		SessionKey:  "session-1",
		ExecutionID: "execution-1",
		Version:     7,
	})

	if len(sender.events) != 1 {
		t.Fatalf("events = %#v", sender.events)
	}
	event := sender.events[0]
	if event.SessionKey != "session-1" ||
		event.Data["execution_id"] != "execution-1" ||
		event.Data["version"] != int64(7) {
		t.Fatalf("event = %#v", event)
	}
}

func TestExecutionInvalidationRegistryCoalescesBurstsPerScope(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	registry.window = 40 * time.Millisecond
	sender := &executionInvalidationTestSender{key: "sender-1"}
	other := &executionInvalidationTestSender{key: "sender-2"}
	registry.Bind("owner-a", "session-1", sender)
	registry.Bind("owner-a", "session-2", other)

	for version := int64(1); version <= 20; version++ {
		registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
			OwnerUserID: "owner-a", SessionKey: "session-1", ExecutionID: "execution-1", Version: version,
		})
	}
	registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a", SessionKey: "session-2", ExecutionID: "execution-2",
	})
	// 首个失效立即投递，另一个 scope 不受该窗口影响。
	if got := sender.snapshot(); len(got) != 1 || got[0].Data["version"] != int64(1) {
		t.Fatalf("leading events = %#v", got)
	}
	if got := other.snapshot(); len(got) != 1 {
		t.Fatalf("independent scope events = %#v", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(sender.snapshot()) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	got := sender.snapshot()
	if len(got) != 2 || got[1].Data["version"] != int64(20) {
		t.Fatalf("trailing events = %#v, want one carrying the latest version", got)
	}

	// 空窗口结束后释放 scope，下一次失效重新立即投递。
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		registry.mu.Lock()
		idle := len(registry.throttles) == 0
		registry.mu.Unlock()
		if idle {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a", SessionKey: "session-1", ExecutionID: "execution-1", Version: 21,
	})
	if got := sender.snapshot(); len(got) != 3 || got[2].Data["version"] != int64(21) {
		t.Fatalf("post-idle events = %#v", got)
	}
}

type executionInvalidationTestSender struct {
	mu     sync.Mutex
	key    string
	closed bool
	events []protocol.EventMessage
}

func (s *executionInvalidationTestSender) Key() string    { return s.key }
func (s *executionInvalidationTestSender) IsClosed() bool { return s.closed }
func (s *executionInvalidationTestSender) SendEvent(
	_ context.Context,
	event protocol.EventMessage,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *executionInvalidationTestSender) snapshot() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.EventMessage(nil), s.events...)
}

type fakeRoomRegistrySender struct {
	key    string
	events chan protocol.EventMessage
}

func newFakeRoomRegistrySender(key string) *fakeRoomRegistrySender {
	return &fakeRoomRegistrySender{
		key:    key,
		events: make(chan protocol.EventMessage, 16),
	}
}

func (s *fakeRoomRegistrySender) Key() string    { return s.key }
func (s *fakeRoomRegistrySender) IsClosed() bool { return false }
func (s *fakeRoomRegistrySender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestNotifyAutomationPermissionEventProjectsToSessionAndRoom(t *testing.T) {
	permission := permissionctx.NewContext()
	roomSubs := newRoomSubscriptionRegistry(8)
	handler := &Handler{permission: permission, roomSubs: roomSubs}
	sessionSender := newFakeRoomRegistrySender("session-sender")
	roomSender := newFakeRoomRegistrySender("room-sender")
	sessionKey := "agent:agent-recipient:ws:dm:conversation-1"
	permission.BindSession(sessionKey, sessionSender)
	if err := roomSubs.SubscribeRoom(
		context.Background(),
		roomSender,
		"room-1",
		"conversation-1",
		nil,
	); err != nil {
		t.Fatalf("SubscribeRoom() error = %v", err)
	}
	event := protocol.NewEvent(protocol.EventTypePermissionRequest, map[string]any{
		"request_id": "automation-permission-1",
	})
	event.SessionKey = sessionKey
	event.RoomID = "room-1"
	event.ConversationID = "conversation-1"

	handler.NotifyAutomationPermissionEvent(context.Background(), event)

	sessionEvent := readRoomRegistryEvent(t, sessionSender.events)
	if sessionEvent.EventType != protocol.EventTypePermissionRequest ||
		sessionEvent.SessionKey != sessionKey || sessionEvent.RoomSeq != nil {
		t.Fatalf("session event = %+v", sessionEvent)
	}
	roomEvent := readRoomRegistryEvent(t, roomSender.events)
	if roomEvent.EventType != protocol.EventTypePermissionRequest ||
		roomEvent.DeliveryMode != protocol.DeliveryModeDurable ||
		roomEvent.RoomSeq == nil || *roomEvent.RoomSeq != 1 {
		t.Fatalf("room event = %+v", roomEvent)
	}
}

func TestActiveChatActivitySourcesAggregatesExactConversationSessions(t *testing.T) {
	permission := permissionctx.NewContext()
	runtime := runtimectx.NewManager()
	handler := &Handler{permission: permission, runtime: runtime}

	bindRunningRound := func(runtimeSessionKey string, dispatchSessionKey string, conversationID string, roundID string) {
		t.Helper()
		permission.BindSessionRoute(runtimeSessionKey, permissionctx.RouteContext{
			DispatchSessionKey: dispatchSessionKey,
			RoomID:             "room-1",
			ConversationID:     conversationID,
		})
		if err := runtime.StartRound(context.Background(), runtimeSessionKey, roundID, func() {}); err != nil {
			t.Fatalf("StartRound(%q): %v", runtimeSessionKey, err)
		}
	}

	bindRunningRound("agent:cindy:ws:dm:conversation-a", "agent:cindy:ws:dm:conversation-a", "conversation-a", "round-a")
	bindRunningRound("agent:cindy:ws:dm:conversation-b", "agent:cindy:ws:dm:conversation-b", "conversation-b", "round-b")
	permission.BindSessionRoute("agent:idle:ws:dm:conversation-idle", permissionctx.RouteContext{
		DispatchSessionKey: "agent:idle:ws:dm:conversation-idle",
		RoomID:             "room-1",
		ConversationID:     "conversation-idle",
	})

	sources := handler.activeChatActivitySources("room-1")
	if len(sources) != 2 {
		t.Fatalf("active source count = %d, want 2: %+v", len(sources), sources)
	}
	if sources[0].SessionKey != "agent:cindy:ws:dm:conversation-a" ||
		sources[0].ConversationID != "conversation-a" ||
		len(sources[0].RunningRoundIDs) != 1 || sources[0].RunningRoundIDs[0] != "round-a" {
		t.Fatalf("first source = %+v", sources[0])
	}
	if sources[1].SessionKey != "agent:cindy:ws:dm:conversation-b" ||
		sources[1].ConversationID != "conversation-b" ||
		len(sources[1].RunningRoundIDs) != 1 || sources[1].RunningRoundIDs[0] != "round-b" {
		t.Fatalf("second source = %+v", sources[1])
	}
}

func TestRoomSubscriptionRegistryReplaysDurableEvents(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()

	senderA := newFakeRoomRegistrySender("sender-a")
	if err := registry.SubscribeRoom(ctx, senderA, "chat-1", "conv-1", nil); err != nil {
		t.Fatalf("首次 subscribe_room 失败: %v", err)
	}
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeRoundStatus, "conv-1"))

	senderB := newFakeRoomRegistrySender("sender-b")
	lastSeenRoomSeq := int64(1)
	if err := registry.SubscribeRoom(ctx, senderB, "chat-1", "conv-1", &lastSeenRoomSeq); err != nil {
		t.Fatalf("重连 subscribe_room 失败: %v", err)
	}

	event := readRoomRegistryEvent(t, senderB.events)
	if event.EventType != protocol.EventTypeRoundStatus {
		t.Fatalf("回放事件类型不正确: %+v", event)
	}
	if event.RoomSeq == nil || *event.RoomSeq != 2 {
		t.Fatalf("回放 room_seq 不正确: %+v", event)
	}
}

func TestRoomSubscriptionRegistryDoesNotReplayClientAck(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	clientAck := protocol.NewChatAckEvent(
		"room:group:conv-1",
		"request-1",
		"client-message-1",
		"root-1",
		"user-message-1",
		0,
		true,
		nil,
	)
	clientAck.ConversationID = "conv-1"
	registry.Broadcast(ctx, "chat-1", clientAck)

	sender := newFakeRoomRegistrySender("sender-client-ack")
	boundary := int64(0)
	if err := registry.SubscribeRoom(
		ctx,
		sender,
		"chat-1",
		"conv-1",
		&boundary,
	); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}
	assertNoRoomRegistryEvent(t, sender.events)
}

func TestRoomSubscriptionRegistryRequestsResyncWhenReplayBufferMissed(t *testing.T) {
	registry := newRoomSubscriptionRegistry(1)
	ctx := context.Background()

	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeRoundStatus, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))

	sender := newFakeRoomRegistrySender("sender-c")
	lastSeenRoomSeq := int64(1)
	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "conv-1", &lastSeenRoomSeq); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}

	event := readRoomRegistryEvent(t, sender.events)
	if event.EventType != protocol.EventTypeRoomResyncRequired {
		t.Fatalf("期望 room_resync_required，实际: %+v", event)
	}
	if event.Data["latest_room_seq"] != int64(3) && event.Data["latest_room_seq"] != float64(3) {
		t.Fatalf("latest_room_seq 不正确: %+v", event.Data)
	}
	if event.Data["buffer_start_room_seq"] != int64(3) && event.Data["buffer_start_room_seq"] != float64(3) {
		t.Fatalf("buffer_start_room_seq 不正确: %+v", event.Data)
	}
}

func TestRoomSubscriptionRegistryKeepsGlobalAndConversationSubscriptions(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	sender := newFakeRoomRegistrySender("sender-global")

	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "", nil); err != nil {
		t.Fatalf("全房间 subscribe_room 失败: %v", err)
	}
	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "conv-1", nil); err != nil {
		t.Fatalf("会话 subscribe_room 失败: %v", err)
	}

	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	event := readRoomRegistryEvent(t, sender.events)
	if event.ConversationID != "conv-2" {
		t.Fatalf("全房间订阅未收到其他会话事件: %+v", event)
	}
	assertNoRoomRegistryEvent(t, sender.events)

	registry.UnsubscribeRoom(sender, "chat-1", "conv-1")
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	event = readRoomRegistryEvent(t, sender.events)
	if event.ConversationID != "conv-2" {
		t.Fatalf("移除会话订阅后全房间订阅应继续生效: %+v", event)
	}

	registry.UnsubscribeRoom(sender, "chat-1", "")
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	assertNoRoomRegistryEvent(t, sender.events)
}

func durableRoomTestEvent(eventType protocol.EventType, conversationID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, map[string]any{
		"conversation_id": conversationID,
	})
	event.DeliveryMode = "durable"
	event.ConversationID = conversationID
	return event
}

func readRoomRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) protocol.EventMessage {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("等待 Room registry 事件超时")
		return protocol.EventMessage{}
	}
}

func assertNoRoomRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("不应收到 Room registry 事件: %+v", event)
	case <-time.After(80 * time.Millisecond):
	}
}
