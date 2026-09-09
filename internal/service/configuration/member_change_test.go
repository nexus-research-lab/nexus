package configuration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	appruntime "github.com/nexus-research-lab/nexus/internal/app/runtime"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memberTestControl struct {
	configurationTestPrincipalVerifier
	members []authsvc.ControlMember
	writes  int
	role    string
}

func (c *memberTestControl) ResolveActivePrincipalRole(context.Context, string) (string, error) {
	return c.role, nil
}
func (c *memberTestControl) ManageMembers(_ context.Context, owner, session, operation, target string, input json.RawMessage, version int64) (json.RawMessage, error) {
	if session != "admin-session" || owner == "" {
		return nil, errors.New("missing trusted binding")
	}
	if operation == "list" {
		return json.Marshal(c.members)
	}
	c.writes++
	var value authsvc.CreateControlMemberInput
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, err
	}
	if value.Password != "human-password" {
		return nil, errors.New("secret not materialized")
	}
	member := authsvc.ControlMember{UserID: "control-member", Username: value.Username, DisplayName: value.DisplayName, Role: value.Role, MembershipStatus: "active", UpdatedAt: time.Now().UTC()}
	c.members = append(c.members, member)
	return json.Marshal(member)
}

func TestMemberConfigurationRequiresHumanSecretAndLiveAdmin(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	control := &memberTestControl{role: authctx.RoleOwner, members: []authsvc.ControlMember{}}
	fixture.services.Configuration.SetPrincipalVerifiers(control, control)
	session := "admin-session"
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: fixture.main.OwnerUserID, Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword, SessionID: &session})
	actor := configurationsvc.Actor{OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID, SessionKey: "agent:" + fixture.main.AgentID + ":ws:dm:members", ContextKind: configurationsvc.ContextKindAgent, ContextID: fixture.main.AgentID, PrincipalRole: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword, AuthSessionID: session}
	bindConfigurationTestRound(t, fixture.services, &actor)
	request := configurationsvc.ChangeRequest{RequestID: "members-create-001", Domain: configurationsvc.DomainMembers, Operation: "create", Input: json.RawMessage(`{"username":"new-user","display_name":"New","role":"member","password":{"$secret":"member-password"}}`)}
	plan, err := fixture.services.Configuration.PlanChange(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresConfirmation || len(plan.SecretSlots) != 1 {
		t.Fatal("missing human password approval")
	}
	request.PlanDigest = plan.PlanDigest
	request.ExpectedRevision = plan.CurrentRevision
	if _, err = fixture.services.Configuration.ApplyChangeFromCLI(ctx, actor, request, configurationsvc.CLIApplyOptions{Confirmed: true}); err == nil {
		t.Fatal("CLI confirmation bypassed human card")
	}
	normalizedBypass := request
	normalizedBypass.Domain = " MEMBERS "
	if _, bypassErr := fixture.services.Configuration.ApplyChangeFromCLI(ctx, actor, normalizedBypass, configurationsvc.CLIApplyOptions{Confirmed: true}); bypassErr == nil || !strings.Contains(bypassErr.Error(), "实时确认卡片") {
		t.Fatalf("normalized domain bypass: %v", bypassErr)
	}
	if _, err = fixture.services.Configuration.ApplyChange(ctx, actor, request); err == nil {
		t.Fatal("missing approval accepted")
	}
	token, err := fixture.services.Configuration.IssueRuntimeCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	sender := &memberTestSender{events: make(chan protocol.EventMessage, 8)}
	fixture.services.Permission.BindSession(actor.SessionKey, sender)
	lease := fixture.services.Permission.BindSessionRoute(actor.LeaseSessionKey, permissionctx.RouteContext{DispatchSessionKey: actor.SessionKey, AgentID: actor.AgentID, RoundID: actor.RoundID, AgentRoundID: actor.LeaseRoundID})
	defer fixture.services.Permission.UnbindSessionRoute(lease)
	body, _ := json.Marshal(map[string]any{"action": "apply", "change": request, "confirmed": true})
	httpRequest := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/internal/runtime/configuration", bytes.NewReader(body)).WithContext(ctx)
	httpRequest.RemoteAddr = "127.0.0.1:1234"
	httpRequest.Header.Set(protocol.NexusConfigCapabilityHeader, token)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		appruntime.NewConfigurationHandler(fixture.services.Configuration, fixture.services.Permission)(response, httpRequest)
	}()
	select {
	case event := <-sender.events:
		if event.EventType != protocol.EventTypePermissionRequest {
			t.Fatalf("unexpected event: %s", event.EventType)
		}
		if !fixture.services.Permission.HandlePermissionResponse(ctx, actor.SessionKey, map[string]any{"request_id": event.Data["request_id"], "decision": "allow", "configuration_secrets": map[string]any{"member-password": "human-password"}}) {
			t.Fatal("approval rejected")
		}
	case <-done:
		t.Fatalf("broker did not wait for approval: %s", response.Body.String())
	case <-time.After(10 * time.Second):
		t.Fatal("missing approval event")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("broker did not resume")
	}
	if response.Code != http.StatusOK {
		t.Fatalf("broker apply: %d %s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("human-password")) {
		t.Fatal("response leaked password")
	}
	if control.writes != 1 {
		t.Fatalf("writes=%d", control.writes)
	}
	if _, err = fixture.services.Configuration.ApplyChange(ctx, actor, request); err != nil {
		t.Fatal(err)
	}
	if control.writes != 1 {
		t.Fatal("replay duplicated write")
	}
	control.role = authctx.RoleMember
	if _, err = fixture.services.Configuration.Inspect(ctx, actor, []string{configurationsvc.DomainMembers}, false); err == nil {
		t.Fatal("demoted user retained member access")
	}
}

type memberTestSender struct{ events chan protocol.EventMessage }

func (*memberTestSender) Key() string    { return "member-test" }
func (*memberTestSender) IsClosed() bool { return false }
func (s *memberTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}
