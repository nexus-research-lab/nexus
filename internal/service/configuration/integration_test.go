// INPUT: 完整 AppServices、owner-main Actor 与各配置域 plan/apply 请求。
// OUTPUT: 配置控制面跨服务持久化、CAS、审计、脱敏和写后核对证明。
// POS: nexuscfg 真实装配的端到端后端集成测试。
package configuration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

func TestConfigurationControlPlaneAppliesAndVerifiesPreferenceChange(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	services, mainAgent := fixture.services, fixture.main
	db := services.DB
	actor := configurationsvc.Actor{
		OwnerUserID: mainAgent.OwnerUserID, AgentID: mainAgent.AgentID, IsMainAgent: true,
		SessionKey: "agent:nexus:ws:dm:main", ContextKind: configurationsvc.ContextKindAgent,
		ContextID:     mainAgent.AgentID,
		PrincipalRole: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal,
		LocalSingleUser: true,
	}
	bindConfigurationTestRound(t, services, &actor)
	before, err := services.Configuration.Inspect(t.Context(), actor, []string{
		configurationsvc.DomainPreferences,
		configurationsvc.DomainProviders,
		configurationsvc.DomainAgents,
		configurationsvc.DomainChannels,
		configurationsvc.DomainConnectors,
		configurationsvc.DomainSkills,
		configurationsvc.DomainHost,
	}, true)
	if err != nil {
		t.Fatalf("inspect full mutable configuration: %v", err)
	}
	preferences := before.Domains[configurationsvc.DomainPreferences]
	input := json.RawMessage(`{"agent_sdk_diagnostics_enabled":true}`)
	plan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainPreferences, Operation: "update", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentRevision != preferences.Revision {
		t.Fatalf("plan revision = %q, inspect revision = %q", plan.CurrentRevision, preferences.Revision)
	}
	if plan.StateVersion <= 0 || plan.StateVersion != preferences.StateVersion {
		t.Fatalf("Preferences plan state_version = %d, inspect = %d", plan.StateVersion, preferences.StateVersion)
	}
	applied, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.RevisionAfter == applied.RevisionBefore {
		t.Fatalf("apply result = %+v", applied)
	}
	stored, err := services.Preferences.Get(t.Context(), actor.OwnerUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.AgentSDKDiagnosticsEnabled {
		t.Fatal("preference change did not reach source of truth")
	}
	if stored.Version != plan.StateVersion+1 {
		t.Fatalf("Preferences CAS version = %d, want %d", stored.Version, plan.StateVersion+1)
	}
	replayed, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.IdempotentReplay {
		t.Fatalf("repeated request was not replayed: %+v", replayed)
	}
	changes, err := services.Configuration.ListChanges(t.Context(), actor, configurationsvc.DomainPreferences, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Status != "success" {
		t.Fatalf("audit changes = %+v", changes)
	}

	const providerSecret = "provider-secret-must-not-leak"
	providerInput := json.RawMessage(`{
		"provider":"dialog-provider",
		"preset_key":"custom",
		"api_format":"responses",
		"display_name":"Dialog Provider",
		"auth_token":{"$secret":"provider.auth_token"},
		"base_url":"https://provider.example.com/v1",
		"models_path":"/models"
	}`)
	providerPlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "create", Input: providerInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	providerRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0001", Domain: configurationsvc.DomainProviders,
		Operation: "create", Input: providerInput, ExpectedRevision: providerPlan.CurrentRevision,
		PlanDigest: providerPlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		services,
		t.Context(),
		actor,
		providerRequest,
		providerPlan,
		map[string]string{"provider.auth_token": providerSecret},
	)
	providerApplied, err := services.Configuration.ApplyChange(t.Context(), actor, providerRequest)
	if err != nil {
		t.Fatal(err)
	}
	appliedJSON, err := json.Marshal(providerApplied)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(appliedJSON), providerSecret) {
		t.Fatalf("apply result leaked Provider secret: %s", appliedJSON)
	}
	createdProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !createdProvider.Enabled {
		t.Fatal("conversational Provider create must default enabled=true")
	}
	if createdProvider.ConfigurationVersion != 1 {
		t.Fatalf("created Provider configuration_version = %d, want 1", createdProvider.ConfigurationVersion)
	}

	updateInput := json.RawMessage(`{"display_name":"Renamed Provider"}`)
	updatePlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "update",
		Target: "dialog-provider", Input: updateInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatePlan.CurrentRevision == "" ||
		updatePlan.Scope.Kind != configurationsvc.ScopeKindOwner ||
		updatePlan.StateVersion != createdProvider.ConfigurationVersion {
		t.Fatalf("provider target plan is incomplete: %+v", updatePlan)
	}
	updateRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0002", Domain: configurationsvc.DomainProviders,
		Operation: "update", Target: "dialog-provider", Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision, PlanDigest: updatePlan.PlanDigest,
	}
	approveConfigurationTestChange(t, services, t.Context(), actor, updateRequest, updatePlan)
	updatedApply, err := services.Configuration.ApplyChange(t.Context(), actor, updateRequest)
	if err != nil {
		t.Fatal(err)
	}
	updatedProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !updatedProvider.Enabled || updatedProvider.DisplayName != "Renamed Provider" {
		t.Fatalf("Provider merge patch reset existing configuration: %+v", updatedProvider)
	}
	if updatedProvider.ConfigurationVersion != updatePlan.StateVersion+1 ||
		!hasConfigurationCheck(updatedApply.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Provider update was not CAS-verified: provider=%+v apply=%+v", updatedProvider, updatedApply)
	}
	var storedToken string
	if err = db.QueryRow(`SELECT auth_token FROM provider WHERE provider = 'dialog-provider'`).Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedToken != providerSecret {
		t.Fatal("Provider merge patch did not preserve omitted auth_token")
	}
	fallbackProvider, err := services.Provider.Create(t.Context(), providersvc.CreateInput{
		Provider: "integration-fallback", PresetKey: "custom",
		APIFormat:   providersvc.APIFormatAnthropicMessages,
		DisplayName: "Integration Fallback", AuthToken: "fallback-token",
		BaseURL: "https://fallback.example.com", ModelsPath: "/models", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = services.Provider.UpdateModel(
		t.Context(),
		fallbackProvider.Provider,
		"fallback-model",
		providersvc.UpdateModelInput{Enabled: true, IsDefault: true},
	); err != nil {
		t.Fatal(err)
	}
	deletePlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "delete",
		Target: "dialog-provider", Input: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if deletePlan.StateVersion != updatedProvider.ConfigurationVersion {
		t.Fatalf("Provider delete plan version = %d, want %d",
			deletePlan.StateVersion, updatedProvider.ConfigurationVersion)
	}
	deleteRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0003", Domain: configurationsvc.DomainProviders,
		Operation: "delete", Target: "dialog-provider", Input: json.RawMessage(`{}`),
		ExpectedRevision: deletePlan.CurrentRevision, PlanDigest: deletePlan.PlanDigest,
	}
	approveConfigurationTestChange(t, services, t.Context(), actor, deleteRequest, deletePlan)
	deletedProvider, err := services.Configuration.ApplyChange(t.Context(), actor, deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(deletedProvider.Checks, "configuration_target_deleted") {
		t.Fatalf("Provider deletion was not verified against source of truth: %+v", deletedProvider)
	}
	if _, err = services.Provider.Get(t.Context(), "dialog-provider"); err == nil {
		t.Fatal("Provider still exists after verified delete")
	}
	allChanges, err := services.Configuration.ListChanges(t.Context(), actor, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	auditJSON, err := json.Marshal(allChanges)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditJSON), providerSecret) {
		t.Fatalf("configuration audit leaked Provider secret: %s", auditJSON)
	}
	var deleteAudit *configurationsvc.AuditRecord
	for index := range allChanges {
		if allChanges[index].RequestID == deleteRequest.RequestID {
			deleteAudit = &allChanges[index]
			break
		}
	}
	if deleteAudit == nil ||
		deleteAudit.HumanApprovalRequestID != "perm-"+deleteRequest.RequestID ||
		deleteAudit.HumanPrincipalUserID != actor.OwnerUserID ||
		deleteAudit.HumanPrincipalRole != authctx.RoleOwner ||
		deleteAudit.HumanAuthMethod != authctx.AuthMethodLocal ||
		deleteAudit.HumanApprovedAt == nil {
		t.Fatalf("destructive change audit lost human approval evidence: %+v", deleteAudit)
	}
}

