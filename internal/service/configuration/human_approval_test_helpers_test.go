package configuration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type configurationTestPrincipalVerifier struct{}

func (configurationTestPrincipalVerifier) VerifyInteractiveHuman(
	_ context.Context,
	principal *authctx.Principal,
) (*authctx.Principal, error) {
	if principal == nil {
		return nil, fmt.Errorf("missing test principal")
	}
	copyPrincipal := *principal
	return &copyPrincipal, nil
}

func (configurationTestPrincipalVerifier) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, func(), error) {
	principal := authctx.PrincipalFromContext(ctx)
	if principal == nil {
		principal = &authctx.Principal{
			UserID: userID, Role: authctx.RoleOwner, AuthMethod: authMethod,
		}
		if strings.TrimSpace(sessionID) != "" {
			value := strings.TrimSpace(sessionID)
			principal.SessionID = &value
		}
	}
	actualSessionID := ""
	if principal.SessionID != nil {
		actualSessionID = strings.TrimSpace(*principal.SessionID)
	}
	if principal.UserID != userID ||
		principal.AuthMethod != authMethod ||
		actualSessionID != strings.TrimSpace(sessionID) {
		return nil, nil, fmt.Errorf("test principal lease binding mismatch")
	}
	copyPrincipal := *principal
	return &copyPrincipal, func() {}, nil
}

func (configurationTestPrincipalVerifier) ResolveActivePrincipalRole(
	_ context.Context,
	_ string,
) (string, error) {
	return authctx.RoleOwner, nil
}

func enableConfigurationTestPrincipalVerification(services *server.AppServices) {
	verifier := configurationTestPrincipalVerifier{}
	services.Configuration.SetPrincipalVerifiers(verifier, verifier)
}

func bindConfigurationTestRound(
	t *testing.T,
	services *server.AppServices,
	actor *configurationsvc.Actor,
) {
	t.Helper()
	if actor == nil {
		t.Fatal("nil configuration actor")
	}
	if strings.TrimSpace(actor.SessionKey) == "" {
		t.Fatal("configuration test actor requires session key")
	}
	switch actor.ContextKind {
	case configurationsvc.ContextKindRoom:
		roomID := strings.TrimSpace(actor.RoomID)
		if roomID == "" {
			roomID = strings.TrimSpace(actor.ContextID)
		}
		actor.SourceContext = configurationsvc.ContextKindRoom + ":" + roomID
	case configurationsvc.ContextKindAgent:
		actor.SourceContext = configurationsvc.ContextKindAgent + ":" + strings.TrimSpace(actor.AgentID)
	}
	actor.RoundID = "round-" + strings.NewReplacer(":", "-", "/", "-").Replace(actor.SessionKey)
	if strings.TrimSpace(actor.LeaseSessionKey) == "" {
		actor.LeaseSessionKey = actor.SessionKey
	}
	if actor.ContextKind == configurationsvc.ContextKindRoom {
		actor.LeaseRoundID = "agent-" + actor.RoundID
	} else {
		actor.LeaseRoundID = actor.RoundID
	}
	actor.RoundLeaseRequired = true
	if err := services.Runtime.StartRound(
		t.Context(),
		actor.LeaseSessionKey,
		actor.LeaseRoundID,
		nil,
	); err != nil {
		t.Fatalf("start configuration test round %s: %v", actor.LeaseRoundID, err)
	}
	t.Cleanup(func() {
		services.Runtime.MarkRoundFinished(actor.LeaseSessionKey, actor.LeaseRoundID)
	})
}

func approveConfigurationTestChange(
	t *testing.T,
	services *server.AppServices,
	ctx context.Context,
	actor configurationsvc.Actor,
	request configurationsvc.ChangeRequest,
	plan *configurationsvc.ChangePlan,
) {
	approveConfigurationTestChangeWithSecrets(
		t,
		services,
		ctx,
		actor,
		request,
		plan,
		nil,
	)
}

func approveConfigurationTestChangeWithSecrets(
	t *testing.T,
	services *server.AppServices,
	ctx context.Context,
	actor configurationsvc.Actor,
	request configurationsvc.ChangeRequest,
	plan *configurationsvc.ChangePlan,
	secrets map[string]string,
) {
	t.Helper()
	if plan == nil || !plan.RequiresConfirmation {
		return
	}
	if authctx.PrincipalFromContext(ctx) == nil {
		role := strings.TrimSpace(actor.PrincipalRole)
		if role == "" {
			role = authctx.RoleOwner
		}
		authMethod := strings.TrimSpace(actor.AuthMethod)
		if authMethod == "" {
			authMethod = authctx.AuthMethodLocal
		}
		ctx = authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: actor.OwnerUserID, Username: actor.AgentID,
			Role: role, AuthMethod: authMethod,
		})
	}
	var input any = map[string]any{}
	if len(request.Input) > 0 {
		if err := json.Unmarshal(request.Input, &input); err != nil {
			t.Fatalf("decode configuration approval input: %v", err)
		}
	}
	toolInput := map[string]any{
		"request_id":        request.RequestID,
		"domain":            request.Domain,
		"operation":         request.Operation,
		"target":            request.Target,
		"input":             input,
		"expected_revision": request.ExpectedRevision,
		"plan_digest":       request.PlanDigest,
	}
	route := permissionctx.RouteContext{
		DispatchSessionKey: actor.SessionKey,
		AgentID:            actor.AgentID,
		RoundID:            actor.RoundID,
		AgentRoundID:       actor.LeaseRoundID,
		RoomID:             actor.RoomID,
		ConversationID:     actor.ConversationID,
	}
	if err := services.Configuration.RecordHumanToolApproval(
		ctx,
		permissionctx.HumanToolApproval{
			PermissionRequestID:      "perm-" + request.RequestID,
			ToolName:                 "mcp__nexus_config__apply_nexus_configuration_change",
			ToolInput:                toolInput,
			ConfigurationSecrets:     secrets,
			ConfigurationSecretSlots: plan.SecretSlots,
			RuntimeSessionKey:        actor.LeaseSessionKey,
			DispatchSessionKey:       actor.SessionKey,
			Route:                    route,
			ExpiresAt:                time.Now().Add(time.Minute),
		},
	); err != nil {
		t.Fatalf("record configuration human approval: %v", err)
	}
}
