package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type authorizationTestAgents struct {
	agent protocol.Agent
}

func (a authorizationTestAgents) GetAgent(
	_ context.Context,
	agentID string,
) (*protocol.Agent, error) {
	if strings.TrimSpace(agentID) != a.agent.AgentID {
		return nil, errors.New("agent not found")
	}
	value := a.agent
	return &value, nil
}

type authorizationTestRuntime struct {
	mu     sync.Mutex
	rounds map[string][]string
}

func (r *authorizationTestRuntime) GetRunningRoundIDs(
	sessionKey string,
) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.rounds[sessionKey]...)
}

func (r *authorizationTestRuntime) set(
	sessionKey string,
	roundIDs ...string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rounds == nil {
		r.rounds = make(map[string][]string)
	}
	r.rounds[sessionKey] = append([]string(nil), roundIDs...)
}

type authorizationTestVerifier struct {
	role string
}

func (v authorizationTestVerifier) VerifyInteractiveHuman(
	_ context.Context,
	principal *authctx.Principal,
) (*authctx.Principal, error) {
	if principal == nil {
		return nil, errors.New("missing human principal")
	}
	value := *principal
	return &value, nil
}

func (v authorizationTestVerifier) ResolveActivePrincipalRole(
	_ context.Context,
	_ string,
) (string, error) {
	return v.role, nil
}

type authorizationControlFixture struct {
	cfg       configForAuthorizationTest
	db        *sql.DB
	service   *Service
	control   *AuthorizationControl
	runtime   *authorizationTestRuntime
	principal *authctx.Principal
	actor     AuthorizationActor
	ctx       context.Context
}

// configForAuthorizationTest keeps the fixture declaration readable while
// retaining the concrete config through the embedded Service.
type configForAuthorizationTest struct {
	databaseURL string
}

func newAuthorizationControlFixture(
	t *testing.T,
	ownerUserID string,
) authorizationControlFixture {
	t.Helper()
	cfg := newConnectorsTestConfig(t)
	cfg.AppMode = "desktop"
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	sessionID := "human-session-1"
	principal := &authctx.Principal{
		UserID: ownerUserID, Username: "owner",
		Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
		SessionID: &sessionID,
	}
	sessionKey := protocol.BuildAgentSessionKey(
		"nexus", protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM, "main", "",
	)
	runtime := &authorizationTestRuntime{}
	runtime.set(sessionKey, "round-1")
	agents := authorizationTestAgents{agent: protocol.Agent{
		AgentID: "nexus", OwnerUserID: ownerUserID,
		IsMain: true, Name: "Nexus",
	}}
	verifier := authorizationTestVerifier{role: authctx.RoleOwner}
	service := NewService(cfg, db)
	control, err := NewAuthorizationControl(
		service, agents, runtime, verifier, verifier,
	)
	if err != nil {
		t.Fatal(err)
	}
	actor := AuthorizationActor{
		OwnerUserID: ownerUserID, AgentID: "nexus",
		BusinessSessionKey: sessionKey, RootRoundID: "round-1",
		RuntimeLeaseSessionKey: sessionKey,
		RuntimeLeaseRoundID:    "round-1",
		PrincipalUserID:        ownerUserID,
		PrincipalRole:          authctx.RoleOwner,
		AuthMethod:             authctx.AuthMethodPassword,
		AuthSessionID:          sessionID,
	}
	ctx := authctx.WithPrincipal(context.Background(), principal)
	return authorizationControlFixture{
		cfg: configForAuthorizationTest{databaseURL: cfg.DatabaseURL},
		db:  db, service: service, control: control,
		runtime: runtime, principal: principal, actor: actor, ctx: ctx,
	}
}

