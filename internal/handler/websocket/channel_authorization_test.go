package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
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