func hasConfigurationCheck(checks []configurationsvc.Check, code string) bool {
	for _, check := range checks {
		if check.Code == code && check.Verified {
			return true
		}
	}
	return false
}

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

func enableConfigurationTestPrincipalVerification(services *app.AppServices) {
	verifier := configurationTestPrincipalVerifier{}
	services.Configuration.SetPrincipalVerifiers(verifier, verifier)
}

func bindConfigurationTestRound(
	t *testing.T,
	services *app.AppServices,
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
	services *app.AppServices,
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
	services *app.AppServices,
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

type fixedConfigurationRoleResolver struct {
	role string
}

func (r fixedConfigurationRoleResolver) ResolveActivePrincipalRole(context.Context, string) (string, error) {
	return r.role, nil
}

func TestMemberMainAuditListingExcludesHostConfiguration(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	db, err := storage.OpenDB(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, record := range []struct {
		requestID string
		domain    string
	}{
		{requestID: "audit-host-boundary", domain: configurationsvc.DomainHost},
		{requestID: "audit-private-boundary", domain: configurationsvc.DomainPreferences},
	} {
		if _, err = db.ExecContext(
			t.Context(),
			`INSERT INTO configuration_changes (
				request_id, owner_user_id, actor_agent_id, domain, operation,
				scope_kind, scope_id, authority, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.requestID,
			fixture.main.OwnerUserID,
			fixture.main.AgentID,
			record.domain,
			"inspect",
			configurationsvc.ScopeKindOwner,
			fixture.main.OwnerUserID,
			configurationsvc.AuthorityOwnerMain,
			"success",
		); err != nil {
			t.Fatal(err)
		}
	}

	fixture.services.Configuration.SetPrincipalVerifiers(
		configurationTestPrincipalVerifier{},
		fixedConfigurationRoleResolver{role: authctx.RoleMember},
	)
	memberCtx := authctx.WithPrincipal(t.Context(), &authctx.Principal{
		UserID: fixture.main.OwnerUserID, Role: authctx.RoleMember,
		AuthMethod: authctx.AuthMethodPassword,
	})
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:audit-boundary",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	records, err := fixture.services.Configuration.ListChanges(memberCtx, actor, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Domain == configurationsvc.DomainHost {
			t.Fatalf("member main Agent received host audit record: %+v", record)
		}
	}
	if _, err = fixture.services.Configuration.ListChanges(
		memberCtx,
		actor,
		configurationsvc.DomainHost,
		20,
	); err == nil {
		t.Fatal("member main Agent explicitly read host audit history")
	}
}
