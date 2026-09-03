package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"

	"github.com/go-chi/chi/v5"
)

type connectorAuthorizationAppControl struct {
	lastMCPActor             connectorsvc.AuthorizationActor
	redirectActor            connectorsvc.AuthorizationActor
	lastRedirectActor        connectorsvc.AuthorizationActor
	resolveFlowID            string
	redirectFlowID           string
	redirectURL              string
	resolveErr               error
	redirectErr              error
	expectedBrowserSessionID string
}

func (*connectorAuthorizationAppControl) Start(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationStartRequest,
) (*connectorsvc.AuthorizationFlowView, error) {
	return &connectorsvc.AuthorizationFlowView{}, nil
}

func (c *connectorAuthorizationAppControl) Status(
	_ context.Context,
	actor connectorsvc.AuthorizationActor,
	_ connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
	c.lastMCPActor = actor
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Status: connectorsvc.AuthorizationStatusPending,
	}, nil
}

func (*connectorAuthorizationAppControl) Cancel(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
	return &connectorsvc.AuthorizationFlowView{}, nil
}

func (c *connectorAuthorizationAppControl) ResolveAuthorizationRedirectActor(
	ctx context.Context,
	flowID string,
) (connectorsvc.AuthorizationActor, error) {
	c.resolveFlowID = flowID
	if c.resolveErr != nil {
		return connectorsvc.AuthorizationActor{}, c.resolveErr
	}
	if c.expectedBrowserSessionID != "" {
		principal := authctx.PrincipalFromContext(ctx)
		sessionID := ""
		if principal != nil && principal.SessionID != nil {
			sessionID = strings.TrimSpace(*principal.SessionID)
		}
		if sessionID != c.expectedBrowserSessionID {
			return connectorsvc.AuthorizationActor{}, errors.New(
				"cross-session provider-secret-token",
			)
		}
	}
	return c.redirectActor, nil
}

func (c *connectorAuthorizationAppControl) GetAuthorizationRedirect(
	_ context.Context,
	actor connectorsvc.AuthorizationActor,
	flowID string,
) (string, error) {
	c.lastRedirectActor = actor
	c.redirectFlowID = flowID
	if c.redirectErr != nil {
		return "", c.redirectErr
	}
	return c.redirectURL, nil
}

func TestConnectorAuthorizationMCPBuilderBindsOwnerMainPrivateDM(
	t *testing.T,
) {
	sessionID := "browser-session-a"
	principal := &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
		SessionID:  &sessionID,
	}
	sessionKey := protocol.BuildAgentSessionKey(
		"nexus",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	ctx := authctx.WithPrincipal(context.Background(), principal)
	ctx = runtimectx.WithRuntimeRoundLease(ctx, sessionKey, "round-a")
	control := &connectorAuthorizationAppControl{}
	builder := NewConnectorAuthorizationToolBuilder(
		control,
		stubRuntimeAgentResolver{record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner-a", IsMain: true,
		}},
	)
	tools := builder(ctx, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
	))
	if len(tools) == 0 {
		t.Fatal("owner main DM missing Connector authorization tools")
	}
	server := sdktool.NewSimpleSDKMCPServer(nexusMCPServerName, "1.0.0", tools)
	response, err := server.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": connectorsvc.ConnectorAuthorizationToolName,
			"arguments": map[string]any{
				"action": "status", "flow_id": "caf_safe", "connector_id": "github",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	if result == nil || result["isError"] == true {
		t.Fatalf("status response = %+v", response)
	}
	got := control.lastMCPActor
	if got.OwnerUserID != "owner-a" ||
		got.AgentID != "nexus" ||
		got.BusinessSessionKey != sessionKey ||
		got.RootRoundID != "round-a" ||
		got.RuntimeLeaseSessionKey != sessionKey ||
		got.RuntimeLeaseRoundID != "round-a" ||
		got.PrincipalUserID != "owner-a" ||
		got.PrincipalRole != authctx.RoleOwner ||
		got.AuthMethod != authctx.AuthMethodPassword ||
		got.AuthSessionID != sessionID {
		t.Fatalf("Connector authorization actor not fully bound: %+v", got)
	}

	ordinaryBuilder := NewConnectorAuthorizationToolBuilder(
		control,
		stubRuntimeAgentResolver{record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner-a", IsMain: false,
		}},
	)
	if gotTools := ordinaryBuilder(ctx, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
	)); len(gotTools) != 0 {
		t.Fatalf("ordinary Agent received authorization tools: %+v", gotTools)
	}
	assertDeniedSurface := func(
		name string,
		callContext context.Context,
		gotTools []sdktool.Tool,
	) {
		t.Helper()
		if len(gotTools) == 0 {
			t.Fatalf("%s changed Connector authorization surface", name)
		}
		control.lastMCPActor = connectorsvc.AuthorizationActor{}
		deniedServer := sdktool.NewSimpleSDKMCPServer(nexusMCPServerName, "1.0.0", gotTools)
		response, callErr := deniedServer.HandleMessage(callContext, map[string]any{
			"jsonrpc": "2.0", "id": 2, "method": "tools/call",
			"params": map[string]any{
				"name": connectorsvc.ConnectorAuthorizationToolName,
				"arguments": map[string]any{
					"action": "status", "flow_id": "caf_safe", "connector_id": "github",
				},
			},
		})
		if callErr != nil {
			t.Fatal(callErr)
		}
		if result, _ := response["result"].(map[string]any); result == nil || result["isError"] == true {
			t.Fatalf("%s status response = %+v", name, response)
		}
		if control.lastMCPActor.PrincipalUserID != "" || control.lastMCPActor.AuthMethod != "" {
			t.Fatalf("%s received Connector authorization authority: %+v", name, control.lastMCPActor)
		}
	}
	assertDeniedSurface("room source", ctx, builder(ctx, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "room", "room-a",
	)))
	mismatchedLease := runtimectx.WithRuntimeRoundLease(
		authctx.WithPrincipal(context.Background(), principal),
		sessionKey,
		"other-round",
	)
	assertDeniedSurface("mismatched lease", mismatchedLease, builder(mismatchedLease, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
	)))
	bearer := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner,
		AuthMethod: "bearer",
	})
	bearer = runtimectx.WithRuntimeRoundLease(bearer, sessionKey, "round-a")
	assertDeniedSurface("bearer principal", bearer, builder(bearer, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
	)))
}