func approveAuthorizationStart(
	t *testing.T,
	fixture authorizationControlFixture,
	request AuthorizationStartRequest,
	expiresAt time.Time,
) {
	t.Helper()
	extras := make(map[string]any, len(request.Extras))
	for key, value := range request.Extras {
		extras[key] = value
	}
	input := map[string]any{
		"action":       ConnectorAuthorizationActionStart,
		"request_id":   request.RequestID,
		"connector_id": request.ConnectorID,
		"method":       request.Method,
	}
	if request.DeviceMode != "" {
		input["device_mode"] = string(request.DeviceMode)
	}
	if len(extras) > 0 {
		input["extras"] = extras
	}
	err := fixture.control.RecordHumanToolApproval(
		fixture.ctx,
		permissionctx.HumanToolApproval{
			PermissionRequestID: "perm-" + request.RequestID,
			ToolName:            "mcp__nexus__" + ConnectorAuthorizationToolName,
			ToolInput:           input,
			RuntimeSessionKey:   fixture.actor.RuntimeLeaseSessionKey,
			DispatchSessionKey:  fixture.actor.BusinessSessionKey,
			Route: permissionctx.RouteContext{
				DispatchSessionKey: fixture.actor.BusinessSessionKey,
				AgentID:            fixture.actor.AgentID,
				RoundID:            fixture.actor.RootRoundID,
			},
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		t.Fatalf("approve Connector authorization: %v", err)
	}
}

func TestAuthorizationOAuthFlowSurvivesRestartAndCompletesAcrossRound(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-oauth")
	tokenServer := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("code") != "callback-code" {
				t.Fatalf("unexpected OAuth exchange form: %v", request.Form)
			}
			_, _ = io.WriteString(
				writer,
				`{"access_token":"oauth-secret-token","refresh_token":"refresh"}`,
			)
		},
	))
	defer tokenServer.Close()
	t.Setenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL", tokenServer.URL)

	request := AuthorizationStartRequest{
		RequestID:   "oauth-restart-0001",
		ConnectorID: "github",
		Method:      AuthorizationMethodOAuthBrowser,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	startJSON, _ := json.Marshal(started)
	if started.FlowID == "" || started.AuthorizationURL == "" ||
		strings.Contains(string(startJSON), "state=") ||
		strings.Contains(string(startJSON), "oauth-secret-token") {
		t.Fatalf("Agent-facing OAuth start leaked provider secret: %s", startJSON)
	}
	var encrypted string
	if err = fixture.db.QueryRow(
		`SELECT secret_encrypted
		 FROM connector_authorization_flows WHERE flow_id = ?`,
		started.FlowID,
	).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if encrypted == "" || strings.Contains(encrypted, "state=") {
		t.Fatalf("provider authorization URL was not encrypted: %q", encrypted)
	}

	// Recreate both services to prove all continuation state is durable.
	cfg := newConnectorsTestConfig(t)
	cfg.DatabaseURL = fixture.cfg.databaseURL
	cfg.AppMode = "desktop"
	restartedService := NewService(cfg, fixture.db)
	restartedService.httpClient = tokenServer.Client()
	restartedRuntime := &authorizationTestRuntime{}
	restartedRuntime.set(fixture.actor.BusinessSessionKey, "round-2")
	agents := authorizationTestAgents{agent: protocol.Agent{
		AgentID: "nexus", OwnerUserID: fixture.actor.OwnerUserID,
		IsMain: true, Name: "Nexus",
	}}
	verifier := authorizationTestVerifier{role: authctx.RoleOwner}
	restartedControl, err := NewAuthorizationControl(
		restartedService, agents, restartedRuntime, verifier, verifier,
	)
	if err != nil {
		t.Fatal(err)
	}
	redirectActor, err := restartedControl.ResolveAuthorizationRedirectActor(
		fixture.ctx, started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	providerURL, err := restartedControl.GetAuthorizationRedirect(
		fixture.ctx, redirectActor, started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(providerURL)
	if err != nil || parsed.Query().Get("state") == "" {
		t.Fatalf("protected browser redirect missing provider state: %q", providerURL)
	}
	var callbackState string
	if err = fixture.db.QueryRow(
		`SELECT state FROM connector_oauth_states
		 WHERE control_flow_id = ?`,
		started.FlowID,
	).Scan(&callbackState); err != nil {
		t.Fatal(err)
	}
	if _, err = restartedService.CompleteOAuthCallback(
		context.Background(), "",
		OAuthCallbackRequest{
			Code: "callback-code", State: callbackState,
			RedirectURI: cfg.ConnectorOAuthRedirectURI,
		},
	); err != nil {
		t.Fatal(err)
	}
	nextActor := fixture.actor
	nextActor.RootRoundID = "round-2"
	nextActor.RuntimeLeaseRoundID = "round-2"
	status, err := restartedControl.Status(
		fixture.ctx, nextActor,
		AuthorizationFlowRef{
			FlowID: started.FlowID, ConnectorID: request.ConnectorID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != AuthorizationStatusConnected ||
		status.CurrentConfigurationVersion !=
			status.StartConfigurationVersion+1 {
		t.Fatalf("restarted OAuth flow not connected: %+v", status)
	}
	record, err := restartedControl.CompletionRecord(
		context.Background(), fixture.actor.OwnerUserID, started.FlowID,
	)
	if err != nil || record == nil ||
		record.Status != AuthorizationStatusConnected ||
		record.CompletedConfigurationVersion == nil {
		t.Fatalf("completion audit record invalid: record=%+v err=%v", record, err)
	}
}

func TestAuthorizationApprovalExpiresBeforeStart(t *testing.T) {
	fixture := newAuthorizationControlFixture(t, "owner-expiry")
	now := time.Now().UTC()
	fixture.control.now = func() time.Time { return now }
	request := AuthorizationStartRequest{
		RequestID:   "oauth-expiry-0001",
		ConnectorID: "github",
		Method:      AuthorizationMethodOAuthBrowser,
	}
	approveAuthorizationStart(
		t, fixture, request, now.Add(30*time.Second),
	)
	fixture.control.now = func() time.Time {
		return now.Add(time.Minute)
	}
	result, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AuthorizationStatusExpired ||
		result.AuthorizationURL != "" {
		t.Fatalf("expired approval should fail closed: %+v", result)
	}
}

func TestAuthorizationFlowRejectsCrossOwnerSessionAndConnector(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-boundary")
	deviceServer := newAuthorizationDeviceServer(t, false)
	defer deviceServer.Close()
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL",
		deviceServer.URL+"/device",
	)
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_TOKEN_URL",
		deviceServer.URL+"/token",
	)
	fixture.service.httpClient = deviceServer.Client()
	request := AuthorizationStartRequest{
		RequestID:   "device-boundary-0001",
		ConnectorID: "github", Method: AuthorizationMethodDevice,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	ref := AuthorizationFlowRef{
		FlowID: started.FlowID, ConnectorID: "github",
	}

	wrongOwner := fixture.actor
	wrongOwner.OwnerUserID = "other-owner"
	wrongOwner.PrincipalUserID = "other-owner"
	otherSessionID := "other-human-session"
	wrongOwner.AuthSessionID = otherSessionID
	wrongOwnerCtx := authctx.WithPrincipal(
		context.Background(),
		&authctx.Principal{
			UserID: "other-owner", Role: authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodPassword,
			SessionID:  &otherSessionID,
		},
	)
	if _, err = fixture.control.Status(
		wrongOwnerCtx, wrongOwner, ref,
	); err == nil {
		t.Fatal("cross-owner flow status must be rejected")
	}

	crossSession := fixture.actor
	crossSession.BusinessSessionKey = protocol.BuildAgentSessionKey(
		"nexus", protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM, "other", "",
	)
	crossSession.RuntimeLeaseSessionKey = crossSession.BusinessSessionKey
	fixture.runtime.set(crossSession.BusinessSessionKey, "round-other")
	crossSession.RootRoundID = "round-other"
	crossSession.RuntimeLeaseRoundID = "round-other"
	if _, err = fixture.control.Status(
		fixture.ctx, crossSession, ref,
	); err == nil || !strings.Contains(err.Error(), "不属于") {
		t.Fatalf("cross-session flow status must be rejected: %v", err)
	}

	if _, err = fixture.control.Status(
		fixture.ctx, fixture.actor,
		AuthorizationFlowRef{
			FlowID: started.FlowID, ConnectorID: "x-twitter",
		},
	); err == nil || !strings.Contains(err.Error(), "connector_id") {
		t.Fatalf("cross-connector flow status must be rejected: %v", err)
	}
}

func TestAuthorizationRedirectBindingRejectsCrossSessionExpiryAndTampering(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-browser-boundary")
	request := AuthorizationStartRequest{
		RequestID:   "oauth-browser-boundary-0001",
		ConnectorID: "github",
		Method:      AuthorizationMethodOAuthBrowser,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}

	otherSessionID := "other-browser-session"
	crossSessionContext := authctx.WithPrincipal(
		context.Background(),
		&authctx.Principal{
			UserID:     fixture.actor.OwnerUserID,
			Role:       authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodPassword,
			SessionID:  &otherSessionID,
		},
	)
	if _, err = fixture.control.ResolveAuthorizationRedirectActor(
		crossSessionContext, started.FlowID,
	); err == nil {
		t.Fatal("cross-session browser open must be rejected")
	}

	otherOwnerSessionID := "other-owner-session"
	otherOwnerContext := authctx.WithPrincipal(
		context.Background(),
		&authctx.Principal{
			UserID:     "other-owner",
			Role:       authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodPassword,
			SessionID:  &otherOwnerSessionID,
		},
	)
	if _, err = fixture.control.ResolveAuthorizationRedirectActor(
		otherOwnerContext, started.FlowID,
	); err == nil {
		t.Fatal("cross-owner browser open must be rejected")
	}

	redirectActor, err := fixture.control.ResolveAuthorizationRedirectActor(
		fixture.ctx, started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*AuthorizationActor){
		func(actor *AuthorizationActor) {
			actor.BusinessSessionKey = protocol.BuildAgentSessionKey(
				actor.AgentID,
				protocol.SessionChannelWebSocketSegment,
				protocol.RoomTypeDM,
				"other",
				"",
			)
		},
		func(actor *AuthorizationActor) {
			actor.RootRoundID = "other-root-round"
		},
		func(actor *AuthorizationActor) {
			actor.RuntimeLeaseSessionKey = "forged-runtime-session"
		},
		func(actor *AuthorizationActor) {
			actor.RuntimeLeaseRoundID = "forged-runtime-round"
		},
	} {
		tampered := redirectActor
		mutate(&tampered)
		if _, err = fixture.control.GetAuthorizationRedirect(
			fixture.ctx, tampered, started.FlowID,
		); err == nil {
			t.Fatalf("tampered browser actor was accepted: %+v", tampered)
		}
	}

	fixture.control.now = func() time.Time {
		return time.Now().UTC().Add(24 * time.Hour)
	}
	if _, err = fixture.control.ResolveAuthorizationRedirectActor(
		fixture.ctx, started.FlowID,
	); err == nil {
		t.Fatal("expired browser-open flow must be rejected")
	}
}

func TestAuthorizationDeviceFlowFailsClosedOnConnectorCASConflict(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-conflict")
	deviceServer := newAuthorizationDeviceServer(t, false)
	defer deviceServer.Close()
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL",
		deviceServer.URL+"/device",
	)
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_TOKEN_URL",
		deviceServer.URL+"/token",
	)
	fixture.service.httpClient = deviceServer.Client()
	request := AuthorizationStartRequest{
		RequestID:   "device-conflict-0001",
		ConnectorID: "github", Method: AuthorizationMethodDevice,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = fixture.service.upsertConnection(
		fixture.ctx,
		connectionRecord{
			OwnerUserID: fixture.actor.OwnerUserID,
			ConnectorID: "github", State: "disconnected",
			AuthType: "oauth2",
		},
	); err != nil {
		t.Fatal(err)
	}
	status, err := fixture.control.Status(
		fixture.ctx, fixture.actor,
		AuthorizationFlowRef{
			FlowID: started.FlowID, ConnectorID: "github",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != AuthorizationStatusConflict ||
		status.UserCode != "" ||
		status.VerificationURI != "" ||
		status.VerificationURIComplete != "" {
		t.Fatalf("CAS conflict did not fail closed: %+v", status)
	}
}

func TestAuthorizationCancelErasesHumanAuthorizationMaterial(t *testing.T) {
	fixture := newAuthorizationControlFixture(t, "owner-device-cancel")
	deviceServer := newAuthorizationDeviceServer(t, false)
	defer deviceServer.Close()
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL",
		deviceServer.URL+"/device",
	)
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_TOKEN_URL",
		deviceServer.URL+"/token",
	)
	fixture.service.httpClient = deviceServer.Client()
	request := AuthorizationStartRequest{
		RequestID:   "device-cancel-0001",
		ConnectorID: "github",
		Method:      AuthorizationMethodDevice,
	}
	approveAuthorizationStart(
		t,
		fixture,
		request,
		time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx,
		fixture.actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if started.UserCode == "" || started.VerificationURI == "" {
		t.Fatalf("active Device Flow missing human authorization material: %+v", started)
	}

	cancelled, err := fixture.control.Cancel(
		fixture.ctx,
		fixture.actor,
		AuthorizationFlowRef{
			FlowID:      started.FlowID,
			ConnectorID: request.ConnectorID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != AuthorizationStatusCanceled ||
		cancelled.UserCode != "" ||
		cancelled.VerificationURI != "" ||
		cancelled.VerificationURIComplete != "" {
		t.Fatalf("canceled flow exposed human authorization material: %+v", cancelled)
	}

	var (
		secret                        string
		publicUserCode                string
		publicVerificationURI         string
		publicVerificationURIComplete string
		publicOpenPath                string
		nextPollAt                    sql.NullTime
	)
	if err = fixture.db.QueryRow(
		`SELECT secret_encrypted, public_user_code, public_verification_uri,
		        public_verification_uri_complete, public_open_path, next_poll_at
		 FROM connector_authorization_flows WHERE flow_id = ?`,
		started.FlowID,
	).Scan(
		&secret,
		&publicUserCode,
		&publicVerificationURI,
		&publicVerificationURIComplete,
		&publicOpenPath,
		&nextPollAt,
	); err != nil {
		t.Fatal(err)
	}
	if secret != "" ||
		publicUserCode != "" ||
		publicVerificationURI != "" ||
		publicVerificationURIComplete != "" ||
		publicOpenPath != "" ||
		nextPollAt.Valid {
		t.Fatalf(
			"canceled flow retained temporary authorization material: "+
				"secret=%q code=%q uri=%q complete=%q open=%q next_poll=%v",
			secret,
			publicUserCode,
			publicVerificationURI,
			publicVerificationURIComplete,
			publicOpenPath,
			nextPollAt,
		)
	}
}

func TestAuthorizationDeviceCompletionUsesCASAndErasesSecret(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-device-complete")
	deviceServer := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/device":
				_, _ = io.WriteString(
					writer,
					`{"device_code":"completion-device-secret",`+
						`"user_code":"SAFE-5678",`+
						`"verification_uri":"https://github.com/login/device",`+
						`"expires_in":900,"interval":1}`,
				)
			case "/token":
				_, _ = io.WriteString(
					writer,
					`{"access_token":"completion-token-secret","scope":"repo"}`,
				)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer deviceServer.Close()
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL",
		deviceServer.URL+"/device",
	)
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_TOKEN_URL",
		deviceServer.URL+"/token",
	)
	fixture.service.httpClient = deviceServer.Client()
	request := AuthorizationStartRequest{
		RequestID:   "device-complete-0001",
		ConnectorID: "github", Method: AuthorizationMethodDevice,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.control.now = func() time.Time {
		return time.Now().UTC().Add(10 * time.Second)
	}
	status, err := fixture.control.Status(
		fixture.ctx, fixture.actor,
		AuthorizationFlowRef{
			FlowID: started.FlowID, ConnectorID: "github",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(status)
	if status.Status != AuthorizationStatusConnected ||
		status.UserCode != "" ||
		status.VerificationURI != "" ||
		status.VerificationURIComplete != "" ||
		strings.Contains(string(encoded), "completion-device-secret") ||
		strings.Contains(string(encoded), "SAFE-5678") ||
		strings.Contains(string(encoded), "completion-token-secret") {
		t.Fatalf("device completion result invalid or leaked: %s", encoded)
	}
	connection, err := fixture.service.LoadActiveConnection(
		fixture.ctx, fixture.actor.OwnerUserID, "github",
	)
	if err != nil || connection == nil ||
		connection.AccessToken != "completion-token-secret" {
		t.Fatalf("device completion was not committed: value=%+v err=%v", connection, err)
	}
	var (
		flowSecret                    string
		publicUserCode                string
		publicVerificationURI         string
		publicVerificationURIComplete string
		publicOpenPath                string
		nextPollAt                    sql.NullTime
	)
	if err = fixture.db.QueryRow(
		`SELECT secret_encrypted, public_user_code, public_verification_uri,
		        public_verification_uri_complete, public_open_path, next_poll_at
		 FROM connector_authorization_flows WHERE flow_id = ?`,
		started.FlowID,
	).Scan(
		&flowSecret,
		&publicUserCode,
		&publicVerificationURI,
		&publicVerificationURIComplete,
		&publicOpenPath,
		&nextPollAt,
	); err != nil {
		t.Fatal(err)
	}
	if flowSecret != "" ||
		publicUserCode != "" ||
		publicVerificationURI != "" ||
		publicVerificationURIComplete != "" ||
		publicOpenPath != "" ||
		nextPollAt.Valid {
		t.Fatalf(
			"completed flow retained temporary authorization material: "+
				"secret=%q code=%q uri=%q complete=%q open=%q next_poll=%v",
			flowSecret,
			publicUserCode,
			publicVerificationURI,
			publicVerificationURIComplete,
			publicOpenPath,
			nextPollAt,
		)
	}
}

func TestAuthorizationOAuthCallbackRevalidatesHumanAndMainAuthority(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-revalidate")
	request := AuthorizationStartRequest{
		RequestID:   "oauth-revalidate-0001",
		ConnectorID: "github",
		Method:      AuthorizationMethodOAuthBrowser,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	redirectActor, err := fixture.control.ResolveAuthorizationRedirectActor(
		fixture.ctx, started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.control.GetAuthorizationRedirect(
		fixture.ctx, redirectActor, started.FlowID,
	); err != nil {
		t.Fatal(err)
	}
	var callbackState string
	if err = fixture.db.QueryRow(
		`SELECT state FROM connector_oauth_states
		 WHERE control_flow_id = ?`,
		started.FlowID,
	).Scan(&callbackState); err != nil {
		t.Fatal(err)
	}
	fixture.control.roleResolver = authorizationTestVerifier{
		role: authctx.RoleAdmin,
	}
	if _, err = fixture.service.CompleteOAuthCallback(
		context.Background(), "",
		OAuthCallbackRequest{
			Code: "must-not-exchange", State: callbackState,
			RedirectURI: fixture.service.config.ConnectorOAuthRedirectURI,
		},
	); err == nil || !strings.Contains(err.Error(), "角色已变化") {
		t.Fatalf("callback must reject changed human authority: %v", err)
	}
	record, err := fixture.control.CompletionRecord(
		context.Background(), fixture.actor.OwnerUserID, started.FlowID,
	)
	if err != nil || record.Status != AuthorizationStatusFailed {
		t.Fatalf("revoked callback did not persist fail-closed status: %+v %v", record, err)
	}
}

func TestAuthorizationDeviceSecretsNeverReachModelOrError(
	t *testing.T,
) {
	fixture := newAuthorizationControlFixture(t, "owner-secrets")
	deviceServer := newAuthorizationDeviceServer(t, true)
	defer deviceServer.Close()
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL",
		deviceServer.URL+"/device",
	)
	t.Setenv(
		"NEXUS_CONNECTOR_GITHUB_TOKEN_URL",
		deviceServer.URL+"/token",
	)
	fixture.service.httpClient = deviceServer.Client()
	request := AuthorizationStartRequest{
		RequestID:   "device-secret-0001",
		ConnectorID: "github", Method: AuthorizationMethodDevice,
	}
	approveAuthorizationStart(
		t, fixture, request, time.Now().Add(time.Minute),
	)
	started, err := fixture.control.Start(
		fixture.ctx, fixture.actor, request,
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(started)
	if strings.Contains(string(encoded), "provider-device-secret") ||
		strings.Contains(string(encoded), "provider-error-secret-token") {
		t.Fatalf("device secret leaked in start result: %s", encoded)
	}
	var storedSecret string
	if err = fixture.db.QueryRow(
		`SELECT secret_encrypted
		 FROM connector_authorization_flows WHERE flow_id = ?`,
		started.FlowID,
	).Scan(&storedSecret); err != nil {
		t.Fatal(err)
	}
	if storedSecret == "" ||
		strings.Contains(storedSecret, "provider-device-secret") {
		t.Fatalf("device code was not encrypted: %q", storedSecret)
	}
	fixture.control.now = func() time.Time {
		return time.Now().UTC().Add(10 * time.Second)
	}
	_, err = fixture.control.Status(
		fixture.ctx, fixture.actor,
		AuthorizationFlowRef{
			FlowID: started.FlowID, ConnectorID: "github",
		},
	)
	if err == nil ||
		strings.Contains(err.Error(), "provider-error-secret-token") {
		t.Fatalf("provider error secret leaked: %v", err)
	}
}

func newAuthorizationDeviceServer(
	t *testing.T,
	failToken bool,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/device":
				_, _ = io.WriteString(
					writer,
					`{"device_code":"provider-device-secret",`+
						`"user_code":"SAFE-1234",`+
						`"verification_uri":"https://github.com/login/device",`+
						`"expires_in":900,"interval":1}`,
				)
			case "/token":
				if failToken {
					http.Error(
						writer,
						`{"error":"server_error",`+
							`"error_description":"provider-error-secret-token"}`,
						http.StatusInternalServerError,
					)
					return
				}
				_, _ = io.WriteString(
					writer,
					`{"error":"authorization_pending"}`,
				)
			default:
				http.NotFound(writer, request)
			}
		},
	))
}