func TestConnectorAuthorizationOpenRouteUsesOnlyBoundFlowIdentity(
	t *testing.T,
) {
	sessionID := "browser-session-a"
	actor := connectorsvc.AuthorizationActor{
		OwnerUserID: "owner-a", AgentID: "nexus",
		BusinessSessionKey:     "agent:nexus:websocket:dm:main",
		RootRoundID:            "round-a",
		RuntimeLeaseSessionKey: "agent:nexus:websocket:dm:main",
		RuntimeLeaseRoundID:    "round-a",
		PrincipalUserID:        "owner-a",
		PrincipalRole:          authctx.RoleOwner,
		AuthMethod:             authctx.AuthMethodPassword,
		AuthSessionID:          sessionID,
	}
	control := &connectorAuthorizationAppControl{
		redirectActor:            actor,
		redirectURL:              "https://provider.example/authorize?state=provider-secret",
		expectedBrowserSessionID: sessionID,
	}
	router := chi.NewRouter()
	MountConnectorAuthorizationRoutes(
		router,
		func(path string) string { return "/nexus/v1" + path },
		control,
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/connectors/authorization-flows/caf_safe/open"+
			"?owner_user_id=attacker&agent_id=worker&session_key=forged",
		nil,
	)
	request = request.WithContext(authctx.WithPrincipal(
		request.Context(),
		&authctx.Principal{
			UserID: "owner-a", Role: authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodPassword,
			SessionID:  &sessionID,
		},
	))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("open status = %d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != control.redirectURL ||
		response.Header().Get("Cache-Control") != "no-store" ||
		response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("open response headers = %+v", response.Header())
	}
	if response.Body.Len() != 0 {
		t.Fatalf("redirect body must not duplicate provider URL: %q", response.Body.String())
	}
	if control.resolveFlowID != "caf_safe" ||
		control.redirectFlowID != "caf_safe" ||
		control.lastRedirectActor != actor {
		t.Fatalf(
			"URL parameters altered bound actor: resolve=%q redirect=%q actor=%+v",
			control.resolveFlowID,
			control.redirectFlowID,
			control.lastRedirectActor,
		)
	}
}

func TestConnectorAuthorizationOpenRouteFailsClosedWithoutLeaking(
	t *testing.T,
) {
	sessionID := "browser-session-a"
	principal := &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
		SessionID:  &sessionID,
	}
	for _, test := range []struct {
		name       string
		control    *connectorAuthorizationAppControl
		principal  *authctx.Principal
		wantStatus int
	}{
		{
			name: "expired",
			control: &connectorAuthorizationAppControl{
				resolveErr: errors.New(
					"expired state=provider-secret token=provider-token",
				),
			},
			principal: principal, wantStatus: http.StatusNotFound,
		},
		{
			name: "cross session",
			control: &connectorAuthorizationAppControl{
				expectedBrowserSessionID: "different-session",
			},
			principal: principal, wantStatus: http.StatusNotFound,
		},
		{
			name: "redirect failure",
			control: &connectorAuthorizationAppControl{
				redirectActor: connectorsvc.AuthorizationActor{
					OwnerUserID: "owner-a",
				},
				redirectErr: errors.New(
					"provider URL https://provider.example/?state=secret",
				),
			},
			principal: principal, wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthenticated",
			control:    &connectorAuthorizationAppControl{},
			wantStatus: http.StatusUnauthorized,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			MountConnectorAuthorizationRoutes(
				router, func(path string) string { return path }, test.control,
			)
			request := httptest.NewRequest(
				http.MethodGet,
				"/connectors/authorization-flows/caf_safe/open",
				nil,
			)
			if test.principal != nil {
				request = request.WithContext(authctx.WithPrincipal(
					request.Context(), test.principal,
				))
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d body=%q",
					response.Code,
					test.wantStatus,
					response.Body.String(),
				)
			}
			body := response.Body.String()
			for _, secret := range []string{
				"provider-secret", "provider-token", "state=secret",
				"https://provider.example",
			} {
				if strings.Contains(body, secret) ||
					strings.Contains(response.Header().Get("Location"), secret) {
					t.Fatalf("failure leaked %q: headers=%v body=%q", secret, response.Header(), body)
				}
			}
			if response.Header().Get("Cache-Control") != "no-store" ||
				response.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatalf("failure security headers = %+v", response.Header())
			}
		})
	}
}
